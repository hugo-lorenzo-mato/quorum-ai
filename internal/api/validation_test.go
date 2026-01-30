package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertValidationErrors(t *testing.T) {
	errs := config.ValidationErrors{
		{Field: "workflow.max_retries", Value: 15, Message: "must be between 0 and 10"},
		{Field: "log.level", Value: "verbose", Message: "must be one of: debug, info, warn, error"},
	}

	response := convertValidationErrors(errs)

	assert.Equal(t, "Configuration validation failed", response.Message)
	assert.Len(t, response.Errors, 2)
	assert.Equal(t, "workflow.max_retries", response.Errors[0].Field)
	assert.Equal(t, ErrCodeInvalidRange, response.Errors[0].Code)
	assert.Equal(t, ErrCodeInvalidEnum, response.Errors[1].Code)
}

func TestInferErrorCode(t *testing.T) {
	tests := []struct {
		message string
		want    string
	}{
		{"path required", ErrCodeRequired},
		{"must be one of: debug, info", ErrCodeInvalidEnum},
		{"invalid duration format", ErrCodeInvalidDuration},
		{"must be between 0 and 10", ErrCodeInvalidRange},
		{"must be >= 1", ErrCodeInvalidRange},
		{"must be positive", ErrCodeInvalidRange},
		{"auto_pr requires auto_push to be enabled", ErrCodeDependencyChain},
		{"cannot be true when moderator.enabled is also true", ErrCodeMutualExclusion},
		{"specified agent must be enabled", ErrCodeAgentNotEnabled},
		{"unknown agent: foobar", ErrCodeUnknownAgent},
		{"unknown phase: badphase", ErrCodeUnknownPhase},
		{"some other error", "VALIDATION_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			got := inferErrorCode("", tt.message)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHandleValidateConfig_ValidConfig(t *testing.T) {
	stateManager := newMockStateManager()
	eventBus := events.New(100)
	server := NewServer(stateManager, eventBus)

	// Valid config update - includes required agents.default
	reqBody := `{
		"workflow": {"timeout": "2h"},
		"log": {"level": "debug"},
		"agents": {"default": "claude", "claude": {"enabled": true}},
		"phases": {"analyze": {"refiner": {"enabled": false}, "moderator": {"enabled": false}}}
	}`

	req := httptest.NewRequest("POST", "/api/v1/config/validate", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.handleValidateConfig(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response ValidationResult
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.Valid)
	assert.Empty(t, response.Errors)
}

func TestHandleValidateConfig_InvalidJSON(t *testing.T) {
	stateManager := newMockStateManager()
	eventBus := events.New(100)
	server := NewServer(stateManager, eventBus)

	req := httptest.NewRequest("POST", "/api/v1/config/validate", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.handleValidateConfig(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleValidateConfig_InvalidLogLevel(t *testing.T) {
	stateManager := newMockStateManager()
	eventBus := events.New(100)
	server := NewServer(stateManager, eventBus)

	// Invalid log level
	reqBody := `{
		"log": {"level": "verbose"}
	}`

	req := httptest.NewRequest("POST", "/api/v1/config/validate", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.handleValidateConfig(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response ValidationResult
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.False(t, response.Valid)
	assert.NotEmpty(t, response.Errors)

	// Should have log.level error
	var found bool
	for _, e := range response.Errors {
		if e.Field == "log.level" {
			found = true
			assert.Equal(t, ErrCodeInvalidEnum, e.Code)
			break
		}
	}
	assert.True(t, found, "should have log.level validation error")
}

func TestHandleUpdateConfig_ValidationErrors(t *testing.T) {
	stateManager := newMockStateManager()
	eventBus := events.New(100)
	server := NewServer(stateManager, eventBus)

	// Invalid log level should be rejected
	reqBody := `{
		"log": {"level": "verbose"}
	}`

	req := httptest.NewRequest("PATCH", "/api/v1/config", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.handleUpdateConfig(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	var response ValidationErrorResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Configuration validation failed", response.Message)
	assert.NotEmpty(t, response.Errors)
}

func TestHandleUpdateConfig_ValidUpdate(t *testing.T) {
	stateManager := newMockStateManager()
	eventBus := events.New(100)
	server := NewServer(stateManager, eventBus)

	// Valid update - includes required agents.default
	reqBody := `{
		"log": {"level": "debug"},
		"agents": {"default": "claude", "claude": {"enabled": true}},
		"phases": {"analyze": {"refiner": {"enabled": false}, "moderator": {"enabled": false}}}
	}`

	req := httptest.NewRequest("PATCH", "/api/v1/config", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.handleUpdateConfig(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestValidationErrorCodes(t *testing.T) {
	// Test that all error codes are defined correctly
	codes := []string{
		ErrCodeRequired,
		ErrCodeInvalidEnum,
		ErrCodeInvalidDuration,
		ErrCodeInvalidRange,
		ErrCodeInvalidPath,
		ErrCodeDependencyChain,
		ErrCodeMutualExclusion,
		ErrCodeAgentNotEnabled,
		ErrCodeUnknownAgent,
		ErrCodeUnknownPhase,
	}

	for _, code := range codes {
		assert.NotEmpty(t, code, "error code should not be empty")
	}
}

func TestValidateWorkflowConfig(t *testing.T) {
	// Setup agents config with enabled and disabled agents
	agents := config.AgentsConfig{
		Claude: config.AgentConfig{Enabled: true},
		Gemini: config.AgentConfig{Enabled: true},
		Codex:  config.AgentConfig{Enabled: false}, // Disabled
	}

	tests := []struct {
		name          string
		config        *WorkflowConfig
		expectError   bool
		errorField    string
		errorCode     string
		errorContains string
	}{
		{
			name:        "nil config is valid",
			config:      nil,
			expectError: false,
		},
		{
			name:        "empty config is valid",
			config:      &WorkflowConfig{},
			expectError: false,
		},
		{
			name: "multi_agent mode is valid",
			config: &WorkflowConfig{
				ExecutionMode: "multi_agent",
			},
			expectError: false,
		},
		{
			name: "single_agent with valid agent is valid",
			config: &WorkflowConfig{
				ExecutionMode:   "single_agent",
				SingleAgentName: "claude",
			},
			expectError: false,
		},
		{
			name: "single_agent with model override is valid",
			config: &WorkflowConfig{
				ExecutionMode:    "single_agent",
				SingleAgentName:  "gemini",
				SingleAgentModel: "gemini-pro",
			},
			expectError: false,
		},
		{
			name: "invalid execution_mode value",
			config: &WorkflowConfig{
				ExecutionMode: "invalid_mode",
			},
			expectError:   true,
			errorField:    "execution_mode",
			errorCode:     ErrCodeInvalidEnum,
			errorContains: "invalid value",
		},
		{
			name: "single_agent without agent name",
			config: &WorkflowConfig{
				ExecutionMode: "single_agent",
			},
			expectError:   true,
			errorField:    "single_agent_name",
			errorCode:     ErrCodeRequired,
			errorContains: "required",
		},
		{
			name: "single_agent with empty agent name",
			config: &WorkflowConfig{
				ExecutionMode:   "single_agent",
				SingleAgentName: "   ", // Whitespace only
			},
			expectError:   true,
			errorField:    "single_agent_name",
			errorCode:     ErrCodeRequired,
			errorContains: "required",
		},
		{
			name: "single_agent with non-existent agent",
			config: &WorkflowConfig{
				ExecutionMode:   "single_agent",
				SingleAgentName: "nonexistent",
			},
			expectError:   true,
			errorField:    "single_agent_name",
			errorCode:     ErrCodeUnknownAgent,
			errorContains: "not configured",
		},
		{
			name: "single_agent with disabled agent",
			config: &WorkflowConfig{
				ExecutionMode:   "single_agent",
				SingleAgentName: "codex", // Disabled in test config
			},
			expectError:   true,
			errorField:    "single_agent_name",
			errorCode:     ErrCodeAgentNotEnabled,
			errorContains: "disabled",
		},
		{
			name: "multi_agent ignores single_agent_name",
			config: &WorkflowConfig{
				ExecutionMode:   "multi_agent",
				SingleAgentName: "nonexistent", // Should be ignored
			},
			expectError: false,
		},
		{
			name: "empty execution_mode defaults to multi_agent behavior",
			config: &WorkflowConfig{
				ExecutionMode:   "",
				SingleAgentName: "nonexistent", // Should be ignored
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkflowConfig(tt.config, agents)

			if tt.expectError {
				require.NotNil(t, err, "expected validation error")
				assert.Equal(t, tt.errorField, err.Field)
				assert.Equal(t, tt.errorCode, err.Code)
				assert.Contains(t, err.Message, tt.errorContains)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestAgentsConfig_EnabledAgentNames(t *testing.T) {
	tests := []struct {
		name     string
		agents   config.AgentsConfig
		expected []string
	}{
		{
			name: "some enabled",
			agents: config.AgentsConfig{
				Claude: config.AgentConfig{Enabled: true},
				Gemini: config.AgentConfig{Enabled: false},
				Codex:  config.AgentConfig{Enabled: true},
			},
			expected: []string{"claude", "codex"},
		},
		{
			name: "none enabled",
			agents: config.AgentsConfig{
				Claude: config.AgentConfig{Enabled: false},
				Gemini: config.AgentConfig{Enabled: false},
				Codex:  config.AgentConfig{Enabled: false},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names := tt.agents.EnabledAgentNames()
			if len(tt.expected) == 0 {
				assert.Empty(t, names)
			} else {
				assert.ElementsMatch(t, tt.expected, names)
			}
		})
	}
}
