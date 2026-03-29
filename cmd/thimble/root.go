package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/inovacc/thimble/internal/mcp"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "thimble",
	Short: "MCP server, hook dispatcher, CLI tools",
	Long:  `thimble is a single-binary MCP plugin: MCP server, hook dispatcher, CLI tools. No daemon required.`,
	// Default command: run MCP server on stdio.
	RunE: runMCPBridge,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.Version = GetVersionJSON()
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

func runMCPBridge(_ *cobra.Command, _ []string) error {
	bridge, err := mcp.New()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to start MCP bridge: %v\n", err)
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	return bridge.Run(ctx)
}
