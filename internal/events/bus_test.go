package events

import (
	"sync"
	"testing"
	"time"
)

func TestEventBus_Subscribe(t *testing.T) {
	bus := New(10)
	defer bus.Close()

	ch := bus.Subscribe()

	event := NewWorkflowStartedEvent("wf-1", "test prompt")
	bus.Publish(event)

	select {
	case received := <-ch:
		if received.EventType() != TypeWorkflowStarted {
			t.Errorf("expected %s, got %s", TypeWorkflowStarted, received.EventType())
		}
		if received.WorkflowID() != "wf-1" {
			t.Errorf("expected wf-1, got %s", received.WorkflowID())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestEventBus_SubscribeByType(t *testing.T) {
	bus := New(10)
	defer bus.Close()

	taskCh := bus.Subscribe(TypeTaskStarted, TypeTaskCompleted)
	allCh := bus.Subscribe()

	bus.Publish(NewWorkflowStartedEvent("wf-1", "prompt"))
	bus.Publish(NewTaskStartedEvent("wf-1", "task-1", "/path"))

	// allCh should receive both
	select {
	case <-allCh:
	case <-time.After(100 * time.Millisecond):
		t.Error("allCh should receive workflow event")
	}
	select {
	case <-allCh:
	case <-time.After(100 * time.Millisecond):
		t.Error("allCh should receive task event")
	}

	// taskCh should only receive task event
	select {
	case received := <-taskCh:
		if received.EventType() != TypeTaskStarted {
			t.Errorf("expected task_started, got %s", received.EventType())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("taskCh should receive task event")
	}
}

func TestEventBus_PriorityNeverDrops(t *testing.T) {
	bus := New(5) // Small buffer
	defer bus.Close()

	priorityCh := bus.SubscribePriority()

	// Saturate with many events
	for i := 0; i < 100; i++ {
		bus.Publish(NewLogEvent("wf-1", "info", "log message", nil))
	}

	// Send priority event
	failedEvent := NewWorkflowFailedEvent("wf-1", "execute", nil)
	bus.PublishPriority(failedEvent)

	// Priority channel should have the event
	select {
	case received := <-priorityCh:
		if received.EventType() != TypeWorkflowFailed {
			t.Errorf("expected workflow_failed, got %s", received.EventType())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("priority event was dropped")
	}
}

func TestEventBus_RingBufferDropsOldest(t *testing.T) {
	bus := New(5)
	defer bus.Close()

	ch := bus.Subscribe()

	// Fill buffer
	for i := 0; i < 10; i++ {
		bus.Publish(NewLogEvent("wf-1", "info", "message", nil))
	}

	// Should have dropped some events
	if bus.DroppedCount() == 0 {
		t.Error("expected some events to be dropped")
	}

	// Drain and verify we can still receive
	received := 0
	for {
		select {
		case <-ch:
			received++
		default:
			goto done
		}
	}
done:

	if received == 0 {
		t.Error("should have received at least some events")
	}
}

func TestEventBus_ConcurrentPublish(t *testing.T) {
	bus := New(100)
	defer bus.Close()

	ch := bus.Subscribe()

	var wg sync.WaitGroup
	numGoroutines := 10
	eventsPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				bus.Publish(NewLogEvent("wf-1", "info", "concurrent", nil))
			}
		}(i)
	}

	wg.Wait()

	// Some events should have been received (accounting for drops)
	received := 0
drainLoop:
	for {
		select {
		case <-ch:
			received++
		default:
			break drainLoop
		}
	}

	if received == 0 {
		t.Error("should have received some events")
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := New(10)
	defer bus.Close()

	ch := bus.Subscribe()
	bus.Unsubscribe(ch)

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Error("channel should be closed after unsubscribe")
	}
}
