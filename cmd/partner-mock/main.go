// Command partner-mock is the starter's fake partner insurer — the unstable external dependency
// against which the student is going to build circuit breaker, cache and fallback.
//
// It is a single binary for the three profiles of the challenge. What changes between
// `partner-slow`, `partner-flaky` and `partner-degrading` is configuration, not code: the
// misbehavior lives in docker-compose.yml, in a single readable place, and the starter stays small.
//
// Contract:
//
//	POST /quotes   partner quote (applies the profile's latency, failure and degradation)
//	GET  /healthz  process health — never degrades, never fails
//	GET  /config   effective configuration of the profile that is running
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// requestLimit cuts off absurd requests before they turn into memory. The mock does not validate
// the content of the request on purpose: taking care of tenant and payload is the quotation-api's
// job, not the partner's.
const requestLimit = 1 << 20 // 1 MiB

func main() {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	behavior := NewBehavior(cfg)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           routes(cfg, behavior),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stopListening := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopListening()

	go func() {
		<-ctx.Done()
		log.Printf("%s: signal received, shutting down", cfg.Name)
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			log.Printf("%s: forced shutdown: %v", cfg.Name, err)
		}
	}()

	log.Printf("%s listening on %s | %s", cfg.Name, server.Addr, cfg.Summary())
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server stopped: %v", err)
	}
}

func routes(cfg Config, behavior *Behavior) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /quotes", quoteHandler(cfg, behavior))
	mux.HandleFunc("GET /healthz", healthHandler(cfg))
	mux.HandleFunc("GET /config", configHandler(cfg))
	return mux
}

// quoteHandler is the only endpoint that suffers the profile: it applies the latency decided on
// arrival and only then responds — success or failure. Failing after waiting is the truly bad case,
// the one that eats up the client's timeout; an instant failure would be far too easy to tolerate.
func quoteHandler(cfg Config, behavior *Behavior) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decision := behavior.Admit()
		defer behavior.Complete()

		request, err := io.ReadAll(http.MaxBytesReader(w, r.Body, requestLimit))
		if err != nil {
			respond(w, cfg, decision, http.StatusRequestEntityTooLarge, jsonError{
				Error:   "request exceeds the limit accepted by the partner",
				Partner: cfg.Name,
			})
			return
		}

		if err := sleep(r.Context(), decision.Latency); err != nil {
			// The client gave up (timeout or cancellation) before the partner answered. There is
			// nobody left to write to: just release the slot through the defer.
			return
		}

		if decision.Fail {
			respond(w, cfg, decision, cfg.FailureStatus, jsonError{
				Error:   "partner unavailable",
				Partner: cfg.Name,
			})
			return
		}

		respond(w, cfg, decision, http.StatusOK, behavior.Quote(request))
	}
}

// healthHandler always answers, right away. The compose healthcheck must not see the profile's
// misbehavior, otherwise the slow partner would never come up "healthy" and the environment would
// never stand up.
func healthHandler(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "partner": cfg.Name})
	}
}

func configHandler(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, cfg)
	}
}

type jsonError struct {
	Error   string `json:"error"`
	Partner string `json:"partner"`
}

// respond attaches the mock's decision to the response headers. It is the starter's diagnostic
// shortcut: you can see the sequence number, the applied latency and the load of the moment without
// opening Jaeger — useful precisely before the student instruments anything.
func respond(w http.ResponseWriter, cfg Config, decision Decision, status int, body any) {
	header := w.Header()
	header.Set("X-Partner-Name", cfg.Name)
	header.Set("X-Partner-Seq", strconv.FormatUint(decision.Sequence, 10))
	header.Set("X-Partner-Latency-Ms", strconv.FormatInt(decision.Latency.Milliseconds(), 10))
	header.Set("X-Partner-Inflight", strconv.FormatInt(decision.InFlight, 10))
	writeJSON(w, status, body)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

// sleep waits while respecting the client's cancellation — without it, a slow partner would hold on
// to goroutines of requests that are already gone.
func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
