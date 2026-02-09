package events

// Event type constants for config-related events.
const (
	TypeConfigLoaded = "config_loaded"
)

// ConfigLoadedEvent is emitted when an execution configuration is loaded for a workflow attempt.
// This helps the Web UI and operators understand which config file was used.
type ConfigLoadedEvent struct {
	BaseEvent
	ConfigPath    string `json:"config_path"`
	ConfigScope   string `json:"config_scope"` // "global" | "project"
	ConfigMode    string `json:"config_mode"`  // "inherit_global" | "custom"
	FileETag      string `json:"file_etag,omitempty"`
	EffectiveETag string `json:"effective_etag,omitempty"`
	ExecutionID   int    `json:"execution_id,omitempty"`  // Predicted execution_id for this attempt
	SnapshotPath  string `json:"snapshot_path,omitempty"` // Where the config snapshot was written
	Warning       string `json:"warning,omitempty"`       // Optional mismatch/diagnostic info
}

// NewConfigLoadedEvent creates a new config_loaded event.
func NewConfigLoadedEvent(workflowID, projectID, configPath, scope, mode, fileETag, effectiveETag string, executionID int, snapshotPath, warning string) ConfigLoadedEvent {
	return ConfigLoadedEvent{
		BaseEvent:     NewBaseEvent(TypeConfigLoaded, workflowID, projectID),
		ConfigPath:    configPath,
		ConfigScope:   scope,
		ConfigMode:    mode,
		FileETag:      fileETag,
		EffectiveETag: effectiveETag,
		ExecutionID:   executionID,
		SnapshotPath:  snapshotPath,
		Warning:       warning,
	}
}
