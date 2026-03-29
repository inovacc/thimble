package mcp

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// handleSignals listens for SIGINT and SIGTERM and cancels the context.
func (b *Bridge) handleSignals(ctx context.Context, cancel context.CancelFunc) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigs:
		b.logger.Warn("received signal, shutting down", "signal", sig)
		cancel()
	case <-ctx.Done():
	}

	signal.Stop(sigs)
}
