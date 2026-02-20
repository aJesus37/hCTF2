// Package telemetry provides OpenTelemetry instrumentation for the application
package telemetry

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	// Tracer is the global tracer
	Tracer trace.Tracer
	// Meter is the global meter
	Meter metric.Meter

	// Metrics
	RequestCounter  metric.Int64Counter
	RequestDuration metric.Float64Histogram
	ActiveUsers     metric.Int64UpDownCounter
	DatabaseQueries metric.Int64Counter
)

// Config holds telemetry configuration
type Config struct {
	ServiceName          string
	ServiceVersion       string
	Environment          string
	EnableStdoutExporter bool
	EnablePrometheus     bool
	OTLPEndpoint         string // e.g. "localhost:4318"
}

// Init initializes OpenTelemetry with the given configuration
func Init(cfg Config) (func(), error) {
	ctx := context.Background()

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create trace exporters
	var traceExporters []sdktrace.SpanExporter

	if cfg.EnableStdoutExporter {
		stdoutExp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("stdout trace exporter: %w", err)
		}
		traceExporters = append(traceExporters, stdoutExp)
	}

	if cfg.OTLPEndpoint != "" {
		otlpExp, err := otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
			otlptracehttp.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("otlp trace exporter: %w", err)
		}
		traceExporters = append(traceExporters, otlpExp)
	}

	// Create tracer provider options
	tpOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
	}

	// Add trace exporters
	for _, exp := range traceExporters {
		tpOpts = append(tpOpts, sdktrace.WithBatcher(exp))
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(tpOpts...)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Initialize tracer
	Tracer = tp.Tracer(cfg.ServiceName)

	// Create meter provider with appropriate exporters
	var meterOpts []sdkmetric.Option
	meterOpts = append(meterOpts, sdkmetric.WithResource(res))

	if cfg.EnablePrometheus {
		promExp, err := prometheus.New()
		if err != nil {
			return nil, fmt.Errorf("prometheus exporter: %w", err)
		}
		meterOpts = append(meterOpts, sdkmetric.WithReader(promExp))
	}

	if cfg.OTLPEndpoint != "" {
		otlpMetricExp, err := otlpmetrichttp.New(ctx,
			otlpmetrichttp.WithEndpoint(cfg.OTLPEndpoint),
			otlpmetrichttp.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("otlp metric exporter: %w", err)
		}
		meterOpts = append(meterOpts, sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(otlpMetricExp),
		))
	}

	mp := sdkmetric.NewMeterProvider(meterOpts...)
	otel.SetMeterProvider(mp)
	Meter = mp.Meter(cfg.ServiceName)

	// Initialize metrics
	if err := initMetrics(); err != nil {
		log.Printf("Failed to initialize metrics: %v", err)
	}

	// Return cleanup function
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
		if err := mp.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down meter provider: %v", err)
		}
	}

	return cleanup, nil
}

func initMetrics() error {
	var err error

	RequestCounter, err = Meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return err
	}

	RequestDuration, err = Meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
	)
	if err != nil {
		return err
	}

	ActiveUsers, err = Meter.Int64UpDownCounter(
		"active_users",
		metric.WithDescription("Number of active users"),
	)
	if err != nil {
		return err
	}

	DatabaseQueries, err = Meter.Int64Counter(
		"database_queries_total",
		metric.WithDescription("Total number of database queries"),
	)
	if err != nil {
		return err
	}

	return nil
}

// PrometheusHandler returns the HTTP handler for /metrics endpoint.
func PrometheusHandler() http.Handler {
	return promhttp.Handler()
}

// Span creates a new span with the given name
func Span(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer.Start(ctx, name, opts...)
}

// AddEvent adds an event to the current span
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// RecordError records an error in the current span
func RecordError(ctx context.Context, err error, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err, trace.WithAttributes(attrs...))
}

// SetAttributes sets attributes on the current span
func SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}
