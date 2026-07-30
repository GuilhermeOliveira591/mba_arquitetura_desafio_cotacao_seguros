package main

import (
	"fmt"
	"io"
	"sort"
	"time"
)

// Summary condenses a phase into the numbers that go in the report.
type Summary struct {
	Requests   int // requests that got an answer (or failed on their own merits)
	Planned    int // requests the phase intended to send
	OK         int
	Elapsed    time.Duration
	Throughput float64 // answers per second
	P50        time.Duration
	P95        time.Duration
	P99        time.Duration
	Max        time.Duration
	Causes     []Cause
}

// Cause is a failure mode and how many times it happened. It is grouped, and not listed request by
// request, because the diagnosis the student needs is "who is failing", not "which call failed".
type Cause struct {
	Label string
	Count int
}

// SuccessRate is the share of requests the platform answered with the aggregated quote, in percent.
func (s Summary) SuccessRate() float64 {
	if s.Requests == 0 {
		return 0
	}
	return float64(s.OK) / float64(s.Requests) * 100
}

func summarize(phase Phase) Summary {
	summary := Summary{
		Requests: len(phase.Results),
		Planned:  phase.Planned,
		Elapsed:  phase.Ended.Sub(phase.Started),
	}

	latencies := make([]time.Duration, 0, len(phase.Results))
	counted := map[string]int{}
	for _, result := range phase.Results {
		latencies = append(latencies, result.Latency)
		if result.OK() {
			summary.OK++
			continue
		}
		counted[cause(result)]++
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	summary.P50 = percentile(latencies, 50)
	summary.P95 = percentile(latencies, 95)
	summary.P99 = percentile(latencies, 99)
	summary.Max = percentile(latencies, 100)

	if summary.Elapsed > 0 {
		summary.Throughput = float64(summary.Requests) / summary.Elapsed.Seconds()
	}

	for label, count := range counted {
		summary.Causes = append(summary.Causes, Cause{Label: label, Count: count})
	}
	sort.Slice(summary.Causes, func(i, j int) bool {
		if summary.Causes[i].Count != summary.Causes[j].Count {
			return summary.Causes[i].Count > summary.Causes[j].Count
		}
		return summary.Causes[i].Label < summary.Causes[j].Label
	})
	return summary
}

func cause(result Result) string {
	switch {
	case result.Failure != "":
		return result.Failure
	case result.Partner != "":
		return fmt.Sprintf("HTTP %d from %s", result.Status, result.Partner)
	default:
		return fmt.Sprintf("HTTP %d", result.Status)
	}
}

// percentile uses the nearest-rank method: it always returns a latency that actually happened,
// never an interpolation between two requests. In a report meant to be read alongside the traces in
// Jaeger, every number here has to have a trace behind it.
func percentile(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := (p*len(sorted) + 99) / 100 // ceil(p% of n)
	if rank < 1 {
		rank = 1
	}
	return sorted[rank-1]
}

// Report is the whole run, ready to be read.
type Report struct {
	Config   Config
	Baseline *Phase // nil when the baseline was skipped with -baseline 0
	Load     Phase
}

// Write prints the report. It is plain text on purpose: the output of this command is meant to be
// pasted into the student's document as evidence of the "before", next to the screenshot of the
// trace.
func (r Report) Write(w io.Writer) {
	fmt.Fprintf(w, "\nloadgen · %s · tenant %s · %d distinct quotes\n",
		r.Config.Endpoint(), r.Config.Tenant, r.Config.Distinct)

	var baseline Summary
	if r.Baseline != nil {
		baseline = summarize(*r.Baseline)
		writePhase(w, *r.Baseline, baseline)
	}
	load := summarize(r.Load)
	writePhase(w, r.Load, load)

	if r.Baseline != nil && baseline.Requests > 0 && load.Requests > 0 {
		writeComparison(w, baseline, load)
	}
	writeEvidence(w, r.Load)
}

func writePhase(w io.Writer, phase Phase, summary Summary) {
	fmt.Fprintf(w, "\n%s — %d requests, %d in flight, %s to %s\n", phase.Name, summary.Planned,
		phase.Concurrency, clock(phase.Started), clock(phase.Ended))

	if summary.Requests == 0 {
		fmt.Fprintf(w, "  %-14s no request completed\n", "answered")
		return
	}
	if summary.Requests < summary.Planned {
		fmt.Fprintf(w, "  %-14s %d of %d (the run hit the -duration limit)\n", "answered",
			summary.Requests, summary.Planned)
	}

	fmt.Fprintf(w, "  %-14s %d of %d (%.0f%%)\n", "success", summary.OK, summary.Requests, summary.SuccessRate())
	fmt.Fprintf(w, "  %-14s p50 %s   p95 %s   p99 %s   max %s\n", "latency",
		short(summary.P50), short(summary.P95), short(summary.P99), short(summary.Max))
	fmt.Fprintf(w, "  %-14s %.1f req/s in %s\n", "throughput", summary.Throughput, short(summary.Elapsed))

	for i, cause := range summary.Causes {
		label := ""
		if i == 0 {
			label = "failures"
		}
		fmt.Fprintf(w, "  %-14s %s: %d\n", label, cause.Label, cause.Count)
	}
}

// writeComparison is the point of the whole command: two numbers side by side. "The platform is
// slow" is an opinion; "p95 went from 1.8s to 9.4s while the load went up" is a measurement — and it
// is the sentence the student needs before deciding what to build.
func writeComparison(w io.Writer, baseline, load Summary) {
	fmt.Fprintf(w, "\nbaseline → load\n")
	fmt.Fprintf(w, "  %-14s %s → %s%s\n", "p95 latency", short(baseline.P95), short(load.P95), times(baseline.P95, load.P95))
	fmt.Fprintf(w, "  %-14s %s → %s%s\n", "max latency", short(baseline.Max), short(load.Max), times(baseline.Max, load.Max))
	fmt.Fprintf(w, "  %-14s %.0f%% → %.0f%%\n", "success", baseline.SuccessRate(), load.SuccessRate())
	fmt.Fprintf(w, "  %-14s %.1f → %.1f req/s\n", "throughput", baseline.Throughput, load.Throughput)
}

// writeEvidence closes with where to go look. The numbers say the platform degraded; only the trace
// says why — and the student who does not know where to click stops at the number.
func writeEvidence(w io.Writer, load Phase) {
	fmt.Fprintf(w, "\nwhere to look\n")
	fmt.Fprintf(w, "  %-14s http://localhost:16686 · service quotation-api · operation POST /quotes\n", "Jaeger")
	fmt.Fprintf(w, "  %-14s %s to %s (sort by longest first)\n", "load window", clock(load.Started), clock(load.Ended))
	fmt.Fprintf(w, "  %-14s docs/roteiro-cenario-de-falha.md\n\n", "step by step")
}

// times expresses the degradation as a multiplier — the form that survives being read out loud in
// the defense of the document.
func times(before, after time.Duration) string {
	if before <= 0 {
		return ""
	}
	return fmt.Sprintf("   %.1fx", float64(after)/float64(before))
}

// clock carries the time zone because loadgen usually runs inside the compose container, whose
// clock is UTC, while Jaeger's UI shows the browser's local time. Without the suffix the student
// looks for the load in the wrong hour of the day.
func clock(t time.Time) string { return t.Format("15:04:05 MST") }

// short prints durations the way a report is read: milliseconds while they are milliseconds,
// seconds with two digits after that. time.Duration's own format would print "1.783458212s".
func short(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// unreachable reports whether not a single request got an answer from the platform, along with the
// most frequent reason. Such a run says nothing about the scenario — it says the environment is not
// up — and it is worth more as one line of diagnosis than as a report full of zeros.
func unreachable(phases ...*Phase) (string, bool) {
	counted := map[string]int{}
	for _, phase := range phases {
		if phase == nil {
			continue
		}
		for _, result := range phase.Results {
			if result.Status != 0 {
				return "", false
			}
			counted[cause(result)]++
		}
	}

	reason, most := "", 0
	for label, count := range counted {
		if count > most || (count == most && label < reason) {
			reason, most = label, count
		}
	}
	return reason, true
}
