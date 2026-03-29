package mcp

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/inovacc/thimble/internal/tracing"
)

// spanTool starts a span around a tool handler execution and records attributes.
// Returns the span-wrapped context and a finish function that should be deferred.
//
// Usage:
//
//	ctx, finish := spanTool(ctx, "ctx_git_status", attribute.Bool("is_query", true))
//	defer finish(nil)
func spanTool(ctx context.Context, toolName string, attrs ...attribute.KeyValue) (context.Context, func(error)) { //nolint:unparam // variadic attrs reserved for future callers
	if !tracing.Enabled() {
		return ctx, func(error) {}
	}

	baseAttrs := make([]attribute.KeyValue, 0, 1+len(attrs))
	baseAttrs = append(baseAttrs, attribute.String("mcp.tool", toolName))
	baseAttrs = append(baseAttrs, attrs...)

	ctx, span := tracing.Tracer().Start(ctx, "mcp.tool."+toolName, //nolint:spancheck // span.End called inside returned closure
		trace.WithAttributes(baseAttrs...),
		trace.WithSpanKind(trace.SpanKindServer),
	)

	start := time.Now()

	return ctx, func(err error) { //nolint:spancheck // span.End called below
		span.SetAttributes(attribute.Int64("duration_ms", time.Since(start).Milliseconds()))

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}

		span.End()
	}
}

// spanHook starts a span around a hook dispatch and records attributes.
func spanHook(ctx context.Context, hookEvent string, platform string, attrs ...attribute.KeyValue) (context.Context, func(error)) {
	if !tracing.Enabled() {
		return ctx, func(error) {}
	}

	baseAttrs := make([]attribute.KeyValue, 0, 2+len(attrs))
	baseAttrs = append(baseAttrs,
		attribute.String("hook.event", hookEvent),
		attribute.String("hook.platform", platform),
	)
	baseAttrs = append(baseAttrs, attrs...)

	ctx, span := tracing.Tracer().Start(ctx, "hook."+hookEvent, //nolint:spancheck // span.End called inside returned closure
		trace.WithAttributes(baseAttrs...),
		trace.WithSpanKind(trace.SpanKindInternal),
	)

	start := time.Now()

	return ctx, func(err error) { //nolint:spancheck // span.End called below
		span.SetAttributes(attribute.Int64("duration_ms", time.Since(start).Milliseconds()))

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}

		span.End()
	}
}
