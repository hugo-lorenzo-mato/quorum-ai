package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// GetAgentColor / GetAgentBorderColor
// ---------------------------------------------------------------------------

func TestGetAgentColor_KnownAgents(t *testing.T) {
	known := []string{"claude", "gemini", "codex", "copilot", "opencode", "llama", "mistral", "gpt"}
	for _, name := range known {
		c := GetAgentColor(name)
		if c == lipgloss.Color("#71717a") {
			t.Errorf("GetAgentColor(%q) should return agent color, not default", name)
		}
	}
}

func TestGetAgentColor_CaseInsensitive(t *testing.T) {
	if GetAgentColor("Claude") != GetAgentColor("claude") {
		t.Error("GetAgentColor should be case-insensitive")
	}
	if GetAgentColor("GEMINI") != GetAgentColor("gemini") {
		t.Error("GetAgentColor should be case-insensitive")
	}
}

func TestGetAgentColor_Unknown(t *testing.T) {
	c := GetAgentColor("unknownagent")
	if c != lipgloss.Color("#71717a") {
		t.Errorf("GetAgentColor for unknown agent should return default gray, got %v", c)
	}
}

func TestGetAgentBorderColor_KnownAgents(t *testing.T) {
	known := []string{"claude", "gemini", "codex", "copilot", "opencode", "llama", "mistral", "gpt"}
	for _, name := range known {
		c := GetAgentBorderColor(name)
		if c == lipgloss.Color("#52525b") {
			t.Errorf("GetAgentBorderColor(%q) should return agent border color, not default", name)
		}
	}
}

func TestGetAgentBorderColor_Unknown(t *testing.T) {
	c := GetAgentBorderColor("unknownagent")
	if c != lipgloss.Color("#52525b") {
		t.Errorf("GetAgentBorderColor for unknown agent should return muted gray, got %v", c)
	}
}

func TestGetAgentBorderColor_CaseInsensitive(t *testing.T) {
	if GetAgentBorderColor("Claude") != GetAgentBorderColor("claude") {
		t.Error("GetAgentBorderColor should be case-insensitive")
	}
}

// ---------------------------------------------------------------------------
// ApplyDarkThemeAgentBorders / ApplyLightThemeAgentBorders
// ---------------------------------------------------------------------------

func TestApplyDarkThemeAgentBorders(t *testing.T) {
	// Switch to light first, then back to dark
	ApplyLightThemeAgentBorders()
	lightColor := GetAgentBorderColor("claude")

	ApplyDarkThemeAgentBorders()
	darkColor := GetAgentBorderColor("claude")

	if lightColor == darkColor {
		t.Error("Dark and light border colors should differ for claude")
	}

	// Restore dark as default
	ApplyDarkThemeAgentBorders()
}

func TestApplyLightThemeAgentBorders(t *testing.T) {
	ApplyLightThemeAgentBorders()
	defer ApplyDarkThemeAgentBorders()

	c := GetAgentBorderColor("claude")
	if c == lipgloss.Color("#6b5a86") { // dark theme color
		t.Error("After ApplyLightThemeAgentBorders, should use light border color")
	}
}

// ---------------------------------------------------------------------------
// RenderAgentsCompact
// ---------------------------------------------------------------------------

func TestRenderAgentsCompact_Empty(t *testing.T) {
	result := RenderAgentsCompact(nil)
	if !strings.Contains(result, "No agents configured") {
		t.Error("Empty agents should show 'No agents configured'")
	}
}

func TestRenderAgentsCompact_AllStatuses(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Color: GetAgentColor("claude"), Status: AgentStatusDisabled},
		{Name: "gemini", Color: GetAgentColor("gemini"), Status: AgentStatusIdle},
		{Name: "codex", Color: GetAgentColor("codex"), Status: AgentStatusRunning},
		{Name: "copilot", Color: GetAgentColor("copilot"), Status: AgentStatusDone, TokensIn: 100, TokensOut: 200},
		{Name: "opencode", Color: GetAgentColor("opencode"), Status: AgentStatusError},
	}

	result := RenderAgentsCompact(agents)

	// Must contain all agent names
	for _, name := range []string{"claude", "gemini", "codex", "copilot", "opencode"} {
		if !strings.Contains(result, name) {
			t.Errorf("RenderAgentsCompact should contain %q", name)
		}
	}
	// Done agent with tokens should show token count
	if !strings.Contains(result, "300") {
		t.Error("Done agent should show total token count 300")
	}
}

func TestRenderAgentsCompact_DoneNoTokens(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Color: GetAgentColor("claude"), Status: AgentStatusDone, TokensIn: 0, TokensOut: 0},
	}
	result := RenderAgentsCompact(agents)
	if !strings.Contains(result, "claude") {
		t.Error("Should contain agent name even with 0 tokens")
	}
}

// ---------------------------------------------------------------------------
// RenderPipeline
// ---------------------------------------------------------------------------

func TestRenderPipeline_Empty(t *testing.T) {
	result := RenderPipeline(nil)
	if !strings.Contains(result, "Pipeline:") {
		t.Error("Should contain 'Pipeline:' header")
	}
}

func TestRenderPipeline_AllDisabled(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Status: AgentStatusDisabled},
	}
	result := RenderPipeline(agents)
	if !strings.Contains(result, "No active agents") {
		t.Error("All disabled agents should show 'No active agents'")
	}
}

func TestRenderPipeline_MixedStatuses(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Color: GetAgentColor("claude"), Status: AgentStatusDone},
		{Name: "gemini", Color: GetAgentColor("gemini"), Status: AgentStatusRunning},
		{Name: "codex", Color: GetAgentColor("codex"), Status: AgentStatusIdle},
		{Name: "disabled", Status: AgentStatusDisabled},
	}
	result := RenderPipeline(agents)
	if !strings.Contains(result, "Pipeline:") {
		t.Error("Should contain 'Pipeline:' header")
	}
	// Should show percentage
	if !strings.Contains(result, "%") {
		t.Error("Pipeline should show percentage")
	}
}

func TestRenderPipeline_AllDone(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Color: GetAgentColor("claude"), Status: AgentStatusDone},
		{Name: "gemini", Color: GetAgentColor("gemini"), Status: AgentStatusDone},
	}
	result := RenderPipeline(agents)
	if !strings.Contains(result, "100%") {
		t.Error("All done should show 100%")
	}
}

// ---------------------------------------------------------------------------
// RenderAgentResults
// ---------------------------------------------------------------------------

func TestRenderAgentResults_NoResults(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Status: AgentStatusRunning},
	}
	result := RenderAgentResults(agents, 80)
	if result != "" {
		t.Error("No done agents should return empty string")
	}
}

func TestRenderAgentResults_WithOutput(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Color: GetAgentColor("claude"), Status: AgentStatusDone,
			Output: "Hello world", Time: "1.2s", TokensIn: 100, TokensOut: 200},
	}
	result := RenderAgentResults(agents, 80)
	if !strings.Contains(result, "claude") {
		t.Error("Should contain agent name")
	}
	if !strings.Contains(result, "Hello world") {
		t.Error("Should contain output")
	}
	if !strings.Contains(result, "1.2s") {
		t.Error("Should contain time")
	}
	if !strings.Contains(result, "300 tok") {
		t.Error("Should contain token count")
	}
}

func TestRenderAgentResults_DoneEmptyOutput(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Status: AgentStatusDone, Output: ""},
	}
	result := RenderAgentResults(agents, 80)
	if result != "" {
		t.Error("Done but empty output should return empty string")
	}
}

func TestRenderAgentResults_NarrowWidth(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Color: GetAgentColor("claude"), Status: AgentStatusDone,
			Output: "Hello"},
	}
	// Width < 52 forces outputWidth to min 40
	result := RenderAgentResults(agents, 30)
	if !strings.Contains(result, "Hello") {
		t.Error("Narrow width should still show output")
	}
}

func TestRenderAgentResults_NoTime(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Color: GetAgentColor("claude"), Status: AgentStatusDone,
			Output: "result", Time: "", TokensIn: 0, TokensOut: 0},
	}
	result := RenderAgentResults(agents, 80)
	if !strings.Contains(result, "claude") {
		t.Error("Should contain agent name even with no time")
	}
}

// ---------------------------------------------------------------------------
// RenderWorkflowLog
// ---------------------------------------------------------------------------

func TestRenderWorkflowLog_AllStatuses(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Status: AgentStatusDone, Time: "1.2s"},
		{Name: "gemini", Status: AgentStatusRunning},
		{Name: "codex", Status: AgentStatusError, Error: "timeout"},
		{Name: "copilot", Status: AgentStatusIdle}, // should be skipped
	}
	result := RenderWorkflowLog(agents, 80)
	if !strings.Contains(result, "Log:") {
		t.Error("Should contain 'Log:' header")
	}
	if !strings.Contains(result, "completado") {
		t.Error("Done agent should show 'completado'")
	}
	if !strings.Contains(result, "procesando") {
		t.Error("Running agent should show 'procesando'")
	}
	if !strings.Contains(result, "timeout") {
		t.Error("Error agent should show error message")
	}
}

func TestRenderWorkflowLog_ErrorNoMessage(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Status: AgentStatusError, Error: ""},
	}
	result := RenderWorkflowLog(agents, 80)
	if !strings.Contains(result, "error") {
		t.Error("Error agent with no message should show 'error'")
	}
}

func TestRenderWorkflowLog_NarrowWidth(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Status: AgentStatusDone},
	}
	// logWidth < 30 should be clamped to 30
	result := RenderWorkflowLog(agents, 20)
	if !strings.Contains(result, "Log:") {
		t.Error("Narrow width should still render")
	}
}

func TestRenderWorkflowLog_DoneNoTime(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Status: AgentStatusDone, Time: ""},
	}
	result := RenderWorkflowLog(agents, 80)
	if !strings.Contains(result, "completado") {
		t.Error("Done agent should show 'completado' even without time")
	}
}

// ---------------------------------------------------------------------------
// GetStats
// ---------------------------------------------------------------------------

func TestGetStats_Empty(t *testing.T) {
	active, total, tokens, running := GetStats(nil)
	if active != 0 || total != 0 || tokens != 0 || running != "" {
		t.Error("Empty agents should return all zeros")
	}
}

func TestGetStats_MixedStatuses(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Status: AgentStatusRunning, TokensIn: 100, TokensOut: 50},
		{Name: "gemini", Status: AgentStatusDone, TokensIn: 200, TokensOut: 100},
		{Name: "codex", Status: AgentStatusDisabled},
	}
	active, total, tokens, running := GetStats(agents)
	if total != 3 {
		t.Errorf("Expected total=3, got %d", total)
	}
	if active != 2 { // running + done (not disabled)
		t.Errorf("Expected active=2, got %d", active)
	}
	if tokens != 450 { // 100+50+200+100
		t.Errorf("Expected tokens=450, got %d", tokens)
	}
	if running != "claude" {
		t.Errorf("Expected running='claude', got %q", running)
	}
}

func TestGetStats_NoRunning(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Status: AgentStatusDone, TokensIn: 100},
	}
	_, _, _, running := GetStats(agents)
	if running != "" {
		t.Errorf("Expected empty running agent, got %q", running)
	}
}

func TestGetStats_MultipleRunning_ReturnsFirst(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Status: AgentStatusRunning},
		{Name: "gemini", Status: AgentStatusRunning},
	}
	_, _, _, running := GetStats(agents)
	if running != "claude" {
		t.Errorf("Expected first running agent 'claude', got %q", running)
	}
}

// ---------------------------------------------------------------------------
// truncateToWidth
// ---------------------------------------------------------------------------

func TestTruncateToWidth_Short(t *testing.T) {
	result := truncateToWidth("abc", 10)
	if result != "abc" {
		t.Errorf("Short string should not be truncated, got %q", result)
	}
}

func TestTruncateToWidth_Exact(t *testing.T) {
	result := truncateToWidth("abcde", 5)
	if result != "abcde" {
		t.Errorf("Exact width should not truncate, got %q", result)
	}
}

func TestTruncateToWidth_NeedsTruncation(t *testing.T) {
	result := truncateToWidth("hello world this is long", 10)
	if !strings.HasSuffix(result, "...") {
		t.Error("Truncated string should end with ...")
	}
	if lipgloss.Width(result) > 10 {
		t.Errorf("Truncated string should be <= 10 wide, got %d", lipgloss.Width(result))
	}
}

func TestTruncateToWidth_VerySmall(t *testing.T) {
	result := truncateToWidth("hello", 3)
	if result != "..." {
		t.Errorf("maxWidth<=3 should return '...', got %q", result)
	}
}

func TestTruncateToWidth_Zero(t *testing.T) {
	result := truncateToWidth("hello", 0)
	if result != "..." {
		t.Errorf("maxWidth=0 should return '...', got %q", result)
	}
}

// ---------------------------------------------------------------------------
// formatElapsed (additional coverage beyond existing tests)
// ---------------------------------------------------------------------------

func TestFormatElapsed_WithTimeout_Minutes(t *testing.T) {
	start := time.Now().Add(-30 * time.Second)
	result := formatElapsed(start, 5*time.Minute)
	if !strings.Contains(result, "/5m") {
		t.Errorf("Should contain /5m for minute timeout, got %q", result)
	}
}

func TestFormatElapsed_WithTimeout_Hours(t *testing.T) {
	start := time.Now().Add(-30 * time.Second)
	result := formatElapsed(start, 2*time.Hour)
	if !strings.Contains(result, "/2h") {
		t.Errorf("Should contain /2h for hour timeout, got %q", result)
	}
}

func TestFormatElapsed_WithTimeout_Seconds(t *testing.T) {
	start := time.Now().Add(-5 * time.Second)
	result := formatElapsed(start, 30*time.Second)
	if !strings.Contains(result, "/30s") {
		t.Errorf("Should contain /30s for seconds timeout, got %q", result)
	}
}

func TestFormatElapsed_NoTimeout(t *testing.T) {
	start := time.Now().Add(-5 * time.Second)
	result := formatElapsed(start, 0)
	if strings.Contains(result, "/") {
		t.Errorf("No timeout should not contain '/', got %q", result)
	}
}

func TestFormatElapsed_LongRunning(t *testing.T) {
	start := time.Now().Add(-3 * time.Minute)
	result := formatElapsed(start, 0)
	if !strings.Contains(result, "m") {
		t.Errorf("3min run should contain 'm', got %q", result)
	}
}

func TestFormatElapsed_RightAligned(t *testing.T) {
	start := time.Now().Add(-5 * time.Second)
	result := formatElapsed(start, 0)
	// Should be right-aligned to 12 chars
	if len(result) < 12 {
		t.Errorf("Expected at least 12 chars for right-alignment, got %d: %q", len(result), result)
	}
}

// ---------------------------------------------------------------------------
// RenderAgentProgressBars
// ---------------------------------------------------------------------------

func TestRenderAgentProgressBars_Empty(t *testing.T) {
	result := RenderAgentProgressBars(nil, 80)
	if !strings.Contains(result, "No agents configured") {
		t.Error("Empty agents should show 'No agents configured'")
	}
}

func TestRenderAgentProgressBars_AllStatuses(t *testing.T) {
	now := time.Now()
	agents := []*AgentInfo{
		{Name: "claude", Color: GetAgentColor("claude"), Status: AgentStatusDisabled},
		{Name: "gemini", Color: GetAgentColor("gemini"), Status: AgentStatusIdle},
		{Name: "codex", Color: GetAgentColor("codex"), Status: AgentStatusRunning,
			StartedAt: now.Add(-10 * time.Second), CurrentActivity: "read_file config.go",
			ActivityIcon: "icon", MaxTimeout: 5 * time.Minute},
		{Name: "copilot", Color: GetAgentColor("copilot"), Status: AgentStatusDone,
			TokensIn: 100, TokensOut: 200, Time: "1.5s"},
		{Name: "opencode", Color: GetAgentColor("opencode"), Status: AgentStatusError,
			Error: "timeout occurred"},
	}

	result := RenderAgentProgressBars(agents, 120)
	lines := strings.Split(result, "\n")
	if len(lines) != 5 {
		t.Errorf("Expected 5 lines (one per agent), got %d", len(lines))
	}

	// Check agent names are present
	for _, name := range []string{"claude", "gemini", "codex", "copilot", "opencode"} {
		if !strings.Contains(result, name) {
			t.Errorf("Should contain agent name %q", name)
		}
	}

	// Check status indicators
	if !strings.Contains(result, "disabled") {
		t.Error("Disabled agent should show 'disabled'")
	}
	if !strings.Contains(result, "idle") {
		t.Error("Idle agent should show 'idle'")
	}
	if !strings.Contains(result, "read_file config.go") {
		t.Error("Running agent should show current activity")
	}
	if !strings.Contains(result, "done") {
		t.Error("Done agent should show 'done'")
	}
}

func TestRenderAgentProgressBars_RunningNoActivity(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Color: GetAgentColor("claude"), Status: AgentStatusRunning,
			StartedAt: time.Now().Add(-5 * time.Second)},
	}
	result := RenderAgentProgressBars(agents, 80)
	if !strings.Contains(result, "processing...") {
		t.Error("Running agent with no activity should show 'processing...'")
	}
}

func TestRenderAgentProgressBars_RunningWithPhase(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Color: GetAgentColor("claude"), Status: AgentStatusRunning,
			StartedAt: time.Now().Add(-5 * time.Second), Phase: "analyze",
			CurrentActivity: "thinking..."},
	}
	result := RenderAgentProgressBars(agents, 120)
	if !strings.Contains(result, "[analyze]") {
		t.Error("Running agent with phase should show '[analyze]'")
	}
}

func TestRenderAgentProgressBars_DoneNoTokens(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Color: GetAgentColor("claude"), Status: AgentStatusDone},
	}
	result := RenderAgentProgressBars(agents, 80)
	if !strings.Contains(result, "done") {
		t.Error("Done agent should show 'done'")
	}
}

func TestRenderAgentProgressBars_ErrorNoMessage(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Color: GetAgentColor("claude"), Status: AgentStatusError},
	}
	result := RenderAgentProgressBars(agents, 80)
	if !strings.Contains(result, "failed") {
		t.Error("Error agent with no message should show 'failed'")
	}
}

func TestRenderAgentProgressBars_NarrowWidth(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Color: GetAgentColor("claude"), Status: AgentStatusRunning,
			StartedAt: time.Now().Add(-5 * time.Second), CurrentActivity: "working"},
	}
	// Very narrow width
	result := RenderAgentProgressBars(agents, 30)
	if !strings.Contains(result, "claude") {
		t.Error("Should still contain agent name at narrow width")
	}
}

func TestRenderAgentProgressBars_DoneWithTime(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude", Color: GetAgentColor("claude"), Status: AgentStatusDone,
			Time: "2.5s"},
	}
	result := RenderAgentProgressBars(agents, 80)
	if !strings.Contains(result, "2.5s") {
		t.Error("Done agent should show elapsed time")
	}
}

// ---------------------------------------------------------------------------
// UpdateAgentActivity
// ---------------------------------------------------------------------------

func TestUpdateAgentActivity_Found(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "claude"},
		{Name: "gemini"},
	}
	ok := UpdateAgentActivity(agents, "claude", "icon", "reading file")
	if !ok {
		t.Error("Should return true when agent found")
	}
	if agents[0].ActivityIcon != "icon" {
		t.Errorf("Expected icon='icon', got %q", agents[0].ActivityIcon)
	}
	if agents[0].CurrentActivity != "reading file" {
		t.Errorf("Expected activity='reading file', got %q", agents[0].CurrentActivity)
	}
}

func TestUpdateAgentActivity_NotFound(t *testing.T) {
	agents := []*AgentInfo{{Name: "claude"}}
	ok := UpdateAgentActivity(agents, "nonexistent", "icon", "act")
	if ok {
		t.Error("Should return false when agent not found")
	}
}

func TestUpdateAgentActivity_CaseInsensitive(t *testing.T) {
	agents := []*AgentInfo{{Name: "Claude"}}
	ok := UpdateAgentActivity(agents, "claude", "icon", "act")
	if !ok {
		t.Error("Should find agent case-insensitively")
	}
}

// ---------------------------------------------------------------------------
// StartAgent
// ---------------------------------------------------------------------------

func TestStartAgent_BasicStart(t *testing.T) {
	agents := []*AgentInfo{{Name: "claude", Status: AgentStatusIdle}}
	ok := StartAgent(agents, "claude", "analyze", 5*time.Minute, "opus")
	if !ok {
		t.Error("Should return true")
	}
	a := agents[0]
	if a.Status != AgentStatusRunning {
		t.Error("Status should be Running")
	}
	if a.Phase != "analyze" {
		t.Errorf("Phase should be 'analyze', got %q", a.Phase)
	}
	if a.MaxTimeout != 5*time.Minute {
		t.Errorf("MaxTimeout should be 5m, got %v", a.MaxTimeout)
	}
	if a.Model != "opus" {
		t.Errorf("Model should be 'opus', got %q", a.Model)
	}
	if a.StartedAt.IsZero() {
		t.Error("StartedAt should be set")
	}
}

func TestStartAgent_NotFound(t *testing.T) {
	agents := []*AgentInfo{{Name: "claude"}}
	ok := StartAgent(agents, "nonexistent", "", 0, "")
	if ok {
		t.Error("Should return false when agent not found")
	}
}

func TestStartAgent_PreserveTimeout(t *testing.T) {
	// Agent already running with a timeout; calling Start with 0 timeout should preserve existing
	agents := []*AgentInfo{{
		Name:       "claude",
		Status:     AgentStatusRunning,
		MaxTimeout: 5 * time.Minute,
		StartedAt:  time.Now().Add(-10 * time.Second),
		Phase:      "analyze",
	}}

	ok := StartAgent(agents, "claude", "analyze", 0, "") // same phase, 0 timeout
	if !ok {
		t.Error("Should return true")
	}
	if agents[0].MaxTimeout != 5*time.Minute {
		t.Errorf("Should preserve timeout, got %v", agents[0].MaxTimeout)
	}
}

func TestStartAgent_PhaseChange_ResetsStartTime(t *testing.T) {
	oldStart := time.Now().Add(-1 * time.Minute)
	agents := []*AgentInfo{{
		Name:       "claude",
		Status:     AgentStatusRunning,
		MaxTimeout: 5 * time.Minute,
		StartedAt:  oldStart,
		Phase:      "analyze",
	}}

	ok := StartAgent(agents, "claude", "critique", 0, "")
	if !ok {
		t.Error("Should return true")
	}
	if agents[0].Phase != "critique" {
		t.Errorf("Phase should be 'critique', got %q", agents[0].Phase)
	}
	if agents[0].StartedAt.Equal(oldStart) {
		t.Error("StartedAt should be reset on phase change")
	}
}

func TestStartAgent_FromDone(t *testing.T) {
	agents := []*AgentInfo{{Name: "claude", Status: AgentStatusDone}}
	ok := StartAgent(agents, "claude", "retry", 2*time.Minute, "")
	if !ok {
		t.Error("Should return true")
	}
	if agents[0].Status != AgentStatusRunning {
		t.Error("Should be Running after restart from Done")
	}
}

func TestStartAgent_CaseInsensitive(t *testing.T) {
	agents := []*AgentInfo{{Name: "Claude", Status: AgentStatusIdle}}
	ok := StartAgent(agents, "claude", "", 0, "")
	if !ok {
		t.Error("Should find agent case-insensitively")
	}
}

// ---------------------------------------------------------------------------
// CompleteAgent
// ---------------------------------------------------------------------------

func TestCompleteAgent_BasicComplete(t *testing.T) {
	start := time.Now().Add(-5 * time.Second)
	agents := []*AgentInfo{{
		Name:      "claude",
		Status:    AgentStatusRunning,
		StartedAt: start,
	}}

	found, rejIn, rejOut := CompleteAgent(agents, "claude", 100, 200)
	if !found {
		t.Error("Should return found=true")
	}
	if rejIn != 0 || rejOut != 0 {
		t.Errorf("Should not reject normal tokens, rejIn=%d rejOut=%d", rejIn, rejOut)
	}
	a := agents[0]
	if a.Status != AgentStatusDone {
		t.Error("Status should be Done")
	}
	if a.TokensIn != 100 {
		t.Errorf("TokensIn should be 100, got %d", a.TokensIn)
	}
	if a.TokensOut != 200 {
		t.Errorf("TokensOut should be 200, got %d", a.TokensOut)
	}
	if a.CurrentActivity != "" {
		t.Error("CurrentActivity should be cleared")
	}
	if a.ActivityIcon != "" {
		t.Error("ActivityIcon should be cleared")
	}
}

func TestCompleteAgent_NotFound(t *testing.T) {
	agents := []*AgentInfo{{Name: "claude"}}
	found, _, _ := CompleteAgent(agents, "nonexistent", 100, 200)
	if found {
		t.Error("Should return found=false")
	}
}

func TestCompleteAgent_RejectsHugeTokens(t *testing.T) {
	agents := []*AgentInfo{{Name: "claude", Status: AgentStatusRunning, StartedAt: time.Now()}}
	found, rejIn, rejOut := CompleteAgent(agents, "claude", 20_000_000, 15_000_000)
	if !found {
		t.Error("Should find agent")
	}
	if rejIn != 20_000_000 {
		t.Errorf("Should reject tokensIn=20M, got rejIn=%d", rejIn)
	}
	if rejOut != 15_000_000 {
		t.Errorf("Should reject tokensOut=15M, got rejOut=%d", rejOut)
	}
	if agents[0].TokensIn != 0 {
		t.Error("Rejected tokens should not be added")
	}
}

func TestCompleteAgent_AccumulatesTokens(t *testing.T) {
	agents := []*AgentInfo{{Name: "claude", TokensIn: 50, TokensOut: 50, Status: AgentStatusRunning, StartedAt: time.Now()}}
	CompleteAgent(agents, "claude", 100, 100)
	if agents[0].TokensIn != 150 {
		t.Errorf("Tokens should accumulate, expected 150, got %d", agents[0].TokensIn)
	}
	if agents[0].TokensOut != 150 {
		t.Errorf("Tokens should accumulate, expected 150, got %d", agents[0].TokensOut)
	}
}

func TestCompleteAgent_NegativeTokensIgnored(t *testing.T) {
	agents := []*AgentInfo{{Name: "claude", Status: AgentStatusRunning, StartedAt: time.Now()}}
	CompleteAgent(agents, "claude", -10, -20)
	if agents[0].TokensIn != 0 || agents[0].TokensOut != 0 {
		t.Error("Negative tokens should be ignored")
	}
}

func TestCompleteAgent_ZeroStartTime(t *testing.T) {
	agents := []*AgentInfo{{Name: "claude", Status: AgentStatusRunning}}
	found, _, _ := CompleteAgent(agents, "claude", 100, 200)
	if !found {
		t.Error("Should find agent")
	}
	if agents[0].Time != "" {
		t.Error("Zero start time should not produce Time string")
	}
}

// ---------------------------------------------------------------------------
// FailAgent
// ---------------------------------------------------------------------------

func TestFailAgent_Basic(t *testing.T) {
	agents := []*AgentInfo{{Name: "claude", Status: AgentStatusRunning, StartedAt: time.Now()}}
	ok := FailAgent(agents, "claude", "timeout")
	if !ok {
		t.Error("Should return true")
	}
	if agents[0].Status != AgentStatusError {
		t.Error("Status should be Error")
	}
	if agents[0].Error != "timeout" {
		t.Errorf("Error should be 'timeout', got %q", agents[0].Error)
	}
	if agents[0].CurrentActivity != "" || agents[0].ActivityIcon != "" {
		t.Error("Activity fields should be cleared")
	}
}

func TestFailAgent_NotFound(t *testing.T) {
	agents := []*AgentInfo{{Name: "claude"}}
	ok := FailAgent(agents, "nonexistent", "err")
	if ok {
		t.Error("Should return false when not found")
	}
}

func TestFailAgent_ZeroStartTime(t *testing.T) {
	agents := []*AgentInfo{{Name: "claude", Status: AgentStatusRunning}}
	FailAgent(agents, "claude", "err")
	if agents[0].Time != "" {
		t.Error("Zero start time should produce empty Time")
	}
}

func TestFailAgent_CaseInsensitive(t *testing.T) {
	agents := []*AgentInfo{{Name: "Claude", Status: AgentStatusRunning}}
	ok := FailAgent(agents, "claude", "err")
	if !ok {
		t.Error("Should find agent case-insensitively")
	}
}

// ---------------------------------------------------------------------------
// estimateProgress (additional cases)
// ---------------------------------------------------------------------------

func TestEstimateProgress_DisabledStatus(t *testing.T) {
	got := estimateProgress(time.Now(), AgentStatusDisabled)
	if got != 0 {
		t.Errorf("Disabled status should return 0, got %d", got)
	}
}
