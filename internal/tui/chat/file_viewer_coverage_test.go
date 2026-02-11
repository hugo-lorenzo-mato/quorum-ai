package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// NewFileViewer
// ---------------------------------------------------------------------------

func TestNewFileViewer(t *testing.T) {
	fv := NewFileViewer()
	if fv == nil {
		t.Fatal("NewFileViewer returned nil")
	}
	if fv.visible {
		t.Error("new FileViewer should not be visible")
	}
	if fv.IsVisible() {
		t.Error("IsVisible should return false initially")
	}
	if fv.GetFilePath() != "" {
		t.Error("initial file path should be empty")
	}
}

// ---------------------------------------------------------------------------
// Toggle / Show / Hide / IsVisible
// ---------------------------------------------------------------------------

func TestFileViewer_ToggleShowHide(t *testing.T) {
	fv := NewFileViewer()

	fv.Toggle()
	if !fv.IsVisible() {
		t.Error("expected visible after Toggle")
	}

	fv.Toggle()
	if fv.IsVisible() {
		t.Error("expected hidden after second Toggle")
	}

	fv.Show()
	if !fv.IsVisible() {
		t.Error("expected visible after Show")
	}

	fv.Hide()
	if fv.IsVisible() {
		t.Error("expected hidden after Hide")
	}
}

// ---------------------------------------------------------------------------
// SetSize
// ---------------------------------------------------------------------------

func TestFileViewer_SetSize(t *testing.T) {
	fv := NewFileViewer()

	// First call should initialise the viewport
	fv.SetSize(80, 40)
	if !fv.ready {
		t.Error("expected ready after SetSize")
	}
	if fv.width != 80 || fv.height != 40 {
		t.Errorf("unexpected dimensions %d x %d", fv.width, fv.height)
	}

	// Second call should just resize
	fv.SetSize(120, 50)
	if fv.width != 120 || fv.height != 50 {
		t.Errorf("unexpected dimensions after resize %d x %d", fv.width, fv.height)
	}
}

func TestFileViewer_SetSize_SmallDimensions(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(10, 5) // very small
	if !fv.ready {
		t.Error("expected ready after SetSize with small dims")
	}
}

// ---------------------------------------------------------------------------
// SetFile - valid text file
// ---------------------------------------------------------------------------

func TestFileViewer_SetFile_Valid(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(80, 40)

	// Create a temp file inside the working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpFile := filepath.Join(cwd, "testdata_fileviewer_valid.txt")
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(tmpFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(tmpFile) })

	if err := fv.SetFile(tmpFile); err != nil {
		t.Fatalf("SetFile returned error: %v", err)
	}

	if fv.fileName != filepath.Base(tmpFile) {
		t.Errorf("unexpected fileName %q", fv.fileName)
	}
	if fv.lineCount == 0 {
		t.Error("lineCount should be > 0")
	}
	if fv.isBinary {
		t.Error("text file should not be marked as binary")
	}
	if fv.GetFilePath() != tmpFile {
		t.Errorf("GetFilePath mismatch: %q", fv.GetFilePath())
	}
}

// ---------------------------------------------------------------------------
// SetFile - nonexistent file
// ---------------------------------------------------------------------------

func TestFileViewer_SetFile_NotExist(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(80, 40)

	err := fv.SetFile("/totally/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// ---------------------------------------------------------------------------
// SetFile - directory
// ---------------------------------------------------------------------------

func TestFileViewer_SetFile_Directory(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(80, 40)

	cwd, _ := os.Getwd()
	err := fv.SetFile(cwd) // current dir is a directory
	if err == nil {
		t.Error("expected error when opening a directory")
	}
	if fv.error != "Cannot view directory" {
		t.Errorf("unexpected error message: %q", fv.error)
	}
}

// ---------------------------------------------------------------------------
// SetFile - outside project root
// ---------------------------------------------------------------------------

func TestFileViewer_SetFile_OutsideRoot(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(80, 40)

	// /tmp is typically outside any project root
	tmpFile, err := os.CreateTemp("", "quorum-outside-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	err = fv.SetFile(tmpFile.Name())
	if err == nil {
		t.Error("expected error for file outside project root")
	}
}

// ---------------------------------------------------------------------------
// SetFile - binary content
// ---------------------------------------------------------------------------

func TestFileViewer_SetFile_Binary(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(80, 40)

	cwd, _ := os.Getwd()
	tmpFile := filepath.Join(cwd, "testdata_fileviewer_binary.bin")
	// Write some null bytes to trigger binary detection
	data := make([]byte, 100)
	data[0] = 0
	data[50] = 0
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(tmpFile) })

	err := fv.SetFile(tmpFile)
	if err != nil {
		t.Fatalf("SetFile should not error for binary file, got: %v", err)
	}
	if !fv.isBinary {
		t.Error("file with null bytes should be detected as binary")
	}
}

// ---------------------------------------------------------------------------
// Scroll methods
// ---------------------------------------------------------------------------

func TestFileViewer_Scroll(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(80, 20)

	cwd, _ := os.Getwd()
	tmpFile := filepath.Join(cwd, "testdata_fileviewer_scroll.txt")
	lines := strings.Repeat("line content here with some text\n", 200)
	if err := os.WriteFile(tmpFile, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(tmpFile) })

	if err := fv.SetFile(tmpFile); err != nil {
		t.Fatal(err)
	}

	// Vertical scroll
	fv.ScrollDown()
	fv.ScrollUp()
	fv.PageDown()
	fv.PageUp()
	fv.ScrollBottom()
	fv.ScrollTop()

	// Horizontal scroll
	fv.ScrollRight()
	fv.ScrollLeft()
	fv.ScrollEnd()
	fv.ScrollHome()

	// Should not panic with any scroll method
	if fv.horizontalOffset != 0 {
		t.Errorf("expected horizontalOffset=0 after ScrollHome, got %d", fv.horizontalOffset)
	}
}

func TestFileViewer_ScrollLeft_AtZero(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(80, 20)
	fv.horizontalOffset = 0
	fv.ScrollLeft() // should be no-op
	if fv.horizontalOffset != 0 {
		t.Error("scrollLeft at 0 should stay at 0")
	}
}

func TestFileViewer_ScrollRight_NarrowContent(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(200, 20)

	cwd, _ := os.Getwd()
	tmpFile := filepath.Join(cwd, "testdata_fileviewer_narrow.txt")
	if err := os.WriteFile(tmpFile, []byte("short\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(tmpFile) })

	_ = fv.SetFile(tmpFile)
	fv.ScrollRight() // maxScroll should be 0; no movement
}

// ---------------------------------------------------------------------------
// Render
// ---------------------------------------------------------------------------

func TestFileViewer_Render_NotVisible(t *testing.T) {
	fv := NewFileViewer()
	if fv.Render() != "" {
		t.Error("hidden viewer should render empty string")
	}
}

func TestFileViewer_Render_Visible(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(80, 30)
	fv.Show()

	cwd, _ := os.Getwd()
	tmpFile := filepath.Join(cwd, "testdata_fileviewer_render.go")
	if err := os.WriteFile(tmpFile, []byte("package main\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(tmpFile) })

	_ = fv.SetFile(tmpFile)
	rendered := fv.Render()
	if rendered == "" {
		t.Fatal("visible viewer should render something")
	}
	if !strings.Contains(rendered, filepath.Base(tmpFile)) {
		t.Error("render should contain the file name")
	}
}

func TestFileViewer_Render_WithError(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(80, 30)
	fv.Show()
	fv.error = "Some error"
	fv.isBinary = true

	rendered := fv.Render()
	if !strings.Contains(rendered, "Some error") {
		t.Error("render should show error message")
	}
}

func TestFileViewer_Render_WithTruncation(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(80, 30)
	fv.Show()

	cwd, _ := os.Getwd()
	tmpFile := filepath.Join(cwd, "testdata_fileviewer_truncated.txt")
	if err := os.WriteFile(tmpFile, []byte("hello\nworld\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(tmpFile) })

	_ = fv.SetFile(tmpFile)
	// Simulate truncation warning
	fv.error = "Showing first 10000 lines (file has more)"

	rendered := fv.Render()
	if !strings.Contains(rendered, "10000") {
		t.Error("render should show truncation warning")
	}
}

// ---------------------------------------------------------------------------
// getVisiblePortion
// ---------------------------------------------------------------------------

func TestGetVisiblePortion_NoOffset(t *testing.T) {
	result := getVisiblePortion("hello", 0, 100)
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestGetVisiblePortion_WithOffset(t *testing.T) {
	result := getVisiblePortion("hello world", 3, 5)
	// Should show portion from offset 3, width 5
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestGetVisiblePortion_OffsetBeyondLine(t *testing.T) {
	result := getVisiblePortion("hi", 100, 10)
	if result != "" {
		t.Errorf("expected empty for offset beyond line, got %q", result)
	}
}

func TestGetVisiblePortion_EmptyLine(t *testing.T) {
	result := getVisiblePortion("", 0, 10)
	if result != "" {
		t.Errorf("expected empty for empty line, got %q", result)
	}
}

func TestGetVisiblePortion_WidthExceedsTruncation(t *testing.T) {
	// Line is wider than display width, with offset > 0
	longLine := strings.Repeat("abcdefghij", 10) // 100 chars
	result := getVisiblePortion(longLine, 5, 20)
	if result == "" {
		t.Error("expected non-empty visible portion")
	}
}

// ---------------------------------------------------------------------------
// runeWidth
// ---------------------------------------------------------------------------

func TestRuneWidth(t *testing.T) {
	if w := runeWidth('A'); w != 1 {
		t.Errorf("expected 1, got %d", w)
	}
	if w := runeWidth(' '); w != 1 {
		t.Errorf("expected 1, got %d", w)
	}
}

// ---------------------------------------------------------------------------
// getSyntaxColor
// ---------------------------------------------------------------------------

func TestGetSyntaxColor_KnownExtensions(t *testing.T) {
	tests := []struct {
		ext string
	}{
		{".go"}, {".js"}, {".jsx"}, {".ts"}, {".tsx"}, {".py"},
		{".rs"}, {".rb"}, {".java"}, {".c"}, {".h"}, {".cpp"},
		{".hpp"}, {".cc"}, {".md"}, {".json"}, {".yaml"}, {".yml"},
		{".toml"}, {".sh"}, {".bash"}, {".sql"}, {".html"}, {".htm"},
		{".css"}, {".xml"},
	}
	for _, tc := range tests {
		color := getSyntaxColor(tc.ext)
		if color == "" {
			t.Errorf("getSyntaxColor(%q) returned empty", tc.ext)
		}
	}
}

func TestGetSyntaxColor_DefaultExtension(t *testing.T) {
	color := getSyntaxColor(".unknown")
	if color == "" {
		t.Error("default extension should return a color")
	}
}

// ---------------------------------------------------------------------------
// getFileIcon
// ---------------------------------------------------------------------------

func TestGetFileIcon(t *testing.T) {
	// Currently returns empty string
	icon := getFileIcon("main.go")
	if icon != "" {
		t.Errorf("getFileIcon currently returns empty, got %q", icon)
	}
}

// ---------------------------------------------------------------------------
// isBinaryContent
// ---------------------------------------------------------------------------

func TestIsBinaryContent_Empty(t *testing.T) {
	if isBinaryContent(nil) {
		t.Error("empty data should not be binary")
	}
	if isBinaryContent([]byte{}) {
		t.Error("zero-length data should not be binary")
	}
}

func TestIsBinaryContent_TextContent(t *testing.T) {
	text := []byte("Hello, world!\nThis is a text file.\n")
	if isBinaryContent(text) {
		t.Error("plain text should not be binary")
	}
}

func TestIsBinaryContent_NullByte(t *testing.T) {
	data := []byte("Hello\x00World")
	if !isBinaryContent(data) {
		t.Error("data with null byte should be binary")
	}
}

func TestIsBinaryContent_NonUTF8(t *testing.T) {
	// Invalid UTF-8 sequence
	data := []byte{0xff, 0xfe, 0x80, 0x81}
	if !isBinaryContent(data) {
		t.Error("invalid UTF-8 should be binary")
	}
}

func TestIsBinaryContent_ManyNonPrintable(t *testing.T) {
	data := make([]byte, 100)
	// Fill with control characters (non-printable, not whitespace)
	for i := range data {
		data[i] = 0x01 // SOH control character
	}
	if !isBinaryContent(data) {
		t.Error("data with >10% non-printable chars should be binary")
	}
}

func TestIsBinaryContent_FewNonPrintable(t *testing.T) {
	// 100 bytes, only 5 non-printable = 5% < 10%
	data := make([]byte, 100)
	for i := range data {
		data[i] = 'a'
	}
	data[0] = 0x01
	data[20] = 0x02
	data[40] = 0x03
	data[60] = 0x04
	data[80] = 0x05
	if isBinaryContent(data) {
		t.Error("data with <10% non-printable chars should not be binary")
	}
}

func TestIsBinaryContent_LargeTextFile(t *testing.T) {
	// Larger than 8KB sample
	data := make([]byte, 16384)
	for i := range data {
		data[i] = 'x'
	}
	if isBinaryContent(data) {
		t.Error("large text file should not be binary")
	}
}

// ---------------------------------------------------------------------------
// formatSize (covered in context_preview but we test it from here too)
// ---------------------------------------------------------------------------

func TestFormatSize(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0b"},
		{512, "512b"},
		{1024, "1.0kb"},
		{1536, "1.5kb"},
		{1048576, "1.0mb"},
	}
	for _, tc := range tests {
		got := formatSize(tc.input)
		if got != tc.expected {
			t.Errorf("formatSize(%d) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// updateViewport - edge cases
// ---------------------------------------------------------------------------

func TestFileViewer_UpdateViewport_NotReady(t *testing.T) {
	fv := NewFileViewer()
	// Should not panic when not ready
	fv.updateViewport()
}

func TestFileViewer_UpdateViewport_NoLines(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(80, 30)
	fv.lines = nil
	fv.updateViewport() // should exit early
}

func TestFileViewer_UpdateViewport_WithTabs(t *testing.T) {
	fv := NewFileViewer()
	fv.SetSize(80, 30)
	fv.lines = []string{"\tindented line", "normal line"}
	fv.lineCount = 2
	fv.fileName = "test.go"
	fv.updateViewport()
	// Just verify no panic
}
