package tui_test

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui"
)

func TestLayout_Right(t *testing.T) {
	layout := tui.NewLayout(80, 24)
	result := layout.Right("test")
	if len(result) == 0 {
		t.Error("Right() returned empty string")
	}
}

func TestLayout_Box(t *testing.T) {
	layout := tui.NewLayout(80, 24)

	// Test with title
	result := layout.Box("Title", "content")
	if len(result) == 0 {
		t.Error("Box() with title returned empty string")
	}

	// Test without title
	result = layout.Box("", "content")
	if len(result) == 0 {
		t.Error("Box() without title returned empty string")
	}

	// Test with small width
	smallLayout := tui.NewLayout(20, 24)
	result = smallLayout.Box("Title", "content")
	if len(result) == 0 {
		t.Error("Box() with small width returned empty string")
	}
}

func TestLayout_Columns_Empty(t *testing.T) {
	layout := tui.NewLayout(80, 24)
	result := layout.Columns()
	if result != "" {
		t.Errorf("Columns() with no args should return empty string, got %q", result)
	}
}

func TestLayout_Columns_Single(t *testing.T) {
	layout := tui.NewLayout(80, 24)
	result := layout.Columns("single")
	if len(result) == 0 {
		t.Error("Columns() with single column returned empty string")
	}
}

func TestSpacer(t *testing.T) {
	tests := []struct {
		lines int
		want  int
	}{
		{0, 0},
		{1, 1},
		{3, 3},
	}

	for _, tt := range tests {
		result := tui.Spacer(tt.lines)
		if len(result) != tt.want {
			t.Errorf("Spacer(%d) length = %d, want %d", tt.lines, len(result), tt.want)
		}
	}
}

func TestBadge(t *testing.T) {
	style := lipgloss.NewStyle().Bold(true)
	result := tui.Badge("test", style)
	if len(result) == 0 {
		t.Error("Badge() returned empty string")
	}
}

func TestTruncate_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"empty string", "", 10, ""},
		{"width 0", "test", 0, ""},
		{"width 1", "test", 1, "t"},
		{"width 2", "test", 2, "te"},
		{"width 3", "test", 3, "tes"},
		{"exact fit", "test", 4, "test"},
		{"needs truncation", "testing", 5, "te..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tui.Truncate(tt.input, tt.width)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
			}
		})
	}
}

func TestWrap_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
	}{
		{"empty string", "", 10},
		{"width 0", "test string", 0},
		{"negative width", "test", -1},
		{"single word", "word", 10},
		{"exact fit", "test", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just ensure it doesn't panic
			_ = tui.Wrap(tt.input, tt.width)
		})
	}
}

func TestDivider_DefaultChar(t *testing.T) {
	result := tui.Divider(10, "")
	if len(result) == 0 {
		t.Error("Divider() with empty char returned empty string")
	}
}

func TestTable_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		rows    [][]string
		width   int
	}{
		{"empty headers", []string{}, nil, 40},
		{"empty rows", []string{"A", "B"}, nil, 40},
		{"single column", []string{"A"}, [][]string{{"val"}}, 40},
		{"row shorter than headers", []string{"A", "B", "C"}, [][]string{{"1"}}, 40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just ensure it doesn't panic
			_ = tui.Table(tt.headers, tt.rows, tt.width)
		})
	}
}
