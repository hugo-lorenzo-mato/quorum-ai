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
			name:    "plain text prompt",
			output:  "Create a hello.txt file containing 'Hello World' with proper error handling",
			want:    "Create a hello.txt file containing 'Hello World' with proper error handling",
			wantErr: false,
		},
		{
			name:    "multiline prompt",
			output:  "First line of prompt.\nSecond line of prompt.\nThird line of prompt.",
			want:    "First line of prompt.\nSecond line of prompt.\nThird line of prompt.",
			wantErr: false,
		},
		{
			name: "Claude CLI JSON wrapper",
			output: `{
				"type": "result",
				"subtype": "success",
				"is_error": false,
				"result": "The enhanced prompt content here with details",
				"session_id": "test-session"
			}`,
			want:    "The enhanced prompt content here with details",
			wantErr: false,
		},
		{
			name:    "with leading/trailing whitespace",
			output:  "\n\n  Enhanced prompt with whitespace  \n\n",
			want:    "Enhanced prompt with whitespace",
			wantErr: false,
		},
		{
			name:    "empty output",
			output:  "",
			want:    "",
			wantErr: true,
		},
		{
			name:    "too short output",
			output:  "short",
			want:    "",
			wantErr: true,
		},
		{
			name: "CLI wrapper with empty result",
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
