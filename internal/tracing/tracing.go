// Package tracing provides OpenTelemetry tracing initialization for thimble.
// Tracing is opt-in via THIMBLE_TRACING=1 or OTEL_TRACES_EXPORTER env var.
//
// Exporter selection:
//   - If OTEL_EXPORTER_OTLP_ENDPOINT is set, uses OTLP gRPC exporter (for Jaeger, Grafana Tempo, etc.)
//   - Otherwise, falls back to stdout exporter writing to stderr
//
// TLS: OTLP connections default to insecure. Set OTEL_EXPORTER_OTLP_INSECURE=false
// to require TLS.
package tracing

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TracerName is the instrumentation scope name used for all thimble spans.
const TracerName = "github.com/inovacc/thimble"

// newExporter creates the appropriate SpanExporter based on environment variables.
// If OTEL_EXPORTER_OTLP_ENDPOINT is set, it returns an OTLP gRPC exporter;
// otherwise it returns a stdout exporter writing to stderr.
func newExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		return stdouttrace.New(stdouttrace.WithWriter(os.Stderr))
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
	}

	// Default to insecure unless explicitly set to "false".
	insecureEnv := os.Getenv("OTEL_EXPORTER_OTLP_INSECURE")
	if !strings.EqualFold(insecureEnv, "false") {
		opts = append(opts, otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	return otlptracegrpc.New(ctx, opts...)
}

// Init initializes the OpenTelemetry tracer provider.
// If OTEL_EXPORTER_OTLP_ENDPOINT is set, traces are exported via OTLP gRPC
// (suitable for Jaeger, Grafana Tempo, and other OTLP-compatible backends).
// Otherwise, traces are written as JSON to stderr so they don't interfere
// with MCP JSON-RPC on stdout.
// Returns a shutdown function that should be deferred.
func Init(ctx context.Context, serviceName, version string) (func(context.Context) error, error) {
	exporter, err := newExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}

	r, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(r),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))),
	)

	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

// Tracer returns a named tracer for the thimble package.
func Tracer() trace.Tracer {
	return otel.Tracer(TracerName)
}

// Enabled returns true if tracing is configured via THIMBLE_TRACING=1
// or OTEL_TRACES_EXPORTER env var.
func Enabled() bool {
	return os.Getenv("THIMBLE_TRACING") == "1" || os.Getenv("OTEL_TRACES_EXPORTER") != ""
}
