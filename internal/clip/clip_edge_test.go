package clip

import (
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// writeAllOSC52: TMUX env var path
// ---------------------------------------------------------------------------

func TestWriteAllOSC52_TMUXEnv(t *testing.T) {
	// writeAllOSC52 returns early because stderr is not a terminal in tests.
	// But we can at least verify the TMUX env branch doesn't panic on empty text.
	// The function checks empty text first, so this exercises that path.
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")
	err := writeAllOSC52("")
	if err == nil {
		t.Error("expected error for empty text even with TMUX set")
	}
	if !strings.Contains(err.Error(), "empty clipboard text") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// writeAllOSC52: STY (Screen) env var path
// ---------------------------------------------------------------------------

func TestWriteAllOSC52_STYEnv(t *testing.T) {
	t.Setenv("STY", "12345.pts-0.hostname")
	err := writeAllOSC52("")
	if err == nil {
		t.Error("expected error for empty text even with STY set")
	}
	if !strings.Contains(err.Error(), "empty clipboard text") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// writeAllOSC52: both TMUX and STY set (TMUX takes priority)
// ---------------------------------------------------------------------------

func TestWriteAllOSC52_TMUXOverSTY(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")
	t.Setenv("STY", "12345.pts-0.hostname")
	err := writeAllOSC52("")
	if err == nil {
		t.Error("expected error for empty text with both TMUX and STY")
	}
}

// ---------------------------------------------------------------------------
// writeAllOSC52: non-empty text, not terminal (stderr check)
// ---------------------------------------------------------------------------

func TestWriteAllOSC52_NotTerminalNonEmpty(t *testing.T) {
	// With no TMUX/STY, non-empty text, but stderr is not a terminal.
	// This exercises the IsTerminal check path.
	t.Setenv("TMUX", "")
	t.Setenv("STY", "")
	err := writeAllOSC52("hello world")
	if err == nil {
		t.Skip("stderr is actually a terminal in this environment")
	}
	if !strings.Contains(err.Error(), "not a terminal") {
		t.Errorf("expected 'not a terminal' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// writeAllOSC52: text exactly at the limit boundary
// ---------------------------------------------------------------------------

func TestWriteAllOSC52_ExactLimit(t *testing.T) {
	exactText := strings.Repeat("x", osc52LimitBytes)
	err := writeAllOSC52(exactText)
	// In test environments stderr is typically not a terminal, so this
	// should fail with "not a terminal" rather than "too large".
	if err == nil {
		t.Skip("stderr is a terminal and OSC52 succeeded")
	}
	// Should NOT be "too large" since we are exactly at the limit
	if strings.Contains(err.Error(), "too large") {
		t.Error("text at exact limit should not be 'too large'")
	}
}

// ---------------------------------------------------------------------------
// writeAllOSC52: text one byte over the limit
// ---------------------------------------------------------------------------

func TestWriteAllOSC52_OneBytOverLimit(t *testing.T) {
	overText := strings.Repeat("x", osc52LimitBytes+1)
	err := writeAllOSC52(overText)
	if err == nil {
		t.Fatal("expected error for text exceeding OSC52 limit")
	}
	// In test environments, stderr is usually not a terminal, so the
	// "not a terminal" check fires before the size check. Both are valid
	// error paths.
	errMsg := err.Error()
	if !strings.Contains(errMsg, "too large") && !strings.Contains(errMsg, "not a terminal") {
		t.Errorf("expected 'too large' or 'not a terminal' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// writeTempFile: verify file path is clean
// ---------------------------------------------------------------------------

func TestWriteTempFile_CleanPath(t *testing.T) {
	path, err := writeTempFile("clean path test")
	if err != nil {
		t.Fatalf("writeTempFile: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	// filepath.Clean should have been applied
	if strings.Contains(path, "//") || strings.Contains(path, "/./") {
		t.Errorf("path should be clean: %q", path)
	}
}

// ---------------------------------------------------------------------------
// writeTempFile: verify file naming pattern
// ---------------------------------------------------------------------------

func TestWriteTempFile_NamingPattern(t *testing.T) {
	path, err := writeTempFile("naming test")
	if err != nil {
		t.Fatalf("writeTempFile: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	if !strings.Contains(path, "quorum-ai-clipboard-") {
		t.Errorf("temp file should contain 'quorum-ai-clipboard-', got: %q", path)
	}
	if !strings.HasSuffix(path, ".txt") {
		t.Errorf("temp file should end with .txt, got: %q", path)
	}
}

// ---------------------------------------------------------------------------
// WriteAll: verify text is passed through to temp file
// ---------------------------------------------------------------------------

func TestWriteAll_TextContentVerified(t *testing.T) {
	t.Cleanup(resetStubs())
	nativeWriteAll = func(_ string) error { return errFake("native down") }
	osc52WriteAll = func(_ string) error { return errFake("osc52 down") }

	content := "multiline\ncontent\twith tabs\nand unicode: \u2603"
	res, err := WriteAll(content)
	if err != nil {
		t.Fatalf("WriteAll: %v", err)
	}
	if res.Method != MethodFile {
		t.Fatalf("Method = %q, want %q", res.Method, MethodFile)
	}
	t.Cleanup(func() { _ = os.Remove(res.FilePath) })

	data, err := os.ReadFile(res.FilePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != content {
		t.Errorf("content mismatch: got %q, want %q", string(data), content)
	}
}

// ---------------------------------------------------------------------------
// WriteAll: native fails, osc52 captures exact content
// ---------------------------------------------------------------------------

func TestWriteAll_OSC52_ContentPassthrough(t *testing.T) {
	t.Cleanup(resetStubs())
	var captured string
	nativeWriteAll = func(_ string) error { return errFake("native down") }
	osc52WriteAll = func(text string) error {
		captured = text
		return nil
	}

	input := "exact content to be passed"
	res, err := WriteAll(input)
	if err != nil {
		t.Fatalf("WriteAll: %v", err)
	}
	if res.Method != MethodOSC52 {
		t.Fatalf("Method = %q, want %q", res.Method, MethodOSC52)
	}
	if captured != input {
		t.Errorf("OSC52 received %q, want %q", captured, input)
	}
}

// ---------------------------------------------------------------------------
// WriteAll: native succeeds with zero-length string
// ---------------------------------------------------------------------------

func TestWriteAll_NativeEmptyString(t *testing.T) {
	t.Cleanup(resetStubs())
	nativeWriteAll = func(_ string) error { return nil }

	res, err := WriteAll("")
	if err != nil {
		t.Fatalf("WriteAll: %v", err)
	}
	if res.Method != MethodNative {
		t.Errorf("Method = %q, want %q", res.Method, MethodNative)
	}
}

// ---------------------------------------------------------------------------
// Result: zero value
// ---------------------------------------------------------------------------

func TestResult_ZeroValue(t *testing.T) {
	var r Result
	if r.Method != "" {
		t.Errorf("zero Method should be empty, got %q", r.Method)
	}
	if r.FilePath != "" {
		t.Errorf("zero FilePath should be empty, got %q", r.FilePath)
	}
}

// ---------------------------------------------------------------------------
// Method type comparison
// ---------------------------------------------------------------------------

func TestMethodComparisons(t *testing.T) {
	tests := []struct {
		a, b Method
		eq   bool
	}{
		{MethodNative, MethodNative, true},
		{MethodOSC52, MethodOSC52, true},
		{MethodFile, MethodFile, true},
		{MethodNative, MethodOSC52, false},
		{MethodNative, MethodFile, false},
		{MethodOSC52, MethodFile, false},
	}
	for _, tc := range tests {
		got := tc.a == tc.b
		if got != tc.eq {
			t.Errorf("%q == %q: got %v, want %v", tc.a, tc.b, got, tc.eq)
		}
	}
}
