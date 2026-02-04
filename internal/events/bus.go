// Package events provides a centralized event bus for the workflow system.
// It implements pub/sub with backpressure control and priority channels.
package events

import (
	"sync"
	"sync/atomic"
	"time"
)

// Event is the base interface for all events.
type Event interface {
	EventType() string
	Timestamp() time.Time
	WorkflowID() string
	ProjectID() string // Returns the project ID for filtering
}

// BaseEvent provides common fields for all events.
type BaseEvent struct {
	Type     string    `json:"type"`
	Time     time.Time `json:"timestamp"`
	Workflow string    `json:"workflow_id"`
	Project  string    `json:"project_id"` // Project identifier for filtering
}

func (e BaseEvent) EventType() string    { return e.Type }
func (e BaseEvent) Timestamp() time.Time { return e.Time }
func (e BaseEvent) WorkflowID() string   { return e.Workflow }
func (e BaseEvent) ProjectID() string    { return e.Project }

// NewBaseEvent creates a new base event with project ID.
func NewBaseEvent(eventType, workflowID, projectID string) BaseEvent {
	return BaseEvent{
		Type:     eventType,
		Time:     time.Now(),
		Workflow: workflowID,
		Project:  projectID,
	}
}

// NewBaseEventLegacy creates a new base event without project ID.
// DEPRECATED: Use NewBaseEvent with projectID for proper project filtering.
func NewBaseEventLegacy(eventType, workflowID string) BaseEvent {
	return NewBaseEvent(eventType, workflowID, "")
}

// Subscriber represents an event subscription.
type Subscriber struct {
	ch        chan Event
	types     map[string]bool // Empty means all types
	projectID string          // Empty means no project filtering (receives all)
	priority  bool
}

// EventBus provides pub/sub with backpressure control.
type EventBus struct {
	mu           sync.RWMutex
	subscribers  []*Subscriber
	prioritySubs []*Subscriber
	bufferSize   int
	droppedCount int64
	closed       bool
}

// New creates a new EventBus with the specified buffer size.
func New(bufferSize int) *EventBus {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &EventBus{
		subscribers:  make([]*Subscriber, 0),
		prioritySubs: make([]*Subscriber, 0),
		bufferSize:   bufferSize,
	}
}

// Subscribe creates a subscription for specific event types.
// If no types are specified, subscribes to all events.
// Returns a channel that receives events from all projects.
func (eb *EventBus) Subscribe(types ...string) <-chan Event {
	return eb.SubscribeForProject("", types...)
}

// SubscribeForProject creates a subscription filtered to a specific project.
// If projectID is empty, all events are received (equivalent to Subscribe).
// types is an optional list of event types to filter by.
func (eb *EventBus) SubscribeForProject(projectID string, types ...string) <-chan Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.closed {
		ch := make(chan Event)
		close(ch)
		return ch
	}

	sub := &Subscriber{
		ch:        make(chan Event, eb.bufferSize),
		types:     make(map[string]bool),
		projectID: projectID,
		priority:  false,
	}
	for _, t := range types {
		sub.types[t] = true
	}
	eb.subscribers = append(eb.subscribers, sub)
	return sub.ch
}

// SubscribePriority creates a priority subscription that never drops events.
// Use for critical events like workflow_failed, workflow_completed.
// Receives events from all projects.
func (eb *EventBus) SubscribePriority() <-chan Event {
	return eb.SubscribeForProjectWithPriority("")
}

// SubscribeForProjectWithPriority creates a priority subscription filtered by project.
// If projectID is empty, all events are received.
func (eb *EventBus) SubscribeForProjectWithPriority(projectID string, types ...string) <-chan Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.closed {
		ch := make(chan Event)
		close(ch)
		return ch
	}

	sub := &Subscriber{
		ch:        make(chan Event, 50), // Smaller buffer, blocking send
		types:     make(map[string]bool),
		projectID: projectID,
		priority:  true,
	}
	for _, t := range types {
		sub.types[t] = true
	}
	eb.prioritySubs = append(eb.prioritySubs, sub)
	return sub.ch
}

// Unsubscribe removes a subscription.
func (eb *EventBus) Unsubscribe(ch <-chan Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.subscribers = removeSubscriber(eb.subscribers, ch)
	eb.prioritySubs = removeSubscriber(eb.prioritySubs, ch)
}

func removeSubscriber(subs []*Subscriber, ch <-chan Event) []*Subscriber {
	result := make([]*Subscriber, 0, len(subs))
	for _, sub := range subs {
		if sub.ch != ch {
			result = append(result, sub)
		} else {
			close(sub.ch)
		}
	}
	return result
}

// Publish sends an event to all matching subscribers.
// Non-priority subscribers may drop events if their buffer is full (ring buffer behavior).
// Events are filtered by project ID if the subscriber has a project filter set.
func (eb *EventBus) Publish(event Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if eb.closed {
		return
	}

	eventType := event.EventType()
	eventProject := event.ProjectID()

	// Send to regular subscribers with ring buffer behavior
	for _, sub := range eb.subscribers {
		if !eb.shouldDeliver(sub, eventType, eventProject) {
			continue
		}
		eb.deliverWithRingBuffer(sub, event)
	}
}

// shouldDeliver checks if an event should be delivered to a subscriber.
// Returns true if the event matches the subscriber's project and type filters.
func (eb *EventBus) shouldDeliver(sub *Subscriber, eventType, eventProject string) bool {
	// Check project filter: if subscriber has a project filter, event must match
	if sub.projectID != "" && eventProject != sub.projectID {
		return false
	}

	// Check type filter: if subscriber has type filters, event type must be in the set
	if len(sub.types) > 0 && !sub.types[eventType] {
		return false
	}

	return true
}

// deliverWithRingBuffer attempts to send an event to a subscriber using ring buffer behavior.
// If the channel is full, it drops the oldest event and tries again.
func (eb *EventBus) deliverWithRingBuffer(sub *Subscriber, event Event) {
	select {
	case sub.ch <- event:
		// Sent successfully
	default:
		// Buffer full, drop oldest and try again (ring buffer)
		select {
		case <-sub.ch: // Drop oldest
			atomic.AddInt64(&eb.droppedCount, 1)
		default:
		}
		select {
		case sub.ch <- event:
		default:
			atomic.AddInt64(&eb.droppedCount, 1)
		}
	}
}

// PublishPriority sends an event to priority subscribers with blocking behavior.
// Use for critical events that must never be dropped.
// Events are filtered by project ID if the subscriber has a project filter set.
func (eb *EventBus) PublishPriority(event Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if eb.closed {
		return
	}

	eventType := event.EventType()
	eventProject := event.ProjectID()

	// Also send to regular subscribers
	for _, sub := range eb.subscribers {
		if !eb.shouldDeliver(sub, eventType, eventProject) {
			continue
		}
		eb.deliverWithRingBuffer(sub, event)
	}

	// Send to priority subscribers (blocking) - only if they match the filter
	for _, sub := range eb.prioritySubs {
		if !eb.shouldDeliver(sub, eventType, eventProject) {
			continue
		}
		sub.ch <- event
	}
}

// DroppedCount returns the total number of dropped events.
func (eb *EventBus) DroppedCount() int64 {
	return atomic.LoadInt64(&eb.droppedCount)
}

// Close closes the event bus and all subscriber channels.
func (eb *EventBus) Close() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.closed {
		return
	}
	eb.closed = true

	for _, sub := range eb.subscribers {
		close(sub.ch)
	}
	for _, sub := range eb.prioritySubs {
		close(sub.ch)
	}
	eb.subscribers = nil
	eb.prioritySubs = nil
}
