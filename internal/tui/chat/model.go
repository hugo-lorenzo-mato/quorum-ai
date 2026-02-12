package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/clip"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/diagnostics"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// Color palette - modern dark theme (default)
var (
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#06B6D4") // Cyan
	successColor   = lipgloss.Color("#10B981") // Green
	warningColor   = lipgloss.Color("#F59E0B") // Amber
	errorColor     = lipgloss.Color("#EF4444") // Red
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	textColor      = lipgloss.Color("#F9FAFB") // White
	dimColor       = lipgloss.Color("#9CA3AF") // Light gray
	borderColor    = lipgloss.Color("#374151") // Border
	surfaceColor   = lipgloss.Color("#1F2937") // Surface background
)

// applyDarkTheme sets the color palette to dark theme.
func applyDarkTheme() {
	primaryColor = lipgloss.Color("#7C3AED")   // Purple
	secondaryColor = lipgloss.Color("#06B6D4") // Cyan
	successColor = lipgloss.Color("#10B981")   // Green
	warningColor = lipgloss.Color("#F59E0B")   // Amber
	errorColor = lipgloss.Color("#EF4444")     // Red
	mutedColor = lipgloss.Color("#6B7280")     // Gray
	textColor = lipgloss.Color("#F9FAFB")      // White
	dimColor = lipgloss.Color("#9CA3AF")       // Light gray
	borderColor = lipgloss.Color("#374151")    // Border
	surfaceColor = lipgloss.Color("#1F2937")   // Surface background
	refreshStyles()
}

// applyLightTheme sets the color palette to light theme.
func applyLightTheme() {
	primaryColor = lipgloss.Color("#6D28D9")   // Darker purple for contrast
	secondaryColor = lipgloss.Color("#0891B2") // Darker cyan
	successColor = lipgloss.Color("#059669")   // Darker green
	warningColor = lipgloss.Color("#D97706")   // Darker amber
	errorColor = lipgloss.Color("#DC2626")     // Darker red
	mutedColor = lipgloss.Color("#6B7280")     // Gray
	textColor = lipgloss.Color("#1F2937")      // Dark text
	dimColor = lipgloss.Color("#6B7280")       // Muted gray
	borderColor = lipgloss.Color("#D1D5DB")    // Light border
	surfaceColor = lipgloss.Color("#F9FAFB")   // Light background
	refreshStyles()
}

// refreshStyles recreates all styles with the current color values.
func refreshStyles() {
	// Header styles
	logoStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor)

	// Tab-style agent bar
	tabActiveStyle = lipgloss.NewStyle().
		Foreground(textColor).
		Background(primaryColor).
		Padding(0, 1).
		Bold(true)

	tabInactiveStyle = lipgloss.NewStyle().
		Foreground(dimColor).
		Padding(0, 1)

	tabRunningStyle = lipgloss.NewStyle().
		Foreground(warningColor).
		Background(surfaceColor).
		Padding(0, 1).
		Bold(true)

	tabErrorStyle = lipgloss.NewStyle().
		Foreground(errorColor).
		Background(surfaceColor).
		Padding(0, 1)

	tabSeparatorStyle = lipgloss.NewStyle().
		Foreground(borderColor)

	// Input styles
	inputContainerStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	inputActiveStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(0, 1)

	inputShellStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#f97316")). // Orange
		Padding(0, 1)

	// Spinner style
	spinnerStyle = lipgloss.NewStyle().
		Foreground(primaryColor)
}

// Nerd Font icons (fallback to Unicode if not available)
const (
	iconFolder        = "" // nf-fa-folder
	iconFolderOpen    = "" // nf-fa-folder_open
	iconFile          = "" // nf-fa-file
	iconLogs          = "" // nf-fa-list_alt
	iconExplorer      = "" // nf-fa-sitemap
	iconDot           = "●"
	iconDotHollow     = "○"
	iconDotHalf       = "◐"
	iconCheck         = "✓"
	iconCross         = "✗"
	iconChevronRight  = "›"
	iconChevronLeft   = "‹"
	iconTriangleDown  = "▼"
	iconTriangleRight = "▶"
	iconSpinner       = "◐"
)

// Input area constants
const (
	minInputLines = 1  // Minimum lines for input
	maxInputLines = 12 // Maximum lines for input before scrolling
)

// Styles for the modern chat UI
var (
	// Header styles
	logoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	// Tab-style agent bar
	tabActiveStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(primaryColor).
			Padding(0, 1).
			Bold(true)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(dimColor).
				Padding(0, 1)

	tabRunningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Background(surfaceColor).
			Padding(0, 1).
			Bold(true)

	tabErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Background(surfaceColor).
			Padding(0, 1)

	tabSeparatorStyle = lipgloss.NewStyle().
				Foreground(borderColor)

	// Input styles
	inputContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(borderColor).
				Padding(0, 1)

	inputActiveStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(0, 1)

	// Shell command input style (orange border)
	inputShellStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#f97316")). // Orange
			Padding(0, 1)

	// Spinner style
	spinnerStyle = lipgloss.NewStyle().
			Foreground(primaryColor)
)

// quorumSystemPrompt contains accurate documentation about Quorum AI.
// This is injected into conversations so the model gives accurate information.
const quorumSystemPrompt = `IMPORTANT IDENTITY OVERRIDE: You are NOT Claude Code. You are the Quorum AI Chat Assistant.
Do NOT identify yourself as Claude Code, Claude, or any Anthropic product. You are Quorum's assistant.

When users greet you or ask who you are, respond: "I am the Quorum AI assistant, here to help you with multi-agent orchestration tasks."

When users ask about Quorum, provide ONLY accurate information based on this documentation:

## What is Quorum AI?
Quorum AI orchestrates multiple AI agents (Claude, Gemini, etc.) to work in parallel with consensus-based validation. It reduces hallucinations by having agents validate each other's outputs through a dialectic protocol (Thesis-Antithesis-Synthesis).

## Available CLI Commands:
- quorum run <prompt>     - Run complete workflow (analyze → plan → execute)
- quorum analyze <prompt> - Run analysis phase only
- quorum plan <prompt>    - Run planning phase only
- quorum execute          - Run execution phase only
- quorum chat             - Start interactive chat mode
- quorum status           - Show workflow status
- quorum doctor           - Check system dependencies
- quorum init             - Initialize a new quorum project
- quorum trace            - Show trace summaries

## Chat Mode Commands (inside quorum chat):
- /analyze <prompt> - Run multi-agent analysis (V1/V2/V3)
- /plan [prompt]    - Continue planning or start new workflow
- /execute          - Execute issues from active workflow
- /run <prompt>     - Run complete workflow
- /workflows        - List available workflows
- /load [id]        - Load and switch to a workflow
- /status           - Show workflow status
- /cancel           - Cancel current workflow
- /model <name>     - Set current model
- /agent <name>     - Set current agent
- /copy             - Copy last response to clipboard
- /copyall          - Copy entire conversation
- /clear            - Clear conversation
- /help             - Show help
- /quit             - Exit chat

## Key Features:
- Multi-Agent Execution: Claude, Gemini, and other agents in parallel
- Consensus Validation: Jaccard similarity measures agreement
- Dialectic Protocol: V1/V2/V3 (Thesis-Antithesis-Synthesis) process
- Git Worktree Isolation: Each task in isolated worktrees
- Resume from Checkpoint: Recover without re-running completed work
- Cost Tracking: Monitor tokens and costs
- Trace Mode: Optional detailed logging

## Common Flags:
- --dry-run        Simulate without executing
- --resume         Resume from checkpoint
- --trace          Enable tracing (off, summary, full)
- --yolo           Skip confirmations
- -o, --output     Output mode (tui, plain, json, quiet)

## Configuration:
Quorum uses .quorum/config.yaml for configuration including agents, consensus threshold, timeouts, and tracing options.

IMPORTANT: Do NOT invent commands or features not listed above. If unsure, say you don't know.
`

// WorkflowRunner interface for running workflows.
type WorkflowRunner interface {
	Run(ctx context.Context, prompt string) error
	Analyze(ctx context.Context, prompt string) error
	Plan(ctx context.Context) error
	Replan(ctx context.Context, additionalContext string) error
	UsePlan(ctx context.Context) error
	Resume(ctx context.Context) error
	GetState(ctx context.Context) (*core.WorkflowState, error)
	SaveState(ctx context.Context, state *core.WorkflowState) error
	ListWorkflows(ctx context.Context) ([]core.WorkflowSummary, error)
	LoadWorkflow(ctx context.Context, workflowID string) (*core.WorkflowState, error)
	DeactivateWorkflow(ctx context.Context) error
	ArchiveWorkflows(ctx context.Context) (int, error)
	PurgeAllWorkflows(ctx context.Context) (int, error)
	DeleteWorkflow(ctx context.Context, workflowID string) error
}

// Model is the Bubble Tea model for the chat interface.
type Model struct {
	// UI components
	viewport viewport.Model
	textarea textarea.Model
	spinner  spinner.Model

	// State
	history         *ConversationHistory
	commands        *CommandRegistry
	suggestions     []string
	showSuggestions bool
	suggestionIndex int
	suggestionType  string                 // "command", "agent", "model", "workflow"
	availableAgents []string               // List of enabled agent names
	agentModels     map[string][]string    // Models available per agent
	workflowCache   []core.WorkflowSummary // Cached workflows for suggestions

	// Workflow integration
	controlPlane  *control.ControlPlane
	agents        core.AgentRegistry
	currentAgent  string
	currentModel  string
	workflowState *core.WorkflowState

	// Agent display state (for compact bar and pipeline)
	agentInfos     []*AgentInfo
	workflowPhase  string // "idle", "running", "done"
	totalTokensIn  int
	totalTokensOut int

	// Workflow runner for /plan and /run commands
	runner      WorkflowRunner
	eventBus    *events.EventBus
	logEventsCh <-chan events.Event // Channel for receiving log events
	logger      *logging.Logger

	// Input request handling
	pendingInputRequest *control.InputRequest

	// Configuration
	editorCmd string // Editor command for file editing (from config)

	// Display state
	width, height     int
	ready             bool
	streaming         bool
	quitting          bool
	workflowRunning   bool
	workflowStartedAt time.Time // When the current workflow started
	chatStartedAt     time.Time // When the current chat message was sent
	chatAgent         string    // Agent handling current chat message
	chatModel         string    // Model used for current chat message
	inputFocused      bool
	darkTheme         bool // Current theme (true=dark, false=light)

	// Logs panel
	logsPanel *LogsPanel
	showLogs  bool
	logsFocus bool // true when logs panel has focus for scrolling

	// Stats panel (token usage and system metrics)
	statsPanel *StatsPanel
	showStats  bool
	statsFocus bool // true when stats panel has focus for scrolling

	// Explorer panel
	explorerPanel *ExplorerPanel
	showExplorer  bool
	explorerFocus bool // true when explorer has focus for navigation

	// Token panel (left sidebar)
	tokenPanel  *TokenPanel
	showTokens  bool
	tokensFocus bool

	// Panel navigation mode (tmux-style with Ctrl+n prefix)
	panelNavMode bool // true when waiting for arrow key to switch panels
	panelNavSeq  int
	panelNavTill time.Time

	// NEW: Enhanced UI panels
	consensusPanel   *ConsensusPanel
	tasksPanel       *TasksPanel
	contextPanel     *ContextPreviewPanel
	diffView         *AgentDiffView
	historySearch    *HistorySearch
	shortcutsOverlay *ShortcutsOverlay
	fileViewer       *FileViewer
	statsWidget      *StatsWidget
	machineCollector *diagnostics.SystemMetricsCollector

	// Cancellation for interrupts
	cancelFunc context.CancelFunc

	// Markdown renderer
	mdRenderer *glamour.TermRenderer

	// Message styles (for new chat message rendering)
	messageStyles *MessageStyles

	// Version info
	version string

	// Chat configuration
	chatTimeout          time.Duration
	chatProgressInterval time.Duration
}

// NewModel creates a new chat model.
func NewModel(cp *control.ControlPlane, agents core.AgentRegistry, defaultAgent, defaultModel string) Model {
	ta := textarea.New()
	ta.Placeholder = "Message Quorum..."
	ta.Focus()
	ta.Prompt = ""
	ta.CharLimit = 4096
	ta.SetWidth(80)
	ta.SetHeight(3) // Allow multi-line input (will grow up to maxInputLines)
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.KeyMap.InsertNewline.SetEnabled(true) // Allow Shift+Enter for newlines

	// Create spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	// Create markdown renderer with custom style to fix inline code rendering
	customStyle := styles.DraculaStyleConfig
	customStyle.Code = ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color:           stringPtr("229"), // Light yellow
			BackgroundColor: stringPtr(""),    // No background
			Prefix:          "",               // No prefix
			Suffix:          "",               // No suffix
		},
	}
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStyles(customStyle),
		glamour.WithWordWrap(80),
	)

	// Initialize agent display info from registry
	var agentInfos []*AgentInfo
	if agents != nil {
		for _, name := range []string{"claude", "gemini", "codex", "copilot", "opencode"} {
			agent, err := agents.Get(name)
			status := AgentStatusDisabled
			if err == nil && agent != nil {
				status = AgentStatusIdle
			}
			agentInfos = append(agentInfos, &AgentInfo{
				Name:   strings.ToUpper(name[:1]) + name[1:],
				Color:  GetAgentColor(name),
				Status: status,
			})
		}
	}

	return Model{
		textarea:      ta,
		spinner:       sp,
		history:       NewConversationHistory(100),
		commands:      NewCommandRegistry(),
		controlPlane:  cp,
		agents:        agents,
		currentAgent:  defaultAgent,
		currentModel:  defaultModel,
		mdRenderer:    renderer,
		inputFocused:  true,
		agentInfos:    agentInfos,
		workflowPhase: "idle",
		logsPanel:     NewLogsPanel(500),
		showLogs:      true, // Open by default
		logsFocus:     false,
		statsPanel:    NewStatsPanel(),
		showStats:     true, // Open by default
		statsFocus:    false,
		explorerPanel: NewExplorerPanel(),
		showExplorer:  true, // Open by default
		explorerFocus: false,
		tokenPanel:    NewTokenPanel(),
		showTokens:    true, // Open by default
		tokensFocus:   false,
		// Initialize new panels
		consensusPanel:   NewConsensusPanel(80.0), // 80% threshold
		tasksPanel:       NewTasksPanel(),
		contextPanel:     NewContextPreviewPanel(),
		diffView:         NewAgentDiffView(),
		historySearch:    NewHistorySearch(),
		shortcutsOverlay: NewShortcutsOverlay(),
		fileViewer:       NewFileViewer(),
		statsWidget:      NewStatsWidget(),
		machineCollector: diagnostics.NewSystemMetricsCollector(),
		darkTheme:        true,                 // Default to dark theme
		messageStyles:    NewMessageStyles(80), // Default width, updated on resize
	}
}

// WithWorkflowRunner adds workflow runner support to the model.
func (m Model) WithWorkflowRunner(runner WorkflowRunner, eventBus *events.EventBus, logger *logging.Logger) Model {
	m.runner = runner
	m.eventBus = eventBus
	m.logger = logger
	// Subscribe to log events and agent events from the workflow
	if eventBus != nil {
		// Subscribe to all event types - we'll filter in the handler
		m.logEventsCh = eventBus.Subscribe()
	}
	return m
}

// WithVersion sets the application version for display.
func (m Model) WithVersion(version string) Model {
	m.version = version
	return m
}

// WithChatConfig sets the chat configuration (timeout, progress interval).
func (m Model) WithChatConfig(timeout, progressInterval time.Duration) Model {
	if timeout > 0 {
		m.chatTimeout = timeout
	} else {
		m.chatTimeout = 20 * time.Minute // Default 20 min
	}
	if progressInterval > 0 {
		m.chatProgressInterval = progressInterval
	} else {
		m.chatProgressInterval = 15 * time.Second // Default 15s
	}
	return m
}

// WithAgentModels sets the available agents and their models for suggestions.
func (m Model) WithAgentModels(agents []string, agentModels map[string][]string) Model {
	m.availableAgents = agents
	m.agentModels = agentModels
	return m
}

// WithEditor sets the editor command for file editing
func (m Model) WithEditor(editor string) Model {
	m.editorCmd = editor
	return m
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		m.listenForInputRequests(),
		m.listenForExplorerChanges(),
		m.listenForLogEvents(),
		m.loadActiveWorkflow(),    // Auto-load active workflow on startup
		statsTickCmd(),            // Start stats updates
		tea.EnableMouseCellMotion, // Enable mouse support for click-to-focus
	)
}

// loadActiveWorkflow returns a command that loads the active workflow on startup.
func (m Model) loadActiveWorkflow() tea.Cmd {
	if m.runner == nil {
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		workflows, err := m.runner.ListWorkflows(ctx)
		if err != nil {
			return nil
		}
		for _, wf := range workflows {
			if wf.IsActive {
				state, err := m.runner.LoadWorkflow(ctx, string(wf.WorkflowID))
				if err == nil && state != nil {
					return ActiveWorkflowLoadMsg{State: state}
				}
				break
			}
		}
		return nil
	}
}

// listenForLogEvents creates a command that listens for log events from the workflow.
// Uses batching with debounce to collect multiple events within a 100ms window,
// reducing the number of UI updates and improving responsiveness.
func (m Model) listenForLogEvents() tea.Cmd {
	if m.logEventsCh == nil {
		return nil
	}
	return func() tea.Msg {
		const (
			debounceWindow = 100 * time.Millisecond
			maxBatchSize   = 50
		)

		var batch []tea.Msg

		// Wait for first event (blocking)
		event, ok := <-m.logEventsCh
		if !ok {
			return nil // Channel closed
		}

		// Convert first event to message
		if msg := m.eventToMsg(event); msg != nil {
			batch = append(batch, msg)
		}

		// Collect additional events within debounce window
		timer := time.NewTimer(debounceWindow)
		defer timer.Stop()

	collectLoop:
		for len(batch) < maxBatchSize {
			select {
			case event, ok := <-m.logEventsCh:
				if !ok {
					break collectLoop // Channel closed
				}
				if msg := m.eventToMsg(event); msg != nil {
					batch = append(batch, msg)
				}
			case <-timer.C:
				break collectLoop // Debounce window expired
			}
		}

		// Return batched events if multiple, single event otherwise
		if len(batch) == 0 {
			return nil
		}
		if len(batch) == 1 {
			return batch[0]
		}
		return BatchedEventsMsg{Events: batch}
	}
}

// eventToMsg converts a workflow event to a tea.Msg
func (m Model) eventToMsg(event interface{}) tea.Msg {
	// Handle log events
	if logEvent, ok := event.(events.LogEvent); ok {
		return WorkflowLogMsg{
			Level:   logEvent.Level,
			Source:  "workflow",
			Message: logEvent.Message,
		}
	}

	// Handle agent stream events
	if agentEvent, ok := event.(events.AgentStreamEvent); ok {
		return AgentStreamMsg{
			Kind:    string(agentEvent.EventKind),
			Agent:   agentEvent.Agent,
			Message: agentEvent.Message,
			Data:    agentEvent.Data,
		}
	}

	// Handle workflow state updates (tasks created, task status changes, etc.)
	// We reload the full state so the issues panel can render the current task list.
	if _, ok := event.(events.WorkflowStateUpdatedEvent); ok {
		if m.runner == nil {
			return nil
		}
		state, err := m.runner.GetState(context.Background())
		if err != nil || state == nil {
			return nil
		}
		return WorkflowUpdateMsg{State: state}
	}

	// Handle task started events
	if taskEvent, ok := event.(events.TaskStartedEvent); ok {
		return TaskUpdateMsg{
			TaskID: core.TaskID(taskEvent.TaskID),
			Status: core.TaskStatusRunning,
		}
	}

	// Handle task completed events
	if taskEvent, ok := event.(events.TaskCompletedEvent); ok {
		return TaskUpdateMsg{
			TaskID: core.TaskID(taskEvent.TaskID),
			Status: core.TaskStatusCompleted,
		}
	}

	// Handle task failed events
	if taskEvent, ok := event.(events.TaskFailedEvent); ok {
		return TaskUpdateMsg{
			TaskID: core.TaskID(taskEvent.TaskID),
			Status: core.TaskStatusFailed,
			Error:  taskEvent.Error,
		}
	}

	// Handle task skipped events
	if taskEvent, ok := event.(events.TaskSkippedEvent); ok {
		return TaskUpdateMsg{
			TaskID: core.TaskID(taskEvent.TaskID),
			Status: core.TaskStatusSkipped,
			Error:  taskEvent.Reason,
		}
	}

	// Handle phase started events
	if phaseEvent, ok := event.(events.PhaseStartedEvent); ok {
		return PhaseUpdateMsg{
			Phase: core.Phase(phaseEvent.Phase),
		}
	}

	// Unknown event type - return nil
	return nil
}

// Message types
type (
	InputRequestMsg struct {
		Request control.InputRequest
	}
	AgentResponseMsg struct {
		Agent     string
		Content   string
		TokensIn  int
		TokensOut int
		Error     error
	}
	WorkflowUpdateMsg struct {
		State *core.WorkflowState
	}
	QuitMsg              struct{}
	WorkflowStartedMsg   struct{ Prompt string }
	WorkflowCompletedMsg struct{ State *core.WorkflowState }
	WorkflowErrorMsg     struct{ Error error }
	WorkflowLogMsg       struct {
		Level   string
		Source  string
		Message string
	}
	AgentStreamMsg struct {
		Kind    string // started, tool_use, thinking, chunk, progress, completed, error
		Agent   string
		Message string
		Data    map[string]any
	}
	TickMsg               struct{ Time time.Time }
	ExplorerRefreshMsg    struct{} // File system change detected
	StatsTickMsg          struct{} // Periodic stats update
	PanelNavTimeoutMsg    struct{ Seq int }
	ChatProgressTickMsg   struct{ Elapsed time.Duration }
	ActiveWorkflowLoadMsg struct{ State *core.WorkflowState } // Auto-load active workflow on startup
	TaskUpdateMsg         struct {
		TaskID core.TaskID
		Status core.TaskStatus
		Error  string
	}
	PhaseUpdateMsg struct {
		Phase core.Phase
	}
	// BatchedEventsMsg contains multiple events collected within a debounce window
	BatchedEventsMsg struct {
		Events []tea.Msg
	}
)

// chatProgressTick returns a command that sends periodic progress updates during chat execution.
func (m Model) chatProgressTick() tea.Cmd {
	interval := m.chatProgressInterval
	if interval == 0 {
		interval = 15 * time.Second
	}
	return tea.Tick(interval, func(_ time.Time) tea.Msg {
		return ChatProgressTickMsg{Elapsed: time.Since(m.chatStartedAt)}
	})
}

func (m Model) listenForInputRequests() tea.Cmd {
	if m.controlPlane == nil {
		return nil
	}
	return func() tea.Msg {
		req := <-m.controlPlane.InputRequestCh()
		return InputRequestMsg{Request: req}
	}
}

// listenForExplorerChanges listens for file system changes in the explorer
func (m Model) listenForExplorerChanges() tea.Cmd {
	return func() tea.Msg {
		<-m.explorerPanel.OnChange()
		return ExplorerRefreshMsg{}
	}
}

// statsTickCmd returns a command that sends StatsTickMsg every second
func statsTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return StatsTickMsg{}
	})
}

func (m *Model) updateQuorumPanel(state *core.WorkflowState) {
	if state == nil {
		m.consensusPanel.SetScore(0)
		return
	}

	// Set current consensus score
	if state.Metrics != nil && state.Metrics.ConsensusScore > 0 {
		m.consensusPanel.SetScore(state.Metrics.ConsensusScore * 100)
	}

	// Extract consensus history from checkpoints
	var history []ConsensusRound
	for _, cp := range state.Checkpoints {
		if cp.Type == "moderator_round" && len(cp.Data) > 0 {
			var cpData map[string]interface{}
			if err := json.Unmarshal(cp.Data, &cpData); err == nil {
				round, hasRound := cpData["round"].(float64)
				score, hasScore := cpData["consensus_score"].(float64)
				if hasRound && hasScore {
					history = append(history, ConsensusRound{
						Round: int(round),
						Score: score * 100, // Convert 0-1 to 0-100
					})
				}
			}
		}
	}
	if len(history) > 0 {
		m.consensusPanel.SetHistory(history)
	}

	// Set analysis path from report path
	if state.ReportPath != "" {
		analysisPath := filepath.Join(state.ReportPath, "analyze")
		m.consensusPanel.SetAnalysisPath(analysisPath)
	}
}

const panelNavWindow = 1500 * time.Millisecond

func panelNavTimeoutCmd(seq int) tea.Cmd {
	return tea.Tick(panelNavWindow, func(_ time.Time) tea.Msg {
		return PanelNavTimeoutMsg{Seq: seq}
	})
}

// handleKeyMsg handles keyboard input.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	// PRIORITY: Escape ALWAYS closes autocomplete dropdown first
	if msg.Type == tea.KeyEsc && m.showSuggestions {
		m.showSuggestions = false
		m.suggestionIndex = 0
		if m.textarea.Value() == "/" {
			m.textarea.Reset()
		}
		return m, nil, true
	}

	// Panel navigation mode (tmux-style: Ctrl+z then arrow keys)
	if m.panelNavMode {
		return m.handlePanelNavKeys(msg)
	}

	// Ctrl+z enters panel navigation mode
	if msg.Type == tea.KeyCtrlZ {
		m.panelNavMode = true
		m.panelNavSeq++
		m.panelNavTill = time.Now().Add(panelNavWindow)
		return m, panelNavTimeoutCmd(m.panelNavSeq), true
	}

	// Core input handling
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		m.explorerPanel.Close()
		return m, tea.Quit, true

	case tea.KeyEnter:
		if m.showSuggestions && len(m.suggestions) > 0 {
			return m.handleEnterWithSuggestions(msg)
		}
		if m.textarea.Value() != "" {
			model, teaCmd := m.handleSubmit()
			return model, teaCmd, true
		}

	case tea.KeyTab:
		return m.handleTabKey(msg)

	case tea.KeyUp:
		if newModel, cmd, handled := m.handleSuggestionNav(msg); handled {
			return newModel, cmd, handled
		}

	case tea.KeyDown:
		if newModel, cmd, handled := m.handleSuggestionNav(msg); handled {
			return newModel, cmd, handled
		}

	case tea.KeyEsc:
		if newModel, cmd, handled := m.handleEscapeInContext(msg); handled {
			return newModel, cmd, handled
		}

	case tea.KeyCtrlAt:
		return m.handleCtrlSpaceKey(msg)

	case tea.KeyCtrlY:
		return m.copyLastResponse()

	case tea.KeyCtrlL, tea.KeyCtrlE, tea.KeyCtrlR, tea.KeyCtrlT,
		tea.KeyCtrlX, tea.KeyCtrlQ, tea.KeyCtrlK, tea.KeyCtrlD, tea.KeyCtrlH:
		if newModel, cmd, handled := m.handlePanelToggleKeys(msg); handled {
			return newModel, cmd, handled
		}
	}

	// Overlay navigation (?, F1, Escape-close-overlays, history search, diff view)
	if newModel, cmd, handled := m.handleOverlayNavKeys(msg); handled {
		return newModel, cmd, handled
	}

	// File viewer (exclusive when visible)
	if m.fileViewer.IsVisible() {
		return m.handleFileViewerKeys(msg)
	}

	// Explorer navigation (when focused)
	if m.explorerFocus && m.showExplorer {
		if newModel, cmd, handled := m.handleExplorerKeys(msg); handled {
			return newModel, cmd, handled
		}
	}

	// Focused panel scrolling (tokens, stats, logs) + copy
	if newModel, cmd, handled := m.handleFocusedPanelKeys(msg); handled {
		return newModel, cmd, handled
	}

	return m, nil, false
}

// handleMouseClick handles mouse clicks to switch panel focus.
func (m Model) handleMouseClick(x, y int) (tea.Model, tea.Cmd, bool) {
	// Calculate panel boundaries based on current layout
	leftSidebarWidth := 0
	rightSidebarWidth := 0
	showLeftSidebar := m.showExplorer || m.showTokens
	showRightSidebar := m.showLogs || m.showStats

	if m.showExplorer {
		leftSidebarWidth = m.explorerPanel.Width()
	} else if m.showTokens {
		leftSidebarWidth = m.tokenPanel.Width()
	}
	if m.showStats {
		rightSidebarWidth = m.statsPanel.Width()
	} else if m.showLogs {
		rightSidebarWidth = m.logsPanel.Width()
	}

	// Determine which panel was clicked based on X coordinate
	mainStart := leftSidebarWidth
	if showLeftSidebar {
		mainStart += 1 // Account for separator
	}
	mainEnd := m.width - rightSidebarWidth
	if showRightSidebar {
		mainEnd -= 1 // Account for separator
	}

	// Check if click is in left sidebar
	if showLeftSidebar && x < leftSidebarWidth {
		return m.handleLeftSidebarClick(y)
	}

	// Check if click is in right sidebar (logs/stats)
	if (m.showLogs || m.showStats) && x > mainEnd {
		return m.handleRightSidebarClick(y)
	}

	// Click is in Main content area - return focus to chat input
	if x >= mainStart && x < mainEnd {
		if m.explorerFocus || m.logsFocus || m.statsFocus || m.tokensFocus {
			m.explorerFocus = false
			m.logsFocus = false
			m.statsFocus = false
			m.tokensFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			m.explorerPanel.SetFocused(false)
			return m, nil, true
		}
		// Already in main area, check if clicking on input
		return m, nil, false
	}

	return m, nil, false
}

// copyLastResponse copies the last agent response to clipboard.
func (m Model) copyLastResponse() (tea.Model, tea.Cmd, bool) {
	msgs := m.history.All()
	// Find last agent message
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != RoleAgent {
			continue
		}

		res, err := clip.WriteAll(msgs[i].Content)
		if err != nil {
			m.logsPanel.AddError("system", "Failed to copy: "+err.Error())
			return m, nil, true
		}
		agent := msgs[i].Agent
		switch res.Method {
		case clip.MethodFile:
			m.logsPanel.AddWarn("system", fmt.Sprintf("Clipboard unavailable; wrote %s response to %s (%d chars)", agent, res.FilePath, len(msgs[i].Content)))
		case clip.MethodOSC52:
			m.logsPanel.AddSuccess("system", fmt.Sprintf("Copied %s response to clipboard via OSC52 (%d chars)", agent, len(msgs[i].Content)))
		default:
			m.logsPanel.AddSuccess("system", fmt.Sprintf("Copied %s response to clipboard (%d chars)", agent, len(msgs[i].Content)))
		}
		return m, nil, true
	}
	m.logsPanel.AddWarn("system", "No response to copy")
	return m, nil, true
}

// copyConversation copies the entire conversation to clipboard
func (m Model) copyConversation() (tea.Model, tea.Cmd, bool) {
	msgs := m.history.All()
	if len(msgs) == 0 {
		m.logsPanel.AddWarn("system", "No conversation to copy")
		return m, nil, true
	}

	var sb strings.Builder
	for _, msg := range msgs {
		switch msg.Role {
		case RoleUser:
			sb.WriteString("You: ")
			sb.WriteString(msg.Content)
			sb.WriteString("\n\n")
		case RoleAgent:
			sb.WriteString(msg.Agent + ": ")
			sb.WriteString(msg.Content)
			sb.WriteString("\n\n")
		case RoleSystem:
			sb.WriteString("[System] ")
			sb.WriteString(msg.Content)
			sb.WriteString("\n\n")
		}
	}

	text := sb.String()
	res, err := clip.WriteAll(text)
	if err != nil {
		m.logsPanel.AddError("system", "Failed to copy conversation: "+err.Error())
	} else {
		switch res.Method {
		case clip.MethodFile:
			m.logsPanel.AddWarn("system", fmt.Sprintf("Clipboard unavailable; wrote entire conversation to %s (%d messages)", res.FilePath, len(msgs)))
		case clip.MethodOSC52:
			m.logsPanel.AddSuccess("system", fmt.Sprintf("Copied entire conversation to clipboard via OSC52 (%d messages)", len(msgs)))
		default:
			m.logsPanel.AddSuccess("system", fmt.Sprintf("Copied entire conversation to clipboard (%d messages)", len(msgs)))
		}
	}
	return m, nil, true
}

// copyLogsToClipboard copies all logs to clipboard as plain text.
func (m Model) copyLogsToClipboard() (tea.Model, tea.Cmd, bool) {
	logsText := m.logsPanel.GetPlainText()
	if logsText == "" {
		m.logsPanel.AddWarn("system", "No logs to copy")
		return m, nil, true
	}

	res, err := clip.WriteAll(logsText)
	if err != nil {
		m.logsPanel.AddError("system", "Failed to copy logs: "+err.Error())
	} else {
		switch res.Method {
		case clip.MethodFile:
			m.logsPanel.AddWarn("system", fmt.Sprintf("Clipboard unavailable; wrote %d logs to %s", m.logsPanel.Count(), res.FilePath))
		case clip.MethodOSC52:
			m.logsPanel.AddSuccess("system", fmt.Sprintf("Copied %d logs to clipboard via OSC52", m.logsPanel.Count()))
		default:
			m.logsPanel.AddSuccess("system", fmt.Sprintf("Copied %d logs to clipboard", m.logsPanel.Count()))
		}
	}
	return m, nil, true
}

// updateSuggestions refreshes the autocomplete suggestions.
func (m *Model) updateSuggestions() {
	// Don't show suggestions while streaming or during workflow
	if m.streaming || m.workflowRunning {
		m.showSuggestions = false
		return
	}

	val := m.textarea.Value()
	valLower := strings.ToLower(val)

	// Check for /agent with space - show agent suggestions
	if strings.HasPrefix(valLower, "/agent ") || strings.HasPrefix(valLower, "/a ") {
		partial := strings.TrimSpace(val[strings.Index(val, " ")+1:])
		newSuggestions := m.suggestAgents(partial)
		if len(newSuggestions) != len(m.suggestions) || m.suggestionType != "agent" {
			m.suggestionIndex = 0
		}
		m.suggestions = newSuggestions
		m.suggestionType = "agent"
		m.showSuggestions = len(m.suggestions) > 0
		return
	}

	// Check for /model with space - show model suggestions
	if strings.HasPrefix(valLower, "/model ") || strings.HasPrefix(valLower, "/m ") {
		partial := strings.TrimSpace(val[strings.Index(val, " ")+1:])
		newSuggestions := m.suggestModels(partial)
		if len(newSuggestions) != len(m.suggestions) || m.suggestionType != "model" {
			m.suggestionIndex = 0
		}
		m.suggestions = newSuggestions
		m.suggestionType = "model"
		m.showSuggestions = len(m.suggestions) > 0
		return
	}

	// Check for /load with space - show workflow suggestions
	if strings.HasPrefix(valLower, "/load ") || strings.HasPrefix(valLower, "/switch ") || strings.HasPrefix(valLower, "/select ") {
		partial := strings.TrimSpace(val[strings.Index(val, " ")+1:])
		newSuggestions := m.suggestWorkflows(partial)
		if len(newSuggestions) != len(m.suggestions) || m.suggestionType != "workflow" {
			m.suggestionIndex = 0
		}
		m.suggestions = newSuggestions
		m.suggestionType = "workflow"
		m.showSuggestions = len(m.suggestions) > 0
		return
	}

	// Check for /theme with space - show theme suggestions
	if strings.HasPrefix(valLower, "/theme ") || strings.HasPrefix(valLower, "/t ") {
		partial := strings.TrimSpace(val[strings.Index(val, " ")+1:])
		newSuggestions := m.suggestThemes(partial)
		if len(newSuggestions) != len(m.suggestions) || m.suggestionType != "theme" {
			m.suggestionIndex = 0
		}
		m.suggestions = newSuggestions
		m.suggestionType = "theme"
		m.showSuggestions = len(m.suggestions) > 0
		return
	}

	// Regular command suggestions (no space yet)
	if strings.HasPrefix(val, "/") && !strings.Contains(val, " ") {
		newSuggestions := m.commands.Suggest(val)
		if len(newSuggestions) != len(m.suggestions) || m.suggestionType != "command" {
			m.suggestionIndex = 0
		}
		m.suggestions = newSuggestions
		m.suggestionType = "command"
		m.showSuggestions = len(m.suggestions) > 0
		return
	}

	m.showSuggestions = false
	m.suggestionIndex = 0
	m.suggestionType = ""
}

// suggestAgents returns agent suggestions filtered by partial input.
func (m *Model) suggestAgents(partial string) []string {
	if len(m.availableAgents) == 0 {
		// Fallback to default agents
		return []string{"claude", "gemini", "codex", "copilot"}
	}

	if partial == "" {
		return m.availableAgents
	}

	partialLower := strings.ToLower(partial)
	var matches []string
	for _, agent := range m.availableAgents {
		if strings.Contains(strings.ToLower(agent), partialLower) {
			matches = append(matches, agent)
		}
	}
	return matches
}

// suggestModels returns model suggestions for the current agent, filtered by partial input.
func (m *Model) suggestModels(partial string) []string {
	agent := m.currentAgent
	if agent == "" {
		agent = "claude"
	}

	var models []string
	if m.agentModels != nil {
		models = m.agentModels[agent]
	}

	// Fallback to default models if none configured
	if len(models) == 0 {
		switch agent {
		case "claude":
			models = []string{
				"claude-opus-4-6",
				"claude-sonnet-4-5-20250929",
				"claude-haiku-4-5-20251001",
				"claude-sonnet-4-20250514",
				"claude-opus-4-20250514",
			}
		case "gemini":
			models = []string{
				"gemini-2.5-pro",
				"gemini-2.5-flash",
				"gemini-2.5-flash-lite",
				"gemini-3-pro-preview",
				"gemini-3-flash-preview",
			}
		case "codex":
			models = []string{
				"gpt-5.3-codex",
				"gpt-5.2-codex",
				"gpt-5.2",
				"gpt-5.1-codex-max",
				"gpt-5.1-codex",
				"gpt-5.1-codex-mini",
				"gpt-5.1",
				"gpt-5-codex",
				"gpt-5-codex-mini",
				"gpt-5",
				"gpt-5-mini",
				"gpt-4.1",
			}
		case "copilot":
			models = []string{
				"claude-sonnet-4.5",
				"claude-opus-4.6",
				"claude-haiku-4.5",
				"claude-sonnet-4",
				"gpt-5.2-codex",
				"gpt-5.1-codex-max",
				"gpt-5.1-codex",
				"gemini-3-pro-preview",
			}
		default:
			models = []string{m.currentModel}
		}
	}

	if partial == "" {
		return models
	}

	partialLower := strings.ToLower(partial)
	var matches []string
	for _, model := range models {
		if strings.Contains(strings.ToLower(model), partialLower) {
			matches = append(matches, model)
		}
	}
	return matches
}

// suggestWorkflows returns workflow ID suggestions filtered by partial input.
func (m *Model) suggestWorkflows(partial string) []string {
	// Refresh workflow cache if empty or stale
	if len(m.workflowCache) == 0 && m.runner != nil {
		ctx := context.Background()
		workflows, err := m.runner.ListWorkflows(ctx)
		if err == nil {
			m.workflowCache = workflows
		}
	}

	if len(m.workflowCache) == 0 {
		return nil
	}

	var matches []string
	partialLower := strings.ToLower(partial)

	for _, wf := range m.workflowCache {
		// Match against workflow ID or prompt
		idLower := strings.ToLower(string(wf.WorkflowID))
		promptLower := strings.ToLower(wf.Prompt)

		if partial == "" || strings.Contains(idLower, partialLower) || strings.Contains(promptLower, partialLower) {
			matches = append(matches, string(wf.WorkflowID))
		}
	}
	return matches
}

// suggestThemes returns theme suggestions filtered by partial input.
func (m *Model) suggestThemes(partial string) []string {
	themes := []string{"dark", "light"}

	if partial == "" {
		return themes
	}

	partialLower := strings.ToLower(partial)
	var matches []string
	for _, theme := range themes {
		if strings.HasPrefix(theme, partialLower) {
			matches = append(matches, theme)
		}
	}
	return matches
}

// getWorkflowDescription returns the description for a workflow ID (for suggestions display).
func (m *Model) getWorkflowDescription(workflowID string) string {
	for _, wf := range m.workflowCache {
		if string(wf.WorkflowID) != workflowID {
			continue
		}
		// Format: [STATUS/PHASE] truncated prompt
		prompt := wf.Prompt
		maxLen := 55
		if len(prompt) > maxLen {
			prompt = prompt[:maxLen-3] + "..."
		}
		status := strings.ToUpper(string(wf.Status))
		phase := string(wf.CurrentPhase)
		if wf.IsActive {
			return fmt.Sprintf("* %s @%s: %s", status, phase, prompt)
		}
		return fmt.Sprintf("%s @%s: %s", status, phase, prompt)
	}
	return ""
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Dispatch workflow-related messages first
	if cmd, handled := m.handleWorkflowMsg(msg); handled {
		cmds = append(cmds, cmd)
	} else {
		// Handle system/UI events
		switch msg := msg.(type) {
		case tea.KeyMsg:
			newModel, cmd, handled := m.handleKeyMsg(msg)
			if handled {
				return newModel, cmd
			}

		case tea.MouseMsg:
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				newModel, cmd, handled := m.handleMouseClick(msg.X, msg.Y)
				if handled {
					return newModel, cmd
				}
			}

		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			m.recalculateLayout()

		case spinner.TickMsg:
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)

		case InputRequestMsg:
			m.pendingInputRequest = &msg.Request
			m.history.Add(NewSystemMessage(msg.Request.Prompt))
			if len(msg.Request.Options) > 0 {
				m.history.Add(NewSystemMessage("Options: " + strings.Join(msg.Request.Options, ", ")))
			}
			m.updateViewport()
			cmds = append(cmds, m.listenForInputRequests())

		case ExplorerRefreshMsg:
			_ = m.explorerPanel.Refresh()
			cmds = append(cmds, m.listenForExplorerChanges())

		case ChatProgressTickMsg:
			if m.streaming {
				cmds = append(cmds, m.chatProgressTick())
			}

		case StatsTickMsg:
			m.statsWidget.Update()
			stats := m.statsWidget.GetStats()
			resourceStats := ResourceStats{
				MemoryMB:      stats.MemoryMB,
				CPUPercent:    stats.CPUPercent,
				CPURawPercent: stats.CPURawPercent,
				Uptime:        stats.Uptime,
				Goroutines:    stats.Goroutines,
			}
			m.logsPanel.SetResourceStats(resourceStats)
			m.statsPanel.SetResourceStats(resourceStats)
			if m.machineCollector != nil {
				machineStats := m.machineCollector.Collect()
				m.logsPanel.SetMachineStats(machineStats)
				m.statsPanel.SetMachineStats(machineStats)
			}
			m.updateLogsPanelTokenStats()
			m.updateTokenPanelStats()
			cmds = append(cmds, statsTickCmd())

		case PanelNavTimeoutMsg:
			if m.panelNavMode && msg.Seq == m.panelNavSeq {
				if time.Now().After(m.panelNavTill) {
					m.panelNavMode = false
				}
			}

		case AgentResponseMsg:
			m.handleAgentResponse(msg)

		case ShellOutputMsg:
			m.handleShellOutput(msg)

		case editorFinishedMsg:
			if msg.err != nil {
				m.logsPanel.AddError("editor", fmt.Sprintf("Editor error: %v", msg.err))
			} else {
				m.logsPanel.AddSuccess("editor", fmt.Sprintf("Edited: %s", filepath.Base(msg.filePath)))
			}
			if m.explorerPanel != nil {
				_ = m.explorerPanel.Refresh()
			}

		case QuitMsg:
			m.quitting = true
			m.explorerPanel.Close()
			return m, tea.Quit
		}
	}

	// Update textarea
	var cmd tea.Cmd
	oldLines := m.calculateInputLines()
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	// Update suggestions AFTER textarea has processed the key
	m.updateSuggestions()

	// Recalculate layout if input height changed
	newLines := m.calculateInputLines()
	if newLines != oldLines && m.ready {
		m.recalculateLayout()
	}

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) handleSubmit() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textarea.Value())
	m.textarea.Reset()
	m.showSuggestions = false

	// Save to command history for Ctrl+H search
	if input != "" {
		m.historySearch.Add(input, m.currentAgent)
	}

	// Handle pending input request
	if m.pendingInputRequest != nil {
		if m.controlPlane != nil {
			_ = m.controlPlane.ProvideUserInput(m.pendingInputRequest.ID, input)
		}
		m.history.Add(NewUserMessage(input))
		m.pendingInputRequest = nil
		m.updateViewport()
		return m, nil
	}

	// Ignore empty input (only whitespace/newlines)
	if input == "" {
		return m, nil
	}

	// Handle shell command with ! prefix
	if strings.HasPrefix(input, "!") {
		shellCmd := strings.TrimPrefix(input, "!")
		shellCmd = strings.TrimSpace(shellCmd)
		if shellCmd != "" {
			return m.executeShellCommand(shellCmd)
		}
	}

	// Check for command
	cmd, args, isCmd := m.commands.Parse(input)
	if isCmd {
		m.history.Add(NewUserMessage(input))
		m.updateViewport()
		return m.handleCommand(cmd, args)
	}

	// Regular message
	m.history.Add(NewUserMessage(input))
	m.updateViewport()

	// Try to send to agent if available
	if m.agents != nil {
		agent := m.currentAgent
		if agent == "" {
			agent = "claude"
		}
		m.streaming = true
		m.chatStartedAt = time.Now()
		m.chatAgent = agent

		// Determine timeout (use configured value or default to 20 min)
		timeout := m.chatTimeout
		if timeout == 0 {
			timeout = 20 * time.Minute
		}

		// Create cancellable context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		m.cancelFunc = cancel

		// Log with detailed context info
		historyCount := len(m.history.All())
		model := m.currentModel
		if model == "" {
			model = "default"
		}
		m.chatModel = model
		logMsg := fmt.Sprintf("▶ %s [%s] (%d chars", agent, model, len(input))
		if historyCount > 1 {
			logMsg += fmt.Sprintf(", %d ctx msgs", historyCount-1)
		}
		logMsg += fmt.Sprintf(", timeout: %s)", formatDuration(timeout))
		m.logsPanel.AddInfo(strings.ToLower(agent), logMsg)

		// Start periodic progress tick
		return m, tea.Batch(m.spinner.Tick, m.sendToAgentWithCtx(ctx, input, agent))
	}

	// No agents configured
	return m, func() tea.Msg {
		return AgentResponseMsg{
			Agent:   "Quorum",
			Content: "No agents configured. Add agent credentials to your .quorum/config.yaml file.\n\nUse `/help` to see available commands.",
		}
	}
}

// ShellOutputMsg represents shell command output
type ShellOutputMsg struct {
	Command  string
	Output   string
	Error    string
	ExitCode int
}

// editorFinishedMsg is sent when the external editor closes
type editorFinishedMsg struct {
	filePath string
	err      error
}

// executeShellCommand runs a shell command and returns the output
func (m Model) executeShellCommand(cmdStr string) (tea.Model, tea.Cmd) {
	// Add user message showing the command
	m.history.Add(NewUserMessage("!" + cmdStr))
	m.updateViewport()

	m.logsPanel.AddInfo("shell", fmt.Sprintf("Executing: %s", cmdStr))

	// Return a command that will execute the shell command asynchronously
	return m, func() tea.Msg {
		// Use sh -c to handle pipes, redirects, etc.
		shPath, err := exec.LookPath("sh")
		if err != nil {
			return ShellOutputMsg{
				Command:  cmdStr,
				Error:    "sh not found in PATH",
				ExitCode: -1,
			}
		}
		cmd := exec.Command(shPath, "-c", cmdStr) // #nosec G204 -- shell path from environment

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()

		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				return ShellOutputMsg{
					Command:  cmdStr,
					Error:    err.Error(),
					ExitCode: -1,
				}
			}
		}

		output := stdout.String()
		errOutput := stderr.String()

		// Combine stdout and stderr
		if errOutput != "" {
			if output != "" {
				output = output + "\n" + errOutput
			} else {
				output = errOutput
			}
		}

		return ShellOutputMsg{
			Command:  cmdStr,
			Output:   output,
			ExitCode: exitCode,
		}
	}
}

// buildConversationMessages creates a core.Message array from conversation history.
// This is the preferred method for passing context to API-based adapters.
func (m Model) buildConversationMessages() []core.Message {
	messages := m.history.All()
	if len(messages) == 0 {
		return nil
	}

	// Include recent messages (limit to avoid token overflow)
	maxMessages := 20 // Keep last 20 messages for context
	start := 0
	if len(messages) > maxMessages {
		start = len(messages) - maxMessages
	}

	var result []core.Message
	for i := start; i < len(messages); i++ {
		msg := messages[i]
		switch msg.Role {
		case RoleUser:
			result = append(result, core.Message{
				Role:    "user",
				Content: msg.Content,
			})
		case RoleAgent:
			// Truncate very long responses to save tokens
			content := msg.Content
			if len(content) > 2000 {
				content = content[:2000] + "... [truncated]"
			}
			result = append(result, core.Message{
				Role:    "assistant",
				Content: content,
			})
			// Skip system messages in context
		}
	}

	return result
}

// sendToAgentWithCtx sends a message to the specified agent with a cancellable context.
func (m Model) sendToAgentWithCtx(ctx context.Context, input, agentName string) tea.Cmd {
	agents := m.agents
	// Capture values before the goroutine to avoid race conditions
	conversationMessages := m.buildConversationMessages()
	currentModel := m.currentModel // Pass the selected model to the agent

	return func() tea.Msg {
		agent, err := agents.Get(agentName)
		if err != nil {
			return AgentResponseMsg{
				Agent: "Quorum",
				Error: fmt.Errorf("agent %s not available: %w", agentName, err),
			}
		}

		opts := core.ExecuteOptions{
			Prompt:       input,
			SystemPrompt: quorumSystemPrompt,
			Messages:     conversationMessages, // Pass structured messages
			Model:        currentModel,         // Use selected model (empty = adapter default)
			Format:       core.OutputFormatText,
			Phase:        core.PhaseExecute,
		}

		result, err := agent.Execute(ctx, opts)
		if err != nil {
			// Check if it was cancelled
			if ctx.Err() == context.Canceled {
				return AgentResponseMsg{
					Agent: agentName,
					Error: fmt.Errorf("request cancelled"),
				}
			}
			return AgentResponseMsg{
				Agent: "Quorum",
				Error: err,
			}
		}

		return AgentResponseMsg{
			Agent:     agentName,
			Content:   result.Output,
			TokensIn:  result.TokensIn,
			TokensOut: result.TokensOut,
		}
	}
}

func (m Model) handleCommand(cmd *Command, args []string) (tea.Model, tea.Cmd) {
	addSystem := func(content string) {
		m.history.Add(NewSystemBubbleMessage(content))
	}

	switch cmd.Name {
	case "load":
		return m.handleCommandLoad(args, addSystem)
	case "new":
		return m.handleCommandNew(args, addSystem)
	case "delete":
		return m.handleCommandDelete(args, addSystem)
	case "plan":
		return m.handleCommandPlan(args, addSystem)
	case "execute":
		return m.handleCommandExecute(args, addSystem)
	case "replan":
		return m.handleCommandReplan(args, addSystem)
	case "useplan", "up", "useplans":
		return m.handleCommandUsePlan(args, addSystem)
	case "quit":
		m.quitting = true
		m.explorerPanel.Close()
		return m, tea.Quit
	}

	// Dispatch to UI commands (help, clear, model, agent, copy, logs, explorer, theme)
	if newModel, teaCmd, handled := m.handleCommandUI(cmd, args, addSystem); handled {
		return newModel, teaCmd
	}

	// Dispatch to workflow operation commands (status, workflows, cancel, analyze, run, retry)
	if newModel, teaCmd, handled := m.handleCommandWorkflowOps(cmd, args, addSystem); handled {
		return newModel, teaCmd
	}

	return m, nil
}

func (m *Model) updateViewport() {
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
}

// updateLogsPanelTokenStats updates the logs panel with current token stats
func (m *Model) updateLogsPanelTokenStats() {
	var tokenStats []TokenStats

	// Collect from agentInfos
	for _, agent := range m.agentInfos {
		if agent.TokensIn > 0 || agent.TokensOut > 0 {
			tokenStats = append(tokenStats, TokenStats{
				Model:     agent.Name,
				TokensIn:  agent.TokensIn,
				TokensOut: agent.TokensOut,
			})
		}
	}

	// Include workflow tokens to match header display
	if m.totalTokensIn > 0 || m.totalTokensOut > 0 {
		tokenStats = append(tokenStats, TokenStats{
			Model:     "workflow",
			TokensIn:  m.totalTokensIn,
			TokensOut: m.totalTokensOut,
		})
	}

	m.logsPanel.SetTokenStats(tokenStats)
}

// updateTokenPanelStats updates the token panel with current token usage.
func (m *Model) updateTokenPanelStats() {
	if m.tokenPanel == nil {
		return
	}
	entries := m.collectTokenEntries()
	m.tokenPanel.SetEntries(entries)
}

func (m *Model) collectTokenEntries() []TokenEntry {
	var entries []TokenEntry

	// Chat-level usage (agent totals)
	for _, agent := range m.agentInfos {
		if agent.TokensIn == 0 && agent.TokensOut == 0 {
			continue
		}
		model := agent.Model
		if model == "" {
			model = "default"
		}
		entries = append(entries, TokenEntry{
			Scope:     "chat",
			CLI:       strings.ToLower(agent.Name),
			Model:     model,
			Phase:     "chat",
			TokensIn:  agent.TokensIn,
			TokensOut: agent.TokensOut,
		})
	}

	// Workflow usage (tasks aggregated by cli/model/phase)
	if m.workflowState != nil && len(m.workflowState.Tasks) > 0 {
		type key struct {
			cli   string
			model string
			phase string
		}
		agg := map[key]*TokenEntry{}
		for _, task := range m.workflowState.Tasks {
			if task.TokensIn == 0 && task.TokensOut == 0 {
				continue
			}
			cli := strings.ToLower(task.CLI)
			if cli == "" {
				cli = "unknown"
			}
			model := task.Model
			if model == "" {
				model = "default"
			}
			phase := string(task.Phase)
			if phase == "" {
				phase = "workflow"
			}
			k := key{cli: cli, model: model, phase: phase}
			entry, ok := agg[k]
			if !ok {
				entry = &TokenEntry{
					Scope: "workflow",
					CLI:   cli,
					Model: model,
					Phase: phase,
				}
				agg[k] = entry
			}
			entry.TokensIn += task.TokensIn
			entry.TokensOut += task.TokensOut
		}
		for _, entry := range agg {
			entries = append(entries, *entry)
		}
	}

	return entries
}

// calculateInputLines calculates how many lines the input textarea needs
func (m *Model) calculateInputLines() int {
	content := m.textarea.Value()
	if content == "" {
		return minInputLines
	}

	// Count actual newlines in content
	lines := 0

	// Calculate wrapped lines for each line segment
	textareaWidth := m.textarea.Width()
	if textareaWidth <= 0 {
		textareaWidth = 80 // Fallback
	}

	for _, line := range strings.Split(content, "\n") {
		lineWidth := lipgloss.Width(line)
		if lineWidth == 0 {
			// Empty line still takes 1 line
			lines++
		} else {
			// Calculate how many visual lines this content line needs
			// Formula: ceil(lineWidth / textareaWidth) = (lineWidth + textareaWidth - 1) / textareaWidth
			wrappedLines := (lineWidth + textareaWidth - 1) / textareaWidth
			lines += wrappedLines
		}
	}

	// Clamp to min/max
	if lines < minInputLines {
		lines = minInputLines
	}
	if lines > maxInputLines {
		lines = maxInputLines
	}

	return lines
}

// recalculateLayout recalculates viewport and logs panel sizes
// This ensures exact calculations by accounting for all borders, padding, and dynamic elements
func (m *Model) recalculateLayout() {
	// === WIDTH CALCULATIONS ===
	leftSidebarWidth, rightSidebarWidth, mainWidth := m.calculateSidebarWidths()
	leftSidebarWidth, rightSidebarWidth, mainWidth = m.normalizePanelWidths(leftSidebarWidth, rightSidebarWidth, mainWidth)

	// === SET TEXTAREA WIDTH FIRST (needed for accurate line calculation) ===
	inputWidth := mainWidth - 8
	if inputWidth < 20 {
		inputWidth = 20
	}
	m.textarea.SetWidth(inputWidth)

	// === EXACT HEIGHT CALCULATIONS ===
	headerHeight := 3
	if m.workflowRunning || m.workflowPhase == "done" {
		headerHeight += 2
	}
	footerHeight := 2
	inputLines := m.calculateInputLines()
	inputHeight := inputLines + 3

	statusHeight := 0
	if m.workflowRunning {
		statusHeight = len(m.agentInfos)
		if statusHeight < 1 {
			statusHeight = 1
		}
	} else if m.streaming {
		statusHeight = 1
	}

	fixedHeight := headerHeight + footerHeight + inputHeight + statusHeight
	viewportHeight := m.height - fixedHeight - 2
	if viewportHeight < 3 {
		viewportHeight = 3
	}

	// === SIDEBAR SIZES ===
	sidebarHeight := m.height
	if sidebarHeight < 10 {
		sidebarHeight = 10
	}
	m.applySidebarSizes(leftSidebarWidth, rightSidebarWidth, sidebarHeight)

	// === VIEWPORT SETUP ===
	if !m.ready {
		m.viewport = viewport.New(mainWidth, viewportHeight)
		m.viewport.SetContent(m.renderHistory())
		m.ready = true
	} else {
		m.viewport.Width = mainWidth
		m.viewport.Height = viewportHeight
	}

	m.textarea.SetHeight(inputLines)
	contentWidth := mainWidth - 4
	m.updateMarkdownRenderer(contentWidth)
	m.messageStyles = NewMessageStyles(m.viewport.Width)
}

func (m Model) runWorkflow(prompt string) tea.Cmd {
	runner := m.runner
	return tea.Batch(
		func() tea.Msg { return WorkflowStartedMsg{Prompt: prompt} },
		func() tea.Msg {
			ctx := context.Background()
			err := runner.Run(ctx, prompt)
			if err != nil {
				return WorkflowErrorMsg{Error: err}
			}
			state, _ := runner.GetState(ctx)
			return WorkflowCompletedMsg{State: state}
		},
	)
}

func (m Model) runAnalyze(prompt string) tea.Cmd {
	runner := m.runner
	return tea.Batch(
		func() tea.Msg { return WorkflowStartedMsg{Prompt: prompt} },
		func() tea.Msg {
			ctx := context.Background()
			err := runner.Analyze(ctx, prompt)
			if err != nil {
				return WorkflowErrorMsg{Error: err}
			}
			state, _ := runner.GetState(ctx)
			return WorkflowCompletedMsg{State: state}
		},
	)
}

// runPlanPhase continues the workflow from the plan phase.
// Uses Plan() instead of Resume() to ensure only planning runs, not execution.
func (m Model) runPlanPhase() tea.Cmd {
	runner := m.runner
	return tea.Batch(
		func() tea.Msg { return WorkflowStartedMsg{Prompt: "(continuing from analysis)"} },
		func() tea.Msg {
			ctx := context.Background()
			err := runner.Plan(ctx)
			if err != nil {
				return WorkflowErrorMsg{Error: err}
			}
			state, _ := runner.GetState(ctx)
			return WorkflowCompletedMsg{State: state}
		},
	)
}

// runReplanPhase clears existing plan data and re-runs planning.
// Optionally prepends additional context to the consolidated analysis.
func (m Model) runReplanPhase(additionalContext string) tea.Cmd {
	runner := m.runner
	return tea.Batch(
		func() tea.Msg { return WorkflowStartedMsg{Prompt: "(replanning)"} },
		func() tea.Msg {
			ctx := context.Background()
			err := runner.Replan(ctx, additionalContext)
			if err != nil {
				return WorkflowErrorMsg{Error: err}
			}
			state, _ := runner.GetState(ctx)
			return WorkflowCompletedMsg{State: state}
		},
	)
}

// runUsePlanPhase loads existing task files from filesystem without re-running the agent.
func (m Model) runUsePlanPhase() tea.Cmd {
	runner := m.runner
	return tea.Batch(
		func() tea.Msg { return WorkflowStartedMsg{Prompt: "(using existing task files)"} },
		func() tea.Msg {
			ctx := context.Background()
			err := runner.UsePlan(ctx)
			if err != nil {
				return WorkflowErrorMsg{Error: err}
			}
			state, _ := runner.GetState(ctx)
			return WorkflowCompletedMsg{State: state}
		},
	)
}

// runExecutePhase continues the workflow from the execute phase.
func (m Model) runExecutePhase() tea.Cmd {
	runner := m.runner
	return tea.Batch(
		func() tea.Msg { return WorkflowStartedMsg{Prompt: "(continuing from plan)"} },
		func() tea.Msg {
			ctx := context.Background()
			err := runner.Resume(ctx)
			if err != nil {
				return WorkflowErrorMsg{Error: err}
			}
			state, _ := runner.GetState(ctx)
			return WorkflowCompletedMsg{State: state}
		},
	)
}

func (m Model) renderHistory() string {
	var sb strings.Builder
	msgs := m.history.All()

	if len(msgs) == 0 {
		// Version banner
		versionText := "quorum-ai"
		if m.version != "" {
			versionText = "quorum-ai " + m.version
		}
		versionStyle := lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)
		sb.WriteString("\n" + versionStyle.Render(versionText) + "\n\n")

		// Welcome message
		welcome := lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true).
			Render("Type /help for commands or start chatting.")
		sb.WriteString(welcome + "\n")
		return sb.String()
	}

	// Use new message styles
	styles := m.messageStyles
	if styles == nil {
		styles = NewMessageStyles(m.viewport.Width)
	}

	for _, msg := range msgs {
		// Format timestamp
		timestamp := msg.Timestamp.Format("15:04")

		switch msg.Role {
		case RoleUser:
			// User message: right-aligned bubble
			isCommand := strings.HasPrefix(strings.TrimSpace(msg.Content), "/")
			rendered := styles.FormatUserMessage(msg.Content, timestamp, isCommand)
			sb.WriteString(rendered)
			sb.WriteString("\n\n")

		case RoleAgent:
			// Agent/Bot message: left-aligned with thick border
			content := msg.Content
			if m.mdRenderer != nil {
				if rendered, err := m.mdRenderer.Render(msg.Content); err == nil {
					content = strings.TrimSpace(rendered)
				}
			}

			// Extract consensus from metadata if available
			consensus := 0
			if msg.Metadata != nil {
				switch c := msg.Metadata["consensus"].(type) {
				case int:
					consensus = c
				case float64:
					consensus = int(c)
				}
			}

			// Get agent name, default to "Quorum"
			agentName := msg.Agent
			if agentName == "" {
				agentName = "Quorum"
			}

			rendered := styles.FormatBotMessage(agentName, content, timestamp, consensus, "")
			sb.WriteString(rendered)
			sb.WriteString("\n\n")

		case RoleSystem:
			// System message: subtle styling (or bubble if requested)
			rendered := ""
			if msg.Metadata != nil {
				if bubble, ok := msg.Metadata["bubble"].(bool); ok && bubble {
					rendered = styles.FormatSystemBubbleMessage(msg.Content, timestamp)
				}
			}
			if rendered == "" {
				rendered = styles.FormatSystemMessage(msg.Content)
			}
			sb.WriteString(rendered)
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}

// View renders the UI.
func (m Model) View() string {
	w := m.width
	h := m.height
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}

	// Helper to ensure output covers full viewport
	ensureFullScreen := func(content string) string {
		return lipgloss.Place(
			w, h,
			lipgloss.Left,
			lipgloss.Top,
			content,
		)
	}

	if m.quitting {
		return ensureFullScreen(lipgloss.NewStyle().Foreground(dimColor).Render("Goodbye!"))
	}

	if !m.ready {
		initStyle := lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)
		return ensureFullScreen(initStyle.Render(m.spinner.View() + " Initializing Quorum..."))
	}

	// === CALCULATE PANEL WIDTHS ===
	// Use actual panel widths (set by recalculateLayout) to ensure consistency
	leftSidebarWidth := 0
	rightSidebarWidth := 0 // Shared by stats and logs panels

	showLeftSidebar := m.showExplorer || m.showTokens
	if m.showExplorer {
		leftSidebarWidth = m.explorerPanel.Width()
	} else if m.showTokens {
		leftSidebarWidth = m.tokenPanel.Width()
	}

	// Right sidebar width comes from whichever panel is visible (they share the same width)
	showRightSidebar := m.showStats || m.showLogs
	if m.showStats {
		rightSidebarWidth = m.statsPanel.Width()
	} else if m.showLogs {
		rightSidebarWidth = m.logsPanel.Width()
	}

	// Calculate mainWidth based on remaining space after sidebars
	// No outer margins - panels fill entire width when joined with JoinHorizontal
	mainWidth := w
	if showLeftSidebar {
		mainWidth -= leftSidebarWidth
	}
	if showRightSidebar {
		mainWidth -= rightSidebarWidth
	}

	// Ensure minimum main width
	if mainWidth < 40 {
		mainWidth = 40
	}

	// === RENDER PANELS ===
	var panels []string

	// Left sidebar (explorer and/or tokens)
	if showLeftSidebar {
		var leftSidebarContent string
		if m.showExplorer && m.showTokens {
			explorerPanel := m.explorerPanel.Render()
			tokenPanel := m.tokenPanel.RenderWithFocus(m.tokensFocus)
			leftSidebarContent = lipgloss.JoinVertical(lipgloss.Left, explorerPanel, tokenPanel)
		} else if m.showExplorer {
			leftSidebarContent = m.explorerPanel.Render()
		} else {
			leftSidebarContent = m.tokenPanel.RenderWithFocus(m.tokensFocus)
		}
		panels = append(panels, leftSidebarContent)
	}

	// Main content (center) - wrapped with border like sidebars
	mainContent := m.renderMainContent(mainWidth - 2) // Content width = total - borders
	// Box style with rounded borders to match sidebars
	// lipgloss Width/Height set CONTENT size, borders are added OUTSIDE.
	// Formula: Width(X-2) + borders(2) = total X
	// DO NOT use MaxWidth/MaxHeight - they truncate AFTER borders, cutting them off.
	mainContentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(mainWidth - 2).
		Height(h - 2)
	panels = append(panels, mainContentStyle.Render(mainContent))

	// Right sidebar (stats and/or logs)
	if showRightSidebar {

		// Render right sidebar content
		var rightSidebarContent string
		if m.showStats && m.showLogs {
			// Stack logs on top, stats on bottom
			logsPanel := m.logsPanel.RenderWithFocus(m.logsFocus)
			statsPanel := m.statsPanel.RenderWithFocus(m.statsFocus)
			rightSidebarContent = lipgloss.JoinVertical(lipgloss.Left, logsPanel, statsPanel)
		} else if m.showStats {
			rightSidebarContent = m.statsPanel.RenderWithFocus(m.statsFocus)
		} else {
			rightSidebarContent = m.logsPanel.RenderWithFocus(m.logsFocus)
		}
		panels = append(panels, rightSidebarContent)
	}

	baseView := lipgloss.JoinHorizontal(lipgloss.Top, panels...)

	// === RENDER OVERLAYS ON TOP ===
	// Overlays are rendered on top of the base view

	// Shortcuts overlay (highest priority) - true full-screen modal
	if m.shortcutsOverlay.IsVisible() {
		m.shortcutsOverlay.SetSize(w-20, h-10)
		overlay := m.shortcutsOverlay.Render()
		return ensureFullScreen(m.renderFullScreenModal(overlay, w, h))
	}

	// History search overlay
	if m.historySearch.IsVisible() {
		m.historySearch.SetSize(w-30, h/2)
		overlay := m.historySearch.Render()
		return ensureFullScreen(m.overlayOnBase(baseView, overlay, w, h))
	}

	// Diff view overlay
	if m.diffView.IsVisible() {
		m.diffView.SetSize(w-20, h-10)
		overlay := m.diffView.Render()
		return ensureFullScreen(m.overlayOnBase(baseView, overlay, w, h))
	}

	// NOTE: Cost panel and Stats widget overlays removed - now integrated into LogsPanel footer

	// Consensus panel overlay
	if m.consensusPanel.IsVisible() {
		m.consensusPanel.SetSize(w-30, h-12)
		overlay := m.consensusPanel.Render()
		return ensureFullScreen(m.overlayOnBase(baseView, overlay, w, h))
	}

	// Tasks panel overlay
	if m.tasksPanel.IsVisible() {
		m.tasksPanel.SetSize(w-30, h-12)
		overlay := m.tasksPanel.Render()
		return ensureFullScreen(m.overlayOnBase(baseView, overlay, w, h))
	}

	// File viewer overlay
	if m.fileViewer.IsVisible() {
		m.fileViewer.SetSize(w-16, h-8)
		overlay := m.fileViewer.Render()
		return ensureFullScreen(m.overlayOnBase(baseView, overlay, w, h))
	}

	// Command suggestions dropdown overlay (positioned above input/footer)
	if m.showSuggestions && len(m.suggestions) > 0 {
		suggestionsOverlay := m.renderInlineSuggestions(mainWidth)
		// Calculate position: above the footer area
		// Footer is 1 line, input is ~3 lines, plus extra margin
		footerOffset := 6
		baseView = m.overlayAtBottom(baseView, suggestionsOverlay, w, h, leftSidebarWidth, footerOffset)
	}

	return ensureFullScreen(baseView)
}

// extractTokenValue safely extracts a token count from event data, handling different types
func extractTokenValue(data map[string]any, key string) int {
	v, ok := data[key]
	if !ok {
		return 0
	}

	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case float32:
		return int(val)
	default:
		return 0
	}
}

// dimLine applies ANSI dim effect to a line while preserving existing styles
func dimLine(line string) string {
	// ANSI dim: \x1b[2m, reset intensity: \x1b[22m
	// We wrap the entire line to dim it
	return "\x1b[2m" + line + "\x1b[22m"
}

// overlayOnBase renders an overlay centered on a dimmed version of the base UI
func (m Model) overlayOnBase(base, overlay string, width, height int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Ensure base has enough lines
	for len(baseLines) < height {
		baseLines = append(baseLines, strings.Repeat(" ", width))
	}

	// Calculate overlay dimensions
	overlayHeight := len(overlayLines)
	overlayWidth := 0
	for _, line := range overlayLines {
		if w := lipgloss.Width(line); w > overlayWidth {
			overlayWidth = w
		}
	}

	// Calculate centered position
	startY := (height - overlayHeight) / 2
	startX := (width - overlayWidth) / 2
	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	// Build result with dimmed base and overlay on top
	var result []string
	for i := 0; i < len(baseLines); i++ {
		if i >= startY && i < startY+overlayHeight {
			// This line has overlay content
			overlayIdx := i - startY
			if overlayIdx < len(overlayLines) {
				overlayLine := overlayLines[overlayIdx]
				// Dim the base, then place overlay
				dimmedBase := dimLine(baseLines[i])
				// Create the composite line: dimmed left + overlay + dimmed right
				leftPad := strings.Repeat(" ", startX)
				result = append(result, leftPad+overlayLine)
				_ = dimmedBase // For lines with overlay, we just show the overlay
			} else {
				result = append(result, dimLine(baseLines[i]))
			}
		} else {
			// No overlay on this line - just dim it
			result = append(result, dimLine(baseLines[i]))
		}
	}

	return strings.Join(result, "\n")
}

// overlayAtBottom renders an overlay positioned at the bottom of the screen,
// above a specified offset (for footer/input area). Does NOT dim the base.
// IMPORTANT: This preserves sidebar content by only replacing the main content area.
func (m Model) overlayAtBottom(base, overlay string, width, height, leftOffset, bottomOffset int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Ensure base has enough lines
	for len(baseLines) < height {
		baseLines = append(baseLines, strings.Repeat(" ", width))
	}

	// Calculate overlay dimensions
	overlayHeight := len(overlayLines)
	overlayWidth := 0
	for _, line := range overlayLines {
		if w := lipgloss.Width(line); w > overlayWidth {
			overlayWidth = w
		}
	}

	// Position at bottom, above the footer offset
	startY := height - overlayHeight - bottomOffset
	startX := leftOffset + 2 // Account for separator and some padding
	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	// Build result with overlay on top (no dimming for dropdown)
	// IMPORTANT: Preserve sidebar content by extracting visible characters
	var result []string
	for i := 0; i < len(baseLines); i++ {
		if i >= startY && i < startY+overlayHeight {
			// This line has overlay content
			overlayIdx := i - startY
			if overlayIdx < len(overlayLines) {
				overlayLine := overlayLines[overlayIdx]
				baseLine := baseLines[i]
				baseWidth := lipgloss.Width(baseLine)
				overlayLineWidth := lipgloss.Width(overlayLine)

				// Extract left sidebar content (preserve ANSI codes)
				leftPart := truncateWithAnsi(baseLine, startX)

				// Calculate right sidebar start position
				rightStart := startX + overlayLineWidth

				// Extract right sidebar content
				rightPart := ""
				if baseWidth > rightStart {
					rightPart = skipCharsWithAnsi(baseLine, rightStart)
				}

				// Pad overlay to ensure alignment
				paddedOverlay := overlayLine
				if lipgloss.Width(paddedOverlay) < overlayLineWidth {
					paddedOverlay += strings.Repeat(" ", overlayLineWidth-lipgloss.Width(paddedOverlay))
				}

				result = append(result, leftPart+paddedOverlay+rightPart)
			} else {
				result = append(result, baseLines[i])
			}
		} else {
			// No overlay on this line
			result = append(result, baseLines[i])
		}
	}

	return strings.Join(result, "\n")
}

// truncateWithAnsi truncates a string to maxWidth visible characters, preserving ANSI codes
func truncateWithAnsi(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	var result strings.Builder
	visibleWidth := 0
	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			result.WriteRune(r)
			continue
		}
		if inEscape {
			result.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}

		charWidth := lipgloss.Width(string(r))
		if visibleWidth+charWidth > maxWidth {
			break
		}
		result.WriteRune(r)
		visibleWidth += charWidth
	}

	// Pad to exact width if needed
	for visibleWidth < maxWidth {
		result.WriteRune(' ')
		visibleWidth++
	}

	return result.String()
}

// skipCharsWithAnsi skips the first skipWidth visible characters and returns the rest
func skipCharsWithAnsi(s string, skipWidth int) string {
	if skipWidth <= 0 {
		return s
	}

	var result strings.Builder
	visibleWidth := 0
	inEscape := false
	skipping := true

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			if !skipping {
				result.WriteRune(r)
			}
			continue
		}
		if inEscape {
			if !skipping {
				result.WriteRune(r)
			}
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}

		charWidth := lipgloss.Width(string(r))
		if skipping {
			visibleWidth += charWidth
			if visibleWidth >= skipWidth {
				skipping = false
			}
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// renderFullScreenModal renders a modal that covers the entire viewport
// with a solid background, centering the content box
func (m Model) renderFullScreenModal(content string, width, height int) string {
	contentLines := strings.Split(content, "\n")

	// Calculate content dimensions
	contentHeight := len(contentLines)
	contentWidth := 0
	for _, line := range contentLines {
		if w := lipgloss.Width(line); w > contentWidth {
			contentWidth = w
		}
	}

	// Calculate centered position
	startY := (height - contentHeight) / 2
	startX := (width - contentWidth) / 2
	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	// Background style - use plain spaces for padding
	bgStyle := lipgloss.NewStyle()

	// Build full-screen result
	var result []string
	for i := 0; i < height; i++ {
		if i >= startY && i < startY+contentHeight {
			// This line has content
			contentIdx := i - startY
			if contentIdx < len(contentLines) {
				contentLine := contentLines[contentIdx]
				lineWidth := lipgloss.Width(contentLine)
				// Left padding with background
				leftPad := bgStyle.Render(strings.Repeat(" ", startX))
				// Right padding with background
				rightPadWidth := width - startX - lineWidth
				if rightPadWidth < 0 {
					rightPadWidth = 0
				}
				rightPad := bgStyle.Render(strings.Repeat(" ", rightPadWidth))
				result = append(result, leftPad+contentLine+rightPad)
			} else {
				// Empty line with background
				result = append(result, bgStyle.Render(strings.Repeat(" ", width)))
			}
		} else {
			// No content on this line - just solid background
			result = append(result, bgStyle.Render(strings.Repeat(" ", width)))
		}
	}

	return strings.Join(result, "\n")
}

// renderMainContent renders the main chat area
func (m Model) renderMainContent(w int) string {
	var sb strings.Builder

	// === HEADER (with integrated tab-style agent bar) ===
	sb.WriteString(m.renderHeader(w))
	sb.WriteString("\n")

	// === SUBTLE DIVIDER ===
	divider := lipgloss.NewStyle().
		Foreground(borderColor).
		Render(strings.Repeat("─", w-2))
	sb.WriteString(divider)
	sb.WriteString("\n")

	// NOTE: Workflow progress bar removed - redundant with agent bar indicators
	// NOTE: Consensus panel is now rendered as overlay in View()
	// NOTE: Pipeline visualization removed - redundant with agent bar

	// === CHAT VIEWPORT ===
	sb.WriteString(m.viewport.View())
	sb.WriteString("\n")

	// === STATUS LINE / PROGRESS BARS (when streaming/running) ===
	if m.workflowRunning {
		// Show progress bars for all agents with real-time activity
		progressBars := RenderAgentProgressBars(m.agentInfos, w-4)
		sb.WriteString("  " + strings.ReplaceAll(progressBars, "\n", "\n  ") + "\n")
	} else if m.streaming {
		elapsed := time.Since(m.chatStartedAt)
		agent := m.chatAgent
		if agent == "" {
			agent = "agent"
		}
		statusLine := lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true).
			Render(fmt.Sprintf("%s %s thinking... %s", m.spinner.View(), agent, formatDuration(elapsed)))
		sb.WriteString("  " + statusLine + "\n")
	}

	// === INPUT AREA ===
	sb.WriteString(m.renderInput(w))
	sb.WriteString("\n")

	// NOTE: Command suggestions are now rendered as an overlay in View()
	// to prevent layout shifts when the dropdown appears

	// === FOOTER ===
	sb.WriteString(m.renderFooter(w))

	return sb.String()
}

func (m Model) renderHeader(width int) string {
	// === LOGO SECTION ===
	logo := logoStyle.Render("◆ Quorum")

	// === TAB-STYLE AGENT BAR ===
	var tabs []string
	for _, agent := range m.agentInfos {
		var tab string
		name := agent.Name

		switch agent.Status {
		case AgentStatusDisabled:
			tab = tabInactiveStyle.Render(iconDotHollow + " " + name)
		case AgentStatusIdle:
			tab = tabInactiveStyle.Foreground(agent.Color).Render(iconDot + " " + name)
		case AgentStatusRunning:
			tab = tabRunningStyle.Render(iconSpinner + " " + name)
		case AgentStatusDone:
			tokenStr := ""
			totalTokens := agent.TokensIn + agent.TokensOut
			if totalTokens > 0 {
				tokenStr = fmt.Sprintf(" (%d)", totalTokens)
			}
			tab = tabActiveStyle.Background(agent.Color).Render(iconCheck + " " + name + tokenStr)
		case AgentStatusError:
			tab = tabErrorStyle.Render(iconCross + " " + name)
		}
		tabs = append(tabs, tab)
	}

	// Join tabs with separators: ‹ Agent1 │ Agent2 │ Agent3 ›
	separator := tabSeparatorStyle.Render(" │ ")
	tabBar := ""
	if len(tabs) > 0 {
		tabBar = tabSeparatorStyle.Render(iconChevronLeft) + " " +
			strings.Join(tabs, separator) + " " +
			tabSeparatorStyle.Render(iconChevronRight)
	}

	// === STATS SECTION ===
	statsStyle := lipgloss.NewStyle().Foreground(dimColor)
	valueStyle := lipgloss.NewStyle().Foreground(textColor)

	var stats []string

	// Tokens (↑out to LLM, ↓in from LLM)
	var tokensIn, tokensOut int
	for _, a := range m.agentInfos {
		tokensIn += a.TokensIn
		tokensOut += a.TokensOut
	}
	tokensIn += m.totalTokensIn
	tokensOut += m.totalTokensOut
	if tokensIn > 0 || tokensOut > 0 {
		stats = append(stats, statsStyle.Render("tok:")+valueStyle.Render(fmt.Sprintf("↑%d ↓%d", tokensIn, tokensOut)))
	}

	// Status badge
	var badge string
	_, _, _, runningAgent := GetStats(m.agentInfos)
	if m.workflowRunning && runningAgent != "" {
		badge = lipgloss.NewStyle().
			Background(warningColor).
			Foreground(lipgloss.Color("#000")).
			Padding(0, 1).
			Bold(true).
			Render(iconSpinner + " " + runningAgent)
	} else if m.workflowPhase == "done" {
		badge = lipgloss.NewStyle().
			Background(successColor).
			Foreground(lipgloss.Color("#000")).
			Padding(0, 1).
			Bold(true).
			Render(iconCheck + " done")
	} else if m.streaming {
		elapsed := time.Since(m.chatStartedAt)
		agent := m.chatAgent
		if agent == "" {
			agent = "thinking"
		}
		badge = lipgloss.NewStyle().
			Background(secondaryColor).
			Foreground(lipgloss.Color("#000")).
			Padding(0, 1).
			Bold(true).
			Render(fmt.Sprintf("%s %s %s", iconSpinner, agent, formatDuration(elapsed)))
	}

	// === COMPOSE HEADER LINE ===
	rightParts := []string{}
	if len(stats) > 0 {
		rightParts = append(rightParts, strings.Join(stats, statsStyle.Render(" │ ")))
	}
	if badge != "" {
		rightParts = append(rightParts, badge)
	}
	rightSection := strings.Join(rightParts, "  ")

	// Calculate spacing for logo + tabBar on left, stats + badge on right
	leftSection := logo + "  " + tabBar
	gap := width - lipgloss.Width(leftSection) - lipgloss.Width(rightSection) - 2
	if gap < 1 {
		gap = 1
	}

	return leftSection + strings.Repeat(" ", gap) + rightSection
}

func (m Model) renderInput(width int) string {
	style := inputContainerStyle
	inputValue := m.textarea.Value()

	// Detect input mode and set appropriate style
	if strings.HasPrefix(inputValue, "!") {
		// Shell command mode - orange border
		style = inputShellStyle
	} else if m.inputFocused {
		style = inputActiveStyle
	}

	prefix := ""
	if m.workflowRunning {
		prefix = m.spinner.View() + " "
	} else if m.pendingInputRequest != nil {
		prefix = lipgloss.NewStyle().Foreground(warningColor).Render("? ")
	} else if strings.HasPrefix(inputValue, "!") {
		// Shell indicator
		prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("#f97316")).Bold(true).Render(" ") + " "
	}

	input := prefix + m.textarea.View()

	// style.Width sets CONTENT width (inside border+padding)
	// width is mainWidth - 2 (from renderMainContent)
	// Container needs: border(2) + padding(2) = 4 chars overhead
	// So content width = width - 4 for the total container to fit
	return style.Width(width - 4).Render(input)
}

// renderInlineSuggestions renders command suggestions as a dropdown below input
func (m Model) renderInlineSuggestions(width int) string {
	if !m.showSuggestions || len(m.suggestions) == 0 {
		return ""
	}

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true)

	selectedStyle := lipgloss.NewStyle().
		Foreground(textColor).
		Background(primaryColor).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(dimColor)

	descStyle := lipgloss.NewStyle().
		Foreground(mutedColor)

	scrollStyle := lipgloss.NewStyle().
		Foreground(secondaryColor).
		Italic(true)

	// Calculate visible window
	maxShow := 6
	total := len(m.suggestions)

	start := 0
	if m.suggestionIndex >= maxShow {
		start = m.suggestionIndex - maxShow + 1
	}
	end := start + maxShow
	if end > total {
		end = total
		start = end - maxShow
		if start < 0 {
			start = 0
		}
	}

	// Build dropdown content
	var lines []string

	// Calculate dropdownWidth FIRST (before using it for maxDescWidth)
	dropdownWidth := width - 6
	if dropdownWidth < 40 {
		dropdownWidth = 40
	}
	if dropdownWidth > 70 {
		dropdownWidth = 70
	}

	// Determine header and item type label based on suggestion type
	headerText := "Commands"
	itemType := "commands"
	showDescription := true

	switch m.suggestionType {
	case "agent":
		headerText = "Agents"
		itemType = "agents"
		showDescription = false
	case "model":
		headerText = fmt.Sprintf("Models for %s", m.currentAgent)
		if m.currentAgent == "" {
			headerText = "Models for claude"
		}
		itemType = "models"
		showDescription = false
	case "workflow":
		headerText = "Workflows"
		itemType = "workflows"
		showDescription = false
	case "theme":
		headerText = "Themes"
		itemType = "themes"
		showDescription = false
	}

	// Scroll up indicator
	if start > 0 {
		lines = append(lines, scrollStyle.Render(fmt.Sprintf("  ↑ %d more %s above", start, itemType)))
	}

	// Calculate max item width for alignment
	maxItemWidth := 12
	for i := start; i < end; i++ {
		itemLen := lipgloss.Width(m.suggestions[i]) + 1
		if itemLen > maxItemWidth {
			maxItemWidth = itemLen
		}
	}

	// Calculate maxDescWidth using dropdownWidth (not width)
	maxDescWidth := dropdownWidth - maxItemWidth - 12
	if maxDescWidth < 10 {
		maxDescWidth = 10
	}

	// Suggestion items
	for i := start; i < end; i++ {
		itemName := m.suggestions[i]
		desc := ""

		// For commands, show description; for agents/models/workflows, show status info
		if showDescription {
			cmd := m.commands.Get(itemName)
			itemName = "/" + itemName
			if cmd != nil {
				desc = cmd.Description
			}
		} else if m.suggestionType == "agent" {
			// Mark current agent
			if strings.EqualFold(itemName, m.currentAgent) {
				desc = "(current)"
			}
		} else if m.suggestionType == "model" {
			// Mark current model
			if strings.EqualFold(itemName, m.currentModel) {
				desc = "(current)"
			}
		} else if m.suggestionType == "workflow" {
			// Show workflow status and truncated prompt
			desc = m.getWorkflowDescription(itemName)
		}

		// Truncate description if needed
		if maxDescWidth > 0 && len(desc) > maxDescWidth {
			desc = desc[:maxDescWidth-3] + "..."
		}

		var line string
		if i == m.suggestionIndex {
			// Selected item with visual highlight (bold + reverse video)
			fullLine := fmt.Sprintf(" ▸ %-*s %s", maxItemWidth, itemName, desc)
			rowStyle := selectedStyle.Reverse(true)
			line = rowStyle.Render(fullLine)
		} else {
			fullLine := fmt.Sprintf("   %-*s %s", maxItemWidth, itemName, desc)
			line = normalStyle.Render(fullLine)
		}
		lines = append(lines, line)
	}

	// Scroll down indicator
	if end < total {
		lines = append(lines, scrollStyle.Render(fmt.Sprintf("  ↓ %d more %s below", total-end, itemType)))
	}

	content := strings.Join(lines, "\n")

	// Box style (dropdownWidth already calculated above)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(0, 1).
		Width(dropdownWidth)

	// Header
	header := headerStyle.Render(headerText)
	headerLine := " " + header + " " + descStyle.Render(fmt.Sprintf("(%d/%d)", m.suggestionIndex+1, total))

	return headerLine + "\n" + boxStyle.Render(content)
}

func (m Model) renderFooter(width int) string {
	// Keyboard hint styles
	keyHintStyle := lipgloss.NewStyle().
		Foreground(dimColor).
		Background(surfaceColor).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Foreground(mutedColor)

	separatorStyle := lipgloss.NewStyle().
		Foreground(borderColor)

	var keys []string

	if m.panelNavMode {
		// Panel navigation mode indicator (tmux-style)
		navModeStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fbbf24")). // Amber/yellow
			Bold(true)
		keys = []string{
			navModeStyle.Render("⎘ PANEL NAV"),
			keyHintStyle.Render("←→") + labelStyle.Render(" sides"),
			keyHintStyle.Render("↑↓") + labelStyle.Render(" stack"),
			keyHintStyle.Render("Esc") + labelStyle.Render(" cancel"),
		}
	} else if m.showSuggestions {
		keys = []string{
			keyHintStyle.Render("↑↓") + labelStyle.Render(" nav"),
			keyHintStyle.Render("Tab") + labelStyle.Render(" complete"),
			keyHintStyle.Render("Esc") + labelStyle.Render(" close"),
		}
	} else if m.historySearch.IsVisible() {
		keys = []string{
			keyHintStyle.Render("↑↓") + labelStyle.Render(" nav"),
			keyHintStyle.Render("↵") + labelStyle.Render(" select"),
			keyHintStyle.Render("Esc") + labelStyle.Render(" close"),
		}
	} else if m.diffView.IsVisible() {
		keys = []string{
			keyHintStyle.Render("↑↓") + labelStyle.Render(" scroll"),
			keyHintStyle.Render("←→") + labelStyle.Render(" agents"),
			keyHintStyle.Render("Tab") + labelStyle.Render(" pair"),
			keyHintStyle.Render("Esc") + labelStyle.Render(" close"),
		}
	} else if m.explorerFocus {
		keys = []string{
			keyHintStyle.Render("↑↓←→") + labelStyle.Render(" nav"),
			keyHintStyle.Render("↵") + labelStyle.Render(" open"),
			keyHintStyle.Render("Tab") + labelStyle.Render(" input"),
			keyHintStyle.Render("Esc") + labelStyle.Render(" close"),
		}
	} else if m.logsFocus {
		keys = []string{
			keyHintStyle.Render("↑↓") + labelStyle.Render(" scroll"),
			keyHintStyle.Render("PgUp/Dn") + labelStyle.Render(" page"),
			keyHintStyle.Render("Home/End") + labelStyle.Render(" top/btm"),
			keyHintStyle.Render("Tab") + labelStyle.Render(" input"),
			keyHintStyle.Render("Esc") + labelStyle.Render(" close"),
		}
	} else if m.tokensFocus {
		keys = []string{
			keyHintStyle.Render("↑↓") + labelStyle.Render(" scroll"),
			keyHintStyle.Render("PgUp/Dn") + labelStyle.Render(" page"),
			keyHintStyle.Render("Home/End") + labelStyle.Render(" top/btm"),
			keyHintStyle.Render("Tab") + labelStyle.Render(" input"),
			keyHintStyle.Render("Esc") + labelStyle.Render(" close"),
		}
	} else if m.streaming || m.workflowRunning {
		keys = []string{
			keyHintStyle.Render("^X") + labelStyle.Render(" stop"),
			keyHintStyle.Render("^E") + labelStyle.Render(" files"),
			keyHintStyle.Render("^L") + labelStyle.Render(" logs"),
			keyHintStyle.Render("?") + labelStyle.Render(" help"),
		}
	} else {
		keys = []string{
			keyHintStyle.Render("^Z") + labelStyle.Render(" nav"),
			keyHintStyle.Render("^Q") + labelStyle.Render(" quorum"),
			keyHintStyle.Render("^I") + labelStyle.Render(" issues"),
			keyHintStyle.Render("^H") + labelStyle.Render(" hist"),
			keyHintStyle.Render("!") + labelStyle.Render(" cmd"),
			keyHintStyle.Render("?") + labelStyle.Render(" help"),
		}
	}

	// Join with subtle separators
	separator := separatorStyle.Render(" │ ")
	content := strings.Join(keys, separator)

	// Pad to width for consistent look
	padding := width - lipgloss.Width(content) - 2
	if padding < 0 {
		padding = 0
	}

	return " " + content + strings.Repeat(" ", padding)
}

func formatWorkflowStatus(state *core.WorkflowState) string {
	var sb strings.Builder

	// Command header
	sb.WriteString("/status\n\n")

	// Status indicator with visual bar
	statusIcon := "○"
	switch state.Status {
	case core.WorkflowStatusCompleted:
		statusIcon = "●"
	case core.WorkflowStatusRunning:
		statusIcon = "◐"
	case core.WorkflowStatusFailed:
		statusIcon = "✗"
	}

	// Phase progress visualization
	phases := []string{"analyze", "plan", "execute"}
	phaseIdx := 0
	for i, p := range phases {
		if string(state.CurrentPhase) == p {
			phaseIdx = i
			break
		}
	}

	// Build phase progress bar
	var progressBar string
	for i, p := range phases {
		if i < phaseIdx {
			progressBar += fmt.Sprintf("[%s] ", p)
		} else if i == phaseIdx {
			progressBar += fmt.Sprintf(">[%s]< ", strings.ToUpper(p))
		} else {
			progressBar += fmt.Sprintf("(%s) ", p)
		}
	}

	sb.WriteString(fmt.Sprintf("%s %s  %s\n\n", statusIcon, strings.ToUpper(string(state.Status)), progressBar))

	// Workflow ID
	sb.WriteString(fmt.Sprintf("ID: %s\n\n", state.WorkflowID))

	// Prompt
	if state.Prompt != "" {
		prompt := state.Prompt
		if len(prompt) > 80 {
			prompt = prompt[:77] + "..."
		}
		sb.WriteString(fmt.Sprintf("Prompt:\n  %q\n\n", prompt))
	}

	// Metrics in a compact format
	var metrics []string
	if state.Metrics != nil {
		if state.Metrics.ConsensusScore > 0 {
			metrics = append(metrics, fmt.Sprintf("Consensus: %.0f%%", state.Metrics.ConsensusScore*100))
		}
		if state.Metrics.TotalTokensIn > 0 || state.Metrics.TotalTokensOut > 0 {
			metrics = append(metrics, fmt.Sprintf("Tokens: %s/%s",
				formatTokens(state.Metrics.TotalTokensIn),
				formatTokens(state.Metrics.TotalTokensOut)))
		}
	}
	if len(metrics) > 0 {
		sb.WriteString(strings.Join(metrics, "  |  ") + "\n\n")
	}

	// Tasks summary in compact format
	if len(state.Tasks) > 0 {
		completed := 0
		failed := 0
		pending := 0
		for _, task := range state.Tasks {
			switch task.Status {
			case core.TaskStatusCompleted:
				completed++
			case core.TaskStatusFailed:
				failed++
			default:
				pending++
			}
		}
		taskStatus := fmt.Sprintf("Issues: %d total", len(state.Tasks))
		if completed > 0 {
			taskStatus += fmt.Sprintf(", %d done", completed)
		}
		if failed > 0 {
			taskStatus += fmt.Sprintf(", %d failed", failed)
		}
		if pending > 0 {
			taskStatus += fmt.Sprintf(", %d pending", pending)
		}
		sb.WriteString(taskStatus + "\n\n")
	}

	// Next action
	sb.WriteString("Next: ")
	switch state.CurrentPhase {
	case core.PhaseAnalyze:
		if state.Status == core.WorkflowStatusCompleted {
			sb.WriteString("/plan")
		} else {
			sb.WriteString("/analyze (resume)")
		}
	case core.PhasePlan:
		if state.Status == core.WorkflowStatusCompleted {
			sb.WriteString("/execute")
		} else {
			sb.WriteString("/plan (resume)")
		}
	case core.PhaseExecute:
		if state.Status == core.WorkflowStatusCompleted {
			sb.WriteString("Done!")
		} else {
			sb.WriteString("/execute (resume)")
		}
	default:
		sb.WriteString("/analyze <prompt>")
	}

	return sb.String()
}

// updateMarkdownRenderer updates the markdown renderer with a new word wrap width
func (m *Model) updateMarkdownRenderer(width int) {
	if width < 40 {
		width = 40
	}
	if width > 120 {
		width = 120 // Maximum reasonable for readability
	}

	customStyle := styles.DraculaStyleConfig
	customStyle.Code = ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color:           stringPtr("229"), // Light yellow
			BackgroundColor: stringPtr(""),    // No background
			Prefix:          "",               // No prefix
			Suffix:          "",               // No suffix
		},
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(customStyle),
		glamour.WithWordWrap(width),
	)
	if err == nil {
		m.mdRenderer = renderer
	}
}

// stringPtr returns a pointer to a string (helper for glamour style config)
func stringPtr(s string) *string {
	return &s
}
