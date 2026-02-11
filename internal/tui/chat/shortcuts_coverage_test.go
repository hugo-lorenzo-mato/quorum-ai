package chat

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// NewShortcutsOverlay
// ---------------------------------------------------------------------------

func TestNewShortcutsOverlay(t *testing.T) {
	s := NewShortcutsOverlay()
	if s == nil {
		t.Fatal("NewShortcutsOverlay returned nil")
	}
	if s.visible {
		t.Error("new overlay should not be visible")
	}
	if len(s.categories) == 0 {
		t.Error("new overlay should have predefined categories")
	}
}

func TestNewShortcutsOverlay_Categories(t *testing.T) {
	s := NewShortcutsOverlay()

	expectedCategories := []string{"Navigation", "Panels", "Actions", "Tools"}
	if len(s.categories) != len(expectedCategories) {
		t.Fatalf("expected %d categories, got %d", len(expectedCategories), len(s.categories))
	}

	for i, name := range expectedCategories {
		if s.categories[i].Name != name {
			t.Errorf("category %d: expected %q, got %q", i, name, s.categories[i].Name)
		}
		if len(s.categories[i].Shortcuts) == 0 {
			t.Errorf("category %q should have shortcuts", name)
		}
	}
}

// ---------------------------------------------------------------------------
// Toggle / Show / Hide / IsVisible
// ---------------------------------------------------------------------------

func TestShortcutsOverlay_ToggleShowHide(t *testing.T) {
	s := NewShortcutsOverlay()

	if s.IsVisible() {
		t.Error("should be hidden initially")
	}

	s.Toggle()
	if !s.IsVisible() {
		t.Error("should be visible after Toggle")
	}

	s.Toggle()
	if s.IsVisible() {
		t.Error("should be hidden after second Toggle")
	}

	s.Show()
	if !s.IsVisible() {
		t.Error("should be visible after Show")
	}

	s.Hide()
	if s.IsVisible() {
		t.Error("should be hidden after Hide")
	}
}

// ---------------------------------------------------------------------------
// SetSize
// ---------------------------------------------------------------------------

func TestShortcutsOverlay_SetSize(t *testing.T) {
	s := NewShortcutsOverlay()
	s.SetSize(100, 40)
	if s.width != 100 || s.height != 40 {
		t.Errorf("unexpected size %d x %d", s.width, s.height)
	}
}

// ---------------------------------------------------------------------------
// Render
// ---------------------------------------------------------------------------

func TestShortcutsOverlay_Render_NotVisible(t *testing.T) {
	s := NewShortcutsOverlay()
	if s.Render() != "" {
		t.Error("hidden overlay should render empty string")
	}
}

func TestShortcutsOverlay_Render_Visible(t *testing.T) {
	s := NewShortcutsOverlay()
	s.SetSize(100, 40)
	s.Show()

	rendered := s.Render()
	if rendered == "" {
		t.Fatal("visible overlay should render something")
	}

	// Check for title
	if !strings.Contains(rendered, "Shortcuts") {
		t.Error("render should contain 'Shortcuts' title")
	}

	// Check for categories
	if !strings.Contains(rendered, "Navigation") {
		t.Error("render should contain 'Navigation' category")
	}
	if !strings.Contains(rendered, "Panels") {
		t.Error("render should contain 'Panels' category")
	}
	if !strings.Contains(rendered, "Actions") {
		t.Error("render should contain 'Actions' category")
	}
	if !strings.Contains(rendered, "Tools") {
		t.Error("render should contain 'Tools' category")
	}

	// Check for some shortcut keys
	if !strings.Contains(rendered, "Tab") {
		t.Error("render should contain 'Tab' shortcut")
	}
	if !strings.Contains(rendered, "Esc") {
		t.Error("render should contain 'Esc' shortcut")
	}

	// Check for footer
	if !strings.Contains(rendered, "Press any key to close") {
		t.Error("render should contain footer text")
	}
}

func TestShortcutsOverlay_Render_SmallWidth(t *testing.T) {
	s := NewShortcutsOverlay()
	s.SetSize(40, 30) // Narrow
	s.Show()

	rendered := s.Render()
	if rendered == "" {
		t.Error("should render even with small width")
	}
}

func TestShortcutsOverlay_Render_VerySmallWidth(t *testing.T) {
	s := NewShortcutsOverlay()
	s.SetSize(5, 10) // Extremely narrow (innerWidth would be negative)
	s.Show()

	rendered := s.Render()
	if rendered == "" {
		t.Error("should render even with very small width")
	}
}

func TestShortcutsOverlay_Render_LargeWidth(t *testing.T) {
	s := NewShortcutsOverlay()
	s.SetSize(200, 60) // Very wide
	s.Show()

	rendered := s.Render()
	if rendered == "" {
		t.Error("should render with large width")
	}
}

// ---------------------------------------------------------------------------
// RenderCentered
// ---------------------------------------------------------------------------

func TestShortcutsOverlay_RenderCentered_NotVisible(t *testing.T) {
	s := NewShortcutsOverlay()
	if s.RenderCentered(100, 40) != "" {
		t.Error("hidden overlay should render empty string")
	}
}

func TestShortcutsOverlay_RenderCentered_Visible(t *testing.T) {
	s := NewShortcutsOverlay()
	s.SetSize(80, 30)
	s.Show()

	rendered := s.RenderCentered(120, 50)
	if rendered == "" {
		t.Fatal("centered render should produce output")
	}

	// Should contain the shortcuts content
	if !strings.Contains(rendered, "Shortcuts") {
		t.Error("centered render should contain 'Shortcuts'")
	}
}

func TestShortcutsOverlay_RenderCentered_SmallScreen(t *testing.T) {
	s := NewShortcutsOverlay()
	s.SetSize(80, 30)
	s.Show()

	// Screen smaller than content
	rendered := s.RenderCentered(20, 10)
	if rendered == "" {
		t.Error("should render even on small screen")
	}
}

func TestShortcutsOverlay_RenderCentered_ExactSize(t *testing.T) {
	s := NewShortcutsOverlay()
	s.SetSize(80, 30)
	s.Show()

	// Screen exactly the same size as content
	rendered := s.RenderCentered(80, 30)
	if rendered == "" {
		t.Error("should render at exact size")
	}
}

// ---------------------------------------------------------------------------
// ShortcutCategory / Shortcut structs
// ---------------------------------------------------------------------------

func TestShortcutStructs(t *testing.T) {
	cat := ShortcutCategory{
		Name: "Test",
		Shortcuts: []Shortcut{
			{Key: "Ctrl+A", Description: "Select all"},
			{Key: "Ctrl+C", Description: "Copy"},
		},
	}

	if cat.Name != "Test" {
		t.Errorf("unexpected category name: %q", cat.Name)
	}
	if len(cat.Shortcuts) != 2 {
		t.Errorf("expected 2 shortcuts, got %d", len(cat.Shortcuts))
	}
	if cat.Shortcuts[0].Key != "Ctrl+A" {
		t.Errorf("unexpected key: %q", cat.Shortcuts[0].Key)
	}
	if cat.Shortcuts[0].Description != "Select all" {
		t.Errorf("unexpected description: %q", cat.Shortcuts[0].Description)
	}
}

// ---------------------------------------------------------------------------
// Render column layout
// ---------------------------------------------------------------------------

func TestShortcutsOverlay_Render_TwoColumnLayout(t *testing.T) {
	s := NewShortcutsOverlay()
	s.SetSize(100, 40)
	s.Show()

	rendered := s.Render()

	// First two categories go to left column, last two to right column
	// All four should appear in the output
	for _, cat := range s.categories {
		if !strings.Contains(rendered, cat.Name) {
			t.Errorf("render should contain category %q", cat.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Render with unequal column lengths
// ---------------------------------------------------------------------------

func TestShortcutsOverlay_Render_UnequalColumns(t *testing.T) {
	// Create an overlay with unequal categories
	s := &ShortcutsOverlay{
		categories: []ShortcutCategory{
			{
				Name: "Short",
				Shortcuts: []Shortcut{
					{Key: "A", Description: "Action"},
				},
			},
			{
				Name: "AlsoShort",
				Shortcuts: []Shortcut{
					{Key: "B", Description: "Beta"},
				},
			},
			{
				Name: "Long",
				Shortcuts: []Shortcut{
					{Key: "C", Description: "Charlie"},
					{Key: "D", Description: "Delta"},
					{Key: "E", Description: "Echo"},
					{Key: "F", Description: "Foxtrot"},
					{Key: "G", Description: "Golf"},
				},
			},
		},
		visible: true,
		width:   100,
		height:  40,
	}

	rendered := s.Render()
	if rendered == "" {
		t.Error("render should produce output")
	}
}
