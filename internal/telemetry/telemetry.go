// Package telemetry provides OpenTelemetry instrumentation for the application
package telemetry

import (
	"context"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
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
	ServiceName    string
	ServiceVersion string
	Environment    string
	// EnableStdoutExporter enables stdout trace exporter for debugging
	EnableStdoutExporter bool
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

	// Create trace exporter
	var traceExporter sdktrace.SpanExporter
	if cfg.EnableStdoutExporter {
		traceExporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
	}

	// Create tracer provider
	var tp *sdktrace.TracerProvider
	if traceExporter != nil {
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithResource(res),
		)
	} else {
		// No-op tracer provider if no exporter configured
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
		)
	}

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Initialize tracer
	Tracer = tp.Tracer(cfg.ServiceName)

	// Initialize meter (metrics)
	meterProvider := otel.GetMeterProvider()
	Meter = meterProvider.Meter(cfg.ServiceName)

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
