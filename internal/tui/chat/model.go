package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui"
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
	totalCost      float64

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
	machineCollector *MachineStatsCollector

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
		machineCollector: NewMachineStatsCollector(),
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
		m.chatTimeout = 3 * time.Minute // Default 3 min
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
		// Also clear the "/" prefix if that's all there is
		if m.textarea.Value() == "/" {
			m.textarea.Reset()
		}
		return m, nil, true
	}

	// Panel navigation mode (tmux-style: Ctrl+z then arrow keys)
	if m.panelNavMode {
		refreshPanelNav := func() tea.Cmd {
			m.panelNavSeq++
			m.panelNavTill = time.Now().Add(panelNavWindow)
			return panelNavTimeoutCmd(m.panelNavSeq)
		}

		focusInput := func() {
			m.explorerFocus = false
			m.logsFocus = false
			m.statsFocus = false
			m.tokensFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			m.explorerPanel.SetFocused(false)
		}

		focusLeft := func(prefer string) {
			if !m.showExplorer && !m.showTokens {
				return
			}
			if !(m.explorerFocus || m.tokensFocus) && prefer == "" {
				prefer = "explorer"
			}
			if prefer == "explorer" && m.showExplorer {
				m.explorerFocus = true
				m.tokensFocus = false
			} else if prefer == "tokens" && m.showTokens {
				m.tokensFocus = true
				m.explorerFocus = false
			} else if m.showExplorer {
				m.explorerFocus = true
				m.tokensFocus = false
			} else {
				m.tokensFocus = true
				m.explorerFocus = false
			}
			if m.explorerFocus || m.tokensFocus {
				m.logsFocus = false
				m.statsFocus = false
				m.inputFocused = false
				m.textarea.Blur()
				m.explorerPanel.SetFocused(m.explorerFocus)
			}
		}

		focusRight := func(prefer string) {
			if !m.showLogs && !m.showStats {
				return
			}
			if !(m.logsFocus || m.statsFocus) && prefer == "" {
				prefer = "logs"
			}
			if prefer == "logs" && m.showLogs {
				m.logsFocus = true
				m.statsFocus = false
			} else if prefer == "stats" && m.showStats {
				m.statsFocus = true
				m.logsFocus = false
			} else if m.showLogs {
				m.logsFocus = true
				m.statsFocus = false
			} else {
				m.statsFocus = true
				m.logsFocus = false
			}
			if m.logsFocus || m.statsFocus {
				m.explorerFocus = false
				m.tokensFocus = false
				m.inputFocused = false
				m.textarea.Blur()
				m.explorerPanel.SetFocused(false)
			}
		}

		switch msg.Type {
		case tea.KeyEsc, tea.KeyEnter, tea.KeySpace:
			// Exit nav mode and return focus to chat
			m.panelNavMode = false
			m.panelNavSeq++
			focusInput()
			return m, nil, true
		case tea.KeyLeft:
			// Move to left sidebar (keep focus if already there)
			if !(m.explorerFocus || m.tokensFocus) {
				focusLeft("")
			}
			return m, refreshPanelNav(), true
		case tea.KeyRight:
			// Move to right sidebar (keep focus if already there)
			if !(m.logsFocus || m.statsFocus) {
				focusRight("")
			}
			return m, refreshPanelNav(), true
		case tea.KeyUp:
			// Move to top panel in the current stack
			if m.logsFocus || m.statsFocus {
				focusRight("logs")
			} else if m.explorerFocus || m.tokensFocus {
				focusLeft("explorer")
			} else if m.showLogs || m.showStats {
				focusRight("logs")
			} else {
				focusLeft("explorer")
			}
			return m, refreshPanelNav(), true
		case tea.KeyDown:
			// Move to bottom panel in the current stack
			if m.logsFocus || m.statsFocus {
				focusRight("stats")
			} else if m.explorerFocus || m.tokensFocus {
				focusLeft("tokens")
			} else if m.showLogs || m.showStats {
				focusRight("stats")
			} else {
				focusLeft("tokens")
			}
			return m, refreshPanelNav(), true
		default:
			// Any other key cancels panel nav mode and returns focus to input
			m.panelNavMode = false
			m.panelNavSeq++
			focusInput()
			return m, nil, true
		}
	}

	// Ctrl+z enters panel navigation mode (tmux-style)
	if msg.Type == tea.KeyCtrlZ {
		m.panelNavMode = true
		m.panelNavSeq++
		m.panelNavTill = time.Now().Add(panelNavWindow)
		return m, panelNavTimeoutCmd(m.panelNavSeq), true
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		m.explorerPanel.Close() // Stop file watcher
		return m, tea.Quit, true

	case tea.KeyEnter:
		// If dropdown is visible, handle based on suggestion type
		if m.showSuggestions && len(m.suggestions) > 0 {
			selected := m.suggestions[m.suggestionIndex]

			switch m.suggestionType {
			case "agent":
				// Complete and execute /agent command
				m.textarea.SetValue("/agent " + selected)
				m.showSuggestions = false
				m.suggestionIndex = 0
				m.suggestionType = ""
				model, teaCmd := m.handleSubmit()
				return model, teaCmd, true

			case "model":
				// Complete and execute /model command
				m.textarea.SetValue("/model " + selected)
				m.showSuggestions = false
				m.suggestionIndex = 0
				m.suggestionType = ""
				model, teaCmd := m.handleSubmit()
				return model, teaCmd, true

			case "workflow":
				// Complete and execute /load command
				m.textarea.SetValue("/load " + selected)
				m.showSuggestions = false
				m.suggestionIndex = 0
				m.suggestionType = ""
				m.workflowCache = nil // Clear cache to refresh on next use
				model, teaCmd := m.handleSubmit()
				return model, teaCmd, true

			case "theme":
				// Complete and execute /theme command
				m.textarea.SetValue("/theme " + selected)
				m.showSuggestions = false
				m.suggestionIndex = 0
				m.suggestionType = ""
				model, teaCmd := m.handleSubmit()
				return model, teaCmd, true

			default:
				// Command suggestion
				selectedCmd := m.commands.Get(selected)

				// If command requires arguments, autocomplete like Tab (let user add args)
				if selectedCmd != nil && selectedCmd.RequiresArg() {
					m.textarea.SetValue("/" + selected + " ")
					m.textarea.CursorEnd()
					m.showSuggestions = false
					m.suggestionIndex = 0
					m.suggestionType = ""
					return m, nil, true
				}

				// Command doesn't require args - execute immediately
				m.textarea.SetValue("/" + selected)
				m.showSuggestions = false
				m.suggestionIndex = 0
				m.suggestionType = ""
				model, teaCmd := m.handleSubmit()
				return model, teaCmd, true
			}
		}
		// Otherwise, normal submit
		if m.textarea.Value() != "" {
			model, teaCmd := m.handleSubmit()
			return model, teaCmd, true
		}

	case tea.KeyTab:
		// Tab/Ctrl+I always toggles issues panel, closing other overlays if needed
		if m.showSuggestions && len(m.suggestions) > 0 {
			// Complete with selected suggestion based on type
			switch m.suggestionType {
			case "agent":
				m.textarea.SetValue("/agent " + m.suggestions[m.suggestionIndex])
			case "model":
				m.textarea.SetValue("/model " + m.suggestions[m.suggestionIndex])
			case "workflow":
				m.textarea.SetValue("/load " + m.suggestions[m.suggestionIndex])
			case "theme":
				m.textarea.SetValue("/theme " + m.suggestions[m.suggestionIndex])
			default:
				m.textarea.SetValue("/" + m.suggestions[m.suggestionIndex] + " ")
			}
			m.textarea.CursorEnd()
			m.showSuggestions = false
			m.suggestionIndex = 0
			m.suggestionType = ""
			return m, nil, true
		}
		// Close other overlays/focus states before toggling issues panel
		if m.explorerFocus {
			m.explorerFocus = false
			m.explorerPanel.SetFocused(false)
		}
		if m.logsFocus {
			m.logsFocus = false
		}
		if m.tokensFocus {
			m.tokensFocus = false
		}
		if m.diffView.IsVisible() {
			m.diffView.Hide()
		}
		if m.historySearch.IsVisible() {
			m.historySearch.Hide()
		}
		m.inputFocused = true
		m.textarea.Focus()
		m.tasksPanel.Toggle()
		return m, nil, true

	case tea.KeyUp:
		if m.showSuggestions && len(m.suggestions) > 0 {
			m.suggestionIndex--
			if m.suggestionIndex < 0 {
				m.suggestionIndex = len(m.suggestions) - 1
			}
			return m, nil, true
		}

	case tea.KeyDown:
		if m.showSuggestions && len(m.suggestions) > 0 {
			m.suggestionIndex++
			if m.suggestionIndex >= len(m.suggestions) {
				m.suggestionIndex = 0
			}
			return m, nil, true
		}

	case tea.KeyEsc:
		// Note: showSuggestions is handled at the top of this function
		if m.explorerFocus && m.showExplorer {
			// Close explorer or return focus to input
			m.explorerFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			m.explorerPanel.SetFocused(false)
			return m, nil, true
		} else if m.logsFocus && m.showLogs {
			// Return focus from logs to input
			m.logsFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		} else if m.pendingInputRequest != nil {
			if m.controlPlane != nil {
				_ = m.controlPlane.CancelUserInput(m.pendingInputRequest.ID)
			}
			m.pendingInputRequest = nil
			m.history.Add(NewSystemMessage("Input cancelled"))
			m.updateViewport()
			return m, nil, true
		} else if m.inputFocused && m.textarea.Value() != "" {
			// Clear input text with Escape when nothing else is active
			m.textarea.Reset()
			m.showSuggestions = false
			m.recalculateLayout() // Recalculate since input height changed
			return m, nil, true
		}

	case tea.KeyCtrlAt: // Ctrl+Space sends NUL (Ctrl+@) in most terminals
		// Ctrl+Space or Ctrl+@ to force show autocomplete (not during streaming)
		if m.streaming || m.workflowRunning {
			return m, nil, true
		}
		val := m.textarea.Value()
		if val == "" {
			m.textarea.SetValue("/")
			val = "/"
		}
		if strings.HasPrefix(val, "/") {
			m.suggestions = m.commands.Suggest(val)
			m.showSuggestions = len(m.suggestions) > 0
			m.suggestionIndex = 0
			return m, nil, true
		}

	case tea.KeyCtrlY:
		// Copy last agent response to clipboard
		return m.copyLastResponse()

	case tea.KeyCtrlL:
		// Toggle logs panel
		m.showLogs = !m.showLogs
		if m.showLogs {
			m.logsFocus = true
			m.explorerFocus = false
			m.tokensFocus = false
			m.inputFocused = false
			m.textarea.Blur()
		} else {
			m.logsFocus = false
			m.inputFocused = true
			m.textarea.Focus()
		}
		// Recalculate layout
		if m.width > 0 && m.height > 0 {
			m.recalculateLayout()
		}
		return m, nil, true

	case tea.KeyCtrlE:
		// Toggle explorer panel
		m.showExplorer = !m.showExplorer
		if m.showExplorer {
			m.explorerFocus = true
			m.tokensFocus = false
			m.inputFocused = false
			m.textarea.Blur()
			m.explorerPanel.SetFocused(true)
		} else {
			m.explorerFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			m.explorerPanel.SetFocused(false)
		}
		// Recalculate layout
		if m.width > 0 && m.height > 0 {
			m.recalculateLayout()
		}
		return m, nil, true

	case tea.KeyCtrlR:
		// Toggle stats panel (resources)
		m.showStats = !m.showStats
		if m.showStats {
			m.statsFocus = true
			m.logsFocus = false
			m.tokensFocus = false
			m.explorerFocus = false
			m.inputFocused = false
			m.textarea.Blur()
			m.explorerPanel.SetFocused(false)
		} else {
			m.statsFocus = false
			m.inputFocused = true
			m.textarea.Focus()
		}
		// Recalculate layout
		if m.width > 0 && m.height > 0 {
			m.recalculateLayout()
		}
		return m, nil, true

	case tea.KeyCtrlT:
		// Toggle tokens panel (left sidebar)
		m.showTokens = !m.showTokens
		if m.showTokens {
			m.tokensFocus = true
			m.explorerFocus = false
			m.logsFocus = false
			m.statsFocus = false
			m.inputFocused = false
			m.textarea.Blur()
			m.explorerPanel.SetFocused(false)
		} else {
			m.tokensFocus = false
			m.inputFocused = true
			m.textarea.Focus()
		}
		// Recalculate layout
		if m.width > 0 && m.height > 0 {
			m.recalculateLayout()
		}
		return m, nil, true

	case tea.KeyCtrlX:
		// Cancel current operation (streaming or workflow)
		if m.streaming || m.workflowRunning {
			if m.cancelFunc != nil {
				m.cancelFunc()
				m.cancelFunc = nil
			}
			if m.controlPlane != nil && m.workflowRunning {
				m.controlPlane.Cancel()
			}
			wasStreaming := m.streaming
			wasWorkflow := m.workflowRunning
			m.streaming = false
			m.workflowRunning = false

			if wasWorkflow {
				m.workflowPhase = "idle"
				// Reset agent states
				for _, a := range m.agentInfos {
					if a.Status == AgentStatusRunning {
						a.Status = AgentStatusIdle
					}
				}
				m.logsPanel.AddWarn("system", "Workflow interrupted by user (Ctrl+X)")
				m.history.Add(NewSystemMessage("Workflow interrupted"))
			} else if wasStreaming {
				m.logsPanel.AddWarn("system", "Request interrupted by user (Ctrl+X)")
				m.history.Add(NewSystemMessage("Request interrupted"))
			}
			m.updateViewport()
			return m, nil, true
		}

	case tea.KeyCtrlQ, tea.KeyCtrlK:
		// Toggle quorum panel
		m.consensusPanel.Toggle()
		return m, nil, true

	// Note: tea.KeyTab is handled earlier in this switch (line ~913) for suggestions
	// Additional Tab handling for issues panel moved there.

	case tea.KeyCtrlD:
		// Toggle diff view
		if m.diffView.HasContent() {
			m.diffView.Toggle()
		}
		return m, nil, true

	case tea.KeyCtrlH:
		// Toggle history search
		m.historySearch.Toggle()
		if m.historySearch.IsVisible() {
			m.inputFocused = false
			m.textarea.Blur()
		} else {
			m.inputFocused = true
			m.textarea.Focus()
		}
		return m, nil, true
	}

	// Handle ? for shortcuts overlay (only when input is empty)
	if msg.String() == "?" && m.textarea.Value() == "" {
		m.shortcutsOverlay.Toggle()
		return m, nil, true
	}

	// Handle F1 for shortcuts overlay (always works)
	if msg.Type == tea.KeyF1 {
		m.shortcutsOverlay.Toggle()
		return m, nil, true
	}

	// F2/F3 previously used for cost/stats overlays - now integrated into logs footer

	// Close any visible overlays on Escape
	if msg.Type == tea.KeyEsc {
		if m.shortcutsOverlay.IsVisible() {
			m.shortcutsOverlay.Hide()
			return m, nil, true
		}
		if m.historySearch.IsVisible() {
			m.historySearch.Hide()
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		}
		if m.diffView.IsVisible() {
			m.diffView.Hide()
			return m, nil, true
		}
		if m.consensusPanel.IsVisible() {
			m.consensusPanel.Toggle()
			return m, nil, true
		}
		if m.tasksPanel.IsVisible() {
			m.tasksPanel.Hide()
			return m, nil, true
		}
		// Note: fileViewer uses 'q' to close, not Escape
	}

	// Handle file viewer navigation when visible
	if m.fileViewer.IsVisible() {
		switch msg.Type {
		case tea.KeyUp:
			m.fileViewer.ScrollUp()
			return m, nil, true
		case tea.KeyDown:
			m.fileViewer.ScrollDown()
			return m, nil, true
		case tea.KeyLeft:
			m.fileViewer.ScrollLeft()
			return m, nil, true
		case tea.KeyRight:
			m.fileViewer.ScrollRight()
			return m, nil, true
		case tea.KeyPgUp:
			m.fileViewer.PageUp()
			return m, nil, true
		case tea.KeyPgDown:
			m.fileViewer.PageDown()
			return m, nil, true
		}
		switch msg.String() {
		case "q":
			m.fileViewer.Hide()
			return m, nil, true
		case "e":
			// Open file in editor (config > $EDITOR > $VISUAL > vi)
			filePath := m.fileViewer.GetFilePath()
			if filePath != "" {
				editor := m.editorCmd
				if editor == "" {
					editor = os.Getenv("EDITOR")
				}
				if editor == "" {
					editor = os.Getenv("VISUAL")
				}
				if editor == "" {
					editor = "vi" // fallback
				}
				editorPath, err := exec.LookPath(editor)
				if err != nil {
					m.logsPanel.AddError("editor", fmt.Sprintf("Editor not found: %s", editor))
					return m, nil, true
				}
				m.fileViewer.Hide()
				// #nosec G204 -- editor is user-configured and resolved via LookPath
				cmd := exec.Command(editorPath, filePath)
				return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
					return editorFinishedMsg{filePath: filePath, err: err}
				}), true
			}
			return m, nil, true
		case "h":
			m.fileViewer.ScrollLeft()
			return m, nil, true
		case "l":
			m.fileViewer.ScrollRight()
			return m, nil, true
		case "0":
			m.fileViewer.ScrollHome()
			return m, nil, true
		case "$":
			m.fileViewer.ScrollEnd()
			return m, nil, true
		case "g":
			m.fileViewer.ScrollTop()
			return m, nil, true
		case "G":
			m.fileViewer.ScrollBottom()
			return m, nil, true
		}
		// Block other keys when file viewer is open
		return m, nil, true
	}

	// Handle history search navigation when visible
	if m.historySearch.IsVisible() {
		switch msg.Type {
		case tea.KeyUp:
			m.historySearch.MoveUp()
			return m, nil, true
		case tea.KeyDown:
			m.historySearch.MoveDown()
			return m, nil, true
		case tea.KeyEnter:
			// Select command and insert into textarea
			selected := m.historySearch.GetSelected()
			if selected != "" {
				m.textarea.SetValue(selected)
				m.textarea.CursorEnd()
			}
			m.historySearch.Hide()
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		default:
			// Pass to history search input
			m.historySearch.UpdateInput(msg)
			return m, nil, true
		}
	}

	// Handle diff view navigation when visible
	if m.diffView.IsVisible() {
		switch msg.Type {
		case tea.KeyUp:
			m.diffView.ScrollUp()
			return m, nil, true
		case tea.KeyDown:
			m.diffView.ScrollDown()
			return m, nil, true
		case tea.KeyTab:
			m.diffView.NextPair()
			return m, nil, true
		case tea.KeyLeft, tea.KeyRight:
			// Switch agent pair
			if msg.Type == tea.KeyLeft {
				m.diffView.PrevPair()
			} else {
				m.diffView.NextPair()
			}
			return m, nil, true
		}
	}

	// Handle explorer navigation when it has focus
	if m.explorerFocus && m.showExplorer {
		switch msg.Type {
		case tea.KeyUp:
			m.explorerPanel.MoveUp()
			return m, nil, true
		case tea.KeyDown:
			m.explorerPanel.MoveDown()
			return m, nil, true
		case tea.KeyLeft:
			// Collapse directory or go up
			entry := m.explorerPanel.GetSelectedEntry()
			if entry != nil && entry.Type == FileTypeDir && entry.Expanded {
				m.explorerPanel.Toggle()
			} else {
				m.explorerPanel.GoUp()
			}
			return m, nil, true
		case tea.KeyRight:
			// Expand directory
			entry := m.explorerPanel.GetSelectedEntry()
			if entry != nil && entry.Type == FileTypeDir && !entry.Expanded {
				m.explorerPanel.Toggle()
			}
			return m, nil, true
		case tea.KeyEnter:
			// Enter directory or open file viewer
			path := m.explorerPanel.Enter()
			if path != "" {
				// File selected - open in file viewer
				if err := m.fileViewer.SetFile(path); err != nil {
					m.logsPanel.AddError("explorer", "Cannot open: "+err.Error())
				} else {
					m.fileViewer.Show()
				}
			}
			return m, nil, true
		case tea.KeyTab:
			// Insert selected path into chat with @ prefix and switch focus
			relPath := m.explorerPanel.GetSelectedRelativePath()
			if relPath != "" {
				// Insert path reference with @ prefix into textarea
				currentValue := m.textarea.Value()
				pathRef := "@" + relPath
				if currentValue != "" && !strings.HasSuffix(currentValue, " ") && !strings.HasSuffix(currentValue, "\n") {
					pathRef = " " + pathRef
				}
				m.textarea.SetValue(currentValue + pathRef)
				// Move cursor to end
				m.textarea.CursorEnd()
			}
			m.explorerFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			m.explorerPanel.SetFocused(false)
			return m, nil, true
		}
	}

	// Handle token panel scrolling when it has focus
	if m.tokensFocus && m.showTokens {
		switch msg.Type {
		case tea.KeyUp:
			m.tokenPanel.ScrollUp()
			return m, nil, true
		case tea.KeyDown:
			m.tokenPanel.ScrollDown()
			return m, nil, true
		case tea.KeyPgUp:
			m.tokenPanel.PageUp()
			return m, nil, true
		case tea.KeyPgDown:
			m.tokenPanel.PageDown()
			return m, nil, true
		case tea.KeyHome:
			m.tokenPanel.GotoTop()
			return m, nil, true
		case tea.KeyEnd:
			m.tokenPanel.GotoBottom()
			return m, nil, true
		case tea.KeyTab:
			// Switch focus back to input
			m.tokensFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		}
	}

	// Handle stats panel scrolling when it has focus
	if m.statsFocus && m.showStats {
		switch msg.Type {
		case tea.KeyUp:
			m.statsPanel.ScrollUp()
			return m, nil, true
		case tea.KeyDown:
			m.statsPanel.ScrollDown()
			return m, nil, true
		case tea.KeyPgUp:
			m.statsPanel.PageUp()
			return m, nil, true
		case tea.KeyPgDown:
			m.statsPanel.PageDown()
			return m, nil, true
		case tea.KeyHome:
			m.statsPanel.GotoTop()
			return m, nil, true
		case tea.KeyEnd:
			m.statsPanel.GotoBottom()
			return m, nil, true
		case tea.KeyTab:
			// Switch focus back to input
			m.statsFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		}
	}

	// Handle logs panel scrolling when it has focus
	if m.logsFocus && m.showLogs {
		switch msg.Type {
		case tea.KeyUp:
			m.logsPanel.ScrollUp()
			return m, nil, true
		case tea.KeyDown:
			m.logsPanel.ScrollDown()
			return m, nil, true
		case tea.KeyPgUp:
			m.logsPanel.PageUp()
			return m, nil, true
		case tea.KeyPgDown:
			m.logsPanel.PageDown()
			return m, nil, true
		case tea.KeyHome:
			m.logsPanel.GotoTop()
			return m, nil, true
		case tea.KeyEnd:
			m.logsPanel.GotoBottom()
			return m, nil, true
		case tea.KeyTab:
			// Switch focus back to input
			m.logsFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		case tea.KeyRunes:
			// 'c' or 'y' to copy logs when logs panel is focused
			if len(msg.Runes) == 1 && (msg.Runes[0] == 'c' || msg.Runes[0] == 'y') {
				return m.copyLogsToClipboard()
			}
		}
	}

	// Handle Ctrl+Shift+C for copy (some terminals)
	if msg.String() == "ctrl+shift+c" {
		return m.copyLastResponse()
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
		if m.showExplorer && m.showTokens {
			explorerHeight := m.explorerPanel.Height()
			if y < explorerHeight {
				if !m.explorerFocus {
					m.explorerFocus = true
					m.tokensFocus = false
					m.logsFocus = false
					m.statsFocus = false
					m.inputFocused = false
					m.textarea.Blur()
					m.explorerPanel.SetFocused(true)
					return m, nil, true
				}
				return m, nil, false
			}
			// Token panel area
			if !m.tokensFocus {
				m.tokensFocus = true
				m.explorerFocus = false
				m.logsFocus = false
				m.statsFocus = false
				m.inputFocused = false
				m.textarea.Blur()
				m.explorerPanel.SetFocused(false)
				return m, nil, true
			}
			// Already focused on tokens - clicking again returns to main
			m.tokensFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		}
		if m.showExplorer {
			if !m.explorerFocus {
				m.explorerFocus = true
				m.tokensFocus = false
				m.logsFocus = false
				m.statsFocus = false
				m.inputFocused = false
				m.textarea.Blur()
				m.explorerPanel.SetFocused(true)
				return m, nil, true
			}
			return m, nil, false // Already focused, let it handle internally
		}
		if m.showTokens {
			if !m.tokensFocus {
				m.tokensFocus = true
				m.explorerFocus = false
				m.logsFocus = false
				m.statsFocus = false
				m.inputFocused = false
				m.textarea.Blur()
				m.explorerPanel.SetFocused(false)
				return m, nil, true
			}
			m.tokensFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		}
	}

	// Check if click is in right sidebar (logs/stats)
	if (m.showLogs || m.showStats) && x > mainEnd {
		if m.showLogs && m.showStats {
			logsHeight := m.logsPanel.Height()
			if y < logsHeight {
				// Logs area
				if !m.logsFocus {
					m.logsFocus = true
					m.statsFocus = false
					m.explorerFocus = false
					m.tokensFocus = false
					m.inputFocused = false
					m.textarea.Blur()
					m.explorerPanel.SetFocused(false)
					return m, nil, true
				}
				m.logsFocus = false
				m.inputFocused = true
				m.textarea.Focus()
				return m, nil, true
			}
			// Stats area
			if !m.statsFocus {
				m.statsFocus = true
				m.logsFocus = false
				m.explorerFocus = false
				m.tokensFocus = false
				m.inputFocused = false
				m.textarea.Blur()
				m.explorerPanel.SetFocused(false)
				return m, nil, true
			}
			m.statsFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		}
		if m.showLogs {
			// Logs only
			if !m.logsFocus {
				m.logsFocus = true
				m.statsFocus = false
				m.explorerFocus = false
				m.tokensFocus = false
				m.inputFocused = false
				m.textarea.Blur()
				m.explorerPanel.SetFocused(false)
				return m, nil, true
			}
			m.logsFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		}
		if m.showStats {
			// Stats only
			if !m.statsFocus {
				m.statsFocus = true
				m.logsFocus = false
				m.explorerFocus = false
				m.tokensFocus = false
				m.inputFocused = false
				m.textarea.Blur()
				m.explorerPanel.SetFocused(false)
				return m, nil, true
			}
			m.statsFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		}
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
		if msgs[i].Role == RoleAgent {
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
				"claude-opus-4-5-20251101",
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
				"gpt-5.2-codex",
				"gpt-5.2",
				"gpt-5.1-codex-max",
				"gpt-5.1-codex",
				"gpt-5.1",
				"gpt-5",
				"gpt-5-mini",
				"gpt-4.1",
			}
		case "copilot":
			models = []string{
				"claude-sonnet-4.5",
				"claude-haiku-4.5",
				"claude-opus-4.5",
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

	switch msg := msg.(type) {
	case tea.KeyMsg:
		newModel, cmd, handled := m.handleKeyMsg(msg)
		if handled {
			return newModel, cmd
		}

	case tea.MouseMsg:
		// Handle mouse clicks for panel focus switching
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
		// File system change detected - refresh explorer
		_ = m.explorerPanel.Refresh()
		// Continue listening for more changes
		cmds = append(cmds, m.listenForExplorerChanges())

	case ChatProgressTickMsg:
		// Progress ticks are intentionally silent (avoid log noise)
		if m.streaming {
			cmds = append(cmds, m.chatProgressTick())
		}

	case StatsTickMsg:
		// Update process statistics
		m.statsWidget.Update()

		// Get resource stats
		stats := m.statsWidget.GetStats()
		resourceStats := ResourceStats{
			MemoryMB:      stats.MemoryMB,
			CPUPercent:    stats.CPUPercent,
			CPURawPercent: stats.CPURawPercent,
			Uptime:        stats.Uptime,
			Goroutines:    stats.Goroutines,
		}

		// Pass to LogsPanel footer
		m.logsPanel.SetResourceStats(resourceStats)

		// Pass to StatsPanel
		m.statsPanel.SetResourceStats(resourceStats)

		// Collect and pass machine stats
		if m.machineCollector != nil {
			machineStats := m.machineCollector.Collect()
			m.logsPanel.SetMachineStats(machineStats)
			m.statsPanel.SetMachineStats(machineStats)
		}

		// Pass token stats from agentInfos
		m.updateLogsPanelTokenStats()
		m.updateTokenPanelStats()

		// Continue ticking
		cmds = append(cmds, statsTickCmd())

	case PanelNavTimeoutMsg:
		if m.panelNavMode && msg.Seq == m.panelNavSeq {
			if time.Now().After(m.panelNavTill) {
				m.panelNavMode = false
			}
		}

	case AgentResponseMsg:
		elapsed := time.Since(m.chatStartedAt)
		m.streaming = false
		agentLower := strings.ToLower(msg.Agent)
		if msg.Error != nil {
			errMsg := msg.Error.Error()
			// Check for timeout errors
			if strings.Contains(errMsg, "context deadline exceeded") || strings.Contains(errMsg, "timed out") {
				m.history.Add(NewSystemMessage(fmt.Sprintf("⏱ Request timed out after %s", formatDuration(elapsed))))
				m.logsPanel.AddError(agentLower, fmt.Sprintf("⏱ Timeout after %s - consider using a faster model", formatDuration(elapsed)))
			} else {
				m.history.Add(NewSystemMessage("Error: " + errMsg))
				m.logsPanel.AddError(agentLower, fmt.Sprintf("✗ Error after %s: %s", formatDuration(elapsed), errMsg))
			}
		} else {
			m.history.Add(NewAgentMessage(msg.Agent, msg.Content))
			// Update token counts for the agent (validate to avoid corrupted values)
			// Cap matches the adapter-level cap (500k) to ensure consistency
			const maxReasonableTokens = 500_000
			for _, a := range m.agentInfos {
				if strings.EqualFold(a.Name, msg.Agent) {
					if m.chatModel != "" {
						a.Model = m.chatModel
					}
					if msg.TokensIn > 0 && msg.TokensIn <= maxReasonableTokens {
						a.TokensIn += msg.TokensIn
					}
					if msg.TokensOut > 0 && msg.TokensOut <= maxReasonableTokens {
						a.TokensOut += msg.TokensOut
					}
					break
				}
			}
			// Build detailed completion log
			stats := []string{fmt.Sprintf("%d chars", len(msg.Content))}
			if msg.TokensIn > 0 || msg.TokensOut > 0 {
				stats = append(stats, fmt.Sprintf("↑%d ↓%d tok", msg.TokensIn, msg.TokensOut))
			}
			stats = append(stats, formatDuration(elapsed))
			m.logsPanel.AddSuccess(agentLower, fmt.Sprintf("✓ Response [%s]", strings.Join(stats, " | ")))
		}
		m.updateViewport()
		m.updateLogsPanelTokenStats()
		m.updateTokenPanelStats()

	case ShellOutputMsg:
		// Handle shell command output
		if msg.Error != "" {
			m.history.Add(NewSystemMessage("Error executing command: " + msg.Error))
			m.logsPanel.AddError("shell", "Command failed: "+msg.Error)
		} else {
			output := msg.Output
			if output == "" {
				output = "(no output)"
			}
			// Format output as code block
			formattedOutput := fmt.Sprintf("```\n%s```", output)
			if msg.ExitCode != 0 {
				formattedOutput += fmt.Sprintf("\n*Exit code: %d*", msg.ExitCode)
			}
			m.history.Add(NewAgentMessage("Shell", formattedOutput))
			m.logsPanel.AddSuccess("shell", fmt.Sprintf("Command completed (exit %d)", msg.ExitCode))
		}
		// Refresh explorer to show any new files created by shell command
		if m.explorerPanel != nil {
			_ = m.explorerPanel.Refresh()
		}
		m.updateViewport()

	case editorFinishedMsg:
		// Handle editor close - refresh explorer and log result
		if msg.err != nil {
			m.logsPanel.AddError("editor", fmt.Sprintf("Editor error: %v", msg.err))
		} else {
			m.logsPanel.AddSuccess("editor", fmt.Sprintf("Edited: %s", filepath.Base(msg.filePath)))
		}
		// Refresh explorer to show any changes
		if m.explorerPanel != nil {
			_ = m.explorerPanel.Refresh()
		}

	case WorkflowUpdateMsg:
		m.workflowState = msg.State
		m.tasksPanel.SetState(msg.State)
		m.updateQuorumPanel(msg.State)

	case TaskUpdateMsg:
		// Update the task status in the workflow state
		if m.workflowState != nil && m.workflowState.Tasks != nil {
			if task, ok := m.workflowState.Tasks[msg.TaskID]; ok {
				task.Status = msg.Status
				m.tasksPanel.SetState(m.workflowState)
				m.updateQuorumPanel(m.workflowState)
				m.updateViewport() // Force re-render to show task status change
			}
		}

	case PhaseUpdateMsg:
		// Update the current phase in the workflow state
		if m.workflowState != nil {
			m.workflowState.CurrentPhase = msg.Phase
			m.tasksPanel.SetState(m.workflowState)
			m.updateQuorumPanel(m.workflowState)
			m.updateViewport() // Force re-render to show phase change
		}

	case BatchedEventsMsg:
		// Process multiple events collected within debounce window
		for _, evt := range msg.Events {
			// Recursively process each event through Update
			// This ensures all event types are handled correctly
			var innerCmd tea.Cmd
			newModel, innerCmd := m.Update(evt)
			m = newModel.(Model)
			if innerCmd != nil {
				cmds = append(cmds, innerCmd)
			}
		}
		// Force a single viewport update after processing all batched events
		m.updateViewport()

	case ActiveWorkflowLoadMsg:
		// Auto-loaded active workflow on startup
		if msg.State != nil {
			m.workflowState = msg.State
			m.tasksPanel.SetState(msg.State)
			m.updateQuorumPanel(msg.State)
			// Show a brief notification
			prompt := msg.State.Prompt
			if len(prompt) > 50 {
				prompt = prompt[:47] + "..."
			}
			m.history.Add(NewSystemBubbleMessage(fmt.Sprintf("Session restored: %s @%s\n%q",
				strings.ToUpper(string(msg.State.Status)),
				msg.State.CurrentPhase,
				prompt)))
			m.updateViewport()
		}

	case WorkflowStartedMsg:
		m.workflowRunning = true
		m.workflowStartedAt = time.Now()
		m.workflowPhase = "running"
		m.consensusPanel.ClearOutputs()
		// Reset all agents to idle - actual agent events will set them to running
		for _, a := range m.agentInfos {
			if a.Status != AgentStatusDisabled {
				a.Status = AgentStatusIdle
			}
		}
		m.history.Add(NewSystemBubbleMessage("Starting workflow..."))
		m.logsPanel.AddInfo("workflow", "Workflow started: "+msg.Prompt)
		// Auto-show logs panel when workflow starts so user can see progress
		if !m.showLogs {
			m.showLogs = true
			// Recalculate layout with logs panel visible
			if m.width > 0 && m.height > 0 {
				m.recalculateLayout()
			}
		}
		m.updateViewport()
		cmds = append(cmds, m.spinner.Tick)

	case WorkflowCompletedMsg:
		elapsed := time.Since(m.workflowStartedAt)
		m.workflowRunning = false
		m.workflowPhase = "done"
		m.workflowState = msg.State
		m.tasksPanel.SetState(msg.State)
		m.updateQuorumPanel(msg.State)
		// Mark all running agents as done
		for _, a := range m.agentInfos {
			if a.Status == AgentStatusRunning {
				a.Status = AgentStatusDone
				m.logsPanel.AddSuccess(strings.ToLower(a.Name), "Agent completed")
			}
		}
		// Update costs from state
		if msg.State != nil && msg.State.Metrics != nil {
			m.totalCost = msg.State.Metrics.TotalCostUSD
			m.totalTokensIn = msg.State.Metrics.TotalTokensIn
			m.totalTokensOut = msg.State.Metrics.TotalTokensOut
		}
		// Build a user-friendly completion summary
		summaryParts := []string{fmt.Sprintf("✓ Workflow completed in %s", formatDuration(elapsed))}
		if msg.State != nil && msg.State.Metrics != nil {
			summaryParts = append(summaryParts, fmt.Sprintf("Tokens: %s in / %s out", formatTokens(m.totalTokensIn), formatTokens(m.totalTokensOut)))
			summaryParts = append(summaryParts, fmt.Sprintf("Cost: $%.4f", m.totalCost))
			if msg.State.Metrics.ConsensusScore > 0 {
				summaryParts = append(summaryParts, fmt.Sprintf("Consensus: %.0f%%", msg.State.Metrics.ConsensusScore*100))
			}
		}
		m.logsPanel.AddSuccess("workflow", strings.Join(summaryParts, " | "))
		m.history.Add(NewSystemBubbleMessage(strings.Join(summaryParts, "\n")))
		if msg.State != nil {
			status := strings.TrimPrefix(formatWorkflowStatus(msg.State), "/status\n\n")
			m.history.Add(NewSystemBubbleMessage(status))
		}
		m.updateViewport()

	case WorkflowErrorMsg:
		elapsed := time.Since(m.workflowStartedAt)
		m.workflowRunning = false
		m.workflowPhase = "idle"
		// Mark running agent as error
		for _, a := range m.agentInfos {
			if a.Status == AgentStatusRunning {
				a.Status = AgentStatusError
				a.Error = msg.Error.Error()
				m.logsPanel.AddError(strings.ToLower(a.Name), "Agent failed: "+msg.Error.Error())
				break
			}
		}
		m.logsPanel.AddError("workflow", fmt.Sprintf("Workflow failed after %s: %s", formatDuration(elapsed), msg.Error.Error()))
		m.history.Add(NewSystemBubbleMessage(fmt.Sprintf("Workflow failed after %s: %v", formatDuration(elapsed), msg.Error)))
		m.updateViewport()

	case WorkflowLogMsg:
		// Handle workflow log messages from the runner
		switch msg.Level {
		case "success":
			m.logsPanel.AddSuccess(msg.Source, msg.Message)
		case "error":
			m.logsPanel.AddError(msg.Source, msg.Message)
		case "warn":
			m.logsPanel.AddWarn(msg.Source, msg.Message)
		case "debug":
			m.logsPanel.AddDebug(msg.Source, msg.Message)
		default:
			m.logsPanel.AddInfo(msg.Source, msg.Message)
		}
		// Continue listening for more log events
		cmds = append(cmds, m.listenForLogEvents())

	case AgentStreamMsg:
		// Handle real-time agent streaming events
		source := msg.Agent
		if source == "" {
			source = "agent"
		}

		switch msg.Kind {
		case "started":
			// Extract phase from event data
			phase := ""
			if p, ok := msg.Data["phase"].(string); ok {
				phase = p
			}
			model := ""
			if m, ok := msg.Data["model"].(string); ok {
				model = m
			}
			// Extract timeout from event data (in seconds or as duration)
			var maxTimeout time.Duration
			if t, ok := msg.Data["timeout_seconds"].(float64); ok && t > 0 {
				maxTimeout = time.Duration(t) * time.Second
			} else if t, ok := msg.Data["timeout_seconds"].(int); ok && t > 0 {
				maxTimeout = time.Duration(t) * time.Second
			}
			// Update agent to running state with start time
			StartAgent(m.agentInfos, msg.Agent, phase, maxTimeout, model)

			// Only log workflow-level started events (those with phase info)
			// Skip CLI adapter events as they're redundant with progress bars
			if phase != "" {
				details := msg.Message
				if model, ok := msg.Data["model"].(string); ok && model != "" {
					details += fmt.Sprintf(" [%s]", model)
				}
				details = fmt.Sprintf("[%s] %s", phase, details)
				m.logsPanel.AddInfo(source, "▶ "+details)
			}

		case "tool_use":
			// Update agent activity (shown in progress bar)
			UpdateAgentActivity(m.agentInfos, msg.Agent, "🔧", msg.Message)
			// Don't log tool_use - shown in progress bar instead

		case "thinking":
			// Update agent activity (shown in progress bar)
			UpdateAgentActivity(m.agentInfos, msg.Agent, "💭", "thinking...")
			// Don't log thinking - shown in progress bar instead

		case "chunk":
			// Skip chunk events - too noisy

		case "progress":
			// Update agent activity (shown in progress bar)
			details := msg.Message
			isRetry := false
			if attempt, ok := msg.Data["attempt"].(int); ok && attempt > 0 {
				isRetry = true
				if errMsg, ok := msg.Data["error"].(string); ok {
					details = fmt.Sprintf("retry #%d: %s", attempt, errMsg)
				}
			}
			UpdateAgentActivity(m.agentInfos, msg.Agent, "⟳", details)
			// Only log retries (important for debugging), skip streaming activity
			if isRetry {
				m.logsPanel.AddWarn(source, "⟳ "+details)
			}

		case "completed":
			// Extract token counts with flexible type handling
			tokensIn := extractTokenValue(msg.Data, "tokens_in")
			tokensOut := extractTokenValue(msg.Data, "tokens_out")
			if model, ok := msg.Data["model"].(string); ok && model != "" {
				for _, a := range m.agentInfos {
					if strings.EqualFold(a.Name, msg.Agent) {
						a.Model = model
						break
					}
				}
			}

			// Update agent to completed state
			found, rejectedIn, rejectedOut := CompleteAgent(m.agentInfos, msg.Agent, tokensIn, tokensOut)

			// Debug: log when values are rejected as suspicious
			if found && (rejectedIn > 0 || rejectedOut > 0) {
				m.logsPanel.AddWarn(source, fmt.Sprintf("⚠ Rejected suspicious tokens: in=%d out=%d (raw types: %T / %T)",
					rejectedIn, rejectedOut, msg.Data["tokens_in"], msg.Data["tokens_out"]))
			}

			// Log completed event (important - keep in logs)
			details := msg.Message
			var stats []string
			if model, ok := msg.Data["model"].(string); ok && model != "" {
				stats = append(stats, model)
			}
			if tokensIn > 0 || tokensOut > 0 {
				stats = append(stats, fmt.Sprintf("↑%d ↓%d tok", tokensIn, tokensOut))
			}
			if cost, ok := msg.Data["cost_usd"].(float64); ok && cost > 0 {
				stats = append(stats, fmt.Sprintf("$%.4f", cost))
			}
			if durationMS, ok := msg.Data["duration_ms"].(int64); ok {
				if durationMS >= 1000 {
					stats = append(stats, fmt.Sprintf("%.1fs", float64(durationMS)/1000))
				} else {
					stats = append(stats, fmt.Sprintf("%dms", durationMS))
				}
			}
			if toolCalls, ok := msg.Data["tool_calls"].(int); ok && toolCalls > 0 {
				stats = append(stats, fmt.Sprintf("%d tools", toolCalls))
			}
			if len(stats) > 0 {
				details += " [" + strings.Join(stats, " | ") + "]"
			}
			m.logsPanel.AddSuccess(source, "✓ "+details)

			// Refresh stats panels after token update
			m.updateLogsPanelTokenStats()
			m.updateTokenPanelStats()

		case "error":
			// Update agent to error state
			FailAgent(m.agentInfos, msg.Agent, msg.Message)

			// Log error event (important - keep in logs)
			details := msg.Message
			var errorInfo []string
			if errType, ok := msg.Data["error_type"].(string); ok && errType != "" {
				errorInfo = append(errorInfo, errType)
			}
			if model, ok := msg.Data["model"].(string); ok && model != "" {
				errorInfo = append(errorInfo, model)
			}
			if phase, ok := msg.Data["phase"].(string); ok {
				errorInfo = append(errorInfo, phase)
			}
			if durationMS, ok := msg.Data["duration_ms"].(int64); ok {
				errorInfo = append(errorInfo, fmt.Sprintf("%dms", durationMS))
			}
			if retries, ok := msg.Data["retries"].(int); ok && retries > 0 {
				errorInfo = append(errorInfo, fmt.Sprintf("%d retries", retries))
			}
			if len(errorInfo) > 0 {
				details += " [" + strings.Join(errorInfo, " | ") + "]"
			}
			m.logsPanel.AddError(source, "✗ "+details)

		default:
			// Skip unknown event types - don't log to reduce noise
		}
		// Continue listening for more events
		cmds = append(cmds, m.listenForLogEvents())

	case QuitMsg:
		m.quitting = true
		m.explorerPanel.Close() // Stop file watcher
		return m, tea.Quit
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

		// Determine timeout (use configured value or default to 3 min)
		timeout := m.chatTimeout
		if timeout == 0 {
			timeout = 3 * time.Minute
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
		cmd := exec.Command("sh", "-c", cmdStr)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()

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
	case "help":
		var helpText string
		if len(args) > 0 {
			helpText = m.commands.Help(args[0])
		} else {
			helpText = m.commands.Help("")
		}
		addSystem(helpText)
		m.updateViewport()

	case "clear":
		m.history.Clear()
		m.updateViewport()

	case "quit":
		m.quitting = true
		m.explorerPanel.Close() // Stop file watcher
		return m, tea.Quit

	case "model":
		if len(args) > 0 {
			m.currentModel = args[0]
			addSystem("Model: " + m.currentModel)
		} else {
			modelInfo := m.currentModel
			if modelInfo == "" {
				// Get the default model for current agent
				if models, ok := m.agentModels[m.currentAgent]; ok && len(models) > 0 {
					modelInfo = models[0] + " (default)"
				} else {
					modelInfo = "(unknown)"
				}
			}
			addSystem("Current model: " + modelInfo)
		}
		m.updateViewport()

	case "agent":
		if len(args) > 0 {
			m.currentAgent = args[0]
			// Reset model to empty so the agent uses its configured default
			m.currentModel = ""
			addSystem("Agent: " + m.currentAgent + " (using default model)")
		} else {
			modelInfo := m.currentModel
			if modelInfo == "" {
				modelInfo = "default"
			}
			addSystem(fmt.Sprintf("Current agent: %s (model: %s)", m.currentAgent, modelInfo))
		}
		m.updateViewport()

	case "status":
		if m.workflowState != nil {
			status := strings.TrimPrefix(formatWorkflowStatus(m.workflowState), "/status\n\n")
			addSystem(status)
		} else {
			// Check if there's an active workflow in state that could be loaded
			var hint string
			if m.runner != nil {
				ctx := context.Background()
				if workflows, err := m.runner.ListWorkflows(ctx); err == nil {
					for _, wf := range workflows {
						if wf.IsActive {
							hint = fmt.Sprintf("\n\nTip: /load %s to continue your previous session", wf.WorkflowID)
							break
						}
					}
				}
			}
			addSystem("No workflow loaded in this session." + hint)
		}
		m.updateViewport()
		return m, nil

	case "workflows":
		if m.runner == nil {
			addSystem("Workflow runner not configured")
			m.updateViewport()
			return m, nil
		}
		// List workflows from runner
		ctx := context.Background()
		workflows, err := m.runner.ListWorkflows(ctx)
		if err != nil {
			addSystem(fmt.Sprintf("Error listing workflows: %v", err))
			m.updateViewport()
			return m, nil
		}
		if len(workflows) == 0 {
			addSystem("No workflows found. Use '/analyze <prompt>' to start one.")
			m.updateViewport()
			return m, nil
		}
		var sb strings.Builder
		for i, wf := range workflows {
			marker := "  "
			if wf.IsActive {
				marker = "> "
			}
			prompt := wf.Prompt
			if len(prompt) > 60 {
				prompt = prompt[:57] + "..."
			}
			status := strings.ToUpper(string(wf.Status))
			sb.WriteString(fmt.Sprintf("%s#%d  %-11s @%-8s  %s\n", marker, i+1, status, wf.CurrentPhase, wf.WorkflowID))
			sb.WriteString(fmt.Sprintf("    %q\n\n", prompt))
		}
		sb.WriteString("> = active  |  /load <ID> to switch")
		addSystem(sb.String())
		m.updateViewport()

	case "load":
		if m.runner == nil {
			addSystem("Workflow runner not configured")
			m.updateViewport()
			return m, nil
		}
		if m.workflowRunning {
			addSystem("Workflow already running. Use /cancel first.")
			m.updateViewport()
			return m, nil
		}

		ctx := context.Background()

		// If no args, show available workflows to select from
		if len(args) == 0 {
			workflows, err := m.runner.ListWorkflows(ctx)
			if err != nil {
				addSystem(fmt.Sprintf("Error listing workflows: %v", err))
				m.updateViewport()
				return m, nil
			}
			if len(workflows) == 0 {
				addSystem("No workflows found. Use '/analyze <prompt>' to start one.")
				m.updateViewport()
				return m, nil
			}
			var sb strings.Builder
			for i, wf := range workflows {
				marker := "  "
				if wf.IsActive {
					marker = "> "
				}
				prompt := wf.Prompt
				if len(prompt) > 60 {
					prompt = prompt[:57] + "..."
				}
				status := strings.ToUpper(string(wf.Status))
				sb.WriteString(fmt.Sprintf("%s#%d  %-11s @%-8s  %s\n", marker, i+1, status, wf.CurrentPhase, wf.WorkflowID))
				sb.WriteString(fmt.Sprintf("    %q\n\n", prompt))
			}
			sb.WriteString("> = active  |  /load <ID> to switch")
			addSystem(sb.String())
			m.updateViewport()
			return m, nil
		}

		// Load the specified workflow
		workflowID := args[0]
		state, err := m.runner.LoadWorkflow(ctx, workflowID)
		if err != nil {
			addSystem(fmt.Sprintf("Error loading workflow: %v", err))
			m.updateViewport()
			return m, nil
		}

		// Update internal state
		m.workflowState = state
		m.tasksPanel.SetState(state)
		m.updateQuorumPanel(state)

		// Show success message with workflow details
		var sb strings.Builder

		// Status with icon
		statusIcon := "○"
		switch state.Status {
		case core.WorkflowStatusCompleted:
			statusIcon = "●"
		case core.WorkflowStatusRunning:
			statusIcon = "◐"
		case core.WorkflowStatusFailed:
			statusIcon = "✗"
		}
		sb.WriteString(fmt.Sprintf("%s Loaded  |  %s @%s\n\n", statusIcon, strings.ToUpper(string(state.Status)), state.CurrentPhase))

		// Prompt
		if state.Prompt != "" {
			prompt := state.Prompt
			if len(prompt) > 70 {
				prompt = prompt[:67] + "..."
			}
			sb.WriteString(fmt.Sprintf("%q\n\n", prompt))
		}

		// Metrics inline
		var info []string
		if state.Metrics != nil && state.Metrics.ConsensusScore > 0 {
			info = append(info, fmt.Sprintf("Consensus: %.0f%%", state.Metrics.ConsensusScore*100))
		}
		if len(state.Tasks) > 0 {
			info = append(info, fmt.Sprintf("Issues: %d", len(state.Tasks)))
		}
		if len(info) > 0 {
			sb.WriteString(strings.Join(info, "  |  ") + "\n\n")
		}

		// Next action
		sb.WriteString("Next: ")
		switch state.CurrentPhase {
		case core.PhaseAnalyze:
			if state.Status == core.WorkflowStatusCompleted {
				sb.WriteString("/plan")
			} else {
				sb.WriteString("/analyze")
			}
		case core.PhasePlan:
			if state.Status == core.WorkflowStatusCompleted {
				sb.WriteString("/execute")
			} else {
				sb.WriteString("/plan")
			}
		case core.PhaseExecute:
			if state.Status == core.WorkflowStatusCompleted {
				sb.WriteString("Done!")
			} else {
				sb.WriteString("/execute")
			}
		default:
			sb.WriteString("/analyze <prompt>")
		}
		sb.WriteString("  |  /status for details")
		addSystem(sb.String())
		m.updateViewport()
		return m, nil // Important: return modified m with workflowState set

	case "new":
		if m.runner == nil {
			addSystem("Workflow runner not configured")
			m.updateViewport()
			return m, nil
		}
		if m.workflowRunning {
			addSystem("Workflow is running. Use /cancel first.")
			m.updateViewport()
			return m, nil
		}

		ctx := context.Background()

		// Parse flags from args
		archive := false
		purge := false
		for _, arg := range args {
			switch arg {
			case "--archive", "-a":
				archive = true
			case "--purge", "-p":
				purge = true
			}
		}

		// Handle purge (most destructive)
		if purge {
			deleted, err := m.runner.PurgeAllWorkflows(ctx)
			if err != nil {
				addSystem(fmt.Sprintf("Error purging workflows: %v", err))
				m.updateViewport()
				return m, nil
			}
			m.workflowState = nil
			m.tasksPanel.SetState(nil)
			m.updateQuorumPanel(nil)
			addSystem(fmt.Sprintf("Purged %d workflow(s). All state deleted.\nUse '/analyze <prompt>' to start fresh.", deleted))
			m.updateViewport()
			return m, nil
		}

		// Handle archive
		if archive {
			archived, err := m.runner.ArchiveWorkflows(ctx)
			if err != nil {
				addSystem(fmt.Sprintf("Error archiving workflows: %v", err))
				m.updateViewport()
				return m, nil
			}
			if err := m.runner.DeactivateWorkflow(ctx); err != nil {
				addSystem(fmt.Sprintf("Error deactivating workflow: %v", err))
				m.updateViewport()
				return m, nil
			}
			m.workflowState = nil
			m.tasksPanel.SetState(nil)
			m.updateQuorumPanel(nil)
			msg := "Ready for new task."
			if archived > 0 {
				msg = fmt.Sprintf("Archived %d completed workflow(s). %s", archived, msg)
			}
			addSystem(msg + "\nUse '/analyze <prompt>' to start a new workflow.")
			m.updateViewport()
			return m, nil
		}

		// Default: just deactivate
		if err := m.runner.DeactivateWorkflow(ctx); err != nil {
			addSystem(fmt.Sprintf("Error deactivating workflow: %v", err))
			m.updateViewport()
			return m, nil
		}
		m.workflowState = nil
		m.tasksPanel.SetState(nil)
		m.updateQuorumPanel(nil)
		addSystem("Workflow deactivated. Ready for new task.\nUse '/analyze <prompt>' to start a new workflow.\nPrevious workflows available via '/workflows'.")
		m.updateViewport()
		return m, nil

	case "delete":
		if m.runner == nil {
			addSystem("Workflow runner not configured")
			m.updateViewport()
			return m, nil
		}
		if m.workflowRunning {
			addSystem("Cannot delete while workflow is running. Use /cancel first.")
			m.updateViewport()
			return m, nil
		}
		if len(args) == 0 {
			addSystem("Usage: /delete <workflow-id>\nUse /workflows to see available IDs.")
			m.updateViewport()
			return m, nil
		}

		ctx := context.Background()
		workflowID := args[0]

		// Load workflow to verify it exists and check status
		wf, err := m.runner.LoadWorkflow(ctx, workflowID)
		if err != nil || wf == nil {
			addSystem(fmt.Sprintf("Workflow not found: %s", workflowID))
			m.updateViewport()
			return m, nil
		}

		// Prevent deletion of running workflows
		if wf.Status == core.WorkflowStatusRunning {
			addSystem("Cannot delete running workflow. Use /cancel first.")
			m.updateViewport()
			return m, nil
		}

		// Delete the workflow
		if err := m.runner.DeleteWorkflow(ctx, workflowID); err != nil {
			addSystem(fmt.Sprintf("Error deleting workflow: %v", err))
			m.updateViewport()
			return m, nil
		}

		// Clear state if we just deleted the active workflow
		if m.workflowState != nil && string(m.workflowState.WorkflowID) == workflowID {
			m.workflowState = nil
			m.tasksPanel.SetState(nil)
			m.updateQuorumPanel(nil)
		}

		addSystem(fmt.Sprintf("Workflow %s deleted.", workflowID))
		m.updateViewport()
		return m, nil

	case "cancel":
		if m.controlPlane != nil && m.workflowRunning {
			m.controlPlane.Cancel()
			m.workflowRunning = false
			addSystem("Workflow cancelled")
		} else {
			addSystem("No active workflow")
		}
		m.updateViewport()

	case "analyze":
		if m.runner == nil {
			addSystem("Workflow runner not configured")
			m.updateViewport()
			return m, nil
		}
		if m.workflowRunning {
			addSystem("Workflow already running. Use /cancel first.")
			m.updateViewport()
			return m, nil
		}
		if len(args) == 0 {
			addSystem("Usage: /analyze <prompt>")
			m.updateViewport()
			return m, nil
		}

		prompt := strings.Join(args, " ")
		m.updateViewport()
		return m, m.runAnalyze(prompt)

	case "plan":
		if m.runner == nil {
			addSystem("Workflow runner not configured")
			m.updateViewport()
			return m, nil
		}
		if m.workflowRunning {
			addSystem("Workflow already running. Use /cancel first.")
			m.updateViewport()
			return m, nil
		}

		// If no args, continue from active workflow (after /analyze)
		if len(args) == 0 {
			// Try to load active workflow state if not in memory
			if m.workflowState == nil {
				if state, err := m.runner.GetState(context.Background()); err == nil && state != nil {
					m.workflowState = state
					m.tasksPanel.SetState(state)
					m.updateQuorumPanel(state)
				}
			}

			// Check if we can continue to plan phase:
			// - Already in plan phase, OR
			// - Analyze phase completed (status=completed with analyze as current phase)
			canContinue := false
			if m.workflowState != nil {
				if m.workflowState.CurrentPhase == core.PhasePlan {
					canContinue = true
				} else if m.workflowState.CurrentPhase == core.PhaseAnalyze && m.workflowState.Status == core.WorkflowStatusCompleted {
					// Analyze completed - can advance to plan
					canContinue = true
				}
			}

			if canContinue {
				addSystem("Continuing to planning phase from active workflow...")
				m.updateViewport()
				return m, m.runPlanPhase()
			}
			addSystem("No active workflow to continue. Use '/plan <prompt>' to start new or '/analyze' first.")
			m.updateViewport()
			return m, nil
		}

		// With args, start new workflow
		prompt := strings.Join(args, " ")
		m.updateViewport()
		return m, m.runWorkflow(prompt)

	case "replan":
		if m.runner == nil {
			addSystem("Workflow runner not configured")
			m.updateViewport()
			return m, nil
		}
		if m.workflowRunning {
			addSystem("Workflow already running. Use /cancel first.")
			m.updateViewport()
			return m, nil
		}

		// Try to load active workflow state if not in memory
		if m.workflowState == nil {
			if state, err := m.runner.GetState(context.Background()); err == nil && state != nil {
				m.workflowState = state
				m.tasksPanel.SetState(state)
				m.updateQuorumPanel(state)
			}
		}

		if m.workflowState == nil {
			addSystem("No active workflow to replan. Use '/analyze' first.")
			m.updateViewport()
			return m, nil
		}

		// Get additional context if provided
		additionalContext := ""
		if len(args) > 0 {
			additionalContext = strings.Join(args, " ")
		}

		if additionalContext != "" {
			addSystem(fmt.Sprintf("Replanning with additional context (%d chars)...", len(additionalContext)))
		} else {
			addSystem("Replanning: clearing existing issues and regenerating...")
		}
		m.updateViewport()
		return m, m.runReplanPhase(additionalContext)

	case "useplan", "up", "useplans":
		if m.runner == nil {
			addSystem("Workflow runner not configured")
			m.updateViewport()
			return m, nil
		}
		if m.workflowRunning {
			addSystem("Workflow already running. Use /cancel first.")
			m.updateViewport()
			return m, nil
		}

		// Try to load active workflow state if not in memory
		if m.workflowState == nil {
			if state, err := m.runner.GetState(context.Background()); err == nil && state != nil {
				m.workflowState = state
				m.tasksPanel.SetState(state)
				m.updateQuorumPanel(state)
			}
		}

		if m.workflowState == nil {
			addSystem("No active workflow found. Use '/analyze' first.")
			m.updateViewport()
			return m, nil
		}

		addSystem("Loading existing task files from filesystem (skipping agent call)...")
		m.updateViewport()
		return m, m.runUsePlanPhase()

	case "run":
		if m.runner == nil {
			addSystem("Workflow runner not configured")
			m.updateViewport()
			return m, nil
		}
		if m.workflowRunning {
			addSystem("Workflow already running. Use /cancel first.")
			m.updateViewport()
			return m, nil
		}
		if len(args) == 0 {
			addSystem("Usage: /run <prompt>")
			m.updateViewport()
			return m, nil
		}

		prompt := strings.Join(args, " ")
		m.updateViewport()
		return m, m.runWorkflow(prompt)

	case "execute":
		if m.runner == nil {
			addSystem("Workflow runner not configured")
			m.updateViewport()
			return m, nil
		}
		if m.workflowRunning {
			addSystem("Workflow already running. Use /cancel first.")
			m.updateViewport()
			return m, nil
		}

		// Try to load active workflow state if not in memory
		if m.workflowState == nil {
			if state, err := m.runner.GetState(context.Background()); err == nil && state != nil {
				m.workflowState = state
				m.tasksPanel.SetState(state)
				m.updateQuorumPanel(state)
			}
		}

		// Check if we can continue to execute phase:
		// - Already in execute phase, OR
		// - Plan phase completed (status=completed with plan as current phase), OR
		// - Tasks exist (even if plan status is "failed" - tasks may have been created successfully)
		canContinue := false
		needsStateRepair := false
		if m.workflowState != nil {
			if m.workflowState.CurrentPhase == core.PhaseExecute {
				canContinue = true
			} else if m.workflowState.CurrentPhase == core.PhasePlan && m.workflowState.Status == core.WorkflowStatusCompleted {
				// Plan completed - can advance to execute
				canContinue = true
			} else if len(m.workflowState.Tasks) > 0 {
				// Tasks exist! Even if plan "failed", we can execute the existing tasks.
				// This handles the case where task files were created but manifest parsing failed.
				canContinue = true
				needsStateRepair = true
				addSystem(fmt.Sprintf("Found %d existing tasks. Recovering workflow state...", len(m.workflowState.Tasks)))
			}
		}

		if canContinue {
			if needsStateRepair {
				// Repair the state before executing
				m.workflowState.Status = core.WorkflowStatusRunning
				m.workflowState.CurrentPhase = core.PhaseExecute
				m.workflowState.UpdatedAt = time.Now()
				// Add a checkpoint to indicate plan is complete
				m.workflowState.Checkpoints = append(m.workflowState.Checkpoints, core.Checkpoint{
					ID:        fmt.Sprintf("cp-repair-%d", time.Now().UnixNano()),
					Type:      "phase_complete",
					Phase:     core.PhasePlan,
					Timestamp: time.Now(),
				})
				// Save the repaired state
				if err := m.runner.SaveState(context.Background(), m.workflowState); err != nil {
					addSystem(fmt.Sprintf("Warning: Failed to save repaired state: %v", err))
				} else {
					addSystem("Workflow state repaired successfully.")
				}
			}
			addSystem("Continuing to execution phase from active workflow...")
			m.updateViewport()
			return m, m.runExecutePhase()
		}
		addSystem("No active workflow to execute. Use '/plan' first.")

	case "retry":
		if m.controlPlane == nil {
			addSystem("No control plane")
			m.updateViewport()
			return m, nil
		}
		if len(args) > 0 {
			m.controlPlane.RetryTask(core.TaskID(args[0]))
			addSystem(fmt.Sprintf("Retrying: %s", args[0]))
		} else {
			addSystem("Usage: /retry <task_id>")
		}
		m.updateViewport()

	case "copy":
		// Copy last agent response (delegate to copyLastResponse)
		newModel, _, _ := m.copyLastResponse()
		return newModel.(Model), nil

	case "copyall":
		// Copy entire conversation (delegate to copyConversation)
		newModel, _, _ := m.copyConversation()
		return newModel.(Model), nil

	case "logs":
		// Toggle logs panel
		m.showLogs = !m.showLogs
		if m.width > 0 && m.height > 0 {
			m.recalculateLayout()
		}
		m.updateViewport()

	case "clearlogs":
		// Clear logs
		m.logsPanel.Clear()
		addSystem("Logs cleared")
		m.updateViewport()

	case "copylogs":
		// Copy logs to clipboard
		newModel, _, _ := m.copyLogsToClipboard()
		m = newModel.(Model)
		m.updateViewport()

	case "explorer":
		// Toggle explorer panel
		m.showExplorer = !m.showExplorer
		if m.showExplorer {
			m.explorerFocus = true
			m.inputFocused = false
			m.textarea.Blur()
			m.explorerPanel.SetFocused(true)
			addSystem("File explorer opened (arrows to navigate, Esc to return)")
		} else {
			m.explorerFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			m.explorerPanel.SetFocused(false)
			addSystem("File explorer closed")
		}
		if m.width > 0 && m.height > 0 {
			m.recalculateLayout()
		}
		m.updateViewport()

	case "theme":
		// Toggle or set theme
		var themeName string
		if len(args) > 0 {
			themeName = strings.ToLower(args[0])
		} else {
			// Toggle: if currently dark, switch to light; if light, switch to dark
			if m.darkTheme {
				themeName = "light"
			} else {
				themeName = "dark"
			}
		}

		switch themeName {
		case "dark":
			m.darkTheme = true
			tui.SetColorScheme(tui.DarkScheme)
			applyDarkTheme()
			ApplyDarkThemeMessages()
			m.messageStyles = NewMessageStyles(m.viewport.Width)
			addSystem("Theme: dark")
		case "light":
			m.darkTheme = false
			tui.SetColorScheme(tui.LightScheme)
			applyLightTheme()
			ApplyLightThemeMessages()
			m.messageStyles = NewMessageStyles(m.viewport.Width)
			addSystem("Theme: light")
		default:
			addSystem("Usage: /theme [dark|light]")
		}
		m.updateViewport()
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
	// === EXACT WIDTH CALCULATIONS (must come first for textarea width) ===
	// No outer margins - panels fill the entire width when joined with JoinHorizontal
	leftSidebarWidth := 0
	rightSidebarWidth := 0 // Shared by stats and logs panels
	mainWidth := m.width

	// Determine how many sidebars are open for dynamic sizing
	showLeftSidebar := m.showExplorer || m.showTokens
	showRightSidebar := m.showStats || m.showLogs
	bothSidebarsOpen := showLeftSidebar && showRightSidebar
	oneSidebarOpen := (showLeftSidebar || showRightSidebar) && !bothSidebarsOpen

	// Left sidebar (explorer and/or tokens)
	if showLeftSidebar {
		if oneSidebarOpen {
			// More space when only one sidebar is open (2/5 of width)
			leftSidebarWidth = m.width * 2 / 5
			if leftSidebarWidth < 35 {
				leftSidebarWidth = 35
			}
			if leftSidebarWidth > 70 {
				leftSidebarWidth = 70
			}
		} else {
			// Less space when both sidebars are open (1/4 of width)
			leftSidebarWidth = m.width / 4
			if leftSidebarWidth < 30 {
				leftSidebarWidth = 30
			}
			if leftSidebarWidth > 50 {
				leftSidebarWidth = 50
			}
		}
		mainWidth -= leftSidebarWidth
	}

	// Right sidebar (contains stats and/or logs)
	if showRightSidebar {
		if oneSidebarOpen {
			// More space when only one sidebar is open (2/5 of width)
			rightSidebarWidth = m.width * 2 / 5
			if rightSidebarWidth < 40 {
				rightSidebarWidth = 40
			}
			if rightSidebarWidth > 80 {
				rightSidebarWidth = 80
			}
		} else {
			// Less space when both sidebars are open (1/4 of width)
			rightSidebarWidth = m.width / 4
			if rightSidebarWidth < 35 {
				rightSidebarWidth = 35
			}
			if rightSidebarWidth > 60 {
				rightSidebarWidth = 60
			}
		}
		mainWidth -= rightSidebarWidth
	}

	// === SET TEXTAREA WIDTH FIRST (needed for accurate line calculation) ===
	// The textarea width must match the available content area in renderInput:
	// - renderMainContent receives mainWidth - 2 (for main panel borders)
	// - renderInput uses style.Width(width - 4) for content area = mainWidth - 6
	// - Prefix can be up to 3 chars (spinner + space)
	// - So textarea width = mainWidth - 6 - 3 = mainWidth - 9
	// Using mainWidth - 8 to be slightly generous while avoiding overflow
	inputWidth := mainWidth - 8
	if inputWidth < 20 {
		inputWidth = 20
	}
	m.textarea.SetWidth(inputWidth)

	// === EXACT HEIGHT CALCULATIONS ===
	// Header: logo line (1) + agents bar (1) + divider (1) = 3
	headerHeight := 3

	// Pipeline takes 2 lines when visible
	if m.workflowRunning || m.workflowPhase == "done" {
		headerHeight += 2
	}

	// Footer: keybindings line (1) + padding (1) = 2
	footerHeight := 2

	// Calculate dynamic input height based on content (uses textarea width set above)
	inputLines := m.calculateInputLines()
	// Input area: border top (1) + content lines + border bottom (1) + margin (1) = inputLines + 3
	inputHeight := inputLines + 3

	// Status line / progress bars height
	statusHeight := 0
	if m.workflowRunning {
		// Progress bars: one line per agent
		statusHeight = len(m.agentInfos)
		if statusHeight < 1 {
			statusHeight = 1
		}
	} else if m.streaming {
		statusHeight = 1
	}

	// NOTE: Dropdown suggestions are now rendered as an overlay in View(), not inline
	// This prevents the layout from shifting when the dropdown appears

	// Total fixed height (everything except viewport)
	fixedHeight := headerHeight + footerHeight + inputHeight + statusHeight

	// Viewport gets remaining height
	// Subtract 2 for the main content box borders
	viewportHeight := m.height - fixedHeight - 2
	if viewportHeight < 3 {
		viewportHeight = 3
	}

	// === NORMALIZATION OF WIDTHS TO PREVENT OVERFLOW ===
	// Total must equal m.width exactly (no outer margins)
	totalUsed := mainWidth
	if showLeftSidebar {
		totalUsed += leftSidebarWidth
	}
	if showRightSidebar {
		totalUsed += rightSidebarWidth
	}

	if totalUsed > m.width {
		excess := totalUsed - m.width

		// Strategy: reduce mainWidth first (has more margin)
		if mainWidth-excess >= 40 {
			mainWidth -= excess
		} else {
			// If not enough, reduce sidebars proportionally
			reduction := excess / 2
			if showLeftSidebar && leftSidebarWidth-reduction >= 25 {
				leftSidebarWidth -= reduction
				excess -= reduction
			}
			if showRightSidebar && rightSidebarWidth-reduction >= 30 {
				rightSidebarWidth -= reduction
				excess -= reduction
			}
			// Any remaining excess goes to mainWidth
			if excess > 0 && mainWidth-excess >= 40 {
				mainWidth -= excess
			}
		}
	}
	// === END NORMALIZATION ===

	// Ensure minimum main width
	if mainWidth < 40 {
		mainWidth = 40
	}

	// === FINAL OVERFLOW CHECK ===
	// After applying minimums, recalculate total and force-reduce sidebars if needed
	finalTotal := mainWidth
	if showLeftSidebar {
		finalTotal += leftSidebarWidth
	}
	if showRightSidebar {
		finalTotal += rightSidebarWidth
	}

	// If still overflowing, aggressively reduce sidebar widths
	for finalTotal > m.width && (leftSidebarWidth > 20 || rightSidebarWidth > 20) {
		if showRightSidebar && rightSidebarWidth > 20 {
			rightSidebarWidth--
			finalTotal--
		}
		if finalTotal > m.width && showLeftSidebar && leftSidebarWidth > 20 {
			leftSidebarWidth--
			finalTotal--
		}
	}

	// If still overflowing, reduce main width below minimum as last resort
	if finalTotal > m.width {
		excess := finalTotal - m.width
		mainWidth -= excess
		if mainWidth < 30 {
			mainWidth = 30
		}
	}
	// === END FINAL OVERFLOW CHECK ===

	// === SIDEBAR HEIGHT ===
	// All panels should have the same total rendered height = m.height
	// Each panel's Render() uses Height(p.height - 2) + borders = p.height
	sidebarHeight := m.height
	if sidebarHeight < 10 {
		sidebarHeight = 10
	}

	// Set sidebar sizes
	if showLeftSidebar {
		if m.showExplorer && m.showTokens {
			explorerHeight := sidebarHeight / 2
			tokenHeight := sidebarHeight - explorerHeight
			m.explorerPanel.SetSize(leftSidebarWidth, explorerHeight)
			m.tokenPanel.SetSize(leftSidebarWidth, tokenHeight)
		} else if m.showExplorer {
			m.explorerPanel.SetSize(leftSidebarWidth, sidebarHeight)
		} else if m.showTokens {
			m.tokenPanel.SetSize(leftSidebarWidth, sidebarHeight)
		}
	}

	// Stats and Logs share the right sidebar space
	// When both visible, split height (logs on top, stats on bottom)
	if m.showStats && m.showLogs {
		logsHeight := sidebarHeight / 2
		statsHeight := sidebarHeight - logsHeight
		m.logsPanel.SetSize(rightSidebarWidth, logsHeight)
		m.statsPanel.SetSize(rightSidebarWidth, statsHeight)
	} else if m.showStats {
		m.statsPanel.SetSize(rightSidebarWidth, sidebarHeight)
	} else if m.showLogs {
		m.logsPanel.SetSize(rightSidebarWidth, sidebarHeight)
	}

	// === VIEWPORT SETUP ===
	if !m.ready {
		m.viewport = viewport.New(mainWidth, viewportHeight)
		m.viewport.SetContent(m.renderHistory())
		m.ready = true
	} else {
		m.viewport.Width = mainWidth
		m.viewport.Height = viewportHeight
	}

	// === TEXTAREA HEIGHT ===
	// Width was set earlier in this function for accurate line calculation
	// Now set the height based on the calculated inputLines
	m.textarea.SetHeight(inputLines)

	// === UPDATE MARKDOWN RENDERER WIDTH ===
	// Update word wrap to match content viewport width
	contentWidth := mainWidth - 4 // Subtract padding
	m.updateMarkdownRenderer(contentWidth)

	// === UPDATE MESSAGE STYLES ===
	// Recreate message styles with current viewport width for proper alignment
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
		if state.Metrics.TotalCostUSD > 0 {
			metrics = append(metrics, fmt.Sprintf("Cost: $%.4f", state.Metrics.TotalCostUSD))
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
