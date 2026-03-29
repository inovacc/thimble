package plugin

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Builtin event names for the event bus.
const (
	EventToolPre         = "tool.pre"
	EventToolPost        = "tool.post"
	EventSessionStart    = "session.start"
	EventSessionCompact  = "session.compact"
	EventGoalSet         = "goal.set"
	EventGoalClear       = "goal.clear"
	EventPluginInstalled = "plugin.installed"
	EventPluginRemoved   = "plugin.removed"
)

// ValidBusEvents lists the event names the bus accepts.
var ValidBusEvents = map[string]bool{
	EventToolPre:         true,
	EventToolPost:        true,
	EventSessionStart:    true,
	EventSessionCompact:  true,
	EventGoalSet:         true,
	EventGoalClear:       true,
	EventPluginInstalled: true,
	EventPluginRemoved:   true,
}

// maxSubscribers is the maximum number of subscribers per event.
const maxSubscribers = 100

// handlerTimeout is the maximum duration a single event handler may run.
const handlerTimeout = 1 * time.Second

// EventPayload carries data for an event bus publication.
type EventPayload struct {
	Event      string         `json:"event"`
	ToolName   string         `json:"tool_name,omitempty"`
	ProjectDir string         `json:"project_dir,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
}

// EventHandler is a function that processes an event payload.
type EventHandler func(EventPayload)

// subscriber pairs a handler with an optional label for logging.
type subscriber struct {
	handler EventHandler
	label   string
}

// EventBus provides asynchronous publish/subscribe for plugin lifecycle events.
// It is safe for concurrent use.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]subscriber
}

// NewEventBus creates an empty EventBus ready for subscriptions.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]subscriber),
	}
}

// ErrMaxSubscribers is returned when the subscriber limit for an event is reached.
var ErrMaxSubscribers = fmt.Errorf("maximum subscribers (%d) reached for event", maxSubscribers)

// ErrInvalidEvent is returned when subscribing to an unknown event name.
var ErrInvalidEvent = fmt.Errorf("invalid event name")

// Subscribe registers a handler for the given event.
// Returns an error if the event name is invalid or the subscriber limit is reached.
func (eb *EventBus) Subscribe(event string, handler EventHandler) error {
	return eb.SubscribeLabeled(event, "", handler)
}

// SubscribeLabeled registers a handler with a descriptive label for logging.
func (eb *EventBus) SubscribeLabeled(event, label string, handler EventHandler) error {
	if !ValidBusEvents[event] {
		return fmt.Errorf("%w: %q", ErrInvalidEvent, event)
	}

	eb.mu.Lock()
	defer eb.mu.Unlock()

	if len(eb.subscribers[event]) >= maxSubscribers {
		return fmt.Errorf("%w: %q", ErrMaxSubscribers, event)
	}

	eb.subscribers[event] = append(eb.subscribers[event], subscriber{
		handler: handler,
		label:   label,
	})

	return nil
}

// SubscriberCount returns the number of subscribers for the given event.
func (eb *EventBus) SubscriberCount(event string) int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	return len(eb.subscribers[event])
}

// Publish fires the event to all registered subscribers asynchronously.
// Each handler runs in its own goroutine with a timeout and panic recovery.
// Publish returns immediately; handler errors are logged but not propagated.
func (eb *EventBus) Publish(event string, payload EventPayload) {
	if !ValidBusEvents[event] {
		slog.Warn("eventbus: publish to unknown event", "event", event)
		return
	}

	// Stamp the event name and timestamp if not already set.
	payload.Event = event
	if payload.Timestamp.IsZero() {
		payload.Timestamp = time.Now()
	}

	eb.mu.RLock()
	subs := make([]subscriber, len(eb.subscribers[event]))
	copy(subs, eb.subscribers[event])
	eb.mu.RUnlock()

	if len(subs) == 0 {
		return
	}

	var wg sync.WaitGroup

	for _, sub := range subs {
		wg.Add(1)

		go func(s subscriber) {
			defer wg.Done()

			runHandler(s, payload)
		}(sub)
	}

	wg.Wait()
}

// PublishAsync fires the event without waiting for handlers to complete.
// Returns immediately. Useful when callers must not block on subscribers.
func (eb *EventBus) PublishAsync(event string, payload EventPayload) {
	if !ValidBusEvents[event] {
		slog.Warn("eventbus: publish to unknown event", "event", event)
		return
	}

	payload.Event = event
	if payload.Timestamp.IsZero() {
		payload.Timestamp = time.Now()
	}

	eb.mu.RLock()
	subs := make([]subscriber, len(eb.subscribers[event]))
	copy(subs, eb.subscribers[event])
	eb.mu.RUnlock()

	for _, sub := range subs {
		go func(s subscriber) {
			runHandler(s, payload)
		}(sub)
	}
}

// runHandler executes a single subscriber handler with timeout and panic recovery.
func runHandler(sub subscriber, payload EventPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	done := make(chan struct{})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("eventbus: handler panicked",
					"event", payload.Event,
					"label", sub.label,
					"panic", fmt.Sprintf("%v", r),
				)
			}

			close(done)
		}()

		sub.handler(payload)
	}()

	select {
	case <-done:
		// Handler completed normally.
	case <-ctx.Done():
		slog.Warn("eventbus: handler timed out",
			"event", payload.Event,
			"label", sub.label,
			"timeout", handlerTimeout,
		)
	}
}
