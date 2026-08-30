package tracing

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Middleware wraps an http.Handler with OpenTelemetry tracing.
// It creates a span per request, injects the trace context into the request
// context, and records the HTTP method, path, and status code.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := propagation.TraceContext{}.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		spanName := r.Method + " " + routePattern(r.URL.Path)
		ctx, span := Tracer.Start(ctx, spanName,
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(r.Method),
				semconv.URLFullKey.String(r.URL.String()),
				semconv.HTTPRouteKey.String(r.URL.Path),
				semconv.UserAgentOriginalKey.String(r.UserAgent()),
			),
		)
		defer span.End()

		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lrw, r.WithContext(ctx))

		span.SetAttributes(semconv.HTTPResponseStatusCodeKey.Int(lrw.statusCode))
		if lrw.statusCode >= 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", lrw.statusCode))
		}
	})
}

// NewSpan creates a named child span from the request context.
func NewSpan(r *http.Request, name string, opts ...trace.SpanStartOption) (trace.Span, context.Context) {
	ctx := r.Context()
	ctx, span := Tracer.Start(ctx, name, opts...)
	return span, ctx
}

// StartSpan creates a named child span from a plain context (non-request).
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (trace.Span, context.Context) {
	ctx, span := Tracer.Start(ctx, name, opts...)
	return span, ctx
}

func routePattern(path string) string {
	if strings.HasPrefix(path, "/api/bylaws/search") {
		return "GET /api/bylaws/search"
	}
	if strings.HasPrefix(path, "/api/bylaws/") {
		return "GET /api/bylaws/{id}"
	}
	if path == "/api/bylaws" {
		return "GET /api/bylaws"
	}
	if path == "/api/chat" {
		return "POST /api/chat"
	}
	if path == "/api/health" {
		return "GET /api/health"
	}
	if path == "/" || strings.HasPrefix(path, "/assets/") {
		return "GET /static/*"
	}
	return path
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}
