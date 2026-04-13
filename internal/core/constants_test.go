package core

import "testing"

func TestIsValidAgent(t *testing.T) {
	tests := []struct {
		agent string
		want  bool
	}{
		{"claude", true},
		{"gemini", true},
		{"codex", true},
		{"copilot", true},
		{"opencode", true},
		{"unknown", false},
		{"", false},
		{"Claude", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			if got := IsValidAgent(tt.agent); got != tt.want {
				t.Errorf("IsValidAgent(%q) = %v, want %v", tt.agent, got, tt.want)
			}
		})
	}
}

func TestIsValidReasoningEffort(t *testing.T) {
	valid := []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}
	for _, e := range valid {
		if !IsValidReasoningEffort(e) {
			t.Errorf("IsValidReasoningEffort(%q) = false, want true", e)
		}
	}

	invalid := []string{"", "ultra", "MEDIUM", "extreme"}
	for _, e := range invalid {
		if IsValidReasoningEffort(e) {
			t.Errorf("IsValidReasoningEffort(%q) = true, want false", e)
		}
	}
}

func TestSupportsReasoning(t *testing.T) {
	tests := []struct {
		agent string
		want  bool
	}{
		{"claude", true},
		{"codex", true},
		{"gemini", false},
		{"copilot", false},
		{"opencode", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			if got := SupportsReasoning(tt.agent); got != tt.want {
				t.Errorf("SupportsReasoning(%q) = %v, want %v", tt.agent, got, tt.want)
			}
		})
	}
}

func TestGetReasoningEfforts(t *testing.T) {
	tests := []struct {
		agent string
		want  []string
	}{
		{"claude", []string{"low", "medium", "high", "max"}},
		{"codex", []string{"none", "minimal", "low", "medium", "high", "xhigh"}},
		{"gemini", ReasoningEfforts},  // fallback to global union
		{"unknown", ReasoningEfforts}, // fallback to global union
	}

	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			got := GetReasoningEfforts(tt.agent)
			if len(got) != len(tt.want) {
				t.Fatalf("GetReasoningEfforts(%q) returned %d items, want %d: %v", tt.agent, len(got), len(tt.want), got)
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("GetReasoningEfforts(%q)[%d] = %q, want %q", tt.agent, i, v, tt.want[i])
				}
			}
		})
	}
}

func TestIsValidReasoningEffortForAgent(t *testing.T) {
	tests := []struct {
		agent  string
		effort string
		want   bool
	}{
		// Claude accepts low/medium/high/max
		{"claude", "low", true},
		{"claude", "medium", true},
		{"claude", "high", true},
		{"claude", "max", true},
		{"claude", "none", false},
		{"claude", "minimal", false},
		{"claude", "xhigh", false},
		// Codex accepts none/minimal/low/medium/high/xhigh
		{"codex", "none", true},
		{"codex", "minimal", true},
		{"codex", "low", true},
		{"codex", "medium", true},
		{"codex", "high", true},
		{"codex", "xhigh", true},
		{"codex", "max", false},
		// Unknown agent falls back to global union
		{"gemini", "max", true},
		{"gemini", "none", true},
		{"unknown", "high", true},
		// Invalid for all
		{"claude", "ultra", false},
		{"codex", "ultra", false},
		{"unknown", "ultra", false},
	}

	for _, tt := range tests {
		name := tt.agent + "/" + tt.effort
		t.Run(name, func(t *testing.T) {
			if got := IsValidReasoningEffortForAgent(tt.agent, tt.effort); got != tt.want {
				t.Errorf("IsValidReasoningEffortForAgent(%q, %q) = %v, want %v", tt.agent, tt.effort, got, tt.want)
			}
		})
	}
}

func TestGetSupportedModels(t *testing.T) {
	for _, agent := range Agents {
		models := GetSupportedModels(agent)
		if len(models) == 0 {
			t.Errorf("GetSupportedModels(%q) returned empty", agent)
		}
	}

	if models := GetSupportedModels("unknown"); models != nil {
		t.Errorf("unknown agent should return nil, got %v", models)
	}
}

func TestGetDefaultModel(t *testing.T) {
	for _, agent := range Agents {
		model := GetDefaultModel(agent)
		if model == "" {
			t.Errorf("GetDefaultModel(%q) returned empty", agent)
		}
	}

	if model := GetDefaultModel("unknown"); model != "" {
		t.Errorf("unknown agent should return empty, got %q", model)
	}
}

func TestIsValidModel(t *testing.T) {
	// Valid model for each agent
	for _, agent := range Agents {
		models := GetSupportedModels(agent)
		if len(models) > 0 {
			if !IsValidModel(agent, models[0]) {
				t.Errorf("IsValidModel(%q, %q) = false, want true", agent, models[0])
			}
		}
	}

	// Invalid model
	if IsValidModel("claude", "nonexistent-model") {
		t.Error("nonexistent model should be invalid")
	}

	// Invalid agent
	if IsValidModel("unknown", "opus") {
		t.Error("unknown agent should have no valid models")
	}
}

func TestIsValidPhaseModelKey(t *testing.T) {
	valid := []string{"refine", "analyze", "moderate", "synthesize", "plan", "execute"}
	for _, k := range valid {
		if !IsValidPhaseModelKey(k) {
			t.Errorf("IsValidPhaseModelKey(%q) = false, want true", k)
		}
	}

	invalid := []string{"", "unknown", "deploy", "test"}
	for _, k := range invalid {
		if IsValidPhaseModelKey(k) {
			t.Errorf("IsValidPhaseModelKey(%q) = true, want false", k)
		}
	}
}
