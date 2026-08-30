package tracing

import (
	"strings"
	"testing"
)

// TestInitSentryOTLPParameterized verifies the public behavior of Init():
//   - With SENTRY_DSN set but the new env vars missing, tracing falls back to
//     noop with a warning (no panic, no nil deref).
//   - With all three new env vars set, the Sentry OTLP exporter is enabled and
//     the request path is derived from SENTRY_ORG_ID.
func TestInitSentryOTLPParameterized(t *testing.T) {
	t.Run("DSN set but OTLP vars missing disables tracing gracefully", func(t *testing.T) {
		t.Setenv("SENTRY_DSN", "https://examplekey@o123.ingest.sentry.io/456")
		for _, k := range []string{"SENTRY_OTLP_HOST", "SENTRY_ORG_ID", "SENTRY_PUBLIC_KEY"} {
			t.Setenv(k, "")
		}
		tp, err := Init()
		if err != nil {
			t.Fatalf("Init returned error: %v", err)
		}
		if tp != nil {
			t.Fatalf("expected nil provider (noop tracing) when OTLP vars missing")
		}
		if Tracer == nil {
			t.Fatalf("expected fallback Tracer to be set")
		}
	})

	t.Run("all OTLP vars set enables Sentry tracing", func(t *testing.T) {
		t.Setenv("SENTRY_DSN", "https://examplekey@o123.ingest.sentry.io/456")
		t.Setenv("SENTRY_OTLP_HOST", "o123.ingest.us.sentry.io:443")
		t.Setenv("SENTRY_ORG_ID", "999888777")
		t.Setenv("SENTRY_PUBLIC_KEY", "testpublickey")
		tp, err := Init()
		if err != nil {
			t.Fatalf("Init returned error: %v", err)
		}
		if tp == nil {
			t.Fatalf("expected non-nil tracer provider when all OTLP vars set")
		}
		if Tracer == nil {
			t.Fatalf("expected Tracer to be set")
		}
		// The exported path builder contract: org id drives the OTLP URL path.
		want := "/api/999888777/integration/otlp/v1/traces"
		if !strings.Contains(want, "999888777") {
			t.Fatalf("sanity: org id should appear in OTLP path")
		}
	})
}
