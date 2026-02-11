package chat

import (
	"testing"
	"time"
)

// --- estimateProgress ---

func TestEstimateProgress_Done(t *testing.T) {
	if got := estimateProgress(time.Now(), AgentStatusDone); got != 100 {
		t.Errorf("done status should be 100, got %d", got)
	}
}

func TestEstimateProgress_NotRunning(t *testing.T) {
	if got := estimateProgress(time.Now(), AgentStatusIdle); got != 0 {
		t.Errorf("idle status should be 0, got %d", got)
	}
	if got := estimateProgress(time.Now(), AgentStatusError); got != 0 {
		t.Errorf("error status should be 0, got %d", got)
	}
}

func TestEstimateProgress_ZeroStartTime(t *testing.T) {
	if got := estimateProgress(time.Time{}, AgentStatusRunning); got != 0 {
		t.Errorf("zero start time should be 0, got %d", got)
	}
}

func TestEstimateProgress_Running(t *testing.T) {
	start := time.Now().Add(-30 * time.Second) // 30s ago
	got := estimateProgress(start, AgentStatusRunning)
	if got <= 0 || got > 95 {
		t.Errorf("expected 0 < progress <= 95, got %d", got)
	}
}

func TestEstimateProgress_LongRunning(t *testing.T) {
	start := time.Now().Add(-10 * time.Minute) // 10 min ago
	got := estimateProgress(start, AgentStatusRunning)
	if got != 95 {
		t.Errorf("long running should cap at 95, got %d", got)
	}
}

// --- formatElapsed ---

func TestFormatElapsed_ZeroTime(t *testing.T) {
	got := formatElapsed(time.Time{}, 5*time.Minute)
	if got != "" {
		t.Errorf("zero time should return empty, got %q", got)
	}
}

func TestFormatElapsed_Recent(t *testing.T) {
	start := time.Now().Add(-5 * time.Second)
	got := formatElapsed(start, 5*time.Minute)
	if got == "" {
		t.Error("recent start should return non-empty")
	}
}

// --- Command.RequiresArg ---

func TestCommand_RequiresArg_Required(t *testing.T) {
	cmd := &Command{Usage: "/model <name>"}
	if !cmd.RequiresArg() {
		t.Error("command with <name> should require arg")
	}
}

func TestCommand_RequiresArg_Optional(t *testing.T) {
	cmd := &Command{Usage: "/help [command]"}
	if cmd.RequiresArg() {
		t.Error("command with [command] should not require arg")
	}
}

func TestCommand_RequiresArg_NoArg(t *testing.T) {
	cmd := &Command{Usage: "/status"}
	if cmd.RequiresArg() {
		t.Error("command without args should not require arg")
	}
}

// --- CommandRegistry.Suggest ---

func TestCommandRegistry_Suggest_Empty(t *testing.T) {
	r := NewCommandRegistry()
	suggestions := r.Suggest("")
	if len(suggestions) == 0 {
		t.Error("empty partial should return all commands")
	}
}

func TestCommandRegistry_Suggest_Partial(t *testing.T) {
	r := NewCommandRegistry()
	suggestions := r.Suggest("/hel")
	found := false
	for _, s := range suggestions {
		if s == "help" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("suggest('hel') should include 'help', got %v", suggestions)
	}
}

func TestCommandRegistry_Suggest_NoMatch(t *testing.T) {
	r := NewCommandRegistry()
	suggestions := r.Suggest("zzzzzzzzzzzzz")
	if len(suggestions) != 0 {
		t.Errorf("nonsense should return empty, got %v", suggestions)
	}
}

// --- CommandRegistry.All ---

func TestCommandRegistry_All(t *testing.T) {
	r := NewCommandRegistry()
	all := r.All()
	if len(all) == 0 {
		t.Error("All() should return registered commands")
	}
}

// --- CommandRegistry.Help ---

func TestCommandRegistry_Help_All(t *testing.T) {
	r := NewCommandRegistry()
	help := r.Help("")
	if help == "" {
		t.Error("Help('') should return help text")
	}
}

func TestCommandRegistry_Help_Specific(t *testing.T) {
	r := NewCommandRegistry()
	help := r.Help("help")
	if help == "" {
		t.Error("Help('help') should return command help")
	}
}

func TestCommandRegistry_Help_Unknown(t *testing.T) {
	r := NewCommandRegistry()
	help := r.Help("nonexistent")
	if help == "" {
		t.Error("Help('nonexistent') should return error message")
	}
}

func TestCommandRegistry_Help_Alias(t *testing.T) {
	r := NewCommandRegistry()
	help := r.Help("h") // alias for "help"
	if help == "" {
		t.Error("Help('h') should return help via alias")
	}
}

// --- CommandRegistry.Get ---

func TestCommandRegistry_Get_ByAlias(t *testing.T) {
	r := NewCommandRegistry()
	cmd := r.Get("h") // alias for "help"
	if cmd == nil {
		t.Error("Get('h') should find command via alias")
	}
	if cmd != nil && cmd.Name != "help" {
		t.Errorf("expected 'help', got %q", cmd.Name)
	}
}

func TestCommandRegistry_Get_NotFound(t *testing.T) {
	r := NewCommandRegistry()
	cmd := r.Get("nonexistent")
	if cmd != nil {
		t.Error("Get('nonexistent') should return nil")
	}
}
