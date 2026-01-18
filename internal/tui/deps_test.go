package tui

import (
	"testing"

	_ "github.com/charmbracelet/bubbles/viewport"
	_ "github.com/charmbracelet/bubbles/textinput"
	_ "github.com/charmbracelet/glamour"
	_ "github.com/sahilm/fuzzy"
	_ "github.com/google/uuid"
)

func TestDependenciesAvailable(t *testing.T) {
	// This test just verifies imports compile
	t.Log("All TUI dependencies are available")
}
