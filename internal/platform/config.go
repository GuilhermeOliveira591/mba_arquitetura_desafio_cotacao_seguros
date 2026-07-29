// Package platform holds the shared infrastructure of the quotation-api: configuration, HTTP server
// and JSON writing. No business rule lives here.
package platform

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Config is the configuration of the quotation-api.
type Config struct {
	Port      string    // PORT
	Partners  []Partner // PARTNER_ENDPOINTS
	Tenants   []string  // TENANTS
	Telemetry Telemetry // OTEL_*
}

// Partner is a partner insurer reachable over HTTP.
type Partner struct {
	Name    string
	BaseURL string
}

// Telemetry is the configuration of the OpenTelemetry SDK, which the starter ships already wired.
//
// The variable names are OpenTelemetry's own, not invented here: a student who already knows OTel
// does not have to learn a local dialect, and whatever they read in the official documentation
// holds.
type Telemetry struct {
	ServiceName string // OTEL_SERVICE_NAME
	Endpoint    string // OTEL_EXPORTER_OTLP_ENDPOINT — Collector URL, OTLP over gRPC
	Enabled     bool   // OTEL_SDK_DISABLED=true turns the whole SDK off
}

// defaultEndpoints points at the ports the docker-compose.yml publishes on the host, so that
// `make run` works against the environment brought up with `make up` without exporting anything.
const defaultEndpoints = "partner-slow=http://localhost:9001," +
	"partner-flaky=http://localhost:9002," +
	"partner-degrading=http://localhost:9003"

// defaultTenants are the brokers the starter already knows about. Multi-tenancy here is not
// decoration: it is what turns cache isolation into a real decision once the student reaches the PoC.
const defaultTenants = "corretora-a,corretora-b"

// defaultCollector, like defaultEndpoints, points at the port the compose publishes on the host.
const defaultCollector = "http://localhost:4317"

const defaultServiceName = "quotation-api"

func LoadConfig(env func(string) string) (Config, error) {
	cfg := Config{Port: text(env, "PORT", "8080")}

	var err error
	if cfg.Partners, err = parsePartners(text(env, "PARTNER_ENDPOINTS", defaultEndpoints)); err != nil {
		return Config{}, err
	}
	if cfg.Tenants, err = parseTenants(text(env, "TENANTS", defaultTenants)); err != nil {
		return Config{}, err
	}
	if cfg.Telemetry, err = parseTelemetry(env); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// parseTelemetry reads the OpenTelemetry variables. The SDK comes on by default: observability that
// has to be switched on is observability nobody uses, and here it is the instrument that proves the
// behaviour of the system.
func parseTelemetry(env func(string) string) (Telemetry, error) {
	telemetry := Telemetry{
		ServiceName: text(env, "OTEL_SERVICE_NAME", defaultServiceName),
		Endpoint:    strings.TrimSpace(text(env, "OTEL_EXPORTER_OTLP_ENDPOINT", defaultCollector)),
	}

	// OTEL_SDK_DISABLED is the standard way of turning the SDK off — useful for running the API
	// with no Collector around and no exporter errors in the log.
	switch disabled := strings.ToLower(strings.TrimSpace(env("OTEL_SDK_DISABLED"))); disabled {
	case "", "false":
		telemetry.Enabled = true
	case "true":
		return Telemetry{Enabled: false}, nil
	default:
		return Telemetry{}, fmt.Errorf("OTEL_SDK_DISABLED: %q is neither true nor false", disabled)
	}

	if u, err := url.Parse(telemetry.Endpoint); err != nil || u.Scheme == "" || u.Host == "" {
		return Telemetry{}, fmt.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT: %q is not an absolute URL", telemetry.Endpoint)
	}
	return telemetry, nil
}

// parsePartners reads the `name=url,name=url` list. Keeping the three partners in a single variable
// keeps the topology of the environment visible in one single place of the docker-compose.yml.
func parsePartners(raw string) ([]Partner, error) {
	var partners []Partner
	seen := map[string]bool{}

	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		name, endpoint, found := strings.Cut(item, "=")
		name, endpoint = strings.TrimSpace(name), strings.TrimSpace(endpoint)
		if !found || name == "" || endpoint == "" {
			return nil, fmt.Errorf("PARTNER_ENDPOINTS: %q is not in the name=url format", item)
		}

		if u, err := url.Parse(endpoint); err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("PARTNER_ENDPOINTS: %q is not an absolute URL", endpoint)
		}
		if seen[name] {
			return nil, fmt.Errorf("PARTNER_ENDPOINTS: partner %q is repeated", name)
		}

		seen[name] = true
		partners = append(partners, Partner{Name: name, BaseURL: strings.TrimRight(endpoint, "/")})
	}

	if len(partners) == 0 {
		return nil, fmt.Errorf("PARTNER_ENDPOINTS: at least one partner is required")
	}
	return partners, nil
}

func parseTenants(raw string) ([]string, error) {
	var tenants []string
	for _, t := range strings.Split(raw, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tenants = append(tenants, t)
		}
	}
	if len(tenants) == 0 {
		return nil, fmt.Errorf("TENANTS: at least one broker is required")
	}
	return tenants, nil
}

// Environment is the standard environment variable lookup.
func Environment() func(string) string { return os.Getenv }

func text(env func(string) string, key, fallback string) string {
	if v := env(key); v != "" {
		return v
	}
	return fallback
}
