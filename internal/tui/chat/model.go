package chat

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// Color palette - modern dark theme
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

	// Message styles
	userLabelStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	userMsgStyle = lipgloss.NewStyle().
			Foreground(textColor).
			PaddingLeft(2)

	agentLabelStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	agentMsgStyle = lipgloss.NewStyle().
			Foreground(textColor).
			PaddingLeft(2)

	systemMsgStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Italic(true).
			PaddingLeft(2)

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
- /plan <prompt>    - Generate a plan from analysis
- /run <prompt>     - Run complete workflow
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
	Resume(ctx context.Context) error
	GetState(ctx context.Context) (*core.WorkflowState, error)
	ListWorkflows(ctx context.Context) ([]core.WorkflowSummary, error)
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
	suggestionType  string              // "command", "agent", "model"
	availableAgents []string            // List of enabled agent names
	agentModels     map[string][]string // Models available per agent

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
	inputFocused      bool

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

	// Panel navigation mode (tmux-style with Ctrl+b prefix)
	panelNavMode bool // true when waiting for arrow key to switch panels

	// NEW: Enhanced UI panels
	consensusPanel   *ConsensusPanel
	contextPanel     *ContextPreviewPanel
	diffView         *AgentDiffView
	historySearch    *HistorySearch
	costPanel        *CostPanel
	shortcutsOverlay *ShortcutsOverlay
	fileViewer       *FileViewer
	statsWidget      *StatsWidget
	machineCollector *MachineStatsCollector

	// Cancellation for interrupts
	cancelFunc context.CancelFunc

	// Markdown renderer
	mdRenderer *glamour.TermRenderer

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
		for _, name := range []string{"claude", "gemini", "codex", "copilot"} {
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
		// Initialize new panels
		consensusPanel:   NewConsensusPanel(80.0), // 80% threshold
		contextPanel:     NewContextPreviewPanel(),
		diffView:         NewAgentDiffView(),
		historySearch:    NewHistorySearch(),
		costPanel:        NewCostPanel(1.0), // $1 default budget
		shortcutsOverlay: NewShortcutsOverlay(),
		fileViewer:       NewFileViewer(),
		statsWidget:      NewStatsWidget(),
		machineCollector: NewMachineStatsCollector(),
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
		statsTickCmd(),            // Start stats updates
		tea.EnableMouseCellMotion, // Enable mouse support for click-to-focus
	)
}

// listenForLogEvents creates a command that listens for log events from the workflow.
func (m Model) listenForLogEvents() tea.Cmd {
	if m.logEventsCh == nil {
		return nil
	}
	return func() tea.Msg {
		for {
			event, ok := <-m.logEventsCh
			if !ok {
				return nil // Channel closed
			}

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

			// Unknown event type - continue listening instead of returning nil
			// This prevents the listener from stopping on unknown events
		}
	}
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
	TickMsg             struct{ Time time.Time }
	ExplorerRefreshMsg  struct{} // File system change detected
	StatsTickMsg        struct{} // Periodic stats update
	ChatProgressTickMsg struct {
		Elapsed time.Duration
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

	// Panel navigation mode (tmux-style: Ctrl+b then arrow keys)
	if m.panelNavMode {
		switch msg.Type {
		case tea.KeyEsc:
			// Cancel panel nav mode
			m.panelNavMode = false
			return m, nil, true
		case tea.KeyLeft:
			// Focus explorer (left panel)
			m.panelNavMode = false
			if m.showExplorer {
				m.explorerFocus = true
				m.logsFocus = false
				m.statsFocus = false
				m.inputFocused = false
				m.textarea.Blur()
				m.explorerPanel.SetFocused(true)
				m.logsPanel.AddInfo("system", "Explorer focused (Esc to return)")
			}
			return m, nil, true
		case tea.KeyRight:
			// Focus right sidebar (stats or logs)
			m.panelNavMode = false
			if m.showStats {
				m.statsFocus = true
				m.logsFocus = false
				m.explorerFocus = false
				m.inputFocused = false
				m.textarea.Blur()
				m.explorerPanel.SetFocused(false)
				m.logsPanel.AddInfo("system", "Stats focused (↑↓ scroll, Esc to return)")
			} else if m.showLogs {
				m.logsFocus = true
				m.statsFocus = false
				m.explorerFocus = false
				m.inputFocused = false
				m.textarea.Blur()
				m.explorerPanel.SetFocused(false)
				m.logsPanel.AddInfo("system", "Logs focused (↑↓ scroll, Esc to return)")
			}
			return m, nil, true
		case tea.KeyUp, tea.KeyDown:
			// Switch between stats and logs in right sidebar
			m.panelNavMode = false
			if m.showStats && m.showLogs {
				if m.statsFocus || msg.Type == tea.KeyUp {
					m.logsFocus = true
					m.statsFocus = false
					m.logsPanel.AddInfo("system", "Logs focused (↑↓ scroll, Esc to return)")
				} else {
					m.statsFocus = true
					m.logsFocus = false
					m.logsPanel.AddInfo("system", "Stats focused (↑↓ scroll, Esc to return)")
				}
				m.explorerFocus = false
				m.inputFocused = false
				m.textarea.Blur()
				m.explorerPanel.SetFocused(false)
			}
			return m, nil, true
		default:
			// Any other key cancels panel nav mode
			m.panelNavMode = false
		}
	}

	// Ctrl+b enters panel navigation mode (tmux-style)
	if msg.Type == tea.KeyCtrlB {
		m.panelNavMode = true
		m.logsPanel.AddInfo("system", "Panel nav: ←→ switch panels, ↑↓ logs/stats, Esc cancel")
		return m, nil, true
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
		// Skip general Tab handling when explorer has focus (handled in explorer section)
		if m.explorerFocus {
			break
		}
		if m.showSuggestions && len(m.suggestions) > 0 {
			// Complete with selected suggestion based on type
			switch m.suggestionType {
			case "agent":
				m.textarea.SetValue("/agent " + m.suggestions[m.suggestionIndex])
			case "model":
				m.textarea.SetValue("/model " + m.suggestions[m.suggestionIndex])
			default:
				m.textarea.SetValue("/" + m.suggestions[m.suggestionIndex] + " ")
			}
			m.textarea.CursorEnd()
			m.showSuggestions = false
			m.suggestionIndex = 0
			m.suggestionType = ""
			return m, nil, true
		} else if (m.textarea.Value() == "" || m.textarea.Value() == "/") && !m.streaming && !m.workflowRunning {
			// Show all commands on Tab with empty or just "/" (not during streaming)
			m.textarea.SetValue("/")
			m.suggestions = m.commands.Suggest("/")
			m.suggestionType = "command"
			m.showSuggestions = len(m.suggestions) > 0
			m.suggestionIndex = 0
			return m, nil, true
		}

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
		} else if m.statsFocus && m.showStats {
			// Return focus from stats to input
			m.statsFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			return m, nil, true
		} else if m.streaming || m.workflowRunning {
			// Cancel current operation
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
				m.logsPanel.AddWarn("system", "Workflow interrupted by user (Esc)")
				m.history.Add(NewSystemMessage("Workflow interrupted"))
			} else if wasStreaming {
				m.logsPanel.AddWarn("system", "Request interrupted by user (Esc)")
				m.history.Add(NewSystemMessage("Request interrupted"))
			}
			m.updateViewport()
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

	case tea.KeyCtrlS:
		// Toggle stats panel
		m.showStats = !m.showStats
		if m.showStats {
			m.statsFocus = true
			m.logsFocus = false
			m.explorerFocus = false
			m.inputFocused = false
			m.textarea.Blur()
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

	case tea.KeyCtrlK:
		// Toggle consensus panel
		m.consensusPanel.Toggle()
		return m, nil, true

	case tea.KeyCtrlD:
		// Toggle diff view
		if m.diffView.HasContent() {
			m.diffView.Toggle()
		}
		return m, nil, true

	case tea.KeyCtrlR:
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
				m.fileViewer.Hide()
				cmd := exec.Command(editor, filePath)
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
					m.logsPanel.AddInfo("explorer", "Viewing: "+filepath.Base(path))
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
func (m Model) handleMouseClick(x, _ int) (tea.Model, tea.Cmd, bool) {
	// Calculate panel boundaries based on current layout
	explorerWidth := 0
	logsWidth := 0

	// Determine how many sidebars are open for dynamic sizing
	bothSidebarsOpen := m.showExplorer && m.showLogs
	oneSidebarOpen := (m.showExplorer || m.showLogs) && !bothSidebarsOpen

	if m.showExplorer {
		if oneSidebarOpen {
			explorerWidth = m.width * 2 / 5
			if explorerWidth < 35 {
				explorerWidth = 35
			}
			if explorerWidth > 70 {
				explorerWidth = 70
			}
		} else {
			explorerWidth = m.width / 4
			if explorerWidth < 30 {
				explorerWidth = 30
			}
			if explorerWidth > 50 {
				explorerWidth = 50
			}
		}
	}

	if m.showLogs {
		if oneSidebarOpen {
			logsWidth = m.width * 2 / 5
			if logsWidth < 40 {
				logsWidth = 40
			}
			if logsWidth > 80 {
				logsWidth = 80
			}
		} else {
			logsWidth = m.width / 4
			if logsWidth < 35 {
				logsWidth = 35
			}
			if logsWidth > 60 {
				logsWidth = 60
			}
		}
	}

	// Determine which panel was clicked based on X coordinate
	mainStart := explorerWidth
	if m.showExplorer {
		mainStart += 1 // Account for separator
	}
	mainEnd := m.width - logsWidth
	if m.showLogs {
		mainEnd -= 1 // Account for separator
	}

	// Check if click is in Explorer panel
	if m.showExplorer && x < explorerWidth {
		if !m.explorerFocus {
			m.explorerFocus = true
			m.inputFocused = false
			m.textarea.Blur()
			m.explorerPanel.SetFocused(true)
			return m, nil, true
		}
		return m, nil, false // Already focused, let it handle internally
	}

	// Check if click is in Logs panel
	if m.showLogs && x > mainEnd {
		if !m.logsFocus {
			m.logsFocus = true
			m.explorerFocus = false
			m.inputFocused = false
			m.textarea.Blur()
			m.explorerPanel.SetFocused(false)
			m.logsPanel.AddInfo("system", "Logs panel focused (↑↓ to scroll)")
			return m, nil, true
		}
		return m, nil, false // Already focused
	}

	// Click is in Main content area
	if x >= mainStart && x < mainEnd {
		if m.explorerFocus || m.logsFocus {
			m.explorerFocus = false
			m.logsFocus = false
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
			err := clipboard.WriteAll(msgs[i].Content)
			if err != nil {
				m.logsPanel.AddError("system", "Failed to copy: "+err.Error())
			} else {
				// Show brief confirmation
				agent := msgs[i].Agent
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

	err := clipboard.WriteAll(sb.String())
	if err != nil {
		m.logsPanel.AddError("system", "Failed to copy conversation: "+err.Error())
	} else {
		m.logsPanel.AddSuccess("system", fmt.Sprintf("Copied entire conversation to clipboard (%d messages)", len(msgs)))
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

	err := clipboard.WriteAll(logsText)
	if err != nil {
		m.logsPanel.AddError("system", "Failed to copy logs: "+err.Error())
	} else {
		m.logsPanel.AddSuccess("system", fmt.Sprintf("Copied %d logs to clipboard", m.logsPanel.Count()))
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
				"claude-sonnet-4-20250514",
				"claude-sonnet-4-5-20251101",
				"claude-haiku-4-5-20251101",
			}
		case "gemini":
			models = []string{
				"gemini-2.5-pro",
				"gemini-2.5-flash",
				"gemini-3-pro-preview",
				"gemini-3-flash-preview",
			}
		case "codex":
			models = []string{
				"gpt-5.1",
				"gpt-5.1-codex",
				"gpt-5.2",
				"gpt-5.2-codex",
				"o3",
				"o3-mini",
			}
		case "copilot":
			models = []string{
				"claude-sonnet-4-5",
				"claude-haiku-4-5",
				"gpt-4o",
				"o3-mini",
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
		// Only emit progress log if still streaming
		if m.streaming {
			elapsed := time.Since(m.chatStartedAt)
			agent := m.chatAgent
			if agent == "" {
				agent = "agent"
			}
			m.logsPanel.AddInfo(strings.ToLower(agent), fmt.Sprintf("⏳ Waiting for response... (%s)", formatDuration(elapsed)))
			// Continue ticking while streaming
			cmds = append(cmds, m.chatProgressTick())
		}

	case StatsTickMsg:
		// Update process statistics
		m.statsWidget.Update()

		// Get resource stats
		stats := m.statsWidget.GetStats()
		resourceStats := ResourceStats{
			MemoryMB:   stats.MemoryMB,
			CPUPercent: stats.CPUPercent,
			Uptime:     stats.Uptime,
			Goroutines: stats.Goroutines,
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
		m.updateStatsPanelTokenStats()

		// Continue ticking
		cmds = append(cmds, statsTickCmd())

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
			// Update token counts for the agent
			for _, a := range m.agentInfos {
				if strings.EqualFold(a.Name, msg.Agent) {
					a.TokensIn += msg.TokensIn
					a.TokensOut += msg.TokensOut
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

	case WorkflowStartedMsg:
		m.workflowRunning = true
		m.workflowStartedAt = time.Now()
		m.workflowPhase = "running"
		// Reset all agents to idle - actual agent events will set them to running
		for _, a := range m.agentInfos {
			if a.Status != AgentStatusDisabled {
				a.Status = AgentStatusIdle
			}
		}
		m.history.Add(NewSystemMessage("Starting workflow..."))
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
			m.logsPanel.AddInfo("workflow", fmt.Sprintf("Total cost: $%.4f, tokens: ↑%d ↓%d", m.totalCost, m.totalTokensIn, m.totalTokensOut))
		}
		m.logsPanel.AddSuccess("workflow", fmt.Sprintf("Workflow completed in %s", formatDuration(elapsed)))
		m.history.Add(NewSystemMessage(fmt.Sprintf("Workflow completed in %s", formatDuration(elapsed))))
		if msg.State != nil {
			m.history.Add(NewSystemMessage(formatWorkflowStatus(msg.State)))
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
		m.history.Add(NewSystemMessage(fmt.Sprintf("Workflow failed after %s: %v", formatDuration(elapsed), msg.Error)))
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
			// Extract timeout from event data (in seconds or as duration)
			var maxTimeout time.Duration
			if t, ok := msg.Data["timeout_seconds"].(float64); ok && t > 0 {
				maxTimeout = time.Duration(t) * time.Second
			} else if t, ok := msg.Data["timeout_seconds"].(int); ok && t > 0 {
				maxTimeout = time.Duration(t) * time.Second
			}
			// Update agent to running state with start time
			StartAgent(m.agentInfos, msg.Agent, phase, maxTimeout)

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
			// Extract token counts
			tokensIn := 0
			tokensOut := 0
			if ti, ok := msg.Data["tokens_in"].(int); ok {
				tokensIn = ti
			}
			if to, ok := msg.Data["tokens_out"].(int); ok {
				tokensOut = to
			}
			// Update agent to completed state
			CompleteAgent(m.agentInfos, msg.Agent, tokensIn, tokensOut)

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
			m.updateStatsPanelTokenStats()

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

	// Save to command history for Ctrl+R search
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
		logMsg := fmt.Sprintf("▶ %s [%s] (%d chars", agent, model, len(input))
		if historyCount > 1 {
			logMsg += fmt.Sprintf(", %d ctx msgs", historyCount-1)
		}
		logMsg += fmt.Sprintf(", timeout: %s)", formatDuration(timeout))
		m.logsPanel.AddInfo(strings.ToLower(agent), logMsg)

		// Start periodic progress tick
		return m, tea.Batch(m.spinner.Tick, m.sendToAgentWithCtx(ctx, input, agent), m.chatProgressTick())
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
	switch cmd.Name {
	case "help":
		var helpText string
		if len(args) > 0 {
			helpText = m.commands.Help(args[0])
		} else {
			helpText = m.commands.Help("")
		}
		m.history.Add(NewSystemMessage(helpText))
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
			m.history.Add(NewSystemMessage("Model: " + m.currentModel))
		} else {
			m.history.Add(NewSystemMessage("Current model: " + m.currentModel))
		}
		m.updateViewport()

	case "agent":
		if len(args) > 0 {
			m.currentAgent = args[0]
			// Reset model to empty so the agent uses its configured default
			m.currentModel = ""
			m.history.Add(NewSystemMessage("Agent: " + m.currentAgent + " (using default model)"))
		} else {
			modelInfo := m.currentModel
			if modelInfo == "" {
				modelInfo = "default"
			}
			m.history.Add(NewSystemMessage(fmt.Sprintf("Current agent: %s (model: %s)", m.currentAgent, modelInfo)))
		}
		m.updateViewport()

	case "status":
		if m.workflowState != nil {
			status := formatWorkflowStatus(m.workflowState)
			m.history.Add(NewSystemMessage(status))
		} else {
			m.history.Add(NewSystemMessage("No active workflow"))
		}
		m.updateViewport()

	case "workflows":
		if m.runner == nil {
			m.history.Add(NewSystemMessage("Workflow runner not configured"))
			m.updateViewport()
			return m, nil
		}
		// List workflows from runner
		ctx := context.Background()
		workflows, err := m.runner.ListWorkflows(ctx)
		if err != nil {
			m.history.Add(NewSystemMessage(fmt.Sprintf("Error listing workflows: %v", err)))
			m.updateViewport()
			return m, nil
		}
		if len(workflows) == 0 {
			m.history.Add(NewSystemMessage("No workflows found. Use '/analyze <prompt>' to start one."))
			m.updateViewport()
			return m, nil
		}
		var sb strings.Builder
		sb.WriteString("Available workflows:\n\n")
		for _, wf := range workflows {
			prefix := "  "
			if wf.IsActive {
				prefix = "* "
			}
			prompt := wf.Prompt
			if len(prompt) > 50 {
				prompt = prompt[:47] + "..."
			}
			sb.WriteString(fmt.Sprintf("%s%s [%s] %s - %s\n",
				prefix,
				wf.WorkflowID,
				wf.Status,
				wf.CurrentPhase,
				prompt,
			))
		}
		sb.WriteString("\n* = active workflow")
		m.history.Add(NewSystemMessage(sb.String()))
		m.updateViewport()

	case "cancel":
		if m.controlPlane != nil && m.workflowRunning {
			m.controlPlane.Cancel()
			m.workflowRunning = false
			m.history.Add(NewSystemMessage("Workflow cancelled"))
		} else {
			m.history.Add(NewSystemMessage("No active workflow"))
		}
		m.updateViewport()

	case "analyze":
		if m.runner == nil {
			m.history.Add(NewSystemMessage("Workflow runner not configured"))
			m.updateViewport()
			return m, nil
		}
		if m.workflowRunning {
			m.history.Add(NewSystemMessage("Workflow already running. Use /cancel first."))
			m.updateViewport()
			return m, nil
		}
		if len(args) == 0 {
			m.history.Add(NewSystemMessage("Usage: /analyze <prompt>"))
			m.updateViewport()
			return m, nil
		}

		prompt := strings.Join(args, " ")
		m.history.Add(NewUserMessage(fmt.Sprintf("/analyze %s", prompt)))
		m.updateViewport()
		return m, m.runAnalyze(prompt)

	case "plan":
		if m.runner == nil {
			m.history.Add(NewSystemMessage("Workflow runner not configured"))
			m.updateViewport()
			return m, nil
		}
		if m.workflowRunning {
			m.history.Add(NewSystemMessage("Workflow already running. Use /cancel first."))
			m.updateViewport()
			return m, nil
		}

		// If no args, continue from active workflow (after /analyze)
		if len(args) == 0 {
			if m.workflowState != nil && m.workflowState.CurrentPhase == core.PhasePlan {
				m.history.Add(NewUserMessage("/plan"))
				m.history.Add(NewSystemMessage("Continuing planning phase from active workflow..."))
				m.updateViewport()
				return m, m.runPlanPhase()
			}
			m.history.Add(NewSystemMessage("No active workflow to continue. Use '/plan <prompt>' to start new or '/analyze' first."))
			m.updateViewport()
			return m, nil
		}

		// With args, start new workflow
		prompt := strings.Join(args, " ")
		m.history.Add(NewUserMessage(fmt.Sprintf("/plan %s", prompt)))
		m.updateViewport()
		return m, m.runWorkflow(prompt)

	case "run":
		if m.runner == nil {
			m.history.Add(NewSystemMessage("Workflow runner not configured"))
			m.updateViewport()
			return m, nil
		}
		if m.workflowRunning {
			m.history.Add(NewSystemMessage("Workflow already running. Use /cancel first."))
			m.updateViewport()
			return m, nil
		}
		if len(args) == 0 {
			m.history.Add(NewSystemMessage("Usage: /run <prompt>"))
			m.updateViewport()
			return m, nil
		}

		prompt := strings.Join(args, " ")
		m.history.Add(NewUserMessage(fmt.Sprintf("/run %s", prompt)))
		m.updateViewport()
		return m, m.runWorkflow(prompt)

	case "execute":
		if m.runner == nil {
			m.history.Add(NewSystemMessage("Workflow runner not configured"))
			m.updateViewport()
			return m, nil
		}
		if m.workflowRunning {
			m.history.Add(NewSystemMessage("Workflow already running. Use /cancel first."))
			m.updateViewport()
			return m, nil
		}

		// Continue from active workflow (after /plan)
		if m.workflowState != nil && m.workflowState.CurrentPhase == core.PhaseExecute {
			m.history.Add(NewUserMessage("/execute"))
			m.history.Add(NewSystemMessage("Continuing execution phase from active workflow..."))
			m.updateViewport()
			return m, m.runExecutePhase()
		}
		m.history.Add(NewSystemMessage("No active workflow to execute. Use '/plan' first."))

	case "retry":
		if m.controlPlane == nil {
			m.history.Add(NewSystemMessage("No control plane"))
			m.updateViewport()
			return m, nil
		}
		if len(args) > 0 {
			m.controlPlane.RetryTask(core.TaskID(args[0]))
			m.history.Add(NewSystemMessage(fmt.Sprintf("Retrying: %s", args[0])))
		} else {
			m.history.Add(NewSystemMessage("Usage: /retry <task_id>"))
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
		m.logsPanel.AddInfo("system", "Logs cleared")
		m.history.Add(NewSystemMessage("Logs cleared"))
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
			m.logsPanel.AddInfo("system", "Explorer opened")
			m.history.Add(NewSystemMessage("File explorer opened (arrows to navigate, Esc to return)"))
		} else {
			m.explorerFocus = false
			m.inputFocused = true
			m.textarea.Focus()
			m.explorerPanel.SetFocused(false)
			m.history.Add(NewSystemMessage("File explorer closed"))
		}
		if m.width > 0 && m.height > 0 {
			m.recalculateLayout()
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

// updateStatsPanelTokenStats updates the stats panel with current token stats
func (m *Model) updateStatsPanelTokenStats() {
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

	// Include workflow tokens
	if m.totalTokensIn > 0 || m.totalTokensOut > 0 {
		tokenStats = append(tokenStats, TokenStats{
			Model:     "workflow",
			TokensIn:  m.totalTokensIn,
			TokensOut: m.totalTokensOut,
		})
	}

	m.statsPanel.SetTokenStats(tokenStats)
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
	explorerWidth := 0
	rightSidebarWidth := 0 // Shared by stats and logs panels
	mainWidth := m.width

	// Determine how many sidebars are open for dynamic sizing
	showRightSidebar := m.showStats || m.showLogs
	bothSidebarsOpen := m.showExplorer && showRightSidebar
	oneSidebarOpen := (m.showExplorer || showRightSidebar) && !bothSidebarsOpen

	// Explorer panel (left sidebar)
	if m.showExplorer {
		if oneSidebarOpen {
			// More space when only one sidebar is open (2/5 of width)
			explorerWidth = m.width * 2 / 5
			if explorerWidth < 35 {
				explorerWidth = 35
			}
			if explorerWidth > 70 {
				explorerWidth = 70
			}
		} else {
			// Less space when both sidebars are open (1/4 of width)
			explorerWidth = m.width / 4
			if explorerWidth < 30 {
				explorerWidth = 30
			}
			if explorerWidth > 50 {
				explorerWidth = 50
			}
		}
		mainWidth -= explorerWidth
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
	if m.showExplorer {
		totalUsed += explorerWidth
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
			if m.showExplorer && explorerWidth-reduction >= 25 {
				explorerWidth -= reduction
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
	if m.showExplorer {
		finalTotal += explorerWidth
	}
	if showRightSidebar {
		finalTotal += rightSidebarWidth
	}

	// If still overflowing, aggressively reduce sidebar widths
	for finalTotal > m.width && (explorerWidth > 20 || rightSidebarWidth > 20) {
		if showRightSidebar && rightSidebarWidth > 20 {
			rightSidebarWidth--
			finalTotal--
		}
		if finalTotal > m.width && m.showExplorer && explorerWidth > 20 {
			explorerWidth--
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
	if m.showExplorer {
		m.explorerPanel.SetSize(explorerWidth, sidebarHeight)
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
func (m Model) runPlanPhase() tea.Cmd {
	runner := m.runner
	return tea.Batch(
		func() tea.Msg { return WorkflowStartedMsg{Prompt: "(continuing from analysis)"} },
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

	// Calculate wrap width for user messages (viewport width - padding)
	wrapWidth := m.viewport.Width - 4
	if wrapWidth < 20 {
		wrapWidth = 80
	}

	for _, msg := range msgs {
		switch msg.Role {
		case RoleUser:
			sb.WriteString(userLabelStyle.Render("You") + "\n")
			// Wrap user message to fit viewport width
			wrappedContent := wrapText(msg.Content, wrapWidth)
			sb.WriteString(userMsgStyle.Render(wrappedContent))
			sb.WriteString("\n\n")

		case RoleAgent:
			sb.WriteString(agentLabelStyle.Render("Quorum") + "\n")
			content := msg.Content
			if m.mdRenderer != nil {
				if rendered, err := m.mdRenderer.Render(msg.Content); err == nil {
					content = strings.TrimSpace(rendered)
				}
			}
			sb.WriteString(agentMsgStyle.Render(content))
			sb.WriteString("\n\n")

		case RoleSystem:
			sb.WriteString(systemMsgStyle.Render("* " + msg.Content))
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
	explorerWidth := 0
	rightSidebarWidth := 0 // Shared by stats and logs panels

	if m.showExplorer {
		explorerWidth = m.explorerPanel.Width()
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
	if m.showExplorer {
		mainWidth -= explorerWidth
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

	// Explorer panel (left)
	if m.showExplorer {
		panels = append(panels, m.explorerPanel.Render())
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
		baseView = m.overlayAtBottom(baseView, suggestionsOverlay, w, h, explorerWidth, footerOffset)
	}

	return ensureFullScreen(baseView)
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

// wrapText wraps text to fit within maxWidth, breaking on word boundaries
func wrapText(text string, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = 80
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		// If line fits, add it directly
		if lipgloss.Width(line) <= maxWidth {
			result.WriteString(line)
			continue
		}

		// Wrap long lines
		words := strings.Fields(line)
		currentLine := ""
		for _, word := range words {
			testLine := currentLine
			if testLine != "" {
				testLine += " "
			}
			testLine += word

			if lipgloss.Width(testLine) <= maxWidth {
				currentLine = testLine
			} else {
				// Write current line and start new one
				if currentLine != "" {
					result.WriteString(currentLine)
					result.WriteString("\n")
				}
				// Handle very long words that exceed maxWidth
				if lipgloss.Width(word) > maxWidth {
					// Break the word
					for _, r := range word {
						if lipgloss.Width(currentLine+string(r)) > maxWidth {
							result.WriteString(currentLine)
							result.WriteString("\n")
							currentLine = string(r)
						} else {
							currentLine += string(r)
						}
					}
				} else {
					currentLine = word
				}
			}
		}
		if currentLine != "" {
			result.WriteString(currentLine)
		}
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

		// For commands, show description; for agents/models, show status info
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
			keyHintStyle.Render("←") + labelStyle.Render(" explorer"),
			keyHintStyle.Render("→") + labelStyle.Render(" sidebar"),
			keyHintStyle.Render("↑↓") + labelStyle.Render(" logs/stats"),
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
	} else if m.streaming || m.workflowRunning {
		keys = []string{
			keyHintStyle.Render("Esc") + labelStyle.Render(" stop"),
			keyHintStyle.Render("^E") + labelStyle.Render(" files"),
			keyHintStyle.Render("^L") + labelStyle.Render(" logs"),
			keyHintStyle.Render("?") + labelStyle.Render(" help"),
		}
	} else {
		keys = []string{
			keyHintStyle.Render("↵") + labelStyle.Render(" send"),
			keyHintStyle.Render("^R") + labelStyle.Render(" hist"),
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
	sb.WriteString(fmt.Sprintf("Workflow: %s\n", state.WorkflowID))
	sb.WriteString(fmt.Sprintf("Phase: %s\n", state.CurrentPhase))

	completed, running, failed := 0, 0, 0
	for _, task := range state.Tasks {
		switch task.Status {
		case core.TaskStatusCompleted:
			completed++
		case core.TaskStatusRunning:
			running++
		case core.TaskStatusFailed:
			failed++
		}
	}

	sb.WriteString(fmt.Sprintf("Tasks: %d total, %d done, %d running, %d failed\n",
		len(state.Tasks), completed, running, failed))

	if state.Metrics != nil {
		sb.WriteString(fmt.Sprintf("Cost: $%.4f\n", state.Metrics.TotalCostUSD))
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
