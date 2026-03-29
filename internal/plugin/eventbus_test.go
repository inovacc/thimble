package plugin

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewEventBus(t *testing.T) {
	bus := NewEventBus()
	if bus == nil {
		t.Fatal("NewEventBus returned nil")
	}

	if bus.SubscriberCount(EventToolPost) != 0 {
		t.Errorf("expected 0 subscribers, got %d", bus.SubscriberCount(EventToolPost))
	}
}

func TestSubscribeAndPublish(t *testing.T) {
	bus := NewEventBus()

	var called atomic.Int32

	err := bus.Subscribe(EventToolPost, func(p EventPayload) {
		called.Add(1)

		if p.Event != EventToolPost {
			t.Errorf("expected event %q, got %q", EventToolPost, p.Event)
		}

		if p.ToolName != "Bash" {
			t.Errorf("expected ToolName %q, got %q", "Bash", p.ToolName)
		}
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	bus.Publish(EventToolPost, EventPayload{
		ToolName:   "Bash",
		ProjectDir: "/tmp/test",
	})

	if called.Load() != 1 {
		t.Errorf("expected handler called once, got %d", called.Load())
	}
}

func TestSubscribeInvalidEvent(t *testing.T) {
	bus := NewEventBus()

	err := bus.Subscribe("invalid.event", func(EventPayload) {})
	if err == nil {
		t.Fatal("expected error for invalid event")
	}

	if !errors.Is(err, ErrInvalidEvent) {
		t.Errorf("expected ErrInvalidEvent, got %v", err)
	}
}

func TestSubscribeMaxSubscribers(t *testing.T) {
	bus := NewEventBus()

	for range maxSubscribers {
		err := bus.Subscribe(EventToolPre, func(EventPayload) {})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
	}

	err := bus.Subscribe(EventToolPre, func(EventPayload) {})
	if err == nil {
		t.Fatal("expected error when exceeding max subscribers")
	}

	if !errors.Is(err, ErrMaxSubscribers) {
		t.Errorf("expected ErrMaxSubscribers, got %v", err)
	}
}

func TestPublishMultipleSubscribers(t *testing.T) {
	bus := NewEventBus()

	var count atomic.Int32

	for range 5 {
		err := bus.Subscribe(EventSessionStart, func(EventPayload) {
			count.Add(1)
		})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
	}

	bus.Publish(EventSessionStart, EventPayload{})

	if count.Load() != 5 {
		t.Errorf("expected 5 handlers called, got %d", count.Load())
	}
}

func TestPublishUnknownEvent(t *testing.T) {
	bus := NewEventBus()

	// Should not panic, just log a warning.
	bus.Publish("unknown.event", EventPayload{})
}

func TestPublishSetsTimestamp(t *testing.T) {
	bus := NewEventBus()

	var received EventPayload

	err := bus.Subscribe(EventGoalSet, func(p EventPayload) {
		received = p
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	before := time.Now()

	bus.Publish(EventGoalSet, EventPayload{Data: map[string]any{"goal": "test"}})

	if received.Timestamp.Before(before) {
		t.Error("expected timestamp to be set")
	}

	if received.Event != EventGoalSet {
		t.Errorf("expected event %q, got %q", EventGoalSet, received.Event)
	}
}

func TestPublishPreservesExplicitTimestamp(t *testing.T) {
	bus := NewEventBus()

	var received EventPayload

	err := bus.Subscribe(EventGoalClear, func(p EventPayload) {
		received = p
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	explicit := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	bus.Publish(EventGoalClear, EventPayload{Timestamp: explicit})

	if !received.Timestamp.Equal(explicit) {
		t.Errorf("expected preserved timestamp %v, got %v", explicit, received.Timestamp)
	}
}

func TestHandlerPanicRecovery(t *testing.T) {
	bus := NewEventBus()

	var calledAfterPanic atomic.Int32

	// First handler panics.
	err := bus.Subscribe(EventPluginInstalled, func(EventPayload) {
		panic("test panic")
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Second handler should still run.
	err = bus.Subscribe(EventPluginInstalled, func(EventPayload) {
		calledAfterPanic.Add(1)
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Should not panic the caller.
	bus.Publish(EventPluginInstalled, EventPayload{})

	if calledAfterPanic.Load() != 1 {
		t.Errorf("expected second handler to run despite panic, got %d calls", calledAfterPanic.Load())
	}
}

func TestHandlerTimeout(t *testing.T) {
	bus := NewEventBus()

	err := bus.Subscribe(EventPluginRemoved, func(EventPayload) {
		// Sleep longer than the handler timeout.
		time.Sleep(3 * time.Second)
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	start := time.Now()

	bus.Publish(EventPluginRemoved, EventPayload{})

	elapsed := time.Since(start)

	// Should return within ~1s (the handler timeout), not 3s.
	if elapsed > 2*time.Second {
		t.Errorf("expected publish to return within timeout, took %v", elapsed)
	}
}

func TestPublishNoSubscribers(t *testing.T) {
	bus := NewEventBus()

	// Should not panic or block.
	bus.Publish(EventSessionCompact, EventPayload{})
}

func TestSubscriberCount(t *testing.T) {
	bus := NewEventBus()

	if bus.SubscriberCount(EventToolPost) != 0 {
		t.Errorf("expected 0, got %d", bus.SubscriberCount(EventToolPost))
	}

	_ = bus.Subscribe(EventToolPost, func(EventPayload) {})
	_ = bus.Subscribe(EventToolPost, func(EventPayload) {})

	if bus.SubscriberCount(EventToolPost) != 2 {
		t.Errorf("expected 2, got %d", bus.SubscriberCount(EventToolPost))
	}

	// Different event should still be 0.
	if bus.SubscriberCount(EventToolPre) != 0 {
		t.Errorf("expected 0 for different event, got %d", bus.SubscriberCount(EventToolPre))
	}
}

func TestPublishAsync(t *testing.T) {
	bus := NewEventBus()

	var called atomic.Int32

	err := bus.Subscribe(EventToolPost, func(EventPayload) {
		time.Sleep(50 * time.Millisecond)
		called.Add(1)
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	bus.PublishAsync(EventToolPost, EventPayload{})

	// Give async handlers time to complete.
	time.Sleep(200 * time.Millisecond)

	if called.Load() != 1 {
		t.Errorf("expected async handler called, got %d", called.Load())
	}
}

func TestValidBusEvents(t *testing.T) {
	expected := []string{
		EventToolPre, EventToolPost,
		EventSessionStart, EventSessionCompact,
		EventGoalSet, EventGoalClear,
		EventPluginInstalled, EventPluginRemoved,
	}

	for _, event := range expected {
		if !ValidBusEvents[event] {
			t.Errorf("expected %q to be a valid bus event", event)
		}
	}

	if len(ValidBusEvents) != len(expected) {
		t.Errorf("expected %d valid events, got %d", len(expected), len(ValidBusEvents))
	}
}

func TestRegisterPluginSubscriptions_NoSubscriptions(t *testing.T) {
	bus := NewEventBus()

	plugins := []PluginDef{
		{
			Name:    "no-subs",
			Version: "1.0.0",
			Tools: []ToolDef{
				{Name: "ctx_test", Description: "test", Command: "echo hi"},
			},
		},
	}

	RegisterPluginSubscriptions(bus, plugins)

	// No subscribers should be registered.
	for event := range ValidBusEvents {
		if bus.SubscriberCount(event) != 0 {
			t.Errorf("expected 0 subscribers for %q, got %d", event, bus.SubscriberCount(event))
		}
	}
}

func TestRegisterPluginSubscriptions_WithSubscriptions(t *testing.T) {
	bus := NewEventBus()

	plugins := []PluginDef{
		{
			Name:    "my-plugin",
			Version: "1.0.0",
			Tools: []ToolDef{
				{Name: "ctx_test", Description: "test", Command: "echo hi"},
			},
			Subscriptions: []EventSubscription{
				{Event: EventToolPost, Command: "echo tool used"},
				{Event: EventPluginInstalled, Command: "echo installed"},
			},
		},
	}

	RegisterPluginSubscriptions(bus, plugins)

	if bus.SubscriberCount(EventToolPost) != 1 {
		t.Errorf("expected 1 subscriber for %q, got %d", EventToolPost, bus.SubscriberCount(EventToolPost))
	}

	if bus.SubscriberCount(EventPluginInstalled) != 1 {
		t.Errorf("expected 1 subscriber for %q, got %d", EventPluginInstalled, bus.SubscriberCount(EventPluginInstalled))
	}
}

func TestRegisterPluginSubscriptions_InvalidEvent(t *testing.T) {
	bus := NewEventBus()

	plugins := []PluginDef{
		{
			Name:    "bad-plugin",
			Version: "1.0.0",
			Tools: []ToolDef{
				{Name: "ctx_test", Description: "test", Command: "echo hi"},
			},
			Subscriptions: []EventSubscription{
				{Event: "invalid.event", Command: "echo fail"},
			},
		},
	}

	// Should not panic — logs a warning and continues.
	RegisterPluginSubscriptions(bus, plugins)
}

func TestEventSubscriptionInPluginDef(t *testing.T) {
	p := PluginDef{
		Name:    "test",
		Version: "1.0.0",
		Tools: []ToolDef{
			{Name: "ctx_test", Description: "test", Command: "echo hi"},
		},
		Subscriptions: []EventSubscription{
			{Event: EventToolPost, Command: "echo post"},
		},
	}

	if len(p.Subscriptions) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(p.Subscriptions))
	}

	if p.Subscriptions[0].Event != EventToolPost {
		t.Errorf("expected event %q, got %q", EventToolPost, p.Subscriptions[0].Event)
	}

	if p.Subscriptions[0].Command != "echo post" {
		t.Errorf("expected command %q, got %q", "echo post", p.Subscriptions[0].Command)
	}
}
