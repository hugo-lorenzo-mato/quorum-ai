package tui_test

import (
	"os"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui"
)

func TestOutputMode_String_Unknown(t *testing.T) {
	t.Parallel()
	// Test unknown mode (value outside enum range)
	mode := tui.OutputMode(999)
	got := mode.String()
	if got != "unknown" {
		t.Errorf("OutputMode(999).String() = %q, want %q", got, "unknown")
	}
}

func TestDetector_NoColor(t *testing.T) {
	t.Parallel()
	d := tui.NewDetector().NoColor(true)
	// After setting NoColor, the detector should have it set
	// We can't directly test the internal field, but we can test behavior
	if d == nil {
		t.Error("NoColor() returned nil")
	}
}

func TestDetector_Detect_CIEnvironment(t *testing.T) {
	// This test mutates process environment variables; keep it serialized.
	// Save current env
	originalCI := os.Getenv("CI")
	originalGH := os.Getenv("GITHUB_ACTIONS")
	defer func() {
		os.Setenv("CI", originalCI)
		os.Setenv("GITHUB_ACTIONS", originalGH)
	}()

	// Clear all env vars first
	os.Setenv("CI", "")
	os.Setenv("GITHUB_ACTIONS", "")

	// Test CI environment
	os.Setenv("CI", "true")
	d := tui.NewDetector()
	mode := d.Detect()
	if mode != tui.ModePlain {
		t.Errorf("Detect() in CI = %v, want %v", mode, tui.ModePlain)
	}

	// Test GitHub Actions environment
	os.Setenv("CI", "")
	os.Setenv("GITHUB_ACTIONS", "true")
	d = tui.NewDetector()
	mode = d.Detect()
	if mode != tui.ModePlain {
		t.Errorf("Detect() in GITHUB_ACTIONS = %v, want %v", mode, tui.ModePlain)
	}
}

func TestDetector_Detect_QuorumOutput(t *testing.T) {
	// This test mutates process environment variables; keep it serialized.
	// Save current env
	originalCI := os.Getenv("CI")
	originalGH := os.Getenv("GITHUB_ACTIONS")
	originalOutput := os.Getenv("QUORUM_OUTPUT")
	defer func() {
		os.Setenv("CI", originalCI)
		os.Setenv("GITHUB_ACTIONS", originalGH)
		os.Setenv("QUORUM_OUTPUT", originalOutput)
	}()

	// Clear CI env vars
	os.Setenv("CI", "")
	os.Setenv("GITHUB_ACTIONS", "")

	// Test QUORUM_OUTPUT=json
	os.Setenv("QUORUM_OUTPUT", "json")
	d := tui.NewDetector()
	mode := d.Detect()
	if mode != tui.ModeJSON {
		t.Errorf("Detect() with QUORUM_OUTPUT=json = %v, want %v", mode, tui.ModeJSON)
	}
}

func TestDetector_Detect_QuorumQuiet(t *testing.T) {
	// This test mutates process environment variables; keep it serialized.
	// Save current env
	originalCI := os.Getenv("CI")
	originalGH := os.Getenv("GITHUB_ACTIONS")
	originalQuiet := os.Getenv("QUORUM_QUIET")
	originalOutput := os.Getenv("QUORUM_OUTPUT")
	defer func() {
		os.Setenv("CI", originalCI)
		os.Setenv("GITHUB_ACTIONS", originalGH)
		os.Setenv("QUORUM_QUIET", originalQuiet)
		os.Setenv("QUORUM_OUTPUT", originalOutput)
	}()

	// Clear env vars
	os.Setenv("CI", "")
	os.Setenv("GITHUB_ACTIONS", "")
	os.Setenv("QUORUM_OUTPUT", "")

	// Test QUORUM_QUIET=1
	os.Setenv("QUORUM_QUIET", "1")
	d := tui.NewDetector()
	mode := d.Detect()
	if mode != tui.ModeQuiet {
		t.Errorf("Detect() with QUORUM_QUIET=1 = %v, want %v", mode, tui.ModeQuiet)
	}
}

func TestDetector_ShouldUseColor_NoColor(t *testing.T) {
	t.Parallel()
	d := tui.NewDetector().NoColor(true)
	result := d.ShouldUseColor()
	if result {
		t.Error("ShouldUseColor() with NoColor(true) should return false")
	}
}

func TestDetector_ShouldUseColor_EnvNoColor(t *testing.T) {
	// This test mutates process environment variables; keep it serialized.
	// Save current env
	originalNoColor := os.Getenv("NO_COLOR")
	defer os.Setenv("NO_COLOR", originalNoColor)

	os.Setenv("NO_COLOR", "1")
	d := tui.NewDetector()
	result := d.ShouldUseColor()
	if result {
		t.Error("ShouldUseColor() with NO_COLOR env should return false")
	}
}

func TestDetector_ShouldUseColor_DumbTerminal(t *testing.T) {
	// This test mutates process environment variables; keep it serialized.
	// Save current env
	originalNoColor := os.Getenv("NO_COLOR")
	originalTerm := os.Getenv("TERM")
	defer func() {
		os.Setenv("NO_COLOR", originalNoColor)
		os.Setenv("TERM", originalTerm)
	}()

	os.Setenv("NO_COLOR", "")
	os.Setenv("TERM", "dumb")
	d := tui.NewDetector()
	result := d.ShouldUseColor()
	if result {
		t.Error("ShouldUseColor() with TERM=dumb should return false")
	}
}

func TestTerminalSize(t *testing.T) {
	t.Parallel()
	// This will likely return defaults since we're not in a real terminal
	w, h := tui.TerminalSize()
	// Should return reasonable defaults or actual values
	if w <= 0 || h <= 0 {
		t.Errorf("TerminalSize() = (%d, %d), expected positive values", w, h)
	}
}
