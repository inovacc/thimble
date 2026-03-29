package tracing

import (
	"context"
	"testing"
)

func TestInit(t *testing.T) {
	ctx := context.Background()

	shutdown, err := Init(ctx, "test-service", "v0.0.1")
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if shutdown == nil {
		t.Fatal("Init() returned nil shutdown function")
	}

	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
}

func TestTracer(t *testing.T) {
	tr := Tracer()
	if tr == nil {
		t.Fatal("Tracer() returned nil")
	}
}

func TestNewExporter_DefaultStdout(t *testing.T) {
	// Ensure OTLP endpoint is not set so we get stdout exporter.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	ctx := context.Background()

	exp, err := newExporter(ctx)
	if err != nil {
		t.Fatalf("newExporter() error = %v", err)
	}

	if exp == nil {
		t.Fatal("newExporter() returned nil exporter")
	}
}

func TestNewExporter_OTLPEndpoint(t *testing.T) {
	// Set an OTLP endpoint to trigger OTLP exporter creation.
	// Use a non-routable address so no real connection is made.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "")

	ctx := context.Background()

	exp, err := newExporter(ctx)
	if err != nil {
		t.Fatalf("newExporter() with OTLP endpoint error = %v", err)
	}

	if exp == nil {
		t.Fatal("newExporter() returned nil exporter for OTLP")
	}
}

func TestNewExporter_OTLPInsecureFalse(t *testing.T) {
	// Setting OTEL_EXPORTER_OTLP_INSECURE=false should use TLS.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "false")

	ctx := context.Background()

	exp, err := newExporter(ctx)
	if err != nil {
		t.Fatalf("newExporter() with TLS error = %v", err)
	}

	if exp == nil {
		t.Fatal("newExporter() returned nil exporter for OTLP TLS")
	}
}

func TestInitWithOTLP(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")

	ctx := context.Background()

	shutdown, err := Init(ctx, "test-otlp", "v0.0.1")
	if err != nil {
		t.Fatalf("Init() with OTLP error = %v", err)
	}

	if shutdown == nil {
		t.Fatal("Init() with OTLP returned nil shutdown")
	}
	// Shutdown — the exporter will fail to flush since there's no real
	// collector, but the provider itself should shut down cleanly.
	_ = shutdown(ctx)
}

func TestEnabled(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envVal   string
		expected bool
	}{
		{
			name:     "disabled by default",
			expected: false,
		},
		{
			name:     "enabled via THIMBLE_TRACING=1",
			envKey:   "THIMBLE_TRACING",
			envVal:   "1",
			expected: true,
		},
		{
			name:     "not enabled via THIMBLE_TRACING=0",
			envKey:   "THIMBLE_TRACING",
			envVal:   "0",
			expected: false,
		},
		{
			name:     "enabled via OTEL_TRACES_EXPORTER",
			envKey:   "OTEL_TRACES_EXPORTER",
			envVal:   "otlp",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean env before each test.
			t.Setenv("THIMBLE_TRACING", "")
			t.Setenv("OTEL_TRACES_EXPORTER", "")

			if tt.envKey != "" {
				t.Setenv(tt.envKey, tt.envVal)
			}

			if got := Enabled(); got != tt.expected {
				t.Errorf("Enabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}
