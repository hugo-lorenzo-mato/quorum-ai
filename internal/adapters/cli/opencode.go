package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// Profile represents the execution profile for OpenCode.
type Profile string

const (
	ProfileCoder     Profile = "coder"
	ProfileArchitect Profile = "architect"
)

// ProfileConfig defines model selection for a profile.
type ProfileConfig struct {
	Primary   string   // Primary model for this profile
	Fallbacks []string // Fallback models if primary unavailable
}

// OpenCodeAdapter implements Agent for OpenCode CLI with Ollama backend.
type OpenCodeAdapter struct {
	*BaseAdapter
	capabilities core.Capabilities
	ollamaURL    string
	ollamaKey    string
	profiles     map[Profile]ProfileConfig
	httpClient   *http.Client
}

// NewOpenCodeAdapter creates a new OpenCode adapter.
func NewOpenCodeAdapter(cfg AgentConfig) (core.Agent, error) {
	if cfg.Path == "" {
		cfg.Path = "opencode"
	}

	logger := logging.NewNop().With("adapter", "opencode")
	base := NewBaseAdapter(cfg, logger)

	// Default Ollama configuration from environment
	ollamaURL := os.Getenv("OPENAI_BASE_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434/v1"
	}

	ollamaKey := os.Getenv("OPENAI_API_KEY")
	if ollamaKey == "" {
		ollamaKey = "ollama"
	}

	adapter := &OpenCodeAdapter{
		BaseAdapter: base,
		capabilities: core.Capabilities{
			SupportsJSON:      true,
			SupportsStreaming: true,
			SupportsImages:    false, // Conservative default
			SupportsTools:     true,  // OpenCode has MCP support
			MaxContextTokens:  128000,
			MaxOutputTokens:   8192,
			SupportedModels:   core.GetSupportedModels(core.AgentOpenCode),
			DefaultModel:      core.GetDefaultModel(core.AgentOpenCode),
		},
		ollamaURL: ollamaURL,
		ollamaKey: ollamaKey,
		profiles: map[Profile]ProfileConfig{
			ProfileCoder: {
				Primary:   "qwen2.5-coder:32b",
				Fallbacks: []string{"qwen3-coder:30b", "codestral:22b"},
			},
			ProfileArchitect: {
				Primary:   "deepseek-r1:32b",
				Fallbacks: []string{"gpt-oss:20b"},
			},
		},
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	return adapter, nil
}

// Name returns the adapter name.
func (o *OpenCodeAdapter) Name() string {
	return "opencode"
}

// Capabilities returns adapter capabilities.
func (o *OpenCodeAdapter) Capabilities() core.Capabilities {
	return o.capabilities
}

// SetLogCallback sets a callback for real-time stderr streaming.
func (o *OpenCodeAdapter) SetLogCallback(cb LogCallback) {
	o.BaseAdapter.SetLogCallback(cb)
}

// SetEventHandler sets the handler for streaming events.
func (o *OpenCodeAdapter) SetEventHandler(handler core.AgentEventHandler) {
	o.BaseAdapter.SetEventHandler(handler)
}

// Ping checks if OpenCode CLI is available.
func (o *OpenCodeAdapter) Ping(ctx context.Context) error {
	if err := o.CheckAvailability(ctx); err != nil {
		return err
	}

	// Optionally verify Ollama connectivity (non-blocking)
	o.checkOllamaConnectivity(ctx)

	return nil
}

// checkOllamaConnectivity attempts to verify Ollama is reachable.
// This is advisory only - execution may still work without connectivity.
func (o *OpenCodeAdapter) checkOllamaConnectivity(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, "GET", o.ollamaURL+"/models", http.NoBody)
	if err != nil {
		o.logger.Debug("ollama check failed to create request", "error", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+o.ollamaKey)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		o.logger.Debug("ollama connectivity check failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		o.logger.Debug("ollama returned non-200", "status", resp.StatusCode)
	}
}

// Execute runs a prompt through OpenCode CLI.
func (o *OpenCodeAdapter) Execute(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
	// Determine model using profile detection
	model := o.resolveModel(opts)

	// Build command arguments
	args := o.buildArgs(opts, model)

	// Build the full prompt with history
	fullPrompt := o.buildPromptWithHistory(opts)

	// Set up Ollama environment
	o.setOllamaEnv()
	defer o.unsetOllamaEnv()

	// Execute command
	var result *CommandResult
	var err error
	if o.eventHandler != nil {
		result, err = o.ExecuteWithStreaming(ctx, "opencode", args, fullPrompt, opts.WorkDir, opts.Timeout)
	} else {
		result, err = o.ExecuteCommand(ctx, args, fullPrompt, opts.WorkDir, opts.Timeout)
	}
	if err != nil {
		return nil, err
	}

	return o.parseOutput(result, model)
}

// setOllamaEnv sets environment variables required for Ollama backend.
func (o *OpenCodeAdapter) setOllamaEnv() {
	if err := os.Setenv("OPENAI_BASE_URL", o.ollamaURL); err != nil {
		o.logger.Warn("failed to set OPENAI_BASE_URL", "error", err)
	}
	if err := os.Setenv("OPENAI_API_KEY", o.ollamaKey); err != nil {
		o.logger.Warn("failed to set OPENAI_API_KEY", "error", err)
	}
}

// unsetOllamaEnv cleans up Ollama environment variables.
// Note: We leave them set as they may be needed by subsequent calls.
func (o *OpenCodeAdapter) unsetOllamaEnv() {
	// Intentionally a no-op for simplicity
	// The env vars are set to the same values we read initially
}

// resolveModel determines the model to use for execution.
// Priority: explicit option > configured model > profile-based selection.
func (o *OpenCodeAdapter) resolveModel(opts core.ExecuteOptions) string {
	// Priority 1: Explicit model in options
	if opts.Model != "" {
		o.emitEvent(core.NewAgentEvent(
			core.AgentEventProgress,
			"opencode",
			fmt.Sprintf("Using explicit model: %s", opts.Model),
		))
		return opts.Model
	}

	// Priority 2: Configured model
	if o.config.Model != "" {
		o.emitEvent(core.NewAgentEvent(
			core.AgentEventProgress,
			"opencode",
			fmt.Sprintf("Using configured model: %s", o.config.Model),
		))
		return o.config.Model
	}

	// Priority 3: Profile-based selection
	profile := o.detectProfile(opts.Prompt)
	model := o.selectModelForProfile(profile)

	o.emitEvent(core.NewAgentEvent(
		core.AgentEventProgress,
		"opencode",
		fmt.Sprintf("Profile detected: %s, selected model: %s", profile, model),
	))

	return model
}

// detectProfile analyzes the prompt to determine the appropriate profile.
func (o *OpenCodeAdapter) detectProfile(prompt string) Profile {
	promptLower := " " + strings.ToLower(prompt) + " "
	// Replace punctuation with spaces to help word boundary detection
	for _, p := range []string{".", ",", "!", "?", ";", ":", "(", ")", "[", "]", "{", "}"} {
		promptLower = strings.ReplaceAll(promptLower, p, " ")
	}

	coderKeywords := []string{
		"create", "implement", "fix", "debug", "refactor",
		"code", "function", "script", "class", "method",
		"write", "add", "modify", "change", "update",
		"bug", "error", "test", "unittest",
	}

	architectKeywords := []string{
		"analyze", "plan", "design", "audit", "review",
		"strategy", "architecture", "compare", "evaluate",
		"assess", "examine", "investigate", "study",
		"pros", "cons", "tradeoff", "decision",
	}

	coderScore := 0
	for _, kw := range coderKeywords {
		if strings.Contains(promptLower, " "+kw+" ") {
			coderScore++
		}
	}

	architectScore := 0
	for _, kw := range architectKeywords {
		if strings.Contains(promptLower, " "+kw+" ") {
			architectScore++
		}
	}

	if architectScore > coderScore {
		return ProfileArchitect
	}
	return ProfileCoder // Default to coder
}

// selectModelForProfile returns the model to use for a given profile.
func (o *OpenCodeAdapter) selectModelForProfile(profile Profile) string {
	cfg, ok := o.profiles[profile]
	if !ok {
		return o.capabilities.DefaultModel
	}

	// Try primary model first
	if o.isModelAvailable(cfg.Primary) {
		return cfg.Primary
	}

	// Try fallbacks
	for _, fallback := range cfg.Fallbacks {
		if o.isModelAvailable(fallback) {
			o.emitEvent(core.NewAgentEvent(
				core.AgentEventProgress,
				"opencode",
				fmt.Sprintf("Primary model %s unavailable, using fallback: %s", cfg.Primary, fallback),
			))
			return fallback
		}
	}

	// Return primary anyway - Ollama may auto-download
	return cfg.Primary
}

// isModelAvailable checks if a model is available in Ollama.
func (o *OpenCodeAdapter) isModelAvailable(model string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", o.ollamaURL+"/models", http.NoBody)
	if err != nil {
		return true // Assume available if we can't check
	}
	req.Header.Set("Authorization", "Bearer "+o.ollamaKey)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return true // Assume available if we can't check
	}
	defer resp.Body.Close()

	// Parse response to check for model
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return true
	}

	return strings.Contains(string(body), model)
}

// buildArgs constructs CLI arguments for opencode run.
func (o *OpenCodeAdapter) buildArgs(_ core.ExecuteOptions, model string) []string {
	args := []string{"run"}

	// Add model flag
	if model != "" {
		args = append(args, "--model", model)
	}

	return args
}

// buildPromptWithHistory constructs a prompt including conversation history.
func (o *OpenCodeAdapter) buildPromptWithHistory(opts core.ExecuteOptions) string {
	// If no messages, just return the prompt
	if len(opts.Messages) == 0 {
		if opts.SystemPrompt != "" {
			return fmt.Sprintf("System: %s\n\n%s", opts.SystemPrompt, opts.Prompt)
		}
		return opts.Prompt
	}

	// Build conversation context from Messages
	var sb strings.Builder

	// Add system prompt if present
	if opts.SystemPrompt != "" {
		sb.WriteString("<system>\n")
		sb.WriteString(opts.SystemPrompt)
		sb.WriteString("\n</system>\n\n")
	}

	sb.WriteString("<conversation_history>\n")
	for _, msg := range opts.Messages {
		switch msg.Role {
		case "user":
			sb.WriteString("<user>\n")
			sb.WriteString(msg.Content)
			sb.WriteString("\n</user>\n")
		case "assistant":
			sb.WriteString("<assistant>\n")
			sb.WriteString(msg.Content)
			sb.WriteString("\n</assistant>\n")
		}
	}
	sb.WriteString("</conversation_history>\n\n")

	sb.WriteString("<current_message>\n")
	sb.WriteString(opts.Prompt)
	sb.WriteString("\n</current_message>")

	return sb.String()
}

// parseOutput parses OpenCode CLI output.
func (o *OpenCodeAdapter) parseOutput(result *CommandResult, model string) (*core.ExecuteResult, error) {
	if result.ExitCode != 0 {
		return nil, o.classifyError(result)
	}

	output := strings.TrimSpace(result.Stdout)

	// Try to parse as JSON first
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err == nil {
		// Extract content from JSON structure if present
		if content, ok := parsed["content"].(string); ok {
			output = content
		}
	}

	execResult := &core.ExecuteResult{
		Output:   output,
		Parsed:   parsed,
		Duration: result.Duration,
		Model:    model, // CRITICAL: Set model for tracking
	}

	// Estimate tokens if not provided
	execResult.TokensOut = o.TokenEstimate(output)
	execResult.TokensIn = execResult.TokensOut / 3 // Heuristic: input typically smaller

	return execResult, nil
}

// =============================================================================
// Stream Parser for OpenCode
// =============================================================================

// OpenCodeStreamParser parses OpenCode CLI output into agent events.
type OpenCodeStreamParser struct{}

// AgentName returns the name of the agent this parser handles.
func (p *OpenCodeStreamParser) AgentName() string {
	return "opencode"
}

// ParseLine processes a single line of output and returns any events.
func (p *OpenCodeStreamParser) ParseLine(line string) []core.AgentEvent {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	var jsonData map[string]any
	if err := json.Unmarshal([]byte(line), &jsonData); err == nil {
		events := p.parseJSONLine(jsonData)
		if len(events) > 0 {
			return events
		}
		return nil
	}

	return p.parsePlainLine(line)
}

// parseJSONLine handles JSON-formatted output from OpenCode.
func (p *OpenCodeStreamParser) parseJSONLine(data map[string]any) []core.AgentEvent {
	var events []core.AgentEvent

	if content, ok := data["content"].(string); ok && content != "" {
		events = append(events, core.NewAgentEvent(
			core.AgentEventChunk,
			"opencode",
			content,
		))
	}

	if thinking, ok := data["thinking"].(string); ok && thinking != "" {
		events = append(events, core.NewAgentEvent(
			core.AgentEventThinking,
			"opencode",
			thinking,
		))
	}

	if tool, ok := data["tool"].(string); ok && tool != "" {
		events = append(events, core.NewAgentEvent(
			core.AgentEventToolUse,
			"opencode",
			"Running: "+tool,
		).WithData(map[string]any{
			"tool": tool,
			"args": data["args"],
		}))
	}

	if errMsg, ok := data["error"].(string); ok && errMsg != "" {
		events = append(events, core.NewAgentEvent(
			core.AgentEventError,
			"opencode",
			errMsg,
		))
	}

	if tokens, ok := data["tokens"].(map[string]any); ok {
		events = append(events, core.NewAgentEvent(
			core.AgentEventProgress,
			"opencode",
			"Token usage",
		).WithData(tokens))
	}

	return events
}

// parsePlainLine handles plain text output from OpenCode.
func (p *OpenCodeStreamParser) parsePlainLine(line string) []core.AgentEvent {
	lineLower := strings.ToLower(line)

	if strings.HasPrefix(lineLower, "thinking") || strings.HasPrefix(lineLower, "analyzing") {
		return []core.AgentEvent{
			core.NewAgentEvent(core.AgentEventThinking, "opencode", line),
		}
	}

	if strings.Contains(lineLower, "running") || strings.Contains(lineLower, "executing") {
		return []core.AgentEvent{
			core.NewAgentEvent(core.AgentEventToolUse, "opencode", line),
		}
	}

	if strings.HasPrefix(lineLower, "error:") || strings.HasPrefix(lineLower, "failed:") {
		return []core.AgentEvent{
			core.NewAgentEvent(core.AgentEventError, "opencode", line),
		}
	}

	return []core.AgentEvent{
		core.NewAgentEvent(core.AgentEventChunk, "opencode", line),
	}
}

// Ensure OpenCodeAdapter implements core.Agent and core.StreamingCapable
var _ core.Agent = (*OpenCodeAdapter)(nil)
var _ core.StreamingCapable = (*OpenCodeAdapter)(nil)
