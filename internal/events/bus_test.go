package events

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestEventBus_Subscribe(t *testing.T) {
	bus := New(10)
	defer bus.Close()

	ch := bus.Subscribe()

	event := NewWorkflowStartedEvent("wf-1", "", "test prompt")
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

	bus.Publish(NewWorkflowStartedEvent("wf-1", "", "prompt"))
	bus.Publish(NewTaskStartedEvent("wf-1", "", "task-1", "/path"))

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
		bus.Publish(NewLogEvent("wf-1", "", "info", "log message", nil))
	}

	// Send priority event
	failedEvent := NewWorkflowFailedEvent("wf-1", "", "execute", nil)
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
		bus.Publish(NewLogEvent("wf-1", "", "info", "message", nil))
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
				bus.Publish(NewLogEvent("wf-1", "", "info", "concurrent", nil))
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

// Project filtering tests

func TestEventBus_SubscribeForProject(t *testing.T) {
	bus := New(10)
	defer bus.Close()

	// Subscribe to project A
	chA := bus.SubscribeForProject("proj-a")

	// Subscribe to project B
	chB := bus.SubscribeForProject("proj-b")

	// Subscribe to all projects (empty projectID)
	chAll := bus.Subscribe()

	// Publish event for project A
	eventA := NewWorkflowStartedEvent("wf-1", "proj-a", "Test A")
	bus.Publish(eventA)

	// Publish event for project B
	eventB := NewWorkflowStartedEvent("wf-2", "proj-b", "Test B")
	bus.Publish(eventB)

	// Wait a bit for delivery
	time.Sleep(10 * time.Millisecond)

	// Check channel A received only project A event
	select {
	case e := <-chA:
		if e.ProjectID() != "proj-a" {
			t.Errorf("chA received wrong project: %s", e.ProjectID())
		}
	default:
		t.Error("chA should have received an event")
	}

	select {
	case e := <-chA:
		t.Errorf("chA should not receive project B event, got: %s", e.ProjectID())
	default:
		// Expected - no more events
	}

	// Check channel B received only project B event
	select {
	case e := <-chB:
		if e.ProjectID() != "proj-b" {
			t.Errorf("chB received wrong project: %s", e.ProjectID())
		}
	default:
		t.Error("chB should have received an event")
	}

	// Check channel All received both events
	count := 0
	for i := 0; i < 2; i++ {
		select {
		case <-chAll:
			count++
		default:
		}
	}
	if count != 2 {
		t.Errorf("chAll should receive 2 events, got %d", count)
	}
}

func TestEventBus_SubscribeForProjectWithTypes(t *testing.T) {
	bus := New(10)
	defer bus.Close()

	// Subscribe to specific type AND project
	ch := bus.SubscribeForProject("proj-a", TypeWorkflowStarted)

	// Publish matching event
	event1 := NewWorkflowStartedEvent("wf-1", "proj-a", "Test")
	bus.Publish(event1)

	// Publish non-matching type (same project)
	event2 := NewWorkflowCompletedEvent("wf-1", "proj-a", 0)
	bus.Publish(event2)

	// Publish non-matching project (same type)
	event3 := NewWorkflowStartedEvent("wf-2", "proj-b", "Test")
	bus.Publish(event3)

	time.Sleep(10 * time.Millisecond)

	// Should only receive event1
	count := 0
	for {
		select {
		case e := <-ch:
			count++
			if e.ProjectID() != "proj-a" || e.EventType() != TypeWorkflowStarted {
				t.Errorf("received unexpected event: project=%s, type=%s",
					e.ProjectID(), e.EventType())
			}
		default:
			goto done
		}
	}
done:

	if count != 1 {
		t.Errorf("expected 1 event, got %d", count)
	}
}

func TestEventBus_ProjectFilteringConcurrent(t *testing.T) {
	bus := New(100)
	defer bus.Close()

	const numProjects = 5
	const eventsPerProject = 100

	// Subscribe to each project
	channels := make([]<-chan Event, numProjects)
	for i := 0; i < numProjects; i++ {
		channels[i] = bus.SubscribeForProject(fmt.Sprintf("proj-%d", i))
	}

	// Publish events concurrently
	var wg sync.WaitGroup
	for p := 0; p < numProjects; p++ {
		wg.Add(1)
		go func(projectNum int) {
			defer wg.Done()
			projectID := fmt.Sprintf("proj-%d", projectNum)
			for e := 0; e < eventsPerProject; e++ {
				event := NewWorkflowStartedEvent(
					fmt.Sprintf("wf-%d-%d", projectNum, e), projectID, "prompt")
				bus.Publish(event)
			}
		}(p)
	}

	wg.Wait()
	time.Sleep(50 * time.Millisecond)

	// Check each channel received correct events
	for i := 0; i < numProjects; i++ {
		count := 0
		expectedProject := fmt.Sprintf("proj-%d", i)
		for {
			select {
			case e := <-channels[i]:
				count++
				if e.ProjectID() != expectedProject {
					t.Errorf("channel %d received event from wrong project: %s",
						i, e.ProjectID())
				}
			default:
				goto nextChannel
			}
		}
	nextChannel:
		if count != eventsPerProject {
			t.Errorf("channel %d received %d events, expected %d",
				i, count, eventsPerProject)
		}
	}
}

func TestEventBus_EmptyProjectIDReceivesAll(t *testing.T) {
	bus := New(10)
	defer bus.Close()

	// Empty project ID = receive all
	ch := bus.SubscribeForProject("")

	// Publish events from different projects
	bus.Publish(NewWorkflowStartedEvent("wf-1", "proj-a", "prompt"))
	bus.Publish(NewWorkflowStartedEvent("wf-2", "proj-b", "prompt"))
	bus.Publish(NewWorkflowStartedEvent("wf-3", "", "prompt"))

	time.Sleep(10 * time.Millisecond)

	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:

	if count != 3 {
		t.Errorf("expected 3 events, got %d", count)
	}
}

func TestEventBus_ProjectIDMethod(t *testing.T) {
	be := NewBaseEvent(TypeWorkflowStarted, "wf-1", "proj-test")

	if be.ProjectID() != "proj-test" {
		t.Errorf("expected ProjectID 'proj-test', got '%s'", be.ProjectID())
	}

	// Test with empty project ID
	be2 := NewBaseEvent(TypeWorkflowStarted, "wf-2", "")
	if be2.ProjectID() != "" {
		t.Errorf("expected empty ProjectID, got '%s'", be2.ProjectID())
	}
}

func TestEventBus_SubscribeForProjectWithPriority(t *testing.T) {
	bus := New(10)
	defer bus.Close()

	// Subscribe to priority events for project A
	chA := bus.SubscribeForProjectWithPriority("proj-a")

	// Publish priority event for project A
	eventA := NewWorkflowFailedEvent("wf-1", "proj-a", "phase", nil)
	bus.PublishPriority(eventA)

	// Publish priority event for project B - should not be received by chA
	eventB := NewWorkflowFailedEvent("wf-2", "proj-b", "phase", nil)
	bus.PublishPriority(eventB)

	time.Sleep(10 * time.Millisecond)

	// Check channel A received only project A event
	count := 0
	for {
		select {
		case e := <-chA:
			count++
			if e.ProjectID() != "proj-a" {
				t.Errorf("chA received wrong project: %s", e.ProjectID())
			}
		default:
			goto done
		}
	}
done:

	if count != 1 {
		t.Errorf("expected 1 event, got %d", count)
	}
}

func TestEventBus_SubscribeOnClosedBus(t *testing.T) {
	bus := New(10)
	bus.Close()

	// Subscribe after close should return a closed channel
	ch := bus.SubscribeForProject("proj-a")

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed")
		}
	default:
		// Channel is closed, this is expected
	}
}

func TestEventBus_BaseEventLegacy(t *testing.T) {
	// Test backward compatibility function
	be := NewBaseEventLegacy(TypeWorkflowStarted, "wf-1")

	if be.WorkflowID() != "wf-1" {
		t.Errorf("expected WorkflowID 'wf-1', got '%s'", be.WorkflowID())
	}

	if be.ProjectID() != "" {
		t.Errorf("expected empty ProjectID for legacy event, got '%s'", be.ProjectID())
	}
}
