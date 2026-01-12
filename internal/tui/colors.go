// Package tui provides terminal user interface components.
package tui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	// Primary colors
	ColorPrimary   = lipgloss.Color("#7C3AED") // Purple
	ColorSecondary = lipgloss.Color("#06B6D4") // Cyan
	ColorAccent    = lipgloss.Color("#F59E0B") // Amber

	// Status colors
	ColorSuccess = lipgloss.Color("#10B981") // Green
	ColorWarning = lipgloss.Color("#F59E0B") // Amber
	ColorError   = lipgloss.Color("#EF4444") // Red
	ColorInfo    = lipgloss.Color("#3B82F6") // Blue

	// Neutral colors
	ColorText       = lipgloss.Color("#E5E7EB") // Light gray
	ColorTextMuted  = lipgloss.Color("#9CA3AF") // Muted gray
	ColorBorder     = lipgloss.Color("#374151") // Dark gray
	ColorBackground = lipgloss.Color("#1F2937") // Dark background
	ColorHighlight  = lipgloss.Color("#374151") // Selection

	// Phase colors
	ColorAnalyze = lipgloss.Color("#8B5CF6") // Purple
	ColorPlan    = lipgloss.Color("#06B6D4") // Cyan
	ColorExecute = lipgloss.Color("#10B981") // Green
)

// ColorScheme defines a complete color scheme.
type ColorScheme struct {
	Primary    lipgloss.Color
	Secondary  lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color
	Text       lipgloss.Color
	TextMuted  lipgloss.Color
	Border     lipgloss.Color
	Background lipgloss.Color
}

// DarkScheme is the default dark color scheme.
var DarkScheme = ColorScheme{
	Primary:    ColorPrimary,
	Secondary:  ColorSecondary,
	Success:    ColorSuccess,
	Warning:    ColorWarning,
	Error:      ColorError,
	Text:       ColorText,
	TextMuted:  ColorTextMuted,
	Border:     ColorBorder,
	Background: ColorBackground,
}

// LightScheme is an alternative light color scheme.
var LightScheme = ColorScheme{
	Primary:    lipgloss.Color("#6D28D9"),
	Secondary:  lipgloss.Color("#0891B2"),
	Success:    lipgloss.Color("#059669"),
	Warning:    lipgloss.Color("#D97706"),
	Error:      lipgloss.Color("#DC2626"),
	Text:       lipgloss.Color("#1F2937"),
	TextMuted:  lipgloss.Color("#6B7280"),
	Border:     lipgloss.Color("#D1D5DB"),
	Background: lipgloss.Color("#F9FAFB"),
}

// CurrentScheme holds the active color scheme.
var CurrentScheme = DarkScheme

// SetColorScheme sets the active color scheme.
func SetColorScheme(scheme ColorScheme) {
	CurrentScheme = scheme
}
