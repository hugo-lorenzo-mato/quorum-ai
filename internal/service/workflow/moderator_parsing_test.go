package workflow

import (
	"fmt"
	"strings"
	"testing"
)

func TestSanitizeRawOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no code blocks",
			input: "Plain text\nMore text",
			want:  "Plain text\nMore text",
		},
		{
			name:  "removes opening code block",
			input: "```yaml\nconsensus_score: 80\n```",
			want:  "consensus_score: 80",
		},
		{
			name:  "removes json code block markers",
			input: "```json\n{\"key\": \"value\"}\n```",
			want:  "{\"key\": \"value\"}",
		},
		{
			name:  "preserves non-code-block content",
			input: "Some text\n```\nCode\n```\nMore text",
			want:  "Some text\nCode\nMore text",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeRawOutput(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeRawOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseYAMLFrontmatterRobust(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		wantFM        bool
		wantFMContain string
		wantBody      string
	}{
		{
			name:          "valid frontmatter",
			input:         "---\nconsensus_score: 85\n---\n\n## Body content",
			wantFM:        true,
			wantFMContain: "consensus_score: 85",
			wantBody:      "\n## Body content",
		},
		{
			name:     "no frontmatter",
			input:    "Just regular text\nNo YAML here",
			wantFM:   false,
			wantBody: "Just regular text\nNo YAML here",
		},
		{
			name:          "frontmatter at start",
			input:         "---\nkey: value\n---\nBody",
			wantFM:        true,
			wantFMContain: "key: value",
		},
		{
			name:     "single delimiter",
			input:    "---\nSome text without closing",
			wantFM:   false,
			wantBody: "---\nSome text without closing",
		},
		{
			name:          "frontmatter with multiple fields",
			input:         "---\nscore: 90\nhigh: 1\nlow: 0\n---\nContent",
			wantFM:        true,
			wantFMContain: "score: 90",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fm, body, ok := parseYAMLFrontmatterRobust(tt.input)
			if ok != tt.wantFM {
				t.Errorf("ok = %v, want %v", ok, tt.wantFM)
			}
			if tt.wantFM && !strings.Contains(fm, tt.wantFMContain) {
				t.Errorf("frontmatter = %q, missing %q", fm, tt.wantFMContain)
			}
			if !tt.wantFM && body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestExtractScoreFromInterface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		val       interface{}
		wantScore float64
		wantOK    bool
	}{
		{"nil", nil, 0, false},
		{"int 85 (percentage)", 85, 0.85, true},
		{"int 0", 0, 0, true},
		{"int 100", 100, 1.0, true},
		{"float64 ratio", float64(0.75), 0.75, true},
		{"float64 percentage", float64(90.0), 0.90, true},
		{"float64 1.0 (ratio)", float64(1.0), 1.0, true},
		{"string 80", "80", 0.80, true},
		{"string 80%", "80%", 0.80, true},
		{"string 0.65", "0.65", 0.65, true},
		{"string 90/100", "90/100", 0.90, true},
		{"string with spaces", " 75 ", 0.75, true},
		{"string invalid", "abc", 0, false},
		{"string empty", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			score, ok := extractScoreFromInterface(tt.val)
			if ok != tt.wantOK {
				t.Errorf("extractScoreFromInterface(%v) ok = %v, want %v", tt.val, ok, tt.wantOK)
			}
			if ok && score != tt.wantScore {
				t.Errorf("extractScoreFromInterface(%v) = %f, want %f", tt.val, score, tt.wantScore)
			}
		})
	}
}

func TestIsModeratorValidationError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"validation error", fmt.Errorf("moderator output validation: no score"), true},
		{"validation failed", fmt.Errorf("moderator validation failed: empty output"), true},
		{"unrelated error", fmt.Errorf("network timeout"), false},
		{"wrapped validation", fmt.Errorf("failed: moderator output validation: x"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsModeratorValidationError(tt.err)
			if got != tt.want {
				t.Errorf("IsModeratorValidationError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestModeratorValidationError_Error(t *testing.T) {
	t.Parallel()

	err := &ModeratorValidationError{
		Reason:    "no score found",
		TokensIn:  1000,
		TokensOut: 500,
		OutputLen: 250,
	}

	msg := err.Error()
	if !strings.Contains(msg, "no score found") {
		t.Errorf("Error() missing reason: %s", msg)
	}
	if !strings.Contains(msg, "tokens_in=1000") {
		t.Errorf("Error() missing tokens_in: %s", msg)
	}
	if !strings.Contains(msg, "tokens_out=500") {
		t.Errorf("Error() missing tokens_out: %s", msg)
	}
	if !strings.Contains(msg, "output_len=250") {
		t.Errorf("Error() missing output_len: %s", msg)
	}
}
