package telemetry

import (
	"context"
	"time"

	"github.com/agentmesh/agentmesh/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	RouteTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "agentmesh_route_total",
		Help: "Total routed requests",
	}, []string{"org", "strategy", "status"})

	RouteLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "agentmesh_route_latency_ms",
		Help:    "Routing decision latency ms",
		Buckets: prometheus.ExponentialBuckets(0.5, 2, 16),
	}, []string{"org", "mode"})
)

func InitTracer(ctx context.Context, cfg config.TraceLensConfig, serviceName string) (func(context.Context) error, error) {
	if !cfg.Enabled || cfg.Endpoint == "" {
		otel.SetTracerProvider(sdktrace.NewTracerProvider())
		return func(context.Context) error { return nil }, nil
	}
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpointURL(cfg.Endpoint),
	}
	if cfg.JWT != "" {
		opts = append(opts, otlptracehttp.WithHeaders(map[string]string{
			"Authorization": "Bearer " + cfg.JWT,
		}))
	}
	exp, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		)),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

func StartRouteSpan(ctx context.Context, org, source, dest, strategy string) (context.Context, trace.Span) {
	return Tracer("agentmesh/dataplane").Start(ctx, "route",
		trace.WithAttributes(
			attribute.String("agentmesh.org_id", org),
			attribute.String("agentmesh.source_agent", source),
			attribute.String("agentmesh.destination_agent", dest),
			attribute.String("agentmesh.routing_strategy", strategy),
		),
	)
}

func ObserveRoute(org, mode, status string, d time.Duration) {
	RouteTotal.WithLabelValues(org, mode, status).Inc()
	RouteLatency.WithLabelValues(org, mode).Observe(float64(d.Milliseconds()))
}
