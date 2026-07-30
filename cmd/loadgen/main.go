// Command loadgen reproduces, in a single command, the failure scenario of the starter.
//
// The platform degrades under load — but degradation nobody measured is a hunch. loadgen measures
// twice: first a baseline with one request at a time, then the same request under concurrency, and
// it puts the two side by side. What comes out is the "before" of the challenge: the numbers the
// student takes to the document and the time window they open in Jaeger to see why.
//
// It has no protection of its own on purpose (no retry, no backoff, no partial tolerance): it
// measures what the platform delivers, it does not soften it.
//
//	loadgen                            reproduces the scenario with the calibrated defaults
//	loadgen -concurrency 40            pushes the load further
//	loadgen -baseline 0                skips the baseline
//	loadgen -h                         all the flags
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.SetFlags(0)

	cfg, err := parseConfig(os.Args[1:], os.Getenv, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		log.Fatalf("loadgen: %v", err)
	}

	if err := run(cfg, os.Stdout); err != nil {
		log.Fatalf("loadgen: %v", err)
	}
}

func run(cfg Config, out io.Writer) error {
	bodies, err := quoteBodies(cfg.Distinct)
	if err != nil {
		return err
	}

	// Ctrl-C does not kill the run: it closes the window early and still prints the report. Whoever
	// gave up waiting deserves to see what has been measured so far.
	ctx, stopListening := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopListening()
	ctx, expire := context.WithTimeout(ctx, cfg.Duration)
	defer expire()

	caller := newCaller(cfg, bodies)
	report := Report{Config: cfg}

	if cfg.Baseline > 0 {
		fmt.Fprintf(out, "baseline: %d requests, one at a time...\n", cfg.Baseline)
		phase := runPhase(ctx, caller, "baseline", cfg.Baseline, 1)
		report.Baseline = &phase
	}

	fmt.Fprintf(out, "load: %d requests, %d at a time...\n", cfg.Requests, cfg.Concurrency)
	report.Load = runPhase(ctx, caller, "load", cfg.Requests, cfg.Concurrency)

	if reason, down := unreachable(report.Baseline, &report.Load); down {
		return fmt.Errorf("no answer from %s (%s) — is the environment up? (make up)", cfg.Endpoint(), reason)
	}

	report.Write(out)
	return nil
}
