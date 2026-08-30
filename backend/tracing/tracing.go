package tracing

import (
	"context"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var Tracer trace.Tracer

func Init() (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("llpoa"),
			semconv.ServiceVersion("1.0.0"),
			attribute.String("deployment.environment", envOr("APP_ENV", "production")),
		),
	)
	if err != nil {
		return nil, err
	}

	var exporters []sdktrace.SpanExporter

	if dsn := os.Getenv("SENTRY_DSN"); dsn != "" {
		sentryHost := envOr("SENTRY_OTLP_HOST", "")
		sentryOrgID := envOr("SENTRY_ORG_ID", "")
		sentryPublicKey := envOr("SENTRY_PUBLIC_KEY", "")
		if sentryHost == "" || sentryOrgID == "" || sentryPublicKey == "" {
			log.Println("WARN: SENTRY_DSN set but SENTRY_OTLP_HOST/SENTRY_ORG_ID/SENTRY_PUBLIC_KEY missing — Sentry OTLP tracing disabled")
		} else {
			sentryPath := "/api/" + sentryOrgID + "/integration/otlp/v1/traces"

			sentryExp, err := otlptracehttp.New(ctx,
				otlptracehttp.WithEndpoint(sentryHost),
				otlptracehttp.WithURLPath(sentryPath),
				otlptracehttp.WithHeaders(map[string]string{
					"x-sentry-auth": sentryPublicKey,
				}),
			)
			if err != nil {
				log.Printf("WARN: failed to create Sentry OTLP exporter: %v", err)
			} else {
				exporters = append(exporters, sentryExp)
				log.Println("Sentry OTLP tracing enabled")
			}
		}
	}

	if len(exporters) == 0 {
		log.Println("WARN: no APM exporters configured — tracing disabled")
		noop := trace.NewNoopTracerProvider()
		otel.SetTracerProvider(noop)
		Tracer = noop.Tracer("llpoa")
		return nil, nil
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
	)

	for _, exp := range exporters {
		tp.RegisterSpanProcessor(sdktrace.NewBatchSpanProcessor(exp))
	}

	otel.SetTracerProvider(tp)
	Tracer = tp.Tracer("llpoa", trace.WithInstrumentationVersion("1.0.0"))

	return tp, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func Shutdown(tp *sdktrace.TracerProvider) {
	if tp == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := tp.Shutdown(ctx); err != nil {
		log.Printf("WARN: tracer provider shutdown error: %v", err)
	}
}
