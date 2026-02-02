# Extend EventBus with Project Filtering

## Summary

Extend the EventBus to support project-filtered subscriptions, ensuring events are properly scoped to their originating project and preventing information leakage between projects.

## Context

The Quorum AI EventBus currently broadcasts all events to all subscribers without project filtering. In a multi-project scenario, this means:

- **Information Leakage**: Events from Project A are visible to clients viewing Project B
- **Performance Issues**: Unnecessary event traffic to disinterested clients
- **UX Complexity**: Frontend must filter events client-side

The solution adds server-side project filtering to the EventBus, enabling SSE connections to receive only events from their target project.

## Implementation Details

### Files to Modify

- `internal/events/bus.go` - Add ProjectID to Event interface and filtering
- `internal/events/workflow.go` - Add Project field to all workflow events
- `internal/events/task.go` - Add Project field to all task events (if exists)
- `internal/events/agent.go` - Add Project field to all agent events (if exists)

### Files to Create

- `internal/events/bus_test.go` - Tests for project filtering

### Key Changes

1. **Event Interface Extension**
   ```go
   type Event interface {
       EventType() string
       Timestamp() time.Time
       WorkflowID() string
       ProjectID() string  // NEW: For filtering
   }
   ```

2. **BaseEvent Enhancement**
   ```go
   type BaseEvent struct {
       Type     string    `json:"type"`
       Time     time.Time `json:"timestamp"`
       Workflow string    `json:"workflow_id"`
       Project  string    `json:"project_id"`  // NEW
   }
   ```

3. **New Subscription Methods**
   ```go
   // Subscribe to events from specific project
   func (eb *EventBus) SubscribeForProject(projectID string, types ...string) <-chan Event

   // Subscribe to events from specific project with priority
   func (eb *EventBus) SubscribeForProjectWithPriority(projectID string, types ...string) <-chan Event
   ```

4. **Publish Filtering**
   ```go
   // shouldDeliver checks if event matches subscriber filters
   func (eb *EventBus) shouldDeliver(sub *Subscriber, eventType, eventProject string) bool {
       // Check project filter
       if sub.projectID != "" && eventProject != sub.projectID {
           return false
       }
       // Check type filter
       if len(sub.types) > 0 && !sub.types[eventType] {
           return false
       }
       return true
   }
   ```

## Acceptance Criteria

- [ ] Event interface includes ProjectID() method
- [ ] BaseEvent has Project field and implements ProjectID()
- [ ] NewBaseEvent accepts projectID parameter
- [ ] SubscribeForProject method created
- [ ] SubscribeForProjectWithPriority method created
- [ ] Publish filters events by subscriber's projectID
- [ ] Empty projectID subscription receives all events (backward compatible)
- [ ] All event constructors accept projectID
- [ ] Existing Subscribe method continues to work
- [ ] Type filtering works in combination with project filtering
- [ ] No race conditions with concurrent filtering
- [ ] Unit tests cover filtering logic
- [ ] Dropped event counter tracks separately

## Testing Strategy

1. **Unit Tests**:
   - Project-filtered subscriptions receive only matching events
   - Empty projectID receives all events
   - Type filtering combined with project filtering
   - Multiple subscribers to different projects
   - Concurrent publish/subscribe operations

2. **Manual Verification**:
   - SSE connection to specific project receives correct events
   - Events from other projects don't appear

## Definition of Done

- All acceptance criteria met
- Unit test coverage >80%
- No race conditions or deadlocks
- Clear error handling for invalid events
