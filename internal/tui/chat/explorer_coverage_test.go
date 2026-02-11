package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// NewExplorerPanel
// ---------------------------------------------------------------------------

func TestNewExplorerPanel(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()

	if p == nil {
		t.Fatal("NewExplorerPanel returned nil")
	}
	cwd, _ := os.Getwd()
	if p.root != cwd {
		t.Errorf("root should be cwd %q, got %q", cwd, p.root)
	}
	if p.initialRoot != cwd {
		t.Errorf("initialRoot should be cwd %q, got %q", cwd, p.initialRoot)
	}
	if !p.showHidden {
		t.Error("showHidden should default to true")
	}
}

// ---------------------------------------------------------------------------
// SetSize / Width / Height
// ---------------------------------------------------------------------------

func TestExplorerPanel_SetSize(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()

	p.SetSize(60, 30)
	if p.Width() != 60 {
		t.Errorf("Width() should be 60, got %d", p.Width())
	}
	if p.Height() != 30 {
		t.Errorf("Height() should be 30, got %d", p.Height())
	}
	if !p.ready {
		t.Error("Panel should be ready after SetSize")
	}
}

func TestExplorerPanel_SetSize_Small(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()

	// Very small height should clamp viewport to min 3
	p.SetSize(40, 5)
	if p.Width() != 40 {
		t.Errorf("Width() should be 40, got %d", p.Width())
	}
}

func TestExplorerPanel_SetSize_UpdateExisting(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()

	p.SetSize(60, 30) // first call
	p.SetSize(80, 40) // second call should update, not reinitialize
	if p.Width() != 80 {
		t.Errorf("Width() should be 80, got %d", p.Width())
	}
}

// ---------------------------------------------------------------------------
// SetFocused / IsFocused
// ---------------------------------------------------------------------------

func TestExplorerPanel_Focus(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()

	if p.IsFocused() {
		t.Error("Should not be focused initially")
	}
	p.SetFocused(true)
	if !p.IsFocused() {
		t.Error("Should be focused after SetFocused(true)")
	}
	p.SetFocused(false)
	if p.IsFocused() {
		t.Error("Should not be focused after SetFocused(false)")
	}
}

// ---------------------------------------------------------------------------
// Render
// ---------------------------------------------------------------------------

func TestExplorerPanel_Render_NotReady(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()

	// Before SetSize, panel is not ready
	result := p.Render()
	if result != "" {
		t.Error("Not-ready panel should render empty string")
	}
}

func TestExplorerPanel_Render_Ready(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()

	p.SetSize(60, 30)
	result := p.Render()
	if result == "" {
		t.Error("Ready panel should render non-empty string")
	}
	if !strings.Contains(result, "Explorer") {
		t.Error("Should contain 'Explorer' header")
	}
}

func TestExplorerPanel_Render_Focused(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()

	p.SetSize(60, 30)
	p.SetFocused(true)
	result := p.Render()
	if result == "" {
		t.Error("Focused panel should render non-empty")
	}
}

// ---------------------------------------------------------------------------
// Refresh / Count
// ---------------------------------------------------------------------------

func TestExplorerPanel_Refresh(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()

	p.SetSize(60, 30)
	err := p.Refresh()
	if err != nil {
		t.Errorf("Refresh should not error: %v", err)
	}
	// After refresh in cwd, should have entries (unless cwd is empty)
	count := p.Count()
	if count < 0 {
		t.Errorf("Count should be >= 0, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// SetRoot
// ---------------------------------------------------------------------------

func TestExplorerPanel_SetRoot_ValidSubdir(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()
	p.SetSize(60, 30)

	// Create a temp dir within the initial root to test SetRoot
	tmpDir := t.TempDir()
	// We need to set initial root to contain tmpDir
	p.mu.Lock()
	p.initialRoot = filepath.Dir(tmpDir)
	p.root = filepath.Dir(tmpDir)
	p.mu.Unlock()

	err := p.SetRoot(tmpDir)
	if err != nil {
		t.Errorf("SetRoot to valid subdir should not error: %v", err)
	}
}

func TestExplorerPanel_SetRoot_AboveInitialRoot(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()
	p.SetSize(60, 30)

	// Try to set root above initial root
	err := p.SetRoot("/")
	if err == nil {
		t.Error("SetRoot above initial root should return error")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot navigate above") {
		t.Errorf("Error should mention boundary, got: %v", err)
	}
}

func TestExplorerPanel_SetRoot_NotADirectory(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()
	p.SetSize(60, 30)

	// Create a temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile.txt")
	os.WriteFile(tmpFile, []byte("hello"), 0644)

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = tmpDir
	p.mu.Unlock()

	err := p.SetRoot(tmpFile)
	if err == nil {
		t.Error("SetRoot to a file should return error")
	}
}

// ---------------------------------------------------------------------------
// Navigation: MoveUp, MoveDown, GoUp, Enter, Toggle
// ---------------------------------------------------------------------------

func TestExplorerPanel_Navigation(t *testing.T) {
	// Create a temp directory structure
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("b"), 0644)

	p := NewExplorerPanel()
	defer p.Close()

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = tmpDir
	p.mu.Unlock()

	p.SetSize(60, 30)
	p.Refresh()

	count := p.Count()
	if count < 2 {
		t.Fatalf("Expected at least 2 entries (subdir + file), got %d", count)
	}

	// Test MoveDown
	p.MoveDown()
	selected := p.GetSelectedPath()
	if selected == "" {
		t.Error("After MoveDown, should have a selection")
	}

	// Test MoveUp
	p.MoveUp()
	first := p.GetSelectedPath()
	if first == "" {
		t.Error("After MoveUp, should have a selection")
	}

	// MoveUp at top should stay at position 0
	p.MoveUp()
	stillFirst := p.GetSelectedPath()
	if stillFirst != first {
		t.Error("MoveUp at top should not change selection")
	}
}

func TestExplorerPanel_GoUp(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subDir, 0755)

	p := NewExplorerPanel()
	defer p.Close()

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = subDir
	p.mu.Unlock()
	p.SetSize(60, 30)

	// GoUp from subdir should go to tmpDir
	p.GoUp()
	p.mu.Lock()
	root := p.root
	p.mu.Unlock()
	if root != tmpDir {
		t.Errorf("After GoUp, root should be %q, got %q", tmpDir, root)
	}

	// GoUp from initial root should stay
	p.GoUp()
	p.mu.Lock()
	root = p.root
	p.mu.Unlock()
	if root != tmpDir {
		t.Error("GoUp at initial root should not change root")
	}
}

func TestExplorerPanel_Enter_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "inner.txt"), []byte("x"), 0644)

	p := NewExplorerPanel()
	defer p.Close()

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = tmpDir
	p.mu.Unlock()
	p.SetSize(60, 30)
	p.Refresh()

	// First entry should be subdir (directories first)
	result := p.Enter()
	if result != "" {
		t.Error("Entering a directory should return empty string")
	}
	p.mu.Lock()
	root := p.root
	p.mu.Unlock()
	if root != subDir {
		t.Errorf("After entering subdir, root should be %q, got %q", subDir, root)
	}
}

func TestExplorerPanel_Enter_File(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("x"), 0644)

	p := NewExplorerPanel()
	defer p.Close()

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = tmpDir
	p.mu.Unlock()
	p.SetSize(60, 30)
	p.Refresh()

	// Only entry should be file.txt
	result := p.Enter()
	expected := filepath.Join(tmpDir, "file.txt")
	if result != expected {
		t.Errorf("Entering a file should return path %q, got %q", expected, result)
	}
}

func TestExplorerPanel_Enter_EmptyList(t *testing.T) {
	tmpDir := t.TempDir()

	p := NewExplorerPanel()
	defer p.Close()

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = tmpDir
	p.mu.Unlock()
	p.SetSize(60, 30)
	p.Refresh()

	result := p.Enter()
	if result != "" {
		t.Error("Enter on empty list should return empty string")
	}
}

func TestExplorerPanel_Toggle(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "inner.txt"), []byte("x"), 0644)

	p := NewExplorerPanel()
	defer p.Close()

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = tmpDir
	p.mu.Unlock()
	p.SetSize(60, 30)
	p.Refresh()

	beforeCount := p.Count()

	// Toggle to expand directory
	p.Toggle()
	expandedCount := p.Count()
	if expandedCount <= beforeCount {
		t.Error("After expanding directory, count should increase")
	}

	// Toggle again to collapse
	p.Toggle()
	collapsedCount := p.Count()
	if collapsedCount != beforeCount {
		t.Errorf("After collapsing, count should return to %d, got %d", beforeCount, collapsedCount)
	}
}

func TestExplorerPanel_Toggle_OnFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("x"), 0644)

	p := NewExplorerPanel()
	defer p.Close()

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = tmpDir
	p.mu.Unlock()
	p.SetSize(60, 30)
	p.Refresh()

	beforeCount := p.Count()
	p.Toggle() // on a file, should do nothing
	afterCount := p.Count()
	if afterCount != beforeCount {
		t.Error("Toggle on a file should not change count")
	}
}

// ---------------------------------------------------------------------------
// ToggleHidden
// ---------------------------------------------------------------------------

func TestExplorerPanel_ToggleHidden(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "visible.txt"), []byte("x"), 0644)

	p := NewExplorerPanel()
	defer p.Close()

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = tmpDir
	p.mu.Unlock()
	p.SetSize(60, 30)
	p.Refresh()

	withHidden := p.Count()

	// Hide hidden files
	p.ToggleHidden()
	withoutHidden := p.Count()
	if withoutHidden >= withHidden {
		t.Error("Hiding hidden files should reduce count")
	}

	// Show hidden files again
	p.ToggleHidden()
	againWithHidden := p.Count()
	if againWithHidden != withHidden {
		t.Errorf("Showing hidden files should restore count to %d, got %d", withHidden, againWithHidden)
	}
}

// ---------------------------------------------------------------------------
// GetSelectedPath / GetSelectedRelativePath / GetSelectedEntry
// ---------------------------------------------------------------------------

func TestExplorerPanel_GetSelectedPath_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	p := NewExplorerPanel()
	defer p.Close()

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = tmpDir
	p.mu.Unlock()
	p.SetSize(60, 30)
	p.Refresh()

	path := p.GetSelectedPath()
	if path != "" {
		t.Error("Empty directory should return empty selected path")
	}
}

func TestExplorerPanel_GetSelectedRelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("x"), 0644)

	p := NewExplorerPanel()
	defer p.Close()

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = tmpDir
	p.mu.Unlock()
	p.SetSize(60, 30)
	p.Refresh()

	relPath := p.GetSelectedRelativePath()
	if relPath != "file.txt" {
		t.Errorf("Relative path should be 'file.txt', got %q", relPath)
	}
}

func TestExplorerPanel_GetSelectedRelativePath_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	p := NewExplorerPanel()
	defer p.Close()

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = tmpDir
	p.mu.Unlock()
	p.SetSize(60, 30)
	p.Refresh()

	relPath := p.GetSelectedRelativePath()
	if relPath != "" {
		t.Error("Empty dir should return empty relative path")
	}
}

func TestExplorerPanel_GetSelectedEntry(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("x"), 0644)

	p := NewExplorerPanel()
	defer p.Close()

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = tmpDir
	p.mu.Unlock()
	p.SetSize(60, 30)
	p.Refresh()

	entry := p.GetSelectedEntry()
	if entry == nil {
		t.Fatal("GetSelectedEntry should return an entry")
	}
	if entry.Name != "file.txt" {
		t.Errorf("Entry name should be 'file.txt', got %q", entry.Name)
	}
}

func TestExplorerPanel_GetSelectedEntry_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	p := NewExplorerPanel()
	defer p.Close()

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = tmpDir
	p.mu.Unlock()
	p.SetSize(60, 30)
	p.Refresh()

	entry := p.GetSelectedEntry()
	if entry != nil {
		t.Error("Empty dir should return nil entry")
	}
}

// ---------------------------------------------------------------------------
// OnChange
// ---------------------------------------------------------------------------

func TestExplorerPanel_OnChange(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()

	ch := p.OnChange()
	if ch == nil {
		t.Error("OnChange should return a non-nil channel")
	}
}

// ---------------------------------------------------------------------------
// formatEntry (covers various file type icons)
// ---------------------------------------------------------------------------

func TestExplorerPanel_FormatEntry_FileTypes(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()
	p.mu.Lock()
	p.width = 80
	p.mu.Unlock()

	entries := []struct {
		name     string
		fileType FileType
		expanded bool
		isHidden bool
	}{
		{"main.go", FileTypeFile, false, false},
		{"app.js", FileTypeFile, false, false},
		{"app.jsx", FileTypeFile, false, false},
		{"index.ts", FileTypeFile, false, false},
		{"index.tsx", FileTypeFile, false, false},
		{"script.py", FileTypeFile, false, false},
		{"lib.rs", FileTypeFile, false, false},
		{"README.md", FileTypeFile, false, false},
		{"config.json", FileTypeFile, false, false},
		{"config.yaml", FileTypeFile, false, false},
		{"config.yml", FileTypeFile, false, false},
		{"config.toml", FileTypeFile, false, false},
		{"Dockerfile", FileTypeFile, false, false},
		{"docker-compose.yml", FileTypeFile, false, false},
		{".gitignore", FileTypeFile, false, true},
		{".gitconfig", FileTypeFile, false, true},
		{"run.sh", FileTypeFile, false, false},
		{"run.bash", FileTypeFile, false, false},
		{"notes.txt", FileTypeFile, false, false},
		{"data.bin", FileTypeFile, false, false},
		{"link", FileTypeSymlink, false, false},
		{"src", FileTypeDir, false, false},
		{"src_open", FileTypeDir, true, false},
	}

	for _, e := range entries {
		entry := &FileEntry{
			Name:     e.name,
			Type:     e.fileType,
			Expanded: e.expanded,
			IsHidden: e.isHidden,
			Level:    0,
		}
		result := p.formatEntry(entry, false)
		if result == "" {
			t.Errorf("formatEntry(%q) should produce non-empty output", e.name)
		}

		// Test with selected=true
		selectedResult := p.formatEntry(entry, true)
		if !strings.Contains(selectedResult, "\u203a") { // "›" cursor
			t.Errorf("Selected entry %q should contain cursor indicator", e.name)
		}
	}
}

func TestExplorerPanel_FormatEntry_LongName(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()
	p.mu.Lock()
	p.width = 30
	p.mu.Unlock()

	entry := &FileEntry{
		Name:  "this_is_a_very_long_filename_that_should_be_truncated.go",
		Type:  FileTypeFile,
		Level: 0,
	}
	result := p.formatEntry(entry, false)
	if result == "" {
		t.Error("Long filename should still produce output")
	}
}

func TestExplorerPanel_FormatEntry_DeepIndent(t *testing.T) {
	p := NewExplorerPanel()
	defer p.Close()
	p.mu.Lock()
	p.width = 80
	p.mu.Unlock()

	entry := &FileEntry{
		Name:  "deep.go",
		Type:  FileTypeFile,
		Level: 5,
	}
	result := p.formatEntry(entry, false)
	if !strings.Contains(result, "deep.go") {
		t.Error("Deep indented entry should still show filename")
	}
}

// ---------------------------------------------------------------------------
// Render with path display
// ---------------------------------------------------------------------------

func TestExplorerPanel_Render_AtRoot(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "f.txt"), []byte("x"), 0644)

	p := NewExplorerPanel()
	defer p.Close()

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = tmpDir
	p.mu.Unlock()
	p.SetSize(60, 30)
	p.Refresh()

	result := p.Render()
	if !strings.Contains(result, ".") {
		t.Error("At project root, should display '.'")
	}
}

func TestExplorerPanel_Render_InSubdir(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	os.Mkdir(subDir, 0755)

	p := NewExplorerPanel()
	defer p.Close()

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = subDir
	p.mu.Unlock()
	p.SetSize(60, 30)
	p.Refresh()

	result := p.Render()
	if !strings.Contains(result, "sub") {
		t.Error("In subdir, should show relative path containing 'sub'")
	}
}

func TestExplorerPanel_Render_LongPath(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a deeply nested path
	deep := tmpDir
	for i := 0; i < 10; i++ {
		deep = filepath.Join(deep, "very_long_directory_name")
	}
	os.MkdirAll(deep, 0755)

	p := NewExplorerPanel()
	defer p.Close()

	p.mu.Lock()
	p.initialRoot = tmpDir
	p.root = deep
	p.mu.Unlock()
	p.SetSize(40, 30) // narrow width to trigger path truncation
	p.Refresh()

	result := p.Render()
	if result == "" {
		t.Error("Long path should still render")
	}
}
