package main

import (
	"fmt"
	"hash/fnv"
	"sync/atomic"
	"time"
)

// Behavior is the partner's misbehavior: how long it takes, when it fails and how it gets worse
// under load.
//
// Determinism is a prerequisite for fair grading — two students running the same script need to see
// the same failure sequence. That is why there is no `math/rand` here: every decision is a pure
// function of the seed and of the request's sequence number. The n-th request to a `partner-flaky`
// with the default seed fails (or not) the same way on everyone's machine, today and a year from
// now.
//
// The trade-off: under concurrency, which request gets which sequence number depends on arrival
// order. What repeats is the *sequence of responses*, not the request-response pairing.
type Behavior struct {
	cfg      Config
	seq      atomic.Uint64
	inFlight atomic.Int64
}

func NewBehavior(cfg Config) *Behavior {
	return &Behavior{cfg: cfg}
}

// Decision is the verdict on a request, made in full at the moment it arrives.
type Decision struct {
	Sequence uint64
	InFlight int64 // requests in flight at the moment of arrival, this one already counted
	Latency  time.Duration
	Fail     bool
}

// Admit records the arrival of a request and decides how it will be served. Every Admit needs a
// matching Complete, otherwise the in-flight request count leaks and the partner degrades forever.
func (b *Behavior) Admit() Decision {
	seq := b.seq.Add(1)
	inFlight := b.inFlight.Add(1)
	return Decision{
		Sequence: seq,
		InFlight: inFlight,
		Latency:  b.latency(seq, inFlight),
		Fail:     b.failure(seq),
	}
}

// Complete gives back the slot of the request that finished.
func (b *Behavior) Complete() {
	b.inFlight.Add(-1)
}

// InFlight exposes the partner's current load.
func (b *Behavior) InFlight() int64 {
	return b.inFlight.Load()
}

// latency sums the three parts of the delay: the profile's constant base, the deterministic jitter
// and the degradation proportional to the load.
func (b *Behavior) latency(seq uint64, inFlight int64) time.Duration {
	total := b.cfg.Latency
	if b.cfg.Jitter > 0 {
		total += time.Duration(fraction(b.cfg.Seed, seq, streamJitter) * float64(b.cfg.Jitter))
	}
	return total + b.degradation(inFlight)
}

// degradation is the `partner-degrading`: while the load fits within the threshold, the partner
// answers in its normal time; above it, each concurrent request adds one latency step, up to the
// cap. It is the simplest queueing model that still teaches the lesson — the partner does not
// break, it sinks.
func (b *Behavior) degradation(inFlight int64) time.Duration {
	if b.cfg.DegradeAfter <= 0 || b.cfg.DegradeStep <= 0 || inFlight <= b.cfg.DegradeAfter {
		return 0
	}
	extra := time.Duration(inFlight-b.cfg.DegradeAfter) * b.cfg.DegradeStep
	if b.cfg.DegradeCap > 0 && extra > b.cfg.DegradeCap {
		return b.cfg.DegradeCap
	}
	return extra
}

// failure is the `partner-flaky`. The fraction derived from the seed spreads the failures in
// bursts, like a real partner — and not regularly alternated, which is what an exact division would
// produce. The burst matters: a circuit breaker that trips on consecutive failures would never open
// with a perfectly interleaved pattern.
func (b *Behavior) failure(seq uint64) bool {
	switch {
	case b.cfg.FailureRate <= 0:
		return false
	case b.cfg.FailureRate >= 1:
		return true
	}
	return fraction(b.cfg.Seed, seq, streamFailure) < b.cfg.FailureRate
}

// Quote is the partner's success response.
type Quote struct {
	Partner         string `json:"partner"`
	QuoteID         string `json:"quote_id"`
	PremiumCents    int64  `json:"premium_cents"`
	Currency        string `json:"currency"`
	CoverageCents   int64  `json:"coverage_cents"`
	ValidForSeconds int64  `json:"valid_for_seconds"`
}

// Quote derives the quote from the content of the request, and not from the clock nor from a
// counter: the same request to the same partner always returns the same quote. That is what makes
// the effect of the cache visible — hit and miss return equal values, so the difference the student
// measures is one of latency and cost, not of content.
func (b *Behavior) Quote(request []byte) Quote {
	h := fnv.New64a()
	_, _ = h.Write([]byte(b.cfg.Name))
	_, _ = h.Write(request)
	sum := h.Sum64()

	return Quote{
		Partner: b.cfg.Name,
		QuoteID: fmt.Sprintf("%s-%016x", b.cfg.Name, sum),
		// Premium between R$ 500.00 and R$ 2,000.00.
		PremiumCents: 50000 + int64(sum%150001),
		Currency:     "BRL",
		// Coverage between R$ 30,000.00 and R$ 100,000.00, in steps of ten thousand.
		CoverageCents:   3000000 + int64((sum>>32)%8)*1000000,
		ValidForSeconds: int64(b.cfg.QuoteTTL.Seconds()),
	}
}

// Independent decision streams: without them, latency and failure would walk together (every slow
// request would also be the one that fails), which would be a pattern easy to memorize and not to
// diagnose.
const (
	streamFailure uint64 = 1
	streamJitter  uint64 = 2
)

// fraction returns a value in [0,1) from the seed, the sequence number and the stream. Same inputs,
// same value — always, on any machine.
func fraction(seed, seq, stream uint64) float64 {
	v := mix(mix(seed+seq*0x9e3779b97f4a7c15) ^ stream)
	return float64(v>>11) / float64(uint64(1)<<53)
}

// mix is the splitmix64 finalizer: it spreads bits cheaply and reproducibly.
func mix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}
