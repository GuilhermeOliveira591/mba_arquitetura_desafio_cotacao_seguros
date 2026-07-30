package main

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

// Config is a load run expressed as data.
//
// Everything comes in as a command-line flag because loadgen is a command, not a service: the
// student runs it, reads the report and it exits. Only the two values the docker-compose.yml has to
// inject (where the API is, which broker to impersonate) also accept an environment variable — the
// same binary has to work both inside the compose network and against `make run` on the host.
type Config struct {
	Target      string        // -target / LOADGEN_TARGET
	Tenant      string        // -tenant / LOADGEN_TENANT
	Concurrency int           // -concurrency
	Requests    int           // -requests
	Baseline    int           // -baseline
	Duration    time.Duration // -duration
	Timeout     time.Duration // -timeout
	Distinct    int           // -distinct
}

const (
	defaultTarget = "http://localhost:8080"
	defaultTenant = "corretora-a"
)

// Defaults calibrated against the environment in docker-compose.yml, not guessed. They exist to
// reproduce the failure scenario, not to stress the machine.
//
//   - 50 concurrent requests. The tempting number would be 6, one above the 5 in-flight that
//     partner-degrading tolerates (PARTNER_DEGRADE_AFTER) — and it does nothing, because the
//     concurrency at the API is not the load at the partner. The aggregation is serial, so each
//     request spends most of its life waiting on partner-slow and only a fraction of it inside
//     partner-degrading; it takes many requests at the door for a handful to pile up there at the
//     same time. Measured on 2026-07-30: at 20 in flight the p95 barely moves (1.1x over the
//     baseline), between 25 and 30 it climbs steeply (2.2x, then 3.2x), and from 40 on the
//     degradation saturates at its 6s cap. 50 lands past the saturation point, which is what makes
//     the run reproduce on a slower machine too.
//   - 200 requests at that concurrency is four rounds — enough for the degradation to reach its
//     plateau and for the p95 to stop moving, about 25 seconds of run.
//   - 10 sequential requests measure the baseline BEFORE the load: without a "before", the "after"
//     is just a number nobody can read. Ten and not five because the baseline also has to give an
//     honest error rate: partner-flaky fails 40% of the time regardless of load, and over five
//     requests that lands anywhere between 0% and 80% — a "before" noisy enough to suggest that the
//     load improved the platform's error rate, which is exactly the wrong lesson. The extra
//     requests cost around ten seconds of run.
const (
	defaultConcurrency = 50
	defaultRequests    = 200
	defaultBaseline    = 10
	defaultDuration    = 2 * time.Minute
	defaultTimeout     = 30 * time.Second
	defaultDistinct    = 5
)

func parseConfig(args []string, env func(string) string, out io.Writer) (Config, error) {
	flags := flag.NewFlagSet("loadgen", flag.ContinueOnError)
	flags.SetOutput(out)

	var cfg Config
	flags.StringVar(&cfg.Target, "target", text(env, "LOADGEN_TARGET", defaultTarget),
		"base URL of the quotation-api")
	flags.StringVar(&cfg.Tenant, "tenant", text(env, "LOADGEN_TENANT", defaultTenant),
		"broker sent in the X-Tenant-Id header")
	flags.IntVar(&cfg.Concurrency, "concurrency", defaultConcurrency,
		"requests in flight during the load phase")
	flags.IntVar(&cfg.Requests, "requests", defaultRequests,
		"total requests in the load phase")
	flags.IntVar(&cfg.Baseline, "baseline", defaultBaseline,
		"sequential requests measured before the load (0 skips the baseline)")
	flags.DurationVar(&cfg.Duration, "duration", defaultDuration,
		"time limit for the run; whatever is missing is not sent")
	flags.DurationVar(&cfg.Timeout, "timeout", defaultTimeout,
		"limit per request (0 waits forever, like the starter's API does with its partners)")
	flags.IntVar(&cfg.Distinct, "distinct", defaultDistinct,
		"how many different quotes to rotate through; with -distinct 1 every request is the same one")

	flags.Usage = func() {
		fmt.Fprintf(out, "loadgen reproduces the failure scenario of the starter against the quotation-api.\n\n")
		fmt.Fprintf(out, "Usage: loadgen [flags]\n\n")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}

	cfg.Target = strings.TrimRight(strings.TrimSpace(cfg.Target), "/")
	cfg.Tenant = strings.TrimSpace(cfg.Tenant)
	return cfg, cfg.validate()
}

func (c Config) validate() error {
	if u, err := url.Parse(c.Target); err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("-target: %q is not an absolute URL", c.Target)
	}

	switch {
	case c.Tenant == "":
		return fmt.Errorf("-tenant cannot be empty")
	case c.Concurrency < 1:
		return fmt.Errorf("-concurrency must be at least 1, got %d", c.Concurrency)
	case c.Requests < 1:
		return fmt.Errorf("-requests must be at least 1, got %d", c.Requests)
	case c.Baseline < 0:
		return fmt.Errorf("-baseline cannot be negative, got %d", c.Baseline)
	case c.Duration <= 0:
		return fmt.Errorf("-duration must be greater than zero, got %s", c.Duration)
	case c.Timeout < 0:
		return fmt.Errorf("-timeout cannot be negative, got %s", c.Timeout)
	case c.Distinct < 1:
		return fmt.Errorf("-distinct must be at least 1, got %d", c.Distinct)
	}
	return nil
}

// Endpoint is the address of the only endpoint loadgen calls.
func (c Config) Endpoint() string { return c.Target + "/quotes" }

func text(env func(string) string, key, fallback string) string {
	if v := strings.TrimSpace(env(key)); v != "" {
		return v
	}
	return fallback
}
