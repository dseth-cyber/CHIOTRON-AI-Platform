// Package telemetry wires the OpenTelemetry baseline for the Control Plane.
//
// VM4 emits traces over OTLP to the existing Tempo collector and is scraped by
// the existing Prometheus; the AI Platform deliberately does not operate a
// monitoring stack of its own (ARCHITECTURE-v1 section 9).
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"

	"github.com/chiotron/ai-control-plane/internal/config"
)

// Provider owns the telemetry pipelines and the scrape endpoint that expose
// them. Shutdown flushes pending spans and must run before the process exits.
type Provider struct {
	MetricsHandler http.Handler
	// Metrics is the platform's own instrument set, emitted through the same
	// registry as the runtime and HTTP metrics so there is one scrape endpoint.
	Metrics  *Metrics
	shutdown []func(context.Context) error
}

// Setup installs the global tracer, meter and propagator.
//
// Metrics are always available: Prometheus scrapes them from /metrics and
// needs no collector to be reachable at start. Trace export is enabled only
// when a collector endpoint is configured, so a developer machine with no
// Tempo running behaves exactly like production minus the spans.
func Setup(ctx context.Context, cfg config.Config, log *slog.Logger) (*Provider, error) {
	// Attribute keys are written out rather than taken from a semconv package:
	// the helper names move between semconv releases, the wire keys do not.
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		attribute.String("service.name", cfg.ServiceName),
		attribute.String("service.version", cfg.ServiceVersion),
		attribute.String("deployment.environment.name", cfg.Environment),
	))
	if err != nil {
		return nil, fmt.Errorf("build telemetry resource: %w", err)
	}

	provider := &Provider{}

	// Runtime and process metrics come from the standard collectors so the
	// existing Prometheus dashboards see the same series as other services.
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	metricExporter, err := otelprom.New(otelprom.WithRegisterer(registry))
	if err != nil {
		return nil, fmt.Errorf("create prometheus exporter: %w", err)
	}
	meterProvider := metricsdk.NewMeterProvider(
		metricsdk.WithResource(res),
		metricsdk.WithReader(metricExporter),
	)
	otel.SetMeterProvider(meterProvider)
	provider.shutdown = append(provider.shutdown, meterProvider.Shutdown)
	provider.MetricsHandler = promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	// Instruments are created after the meter provider is installed, or they
	// would bind to the no-op default and record nothing.
	provider.Metrics = NewMetrics(cfg.TokenPrices, cfg.Currency, log)

	// W3C trace context lets a span started at Nginx or the portal continue
	// through the Control Plane and on to downstream services.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	if !cfg.TracingEnabled() {
		log.Info("trace export disabled", "reason", "OTEL_EXPORTER_OTLP_ENDPOINT is empty")
		return provider, nil
	}

	options := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(cfg.OTLPEndpoint)}
	if cfg.OTLPInsecure {
		options = append(options, otlptracehttp.WithInsecure())
	}
	traceExporter, err := otlptracehttp.New(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("create otlp trace exporter: %w", err)
	}
	tracerProvider := tracesdk.NewTracerProvider(
		tracesdk.WithResource(res),
		tracesdk.WithBatcher(traceExporter),
		tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(cfg.TraceSampleRatio))),
	)
	otel.SetTracerProvider(tracerProvider)
	provider.shutdown = append(provider.shutdown, tracerProvider.Shutdown)
	log.Info("trace export enabled", "endpoint", cfg.OTLPEndpoint, "sampleRatio", cfg.TraceSampleRatio)

	return provider, nil
}

// Shutdown flushes every pipeline, reporting all failures rather than the first.
func (p *Provider) Shutdown(ctx context.Context) error {
	var errs []error
	for i := len(p.shutdown) - 1; i >= 0; i-- {
		if err := p.shutdown[i](ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
