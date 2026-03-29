package plugin

import (
	"bytes"
	"context"
	"log/slog"
	"os/exec"
	"runtime"
)

// RegisterPluginSubscriptions registers all event subscriptions from the given
// plugins into the EventBus. Each subscription spawns a shell command in a
// goroutine when the event fires. The handler inherits the bus timeout (1s)
// and panic recovery.
//
// Plugins without subscriptions are silently skipped.
func RegisterPluginSubscriptions(bus *EventBus, plugins []PluginDef) {
	for _, p := range plugins {
		for _, sub := range p.Subscriptions {
			pluginName := p.Name
			command := sub.Command
			event := sub.Event

			label := pluginName + ":" + event

			err := bus.SubscribeLabeled(event, label, func(payload EventPayload) {
				executeSubscriptionCommand(pluginName, command, payload)
			})
			if err != nil {
				slog.Warn("eventbus: failed to register plugin subscription",
					"plugin", pluginName,
					"event", event,
					"error", err,
				)
			}
		}
	}
}

// executeSubscriptionCommand runs a shell command for a plugin subscription.
// It uses a 1s context timeout (matching handlerTimeout) and captures output
// for logging.
func executeSubscriptionCommand(pluginName, command string, payload EventPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	}

	// Set environment variables with event context.
	cmd.Env = append(cmd.Environ(),
		"THIMBLE_EVENT="+payload.Event,
		"THIMBLE_TOOL_NAME="+payload.ToolName,
		"THIMBLE_PROJECT_DIR="+payload.ProjectDir,
	)

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		slog.Warn("eventbus: subscription command failed",
			"plugin", pluginName,
			"event", payload.Event,
			"command", command,
			"error", err,
			"stderr", stderr.String(),
		)
	}
}
