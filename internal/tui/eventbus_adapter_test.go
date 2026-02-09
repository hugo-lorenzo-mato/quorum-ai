package tui

import (
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

func TestEventBusAdapter_ConvertsEvents(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()

	adapter := NewEventBusAdapter(bus)
	defer adapter.Close()

	// Publish an event
	bus.Publish(events.NewPhaseStartedEvent("wf-1", "", "analyze"))

	// Should receive converted message
	select {
	case msg := <-adapter.MsgChannel():
		phaseMsg, ok := msg.(PhaseUpdateMsg)
		if !ok {
			t.Errorf("Expected PhaseUpdateMsg, got %T", msg)
		}
		if phaseMsg.Phase != "analyze" {
			t.Errorf("Expected analyze phase, got %s", phaseMsg.Phase)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for message")
	}
}

func TestEventBusAdapter_HandlesPriority(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()

	adapter := NewEventBusAdapter(bus)
	defer adapter.Close()

	// Publish priority event
	bus.PublishPriority(events.NewWorkflowFailedEvent("wf-1", "", "test", nil))

	// Should receive error message
	select {
	case msg := <-adapter.MsgChannel():
		_, ok := msg.(ErrorMsg)
		if !ok {
			t.Errorf("Expected ErrorMsg, got %T", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for priority message")
	}
}
