package core

import "testing"

func TestNormalizeReasoningEffortForModel(t *testing.T) {
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
	tests := []struct {
		model  string
		expect []string
	}{
		{"gpt-5.3-codex", []string{"minimal", "low", "medium", "high", "xhigh"}},
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
