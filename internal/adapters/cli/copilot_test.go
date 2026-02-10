package cli

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestNewCopilotAdapter(t *testing.T) {
	t.Parallel()

	t.Run("default path", func(t *testing.T) {
		t.Parallel()
		agent, err := NewCopilotAdapter(AgentConfig{})
		if err != nil {
			t.Fatalf("NewCopilotAdapter() error = %v", err)
		}
		c := agent.(*CopilotAdapter)
		if c.config.Path != "copilot" {
			t.Errorf("config.Path = %q, want %q", c.config.Path, "copilot")
		}
	})

	t.Run("custom path", func(t *testing.T) {
		t.Parallel()
		agent, err := NewCopilotAdapter(AgentConfig{Path: "/usr/local/bin/copilot"})
		if err != nil {
			t.Fatalf("NewCopilotAdapter() error = %v", err)
		}
		c := agent.(*CopilotAdapter)
		if c.config.Path != "/usr/local/bin/copilot" {
			t.Errorf("config.Path = %q", c.config.Path)
		}
	})
}

func TestCopilotAdapter_BuildArgs(t *testing.T) {
	t.Parallel()

	c := &CopilotAdapter{config: AgentConfig{}}
	args := c.buildArgs(core.ExecuteOptions{
		Model: "gpt-4o",
	})

	// Should always include these flags
	wantFlags := []string{
		"--allow-all-tools",
		"--allow-all-paths",
		"--allow-all-urls",
		"--silent",
	}

	for _, want := range wantFlags {
		found := false
		for _, arg := range args {
			if arg == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("buildArgs() missing %q in %v", want, args)
		}
	}
}

func TestCopilotAdapter_ParseOutput(t *testing.T) {
	t.Parallel()

	c := &CopilotAdapter{}
	tests := []struct {
		name   string
		stdout string
		want   string
	}{
		{
			name:   "plain text",
			stdout: "Hello, this is the output",
			want:   "Hello, this is the output",
		},
		{
			name:   "with ansi codes",
			stdout: "\x1b[1mBold output\x1b[0m",
			want:   "Bold output",
		},
		{
			name:   "with whitespace",
			stdout: "  \n  Output text  \n  ",
			want:   "Output text",
		},
		{
			name:   "empty",
			stdout: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := c.parseOutput(&CommandResult{Stdout: tt.stdout}, core.OutputFormatText)
			if err != nil {
				t.Fatalf("parseOutput() error = %v", err)
			}
			if result.Output != tt.want {
				t.Errorf("parseOutput() output = %q, want %q", result.Output, tt.want)
			}
		})
	}
}

func TestCopilotAdapter_SetEventHandler(t *testing.T) {
	t.Parallel()

	agent, _ := NewCopilotAdapter(AgentConfig{})
	c := agent.(*CopilotAdapter)

	if c.eventHandler != nil {
		t.Error("eventHandler should be nil initially")
	}
	if c.aggregator != nil {
		t.Error("aggregator should be nil initially")
	}

	called := false
	c.SetEventHandler(func(_ core.AgentEvent) {
		called = true
	})

	if c.eventHandler == nil {
		t.Error("eventHandler should be set after SetEventHandler")
	}
	if c.aggregator == nil {
		t.Error("aggregator should be created when handler is set")
	}

	// Setting nil should be fine
	c.SetEventHandler(nil)
	if c.eventHandler != nil {
		t.Error("eventHandler should be nil after setting nil")
	}

	_ = called
}

func TestCopilotAdapter_Config(t *testing.T) {
	t.Parallel()

	cfg := AgentConfig{
		Name:  "test-copilot",
		Path:  "/bin/copilot",
		Model: "gpt-4o",
	}
	agent, _ := NewCopilotAdapter(cfg)
	c := agent.(*CopilotAdapter)

	got := c.Config()
	if got.Name != "test-copilot" {
		t.Errorf("Config().Name = %q, want %q", got.Name, "test-copilot")
	}
	if got.Model != "gpt-4o" {
		t.Errorf("Config().Model = %q, want %q", got.Model, "gpt-4o")
	}
}
