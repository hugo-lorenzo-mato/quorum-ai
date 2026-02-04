package events

// Event type constants for log events.
const (
	TypeLog = "log"
)

// LogEvent represents a log message.
type LogEvent struct {
	BaseEvent
	Level   string                 `json:"level"`
	Message string                 `json:"message"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
}

// NewLogEvent creates a new log event.
func NewLogEvent(workflowID, projectID, level, message string, fields map[string]interface{}) LogEvent {
	return LogEvent{
		BaseEvent: NewBaseEvent(TypeLog, workflowID, projectID),
		Level:     level,
		Message:   message,
		Fields:    fields,
	}
}
