package chat

import (
	"sort"
	"strings"

	"github.com/sahilm/fuzzy"
)

// CommandHandler is a function that handles a slash command.
type CommandHandler func(args []string, ctx *CommandContext) error

// Command represents a slash command.
type Command struct {
	Name        string
	Aliases     []string
	Description string
	Usage       string
	Handler     CommandHandler
}

// RequiresArg returns true if the command requires an argument.
// Uses convention: <arg> means required, [arg] means optional.
func (c *Command) RequiresArg() bool {
	// If Usage contains <...> pattern, argument is required
	return strings.Contains(c.Usage, "<") && strings.Contains(c.Usage, ">")
}

// CommandContext provides context for command execution.
type CommandContext struct {
	History     *ConversationHistory
	Agent       string
	Model       string
	WorkflowID  string
	SendMessage func(Message)
	SendSystem  func(string)

	// Workflow operation callbacks
	OnPlan    func(prompt string) error
	OnRun     func(prompt string) error
	OnExecute func() error
	OnStatus  func() error
	OnRetry   func(taskID string) error
	OnCancel  func() error
	OnModel   func(model string) error
	OnAgent   func(agent string) error
	OnNew     func(archive, purge bool) error
}

// CommandRegistry manages slash commands.
type CommandRegistry struct {
	commands map[string]*Command
	aliases  map[string]string
	names    []string // For fuzzy matching
}

// NewCommandRegistry creates a new command registry with default commands.
func NewCommandRegistry() *CommandRegistry {
	r := &CommandRegistry{
		commands: make(map[string]*Command),
		aliases:  make(map[string]string),
		names:    make([]string, 0),
	}

	// Register default commands
	r.Register(&Command{
		Name:        "help",
		Aliases:     []string{"h", "?"},
		Description: "Show available commands",
		Usage:       "/help [command]",
	})

	r.Register(&Command{
		Name:        "analyze",
		Aliases:     []string{"an"},
		Description: "Run multi-agent analysis (V1/V2/V3)",
		Usage:       "/analyze <prompt>",
	})

	r.Register(&Command{
		Name:        "plan",
		Aliases:     []string{"p"},
		Description: "Continue planning or start new workflow",
		Usage:       "/plan [prompt]",
	})

	r.Register(&Command{
		Name:        "replan",
		Aliases:     []string{"rp"},
		Description: "Re-run planning phase (clears existing issues)",
		Usage:       "/replan [additional context]",
	})

	r.Register(&Command{
		Name:        "useplan",
		Aliases:     []string{"up", "useplans"},
		Description: "Use existing task files from filesystem (skip agent call)",
		Usage:       "/useplan",
	})

	r.Register(&Command{
		Name:        "run",
		Aliases:     []string{"r"},
		Description: "Run a complete workflow",
		Usage:       "/run <prompt>",
	})

	r.Register(&Command{
		Name:        "execute",
		Aliases:     []string{"exec"},
		Description: "Execute issues from active workflow",
		Usage:       "/execute",
	})

	r.Register(&Command{
		Name:        "workflows",
		Aliases:     []string{"wf", "wfs"},
		Description: "List available workflows",
		Usage:       "/workflows",
	})

	r.Register(&Command{
		Name:        "load",
		Aliases:     []string{"switch", "select"},
		Description: "Load and activate a workflow",
		Usage:       "/load [workflow-id]",
	})

	r.Register(&Command{
		Name:        "new",
		Aliases:     []string{"reset", "fresh"},
		Description: "Reset workflow state and start fresh",
		Usage:       "/new [--archive|--purge]",
	})

	r.Register(&Command{
		Name:        "delete",
		Aliases:     []string{"del", "rm"},
		Description: "Delete a specific workflow",
		Usage:       "/delete <workflow-id>",
	})

	r.Register(&Command{
		Name:        "status",
		Aliases:     []string{"s", "st"},
		Description: "Show workflow status",
		Usage:       "/status",
	})

	r.Register(&Command{
		Name:        "retry",
		Aliases:     []string{"re"},
		Description: "Retry a failed task",
		Usage:       "/retry [task_id]",
	})

	r.Register(&Command{
		Name:        "cancel",
		Aliases:     []string{"c", "stop"},
		Description: "Cancel current workflow",
		Usage:       "/cancel",
	})

	r.Register(&Command{
		Name:        "model",
		Aliases:     []string{"m"},
		Description: "Set or show current model",
		Usage:       "/model [name]",
	})

	r.Register(&Command{
		Name:        "agent",
		Aliases:     []string{"a"},
		Description: "Set or show current agent",
		Usage:       "/agent [name]",
	})

	r.Register(&Command{
		Name:        "clear",
		Aliases:     []string{"cls"},
		Description: "Clear conversation history",
		Usage:       "/clear",
	})

	r.Register(&Command{
		Name:        "quit",
		Aliases:     []string{"q", "exit"},
		Description: "Exit chat mode",
		Usage:       "/quit",
	})

	r.Register(&Command{
		Name:        "copy",
		Aliases:     []string{"cp", "y"},
		Description: "Copy last response to clipboard",
		Usage:       "/copy",
	})

	r.Register(&Command{
		Name:        "copyall",
		Aliases:     []string{"cpa"},
		Description: "Copy entire conversation to clipboard",
		Usage:       "/copyall",
	})

	r.Register(&Command{
		Name:        "logs",
		Aliases:     []string{"l", "log"},
		Description: "Toggle logs panel (or Ctrl+L)",
		Usage:       "/logs",
	})

	r.Register(&Command{
		Name:        "clearlogs",
		Aliases:     []string{"cll"},
		Description: "Clear logs panel",
		Usage:       "/clearlogs",
	})

	r.Register(&Command{
		Name:        "copylogs",
		Aliases:     []string{"cpl"},
		Description: "Copy all logs to clipboard",
		Usage:       "/copylogs",
	})

	r.Register(&Command{
		Name:        "explorer",
		Aliases:     []string{"e", "files", "tree"},
		Description: "Toggle file explorer (or Ctrl+E)",
		Usage:       "/explorer",
	})

	r.Register(&Command{
		Name:        "theme",
		Aliases:     []string{"t"},
		Description: "Toggle between dark and light theme",
		Usage:       "/theme [dark|light]",
	})

	return r
}

// Register adds a command to the registry.
func (r *CommandRegistry) Register(cmd *Command) {
	r.commands[cmd.Name] = cmd
	r.names = append(r.names, cmd.Name)

	for _, alias := range cmd.Aliases {
		r.aliases[alias] = cmd.Name
	}
}

// Parse parses input and returns the command, arguments, and whether it's a command.
func (r *CommandRegistry) Parse(input string) (*Command, []string, bool) {
	input = strings.TrimSpace(input)

	if !strings.HasPrefix(input, "/") {
		return nil, nil, false
	}

	// Remove leading slash and split
	parts := strings.Fields(input[1:])
	if len(parts) == 0 {
		return nil, nil, false
	}

	cmdName := strings.ToLower(parts[0])
	args := parts[1:]

	// Check for direct command name
	if cmd, ok := r.commands[cmdName]; ok {
		return cmd, args, true
	}

	// Check for alias
	if realName, ok := r.aliases[cmdName]; ok {
		if cmd, ok := r.commands[realName]; ok {
			return cmd, args, true
		}
	}

	return nil, nil, false
}

// Suggest returns command suggestions for partial input using fuzzy matching.
func (r *CommandRegistry) Suggest(partial string) []string {
	partial = strings.TrimPrefix(partial, "/")
	partial = strings.ToLower(partial)

	if partial == "" {
		// Return all commands
		result := make([]string, len(r.names))
		copy(result, r.names)
		sort.Strings(result)
		return result
	}

	// Collect all names and aliases
	allNames := make([]string, 0, len(r.names)+len(r.aliases))
	allNames = append(allNames, r.names...)
	for alias := range r.aliases {
		allNames = append(allNames, alias)
	}

	// Fuzzy match
	matches := fuzzy.Find(partial, allNames)
	result := make([]string, 0, len(matches))

	// Deduplicate (aliases map to same command)
	seen := make(map[string]bool)
	for _, match := range matches {
		name := match.Str
		// Resolve alias to command name
		if realName, ok := r.aliases[name]; ok {
			name = realName
		}
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}

	return result
}

// Help returns help text for a command or all commands.
func (r *CommandRegistry) Help(cmdName string) string {
	if cmdName != "" {
		// Help for specific command
		if cmd, ok := r.commands[cmdName]; ok {
			return formatCommandHelp(cmd)
		}
		// Check alias
		if realName, ok := r.aliases[cmdName]; ok {
			if cmd, ok := r.commands[realName]; ok {
				return formatCommandHelp(cmd)
			}
		}
		return "Unknown command: " + cmdName
	}

	// Help for all commands
	var sb strings.Builder
	sb.WriteString("Available commands:\n\n")

	// Sort commands by name
	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		cmd := r.commands[name]
		sb.WriteString("  /")
		sb.WriteString(name)
		if len(cmd.Aliases) > 0 {
			sb.WriteString(" (")
			sb.WriteString(strings.Join(cmd.Aliases, ", "))
			sb.WriteString(")")
		}
		sb.WriteString("\n    ")
		sb.WriteString(cmd.Description)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// Get returns a command by name.
func (r *CommandRegistry) Get(name string) *Command {
	if cmd, ok := r.commands[name]; ok {
		return cmd
	}
	if realName, ok := r.aliases[name]; ok {
		return r.commands[realName]
	}
	return nil
}

// All returns all registered commands.
func (r *CommandRegistry) All() []*Command {
	result := make([]*Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		result = append(result, cmd)
	}
	return result
}

func formatCommandHelp(cmd *Command) string {
	var sb strings.Builder
	sb.WriteString(cmd.Name)
	if len(cmd.Aliases) > 0 {
		sb.WriteString(" (aliases: ")
		sb.WriteString(strings.Join(cmd.Aliases, ", "))
		sb.WriteString(")")
	}
	sb.WriteString("\n\n")
	sb.WriteString(cmd.Description)
	sb.WriteString("\n\nUsage: ")
	sb.WriteString(cmd.Usage)
	return sb.String()
}
