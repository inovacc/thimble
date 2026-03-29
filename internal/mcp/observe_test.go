package mcp

import (
	"context"
	"errors"
	"testing"
)

func TestSpanTool_Disabled(t *testing.T) {
	t.Parallel()

	// With tracing disabled (default), spanTool should be a no-op.
	ctx, finish := spanTool(context.Background(), "test_tool")
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	// finish should not panic.
	finish(nil)
	finish(errors.New("test error"))
}

func TestSpanHook_Disabled(t *testing.T) {
	t.Parallel()

	// With tracing disabled (default), spanHook should be a no-op.
	ctx, finish := spanHook(context.Background(), "PreToolUse", "claude-code")
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	// finish should not panic.
	finish(nil)
	finish(errors.New("test error"))
}
