package tui

import (
	"os"

	"golang.org/x/term"
)

// OutputMode represents the output mode.
type OutputMode int

const (
	// ModeTUI uses full TUI with Bubbletea.
	ModeTUI OutputMode = iota

	// ModePlain uses plain text output.
	ModePlain

	// ModeJSON uses JSON structured output.
	ModeJSON

	// ModeQuiet suppresses most output.
	ModeQuiet
)

// String returns the string representation of the output mode.
func (m OutputMode) String() string {
	switch m {
	case ModeTUI:
		return "tui"
	case ModePlain:
		return "plain"
	case ModeJSON:
		return "json"
	case ModeQuiet:
		return "quiet"
	default:
		return "unknown"
	}
}

// Detector determines the appropriate output mode.
type Detector struct {
	forceMode *OutputMode
	noColor   bool
}

// NewDetector creates a new output mode detector.
func NewDetector() *Detector {
	return &Detector{}
}

// ForceMode forces a specific output mode.
func (d *Detector) ForceMode(mode OutputMode) *Detector {
	d.forceMode = &mode
	return d
}

// NoColor disables color output.
func (d *Detector) NoColor(disable bool) *Detector {
	d.noColor = disable
	return d
}

// Detect determines the appropriate output mode.
func (d *Detector) Detect() OutputMode {
	// Check forced mode
	if d.forceMode != nil {
		return *d.forceMode
	}

	// Check environment variables
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		return ModePlain
	}

	if os.Getenv("QUORUM_OUTPUT") == "json" {
		return ModeJSON
	}

	if os.Getenv("QUORUM_QUIET") == "1" {
		return ModeQuiet
	}

	// Check if stdout is a TTY
	if !d.isTTY() {
		return ModePlain
	}

	return ModeTUI
}

// isTTY checks if stdout is a terminal.
func (d *Detector) isTTY() bool {
	fd := int(os.Stdout.Fd())
	return term.IsTerminal(fd)
}

// ShouldUseColor determines if color should be used.
func (d *Detector) ShouldUseColor() bool {
	if d.noColor {
		return false
	}

	// Check NO_COLOR convention
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check TERM
	if os.Getenv("TERM") == "dumb" {
		return false
	}

	return d.isTTY()
}

// TerminalSize returns terminal dimensions.
func TerminalSize() (width, height int) {
	fd := int(os.Stdout.Fd())
	w, h, err := term.GetSize(fd)
	if err != nil {
		return 80, 24 // Default
	}
	return w, h
}

// ParseOutputMode parses an output mode from string.
func ParseOutputMode(s string) OutputMode {
	switch s {
	case "tui":
		return ModeTUI
	case "plain":
		return ModePlain
	case "json":
		return ModeJSON
	case "quiet":
		return ModeQuiet
	default:
		return ModeTUI
	}
}
