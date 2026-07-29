package platform

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

// metricInterval is how often the SDK pushes metrics to the Collector. It is short because the
// failure scenario of the challenge lasts minutes, not hours: with the default of 60s the student
// would see three data points and no shape at all.
const metricInterval = 10 * time.Second

// Shutdown flushes what is still buffered and closes the exporters. Calling it is what keeps the
// last spans of a run from being lost when the process exits.
type Shutdown func(context.Context) error

// StartTelemetry boots the OpenTelemetry SDK and installs it globally: tracer provider, meter
// provider and context propagation.
//
// This is the plumbing the starter delivers already done. It is deliberate: the challenge is the
// circuit breaker, not the SDK boilerplate, and a student who spends three hours here does not get
// to the part that is being assessed. Everything below is generic instrumentation — HTTP in, HTTP
// out, Go runtime. What is NOT here, and is the student's job, is the business instrumentation:
// breaker state transitions, cache hit/miss and latency per partner.
//
// Nothing here talks to Jaeger or Prometheus directly: the only destination is the Collector, which
// decides where the data lands. Swapping the observability backend is a change in
// deploy/otel/collector.yaml, not in this file.
func StartTelemetry(ctx context.Context, cfg Telemetry) (Shutdown, error) {
	if !cfg.Enabled {
		log.Print("quotation-api: OpenTelemetry off (OTEL_SDK_DISABLED)")
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(cfg.ServiceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry resource: %w", err)
	}

	// The exporters connect lazily: the API starts even with the Collector down, and reconnects on
	// its own when it comes back. The cost of that choice is honest — telemetry produced meanwhile
	// is dropped, and the SDK logs the failure.
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpointURL(cfg.Endpoint))
	if err != nil {
		return nil, fmt.Errorf("otlp trace exporter: %w", err)
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter),
		// Everything is sampled. In production this would be reckless; here it is the point: the
		// student needs to find THE request that failed, not a statistical sample of the traffic.
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpointURL(cfg.Endpoint))
	if err != nil {
		return nil, errors.Join(fmt.Errorf("otlp metric exporter: %w", err), tracerProvider.Shutdown(ctx))
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(metricInterval))),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	// TraceContext is what makes the trace survive the hop to the partner: it writes the traceparent
	// header on the way out and reads it on the way in. Baggage rides along because a multi-tenant
	// platform eventually wants to carry the broker down the call chain.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Go runtime metrics (goroutines, GC, heap). Under load they are what separates "the partner is
	// slow" from "we ran out of goroutines waiting for the partner".
	if err := runtime.Start(runtime.WithMeterProvider(meterProvider)); err != nil {
		return nil, errors.Join(
			fmt.Errorf("runtime metrics: %w", err),
			tracerProvider.Shutdown(ctx),
			meterProvider.Shutdown(ctx),
		)
	}

	log.Printf("quotation-api: OpenTelemetry on | service=%q collector=%s", cfg.ServiceName, cfg.Endpoint)

	return func(ctx context.Context) error {
		return errors.Join(tracerProvider.Shutdown(ctx), meterProvider.Shutdown(ctx))
	}, nil
}

// InstrumentHandler wraps the API router with the OpenTelemetry HTTP instrumentation: one server
// span per request, plus the http.server.* metrics — with no line of code inside the handlers.
func InstrumentHandler(h http.Handler, serviceName string) http.Handler {
	return otelhttp.NewHandler(h, serviceName,
		// /healthz is called every 5s by the compose healthcheck. Left in, it would bury the
		// student's quote trace under hundreds of irrelevant ones.
		otelhttp.WithFilter(func(r *http.Request) bool { return r.URL.Path != "/healthz" }),
		// Default naming would leave every span called after the service. "POST /quotes" is what
		// makes the trace list readable at a glance.
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	)
}

// InstrumentTransport wraps an HTTP transport so that every outgoing call becomes a client span
// inside the current trace, carrying the traceparent header to the callee.
//
// It adds no timeout, no retry and no circuit breaker: this is observability, not resilience. The
// serial waterfall of the three partners becomes visible in Jaeger exactly as it is — which is the
// evidence the student needs before touching anything.
func InstrumentTransport(base http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(base)
}
