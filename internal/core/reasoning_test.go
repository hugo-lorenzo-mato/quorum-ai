package core

import "testing"

func TestNormalizeReasoningEffortForModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		model  string
		in     string
		expect string
	}{
		{
			name:   "empty",
			model:  "gpt-5.3-codex",
			in:     "",
			expect: "",
		},
		{
			name:   "unknown model passthrough",
			model:  "some-new-model",
			in:     "minimal",
			expect: "minimal",
		},
		{
			name:   "gpt-5.3-codex exact match minimal",
			model:  "gpt-5.3-codex",
			in:     "minimal",
			expect: "minimal",
		},
		{
			name:   "gpt-5.2-codex normalizes minimal to low",
			model:  "gpt-5.2-codex",
			in:     "minimal",
			expect: "low",
		},
		{
			name:   "gpt-5.2-codex exact match xhigh",
			model:  "gpt-5.2-codex",
			in:     "xhigh",
			expect: "xhigh",
		},
		{
			name:   "gpt-5 clamps xhigh to high",
			model:  "gpt-5",
			in:     "xhigh",
			expect: "high",
		},
		{
			name:   "gpt-5.1 clamps xhigh to high",
			model:  "gpt-5.1",
			in:     "xhigh",
			expect: "high",
		},
		{
			name:   "gpt-5-codex-mini clamps xhigh to high",
			model:  "gpt-5-codex-mini",
			in:     "xhigh",
			expect: "high",
		},
		{
			name:   "max maps to xhigh when available",
			model:  "gpt-5.2-codex",
			in:     "max",
			expect: "xhigh",
		},
		{
			name:   "max maps to highest when xhigh not available",
			model:  "gpt-5.1-codex",
			in:     "max",
			expect: "high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeReasoningEffortForModel(tt.model, tt.in)
			if got != tt.expect {
				t.Fatalf("NormalizeReasoningEffortForModel(%q, %q) = %q, want %q", tt.model, tt.in, got, tt.expect)
			}
		})
	}
}

func TestNormalizeClaudeEffort(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		model  string
		in     string
		expect string
	}{
		{
			name:   "empty",
			model:  "claude-opus-4-6",
			in:     "",
			expect: "",
		},
		{
			name:   "unknown model passthrough",
			model:  "unknown-model",
			in:     "high",
			expect: "high",
		},
		{
			name:   "opus exact match low",
			model:  "claude-opus-4-6",
			in:     "low",
			expect: "low",
		},
		{
			name:   "opus exact match max",
			model:  "claude-opus-4-6",
			in:     "max",
			expect: "max",
		},
		{
			name:   "opus alias exact match",
			model:  "opus",
			in:     "high",
			expect: "high",
		},
		{
			name:   "opus maps codex minimal to low",
			model:  "claude-opus-4-6",
			in:     "minimal",
			expect: "low",
		},
		{
			name:   "opus maps codex xhigh to max",
			model:  "claude-opus-4-6",
			in:     "xhigh",
			expect: "max",
		},
		{
			name:   "opus maps codex none to low",
			model:  "claude-opus-4-6",
			in:     "none",
			expect: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeClaudeEffort(tt.model, tt.in)
			if got != tt.expect {
				t.Fatalf("NormalizeClaudeEffort(%q, %q) = %q, want %q", tt.model, tt.in, got, tt.expect)
			}
		})
	}
}

func TestSupportedReasoningEffortsForModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		model  string
		expect []string
	}{
		{"gpt-5.3-codex", []string{"none", "minimal", "low", "medium", "high", "xhigh"}},
		{"gpt-5.2-codex", []string{"low", "medium", "high", "xhigh"}},
		{"gpt-5.1-codex", []string{"low", "medium", "high"}},
		{"gpt-5", []string{"minimal", "low", "medium", "high"}},
		{"unknown", nil},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := SupportedReasoningEffortsForModel(tt.model)
			if len(got) != len(tt.expect) {
				t.Fatalf("SupportedReasoningEffortsForModel(%q) = %v, want %v", tt.model, got, tt.expect)
			}
			for i := range got {
				if got[i] != tt.expect[i] {
					t.Fatalf("SupportedReasoningEffortsForModel(%q)[%d] = %q, want %q", tt.model, i, got[i], tt.expect[i])
				}
			}
		})
	}
}

func TestSupportedEffortsForClaudeModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		model  string
		expect []string
	}{
		{"claude-opus-4-6", []string{"low", "medium", "high", "max"}},
		{"opus", []string{"low", "medium", "high", "max"}},
		{"claude-sonnet-4-5-20250929", nil},
		{"unknown", nil},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := SupportedEffortsForClaudeModel(tt.model)
			if len(got) != len(tt.expect) {
				t.Fatalf("SupportedEffortsForClaudeModel(%q) = %v, want %v", tt.model, got, tt.expect)
			}
			for i := range got {
				if got[i] != tt.expect[i] {
					t.Fatalf("SupportedEffortsForClaudeModel(%q)[%d] = %q, want %q", tt.model, i, got[i], tt.expect[i])
				}
			}
		})
	}
}

func TestGetModelReasoningEfforts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		agent  string
		model  string
		expect []string
	}{
		{"claude opus", AgentClaude, "claude-opus-4-6", []string{"low", "medium", "high", "max"}},
		{"claude opus alias", AgentClaude, "opus", []string{"low", "medium", "high", "max"}},
		{"claude unknown model", AgentClaude, "claude-sonnet-4-5-20250929", nil},
		{"codex gpt-5.3", AgentCodex, "gpt-5.3-codex", []string{"none", "minimal", "low", "medium", "high", "xhigh"}},
		{"codex gpt-5.2", AgentCodex, "gpt-5.2-codex", []string{"low", "medium", "high", "xhigh"}},
		{"codex gpt-5.1", AgentCodex, "gpt-5.1-codex", []string{"low", "medium", "high"}},
		{"codex gpt-5", AgentCodex, "gpt-5", []string{"minimal", "low", "medium", "high"}},
		{"codex unknown model", AgentCodex, "unknown", nil},
		{"unknown agent", "unknown", "gpt-5.3-codex", nil},
		{"gemini has no reasoning", AgentGemini, "gemini-2.5-pro", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetModelReasoningEfforts(tt.agent, tt.model)
			if len(got) != len(tt.expect) {
				t.Fatalf("GetModelReasoningEfforts(%q, %q) = %v, want %v", tt.agent, tt.model, got, tt.expect)
			}
			for i := range got {
				if got[i] != tt.expect[i] {
					t.Fatalf("GetModelReasoningEfforts(%q, %q)[%d] = %q, want %q", tt.agent, tt.model, i, got[i], tt.expect[i])
				}
			}
		})
	}
}

func TestGetMaxReasoningEffort(t *testing.T) {
	t.Parallel()
	tests := []struct {
		model  string
		expect string
	}{
		{"gpt-5.3-codex", "xhigh"},
		{"gpt-5.2-codex", "xhigh"},
		{"gpt-5.1-codex-max", "xhigh"},
		{"gpt-5.1-codex", "high"},
		{"gpt-5", "high"},
		{"unknown", "high"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := GetMaxReasoningEffort(tt.model)
			if got != tt.expect {
				t.Fatalf("GetMaxReasoningEffort(%q) = %q, want %q", tt.model, got, tt.expect)
			}
		})
	}
}

func TestGetMaxClaudeEffort(t *testing.T) {
	t.Parallel()
	tests := []struct {
		model  string
		expect string
	}{
		{"claude-opus-4-6", "max"},
		{"opus", "max"},
		{"claude-sonnet-4-5-20250929", ""},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := GetMaxClaudeEffort(tt.model)
			if got != tt.expect {
				t.Fatalf("GetMaxClaudeEffort(%q) = %q, want %q", tt.model, got, tt.expect)
			}
		})
	}
}
