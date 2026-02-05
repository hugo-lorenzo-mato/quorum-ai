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
			in:     ReasoningMinimal,
			expect: ReasoningMinimal,
		},
		{
			name:   "gpt-5.3-codex maps minimal to none",
			model:  "gpt-5.3-codex",
			in:     ReasoningMinimal,
			expect: ReasoningNone,
		},
		{
			name:   "gpt-5.2-codex maps none to low",
			model:  "gpt-5.2-codex",
			in:     ReasoningNone,
			expect: ReasoningLow,
		},
		{
			name:   "gpt-5.2-codex maps minimal to low",
			model:  "gpt-5.2-codex",
			in:     ReasoningMinimal,
			expect: ReasoningLow,
		},
		{
			name:   "gpt-5 maps none to minimal",
			model:  "gpt-5",
			in:     ReasoningNone,
			expect: ReasoningMinimal,
		},
		{
			name:   "gpt-5 clamps xhigh to high",
			model:  "gpt-5",
			in:     ReasoningXHigh,
			expect: ReasoningHigh,
		},
		{
			name:   "gpt-5.1 clamps xhigh to high",
			model:  "gpt-5.1",
			in:     ReasoningXHigh,
			expect: ReasoningHigh,
		},
		{
			name:   "gpt-5-codex-mini clamps xhigh to high",
			model:  "gpt-5-codex-mini",
			in:     ReasoningXHigh,
			expect: ReasoningHigh,
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
