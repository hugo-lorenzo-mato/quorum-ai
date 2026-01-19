package workflow

import "testing"

func TestParseOptimizationResult(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{
			name: "direct JSON with optimized_prompt",
			output: `{
				"optimized_prompt": "Create a hello.txt file containing 'Hello World'",
				"changes_made": ["Translated to English"],
				"reasoning": "English is clearer"
			}`,
			want:    "Create a hello.txt file containing 'Hello World'",
			wantErr: false,
		},
		{
			name: "Claude CLI JSON wrapper with direct JSON in result",
			output: `{
				"type": "result",
				"subtype": "success",
				"is_error": false,
				"result": "{\"optimized_prompt\": \"Optimized prompt here\", \"changes_made\": [], \"reasoning\": \"test\"}",
				"session_id": "test-session"
			}`,
			want:    "Optimized prompt here",
			wantErr: false,
		},
		{
			name:    "Claude CLI JSON wrapper with markdown in result",
			output:  `{"type": "result", "result": "Here is the optimized prompt:\n\n` + "```" + `json\n{\"optimized_prompt\": \"Markdown extracted prompt\", \"changes_made\": [], \"reasoning\": \"test\"}\n` + "```" + `", "session_id": "test-session"}`,
			want:    "Markdown extracted prompt",
			wantErr: false,
		},
		{
			name:    "markdown code block in direct output",
			output:  "Here is the result:\n```json\n{\"optimized_prompt\": \"From markdown block\", \"changes_made\": [], \"reasoning\": \"test\"}\n```",
			want:    "From markdown block",
			wantErr: false,
		},
		{
			name:    "empty output",
			output:  "",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			output:  "this is not json",
			want:    "",
			wantErr: true,
		},
		{
			name: "JSON without optimized_prompt field",
			output: `{
				"some_field": "value",
				"other_field": "another value"
			}`,
			want:    "",
			wantErr: true,
		},
		{
			name: "Claude CLI wrapper with empty result",
			output: `{
				"type": "result",
				"result": "",
				"session_id": "test-session"
			}`,
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOptimizationResult(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOptimizationResult() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseOptimizationResult() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractJSONFromMarkdown(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "json code block",
			text: "Some text\n```json\n{\"key\": \"value\"}\n```\nMore text",
			want: `{"key": "value"}`,
		},
		{
			name: "plain code block",
			text: "Some text\n```\n{\"key\": \"value\"}\n```\nMore text",
			want: `{"key": "value"}`,
		},
		{
			name: "no code block",
			text: "Some text without code blocks",
			want: "",
		},
		{
			name: "empty string",
			text: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONFromMarkdown(tt.text)
			if got != tt.want {
				t.Errorf("extractJSONFromMarkdown() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractOptimizedPromptFromMarkdown(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "standard markdown section",
			text: `## Optimized Prompt
Create a file named hello.txt with the content "Hello World"

## Changes Made
- Clarified the request`,
			want: `Create a file named hello.txt with the content "Hello World"`,
		},
		{
			name: "with h3 header",
			text: `### Optimized Prompt
Do the thing

### Changes Made
- Did stuff`,
			want: "Do the thing",
		},
		{
			name: "multiline prompt",
			text: `## Optimized Prompt
First line of prompt.
Second line of prompt.
Third line of prompt.

## Changes Made
- Change 1`,
			want: `First line of prompt.
Second line of prompt.
Third line of prompt.`,
		},
		{
			name: "no optimized prompt section",
			text: `## Some Other Section
Content here`,
			want: "",
		},
		{
			name: "case insensitive",
			text: `## OPTIMIZED PROMPT
The prompt content

## Changes`,
			want: "The prompt content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractOptimizedPromptFromMarkdown(tt.text)
			if got != tt.want {
				t.Errorf("extractOptimizedPromptFromMarkdown() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseOptimizationResult_Markdown(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{
			name: "markdown format",
			output: `## Optimized Prompt
Create a hello.txt file containing 'Hello World'

## Changes Made
- Translated to English

## Reasoning
English is clearer`,
			want:    "Create a hello.txt file containing 'Hello World'",
			wantErr: false,
		},
		{
			name: "json takes precedence over markdown",
			output: `{"optimized_prompt": "JSON prompt", "changes_made": [], "reasoning": "test"}`,
			want:    "JSON prompt",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOptimizationResult(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOptimizationResult() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseOptimizationResult() = %q, want %q", got, tt.want)
			}
		})
	}
}
