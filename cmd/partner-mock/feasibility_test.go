package main

// Feasibility smoke test: proof that the challenge is buildable on top of these defaults.
//
// The risk this file defends against is the one recorded in the spec: a `partner-flaky` unstable
// enough to look broken, but not unstable enough for a circuit breaker to ever open. If that were
// the case the student would spend hours on a breaker that stays closed forever and would conclude,
// correctly, that the exercise is broken.
//
// So the assertion here is not about this file's breaker — that one is only a measuring instrument,
// a throwaway model of what the student is asked to build. What is under test is the *mock's default
// parameters*: whether the failure sequence they produce gives a breaker a reason to open, and later
// a reason to close again, inside a single `make reproduce`.
//
// Everything below is deterministic — same seed, same sequence, same verdict on every machine. The
// empirical half of the smoke test (the platform visibly degrading under load) lives in
// docs/smoke-test-factibilidade.md, because it needs the environment up.

import (
	"fmt"
	"testing"
)

// requestsPerRun is what one `make reproduce` sends: 10 sequential baseline requests plus 200 under
// load. Since the aggregation is serial and `partner-slow` never fails, every one of them reaches
// `partner-flaky` — so this is also the number of partner calls a student sees on their first run.
const requestsPerRun = 210

// breakerPolicy is a trip rule, in the two shapes libraries actually ship: counting consecutive
// failures, or measuring the failure ratio over a rolling window. The names are the ones a student
// would recognize from gobreaker, resilience4j or Polly.
type breakerPolicy struct {
	name             string
	consecutive      int     // opens after this many consecutive failures (0 disables the rule)
	window           int     // opens when the failure ratio over the last `window` calls reaches `ratio` (0 disables)
	ratio            float64 // failure ratio that trips the window rule
	cooldown         int     // requests rejected while open, before the breaker probes again
	successesToClose int     // probe successes in a row that close the circuit
	opensWithin      int     // the assertion: it has to open no later than this request
}

func (p breakerPolicy) trips(consecutiveFailures int, recent []bool) bool {
	if p.consecutive > 0 && consecutiveFailures >= p.consecutive {
		return true
	}
	if p.window > 0 && len(recent) == p.window {
		failures := 0
		for _, failed := range recent {
			if failed {
				failures++
			}
		}
		if float64(failures)/float64(p.window) >= p.ratio {
			return true
		}
	}
	return false
}

type breakerState int

const (
	closed breakerState = iota
	open
	halfOpen
)

type breakerOutcome struct {
	openedAt       int // client request on which the circuit opened for the first time (0 = never opened)
	callsUntilOpen int // partner calls spent until then
	opens          int
	recoveries     int // times the circuit went back to closed after being open
	shortCircuited int // requests the breaker answered without touching the partner
}

// simulate runs `requests` client requests through the policy against the real Behavior of the mock.
//
// The detail that makes this faithful: while the circuit is open the partner is *not called*, and so
// its sequence number does not advance. A simulation reading the failure sequence straight through
// would credit the breaker with failures it never saw.
func simulate(cfg Config, p breakerPolicy, requests int) breakerOutcome {
	behavior := NewBehavior(cfg)
	state := closed
	consecutiveFailures, consecutiveSuccesses, rejected, calls := 0, 0, 0, 0
	recent := make([]bool, 0, p.window)

	var out breakerOutcome
	for request := 1; request <= requests; request++ {
		if state == open {
			rejected++
			out.shortCircuited++
			if rejected >= p.cooldown {
				state, consecutiveSuccesses = halfOpen, 0
			}
			continue
		}

		decision := behavior.Admit()
		behavior.Complete()
		calls++

		if state == halfOpen {
			switch {
			case decision.Fail:
				state, rejected = open, 0
				out.opens++
			default:
				consecutiveSuccesses++
				if consecutiveSuccesses >= p.successesToClose {
					state, consecutiveFailures, recent = closed, 0, recent[:0]
					out.recoveries++
				}
			}
			continue
		}

		if decision.Fail {
			consecutiveFailures++
		} else {
			consecutiveFailures = 0
		}
		recent = append(recent, decision.Fail)
		if p.window > 0 && len(recent) > p.window {
			recent = recent[1:]
		}

		if p.trips(consecutiveFailures, recent) {
			state, rejected = open, 0
			out.opens++
			if out.openedAt == 0 {
				out.openedAt, out.callsUntilOpen = request, calls
			}
		}
	}
	return out
}

func (o breakerOutcome) String() string {
	if o.openedAt == 0 {
		return "never opened"
	}
	return fmt.Sprintf("opened on request %d (%d partner calls), %d openings, %d recoveries, %d requests short-circuited",
		o.openedAt, o.callsUntilOpen, o.opens, o.recoveries, o.shortCircuited)
}

// policies are the trip rules the challenge has to survive. The `opensWithin` values are measured,
// not guessed: they are the current behavior of the defaults, and locking them here is what turns a
// silent regression (someone tuning PARTNER_SEED or PARTNER_FAILURE_RATE) into a failing test.
func policies() []breakerPolicy {
	return []breakerPolicy{
		{name: "3 consecutive failures", consecutive: 3, cooldown: 5, successesToClose: 2, opensWithin: 10},
		{name: "5 consecutive failures", consecutive: 5, cooldown: 5, successesToClose: 2, opensWithin: 45},
		{name: "50% failures over a window of 10", window: 10, ratio: 0.5, cooldown: 5, successesToClose: 2, opensWithin: 15},
		{name: "60% failures over a window of 20", window: 20, ratio: 0.6, cooldown: 10, successesToClose: 3, opensWithin: 60},
	}
}

// TestFeasibilityBreakersOpenOnTheDefaultProfile is the acceptance criterion of the smoke test: with
// the parameters that ship in docker-compose.yml, a circuit breaker does open — and it opens early
// enough that the student sees it on the first run, not after a night of load.
func TestFeasibilityBreakersOpenOnTheDefaultProfile(t *testing.T) {
	for _, policy := range policies() {
		t.Run(policy.name, func(t *testing.T) {
			out := simulate(flakyProfile(), policy, requestsPerRun)
			t.Log(out)

			if out.openedAt == 0 {
				t.Fatalf("the circuit never opened in %d requests — the defaults of partner-flaky make the challenge unbuildable", requestsPerRun)
			}
			if out.openedAt > policy.opensWithin {
				t.Fatalf("the circuit opened only on request %d, expected by request %d at the latest", out.openedAt, policy.opensWithin)
			}
		})
	}
}

// TestFeasibilityBreakersCloseAgain guards the other end of the exercise. A partner that failed
// forever would be just as useless as one that never failed: the deliverable asks for a breaker with
// *three* states, and half-open only means something if there are successes on the other side of the
// cooldown. Here the circuit has to come back to closed on its own, without anyone fixing the
// partner.
func TestFeasibilityBreakersCloseAgain(t *testing.T) {
	for _, policy := range policies() {
		t.Run(policy.name, func(t *testing.T) {
			out := simulate(flakyProfile(), policy, requestsPerRun)
			if out.recoveries == 0 {
				t.Fatalf("the circuit opened %d times and never closed again in %d requests: the half-open state is unreachable with these defaults", out.opens, requestsPerRun)
			}
			t.Logf("%s — the breaker flaps, which is the point: a 40%% partner neither dies nor recovers for good", out)
		})
	}
}

// TestFeasibilityBurstIsWhereTheDocsSayItIs pins the number cited in docker-compose.yml and in the
// smoke test document: the burst of 9 consecutive failures at sequence 49-57 is *the* reason any
// consecutive-failure breaker opens on the first run. If a change to the seed or to the failure rate
// moves this burst, the defaults may still be fine — but the docs are not, and the smoke test has to
// be run again.
func TestFeasibilityBurstIsWhereTheDocsSayItIs(t *testing.T) {
	const (
		wantLongest = 9
		wantEndsAt  = 57
	)

	longest, endsAt, current := 0, 0, 0
	for i, failed := range failureSequence(flakyProfile(), requestsPerRun) {
		if !failed {
			current = 0
			continue
		}
		current++
		if current > longest {
			longest, endsAt = current, i+1
		}
	}

	if longest != wantLongest || endsAt != wantEndsAt {
		t.Fatalf("longest burst of %d failures ending at sequence %d; the docs say %d ending at %d — re-run the smoke test and update docs/smoke-test-factibilidade.md",
			longest, endsAt, wantLongest, wantEndsAt)
	}
	t.Logf("burst of %d consecutive failures at sequences %d-%d, inside a single `make reproduce`", longest, endsAt-longest+1, endsAt)
}
