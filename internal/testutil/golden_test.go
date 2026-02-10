package testutil_test

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "CRLF to LF",
			input: "line1\r\nline2\r\n",
			want:  "line1\nline2",
		},
		{
			name:  "trailing whitespace",
			input: "line1   \nline2\t\n",
			want:  "line1\nline2",
		},
		{
			name:  "trailing newlines",
			input: "line1\nline2\n\n\n",
			want:  "line1\nline2",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "already clean",
			input: "line1\nline2",
			want:  "line1\nline2",
		},
		{
			name:  "mixed line endings",
			input: "a\r\nb  \nc\t\r\n",
			want:  "a\nb\nc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := testutil.Normalize(tt.input)
			testutil.AssertEqual(t, got, tt.want)
		})
	}
}

func TestScrubTimestamps(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "ISO format with timezone",
			input: "started at 2024-01-15T10:30:45Z",
			want:  "started at [TIMESTAMP]",
		},
		{
			name:  "standard datetime",
			input: "created 2024-01-15 10:30:45 done",
			want:  "created [TIMESTAMP] done",
		},
		{
			name:  "time only",
			input: "run at 10:30:45",
			want:  "run at [TIMESTAMP]",
		},
		{
			name:  "no timestamps",
			input: "no timestamps here",
			want:  "no timestamps here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := testutil.ScrubTimestamps(tt.input)
			testutil.AssertEqual(t, got, tt.want)
		})
	}
}

func TestScrubDurations(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "seconds with decimals",
			input: "took 1.234s to complete",
			want:  "took [DURATION] to complete",
		},
		{
			name:  "minutes and seconds",
			input: "elapsed 5m30s",
			want:  "elapsed [DURATION][DURATION]",
		},
		{
			name:  "milliseconds",
			input: "latency: 150ms",
			want:  "latency: [DURATION]",
		},
		{
			name:  "no durations",
			input: "hello world",
			want:  "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := testutil.ScrubDurations(tt.input)
			testutil.AssertEqual(t, got, tt.want)
		})
	}
}

func TestScrubPaths(t *testing.T) {
	got := testutil.ScrubPaths("file at /home/user/project/main.go", "/home/user/project")
	testutil.AssertEqual(t, got, "file at [WORKDIR]/main.go")

	// No match
	got = testutil.ScrubPaths("file at /other/path", "/home/user/project")
	testutil.AssertEqual(t, got, "file at /other/path")
}

func TestScrubUUIDs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "single UUID",
			input: "id=550e8400-e29b-41d4-a716-446655440000",
			want:  "id=[UUID]",
		},
		{
			name:  "multiple UUIDs",
			input: "a=550e8400-e29b-41d4-a716-446655440000 b=12345678-1234-1234-1234-123456789012",
			want:  "a=[UUID] b=[UUID]",
		},
		{
			name:  "no UUIDs",
			input: "plain text",
			want:  "plain text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := testutil.ScrubUUIDs(tt.input)
			testutil.AssertEqual(t, got, tt.want)
		})
	}
}

func TestScrubHashes(t *testing.T) {
	got := testutil.ScrubHashes("commit da39a3ee5e6b4b0d3255bfef95601890afd80709")
	testutil.AssertEqual(t, got, "commit [HASH]")

	// Short strings should not match
	got = testutil.ScrubHashes("not a hash: abcdef12")
	testutil.AssertEqual(t, got, "not a hash: abcdef12")
}

func TestScrubAll(t *testing.T) {
	input := "workflow 550e8400-e29b-41d4-a716-446655440000 started at 2024-01-15T10:30:45Z in /home/user/project took 1.234s commit da39a3ee5e6b4b0d3255bfef95601890afd80709  \r\n"
	got := testutil.ScrubAll(input, "/home/user/project")

	testutil.AssertContains(t, got, "[UUID]")
	testutil.AssertContains(t, got, "[TIMESTAMP]")
	testutil.AssertContains(t, got, "[WORKDIR]")
	testutil.AssertContains(t, got, "[DURATION]")
	testutil.AssertContains(t, got, "[HASH]")
	testutil.AssertNotContains(t, got, "\r\n")
}

func TestNewGolden(t *testing.T) {
	g := testutil.NewGolden(t, t.TempDir())
	if g == nil {
		t.Fatal("expected non-nil Golden")
	}
}

func TestTempDir(t *testing.T) {
	dir := testutil.TempDir(t)
	if dir == "" {
		t.Fatal("expected non-empty temp dir")
	}
}

func TestTempFile(t *testing.T) {
	dir := testutil.TempDir(t)
	path := testutil.TempFile(t, dir, "test.txt", "hello")
	if path == "" {
		t.Fatal("expected non-empty path")
	}
}

func TestNewTestState(t *testing.T) {
	state := testutil.NewTestState()
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	testutil.AssertEqual(t, string(state.WorkflowID), "wf-test")
}

func TestNewTestState_WithOptions(t *testing.T) {
	state := testutil.NewTestState(func(s *core.WorkflowState) {
		s.Prompt = "custom prompt"
	})
	testutil.AssertEqual(t, state.Prompt, "custom prompt")
}
