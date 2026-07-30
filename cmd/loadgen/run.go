package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/platform"
	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/quotation"
)

// Result is what a single call to POST /quotes produced.
//
// A failure is recorded with its latency like any other request: in this scenario the expensive
// failure is exactly the one that took long to arrive — the partner answers 503 only after the
// serial aggregation has already burned through the slow partner. Throwing away the time of failed
// requests would hide the worst part of the problem.
type Result struct {
	Latency time.Duration
	Status  int    // HTTP status; 0 when no response came back
	Partner string // partner blamed by the API, when it says which one
	Failure string // transport-level reason (timeout, connection refused); empty when there was a response
}

// OK reports whether the platform delivered the aggregated quote.
func (r Result) OK() bool { return r.Status == http.StatusOK }

// caller performs one request against the quotation-api.
type caller struct {
	http     *http.Client
	endpoint string
	tenant   string
	timeout  time.Duration
	bodies   [][]byte
}

func newCaller(cfg Config, bodies [][]byte) *caller {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// The default idle pool holds two connections per host, so a run with 20 workers would spend
	// its time opening and closing sockets and would measure the load generator's own overhead.
	// The pool has to fit the load being produced.
	transport.MaxIdleConns = cfg.Concurrency
	transport.MaxIdleConnsPerHost = cfg.Concurrency

	return &caller{
		// No timeout on the http.Client: the deadline is a per-request context, which is what lets
		// -timeout 0 mean "wait forever", the same thing the starter's API does with its partners.
		http:     &http.Client{Transport: transport},
		endpoint: cfg.Endpoint(),
		tenant:   cfg.Tenant,
		timeout:  cfg.Timeout,
		bodies:   bodies,
	}
}

// call sends request number n and returns what came back. The second return value is false when the
// request was cut short by the end of the run (-duration or Ctrl-C): that is the load generator
// giving up, not the platform failing, and counting it as an error would make the report lie.
func (c *caller) call(ctx context.Context, n int) (Result, bool) {
	requestCtx := ctx
	if c.timeout > 0 {
		var cancel context.CancelFunc
		requestCtx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	body := c.bodies[n%len(c.bodies)]
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{Failure: err.Error()}, true
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(quotation.TenantHeader, c.tenant)

	started := time.Now()
	response, err := c.http.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, false
		}
		return Result{Latency: time.Since(started), Failure: reason(err)}, true
	}
	defer func() { _ = response.Body.Close() }()

	result := Result{Status: response.StatusCode}
	if !result.OK() {
		var failure platform.ErrorBody
		if err := json.NewDecoder(io.LimitReader(response.Body, responseLimit)).Decode(&failure); err == nil {
			result.Partner = failure.Partner
		}
	}
	// Drain what is left so the connection goes back to the pool instead of being dropped.
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, responseLimit))
	result.Latency = time.Since(started)
	return result, true
}

// responseLimit cuts off an absurd response before it turns into memory.
const responseLimit = 1 << 20 // 1 MiB

// reason turns the error into the short sentence that goes in the report. net/http wraps everything
// in a *url.Error whose message repeats the full URL on every line — unreadable when it is the same
// failure eighty times over.
func reason(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		var netErr *net.OpError
		if errors.As(urlErr.Err, &netErr) {
			return netErr.Err.Error()
		}
		return urlErr.Err.Error()
	}
	return err.Error()
}

// Phase is a block of load with a single shape, measured from end to end.
type Phase struct {
	Name        string
	Planned     int // requests the phase set out to send
	Concurrency int
	Started     time.Time
	Ended       time.Time
	Results     []Result
}

// runPhase sends `requests` requests keeping `concurrency` of them in flight, and returns as soon as
// the last answer arrives or the context ends.
//
// It is a closed-loop generator: a worker only sends the next request after receiving the previous
// answer. That is not a shortcut, it is the model of the client this platform actually has — a
// broker's screen waiting for the quote — and it is what makes the load back off as the platform
// slows down, instead of piling up an artificial queue that no real caller would produce.
func runPhase(ctx context.Context, c *caller, name string, requests, concurrency int) Phase {
	phase := Phase{Name: name, Planned: requests, Concurrency: concurrency, Started: time.Now()}

	jobs := make(chan int)
	results := make(chan Result, requests)

	var workers sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for n := range jobs {
				if result, counted := c.call(ctx, n); counted {
					results <- result
				}
			}
		}()
	}

dispatch:
	for n := 0; n < requests; n++ {
		select {
		case jobs <- n:
		case <-ctx.Done():
			break dispatch // whatever is missing does not get sent
		}
	}
	close(jobs)
	workers.Wait()
	close(results)

	phase.Ended = time.Now()
	phase.Results = make([]Result, 0, requests)
	for result := range results {
		phase.Results = append(phase.Results, result)
	}
	return phase
}
