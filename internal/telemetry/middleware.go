package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Middleware creates an HTTP middleware that traces requests and records metrics
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := Span(r.Context(), "http_request",
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("http.path", r.URL.Path),
				attribute.String("http.host", r.Host),
				attribute.String("http.user_agent", r.UserAgent()),
				attribute.String("http.remote_addr", r.RemoteAddr),
			),
		)
		defer span.End()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		start := time.Now()
		next.ServeHTTP(wrapped, r.WithContext(ctx))
		duration := time.Since(start)

		// Record span attributes
		span.SetAttributes(
			attribute.Int("http.status_code", wrapped.statusCode),
			attribute.Float64("http.duration_ms", float64(duration.Milliseconds())),
		)

		// Record metrics
		if RequestCounter != nil {
			RequestCounter.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("method", r.Method),
					attribute.String("path", r.URL.Path),
					attribute.Int("status", wrapped.statusCode),
				),
			)
		}

		if RequestDuration != nil {
			RequestDuration.Record(ctx, duration.Seconds(),
				metric.WithAttributes(
					attribute.String("method", r.Method),
					attribute.String("path", r.URL.Path),
				),
			)
		}

		// Set span status based on HTTP status code
		if wrapped.statusCode >= 500 {
			span.SetAttributes(attribute.Bool("error", true))
		}
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) Header() http.Header {
	return rw.ResponseWriter.Header()
}

// Flush implements http.Flusher if the underlying writer supports it
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// DatabaseQuery wraps a database query with tracing
func DatabaseQuery(ctx context.Context, query string, args ...interface{}) (context.Context, func()) {
	ctx, span := Span(ctx, "db_query",
		trace.WithAttributes(
			attribute.String("db.query", query),
			attribute.Int("db.args_count", len(args)),
		),
	)

	if DatabaseQueries != nil {
		DatabaseQueries.Add(ctx, 1)
	}

	return ctx, func() {
		span.End()
	}
}

// LogOperation logs an operation with tracing
func LogOperation(ctx context.Context, operation string, attrs ...attribute.KeyValue) {
	AddEvent(ctx, operation, attrs...)
}

// LogError logs an error with tracing
func LogError(ctx context.Context, err error, msg string) {
	RecordError(ctx, err, attribute.String("error.message", msg))
}

// FormatQuery formats a query string for logging (truncates if too long)
func FormatQuery(query string) string {
	const maxLen = 200
	if len(query) > maxLen {
		return query[:maxLen] + "..."
	}
	return query
}

// FormatArgs formats query arguments for logging
func FormatArgs(args ...interface{}) string {
	if len(args) == 0 {
		return ""
	}
	result := fmt.Sprintf("%v", args)
	const maxLen = 100
	if len(result) > maxLen {
		return result[:maxLen] + "..."
	}
	return result
}
