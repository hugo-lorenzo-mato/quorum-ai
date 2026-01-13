package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestColorConstants(t *testing.T) {
	tests := []struct {
		name  string
		color lipgloss.Color
	}{
		{"ColorPrimary", ColorPrimary},
		{"ColorSecondary", ColorSecondary},
		{"ColorAccent", ColorAccent},
		{"ColorSuccess", ColorSuccess},
		{"ColorWarning", ColorWarning},
		{"ColorError", ColorError},
		{"ColorInfo", ColorInfo},
		{"ColorText", ColorText},
		{"ColorTextMuted", ColorTextMuted},
		{"ColorBorder", ColorBorder},
		{"ColorBackground", ColorBackground},
		{"ColorHighlight", ColorHighlight},
		{"ColorAnalyze", ColorAnalyze},
		{"ColorPlan", ColorPlan},
		{"ColorExecute", ColorExecute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Colors should not be empty
			if string(tt.color) == "" {
				t.Errorf("%s is empty", tt.name)
			}
		})
	}
}

func TestDarkScheme(t *testing.T) {
	if string(DarkScheme.Primary) == "" {
		t.Error("DarkScheme.Primary is empty")
	}
	if string(DarkScheme.Secondary) == "" {
		t.Error("DarkScheme.Secondary is empty")
	}
	if string(DarkScheme.Success) == "" {
		t.Error("DarkScheme.Success is empty")
	}
	if string(DarkScheme.Warning) == "" {
		t.Error("DarkScheme.Warning is empty")
	}
	if string(DarkScheme.Error) == "" {
		t.Error("DarkScheme.Error is empty")
	}
	if string(DarkScheme.Text) == "" {
		t.Error("DarkScheme.Text is empty")
	}
	if string(DarkScheme.TextMuted) == "" {
		t.Error("DarkScheme.TextMuted is empty")
	}
	if string(DarkScheme.Border) == "" {
		t.Error("DarkScheme.Border is empty")
	}
	if string(DarkScheme.Background) == "" {
		t.Error("DarkScheme.Background is empty")
	}
}

func TestLightScheme(t *testing.T) {
	if string(LightScheme.Primary) == "" {
		t.Error("LightScheme.Primary is empty")
	}
	if string(LightScheme.Secondary) == "" {
		t.Error("LightScheme.Secondary is empty")
	}
	if string(LightScheme.Success) == "" {
		t.Error("LightScheme.Success is empty")
	}
	if string(LightScheme.Warning) == "" {
		t.Error("LightScheme.Warning is empty")
	}
	if string(LightScheme.Error) == "" {
		t.Error("LightScheme.Error is empty")
	}
	if string(LightScheme.Text) == "" {
		t.Error("LightScheme.Text is empty")
	}
	if string(LightScheme.TextMuted) == "" {
		t.Error("LightScheme.TextMuted is empty")
	}
	if string(LightScheme.Border) == "" {
		t.Error("LightScheme.Border is empty")
	}
	if string(LightScheme.Background) == "" {
		t.Error("LightScheme.Background is empty")
	}
}

func TestSetColorScheme(t *testing.T) {
	// Save original
	original := CurrentScheme

	// Set to light scheme
	SetColorScheme(LightScheme)
	if CurrentScheme.Primary != LightScheme.Primary {
		t.Error("SetColorScheme did not set LightScheme")
	}

	// Set to dark scheme
	SetColorScheme(DarkScheme)
	if CurrentScheme.Primary != DarkScheme.Primary {
		t.Error("SetColorScheme did not set DarkScheme")
	}

	// Restore original
	CurrentScheme = original
}

func TestColorScheme_Fields(t *testing.T) {
	scheme := ColorScheme{
		Primary:    lipgloss.Color("#123456"),
		Secondary:  lipgloss.Color("#234567"),
		Success:    lipgloss.Color("#345678"),
		Warning:    lipgloss.Color("#456789"),
		Error:      lipgloss.Color("#567890"),
		Text:       lipgloss.Color("#678901"),
		TextMuted:  lipgloss.Color("#789012"),
		Border:     lipgloss.Color("#890123"),
		Background: lipgloss.Color("#901234"),
	}

	if string(scheme.Primary) != "#123456" {
		t.Errorf("Primary = %q, want %q", scheme.Primary, "#123456")
	}
	if string(scheme.Secondary) != "#234567" {
		t.Errorf("Secondary = %q, want %q", scheme.Secondary, "#234567")
	}
	if string(scheme.Success) != "#345678" {
		t.Errorf("Success = %q, want %q", scheme.Success, "#345678")
	}
}
