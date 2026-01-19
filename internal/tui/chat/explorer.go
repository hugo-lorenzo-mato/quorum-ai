package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
)

// FileType represents the type of file system entry
type FileType int

const (
	FileTypeFile FileType = iota
	FileTypeDir
	FileTypeSymlink
)

// FileEntry represents a file system entry
type FileEntry struct {
	Name     string
	Path     string
	Type     FileType
	Size     int64
	IsHidden bool
	Children []*FileEntry
	Expanded bool
	Level    int
}

// ExplorerPanel manages the directory explorer display
type ExplorerPanel struct {
	mu          sync.Mutex
	root        string
	initialRoot string // The initial root directory - cannot navigate above this
	entries     []*FileEntry
	flatList    []*FileEntry // Flattened list for display
	cursor      int
	viewport    viewport.Model
	width       int
	height      int
	ready       bool
	showHidden  bool
	focused     bool // Whether the panel has focus

	// File watcher
	watcher       *fsnotify.Watcher
	watchedDirs   map[string]bool // Track watched directories
	onChange      chan struct{}   // Notification channel for changes
	stopWatcher   chan struct{}   // Signal to stop watcher goroutine
	debounceTimer *time.Timer     // Debounce rapid file changes
}

// NewExplorerPanel creates a new explorer panel
func NewExplorerPanel() *ExplorerPanel {
	cwd, _ := os.Getwd()
	p := &ExplorerPanel{
		root:        cwd,
		initialRoot: cwd, // Store the initial root - this is the boundary
		entries:     make([]*FileEntry, 0),
		flatList:    make([]*FileEntry, 0),
		showHidden:  true, // Show hidden files by default
		watchedDirs: make(map[string]bool),
		onChange:    make(chan struct{}, 1), // Buffered to avoid blocking
		stopWatcher: make(chan struct{}),
	}

	// Start file watcher
	p.startWatcher()

	return p
}

// startWatcher initializes and starts the file system watcher
func (p *ExplorerPanel) startWatcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return // Silently fail - watcher is optional
	}
	p.watcher = watcher

	// Watch the initial root
	p.addWatch(p.initialRoot)

	// Start the watcher goroutine
	go p.watchLoop()
}

// watchLoop handles file system events
func (p *ExplorerPanel) watchLoop() {
	if p.watcher == nil {
		return
	}

	for {
		select {
		case <-p.stopWatcher:
			return
		case event, ok := <-p.watcher.Events:
			if !ok {
				return
			}
			// Handle create, remove, rename events
			if event.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
				p.scheduleRefresh()

				// If a new directory was created, watch it too
				if event.Op&fsnotify.Create != 0 {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						p.addWatch(event.Name)
					}
				}
			}
		case _, ok := <-p.watcher.Errors:
			if !ok {
				return
			}
			// Ignore errors silently
		}
	}
}

// scheduleRefresh debounces refresh calls to avoid rapid updates
func (p *ExplorerPanel) scheduleRefresh() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Cancel previous timer if exists
	if p.debounceTimer != nil {
		p.debounceTimer.Stop()
	}

	// Schedule refresh after 100ms debounce
	p.debounceTimer = time.AfterFunc(100*time.Millisecond, func() {
		// Send notification (non-blocking)
		select {
		case p.onChange <- struct{}{}:
		default:
		}
	})
}

// addWatch adds a directory to the watcher
func (p *ExplorerPanel) addWatch(path string) {
	if p.watcher == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.watchedDirs[path] {
		return // Already watching
	}

	if err := p.watcher.Add(path); err == nil {
		p.watchedDirs[path] = true
	}
}

// OnChange returns the channel that signals file system changes
func (p *ExplorerPanel) OnChange() <-chan struct{} {
	return p.onChange
}

// Close stops the watcher and cleans up resources
func (p *ExplorerPanel) Close() {
	close(p.stopWatcher)
	if p.watcher != nil {
		p.watcher.Close()
	}
	if p.debounceTimer != nil {
		p.debounceTimer.Stop()
	}
}

// SetRoot sets the root directory (only within initialRoot boundaries)
func (p *ExplorerPanel) SetRoot(path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// Check if the new path is within the initial root boundaries
	if !strings.HasPrefix(absPath, p.initialRoot) {
		return fmt.Errorf("cannot navigate above project root: %s", p.initialRoot)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", absPath)
	}

	p.root = absPath
	p.cursor = 0
	return p.refresh()
}

// Refresh reloads the directory contents
func (p *ExplorerPanel) Refresh() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.refresh()
}

// refresh internal (must be called with lock held)
func (p *ExplorerPanel) refresh() error {
	entries, err := p.readDir(p.root, 0)
	if err != nil {
		return err
	}
	p.entries = entries
	p.rebuildFlatList()
	p.updateContent()
	return nil
}

// readDir reads a directory and returns file entries
func (p *ExplorerPanel) readDir(path string, level int) ([]*FileEntry, error) {
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var entries []*FileEntry
	var dirs, files []*FileEntry

	for _, de := range dirEntries {
		name := de.Name()
		isHidden := strings.HasPrefix(name, ".")

		// Skip hidden files unless showHidden is true
		if isHidden && !p.showHidden {
			continue
		}

		fullPath := filepath.Join(path, name)
		info, err := de.Info()
		if err != nil {
			continue
		}

		entry := &FileEntry{
			Name:     name,
			Path:     fullPath,
			IsHidden: isHidden,
			Level:    level,
		}

		if de.IsDir() {
			entry.Type = FileTypeDir
		} else if info.Mode()&os.ModeSymlink != 0 {
			entry.Type = FileTypeSymlink
			entry.Size = info.Size()
		} else {
			entry.Type = FileTypeFile
			entry.Size = info.Size()
		}

		if entry.Type == FileTypeDir {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	// Sort directories and files alphabetically
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	// Directories first, then files
	entries = append(entries, dirs...)
	entries = append(entries, files...)

	return entries, nil
}

// rebuildFlatList rebuilds the flattened list for display
func (p *ExplorerPanel) rebuildFlatList() {
	p.flatList = p.flatList[:0]
	p.flattenEntries(p.entries)
}

// flattenEntries recursively flattens entries
func (p *ExplorerPanel) flattenEntries(entries []*FileEntry) {
	for _, entry := range entries {
		p.flatList = append(p.flatList, entry)
		if entry.Type == FileTypeDir && entry.Expanded && len(entry.Children) > 0 {
			p.flattenEntries(entry.Children)
		}
	}
}

// Toggle expands or collapses the current directory
func (p *ExplorerPanel) Toggle() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cursor < 0 || p.cursor >= len(p.flatList) {
		return
	}

	entry := p.flatList[p.cursor]
	if entry.Type != FileTypeDir {
		return
	}

	if entry.Expanded {
		entry.Expanded = false
		entry.Children = nil
	} else {
		children, err := p.readDir(entry.Path, entry.Level+1)
		if err == nil {
			entry.Children = children
			entry.Expanded = true
		}
	}

	p.rebuildFlatList()
	p.updateContent()
}

// MoveUp moves the cursor up
func (p *ExplorerPanel) MoveUp() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cursor > 0 {
		p.cursor--
		p.updateContent()
		p.ensureCursorVisible()
	}
}

// MoveDown moves the cursor down
func (p *ExplorerPanel) MoveDown() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cursor < len(p.flatList)-1 {
		p.cursor++
		p.updateContent()
		p.ensureCursorVisible()
	}
}

// ensureCursorVisible scrolls viewport to keep cursor visible
func (p *ExplorerPanel) ensureCursorVisible() {
	if !p.ready {
		return
	}
	// Each line is 1 row high
	cursorY := p.cursor
	viewStart := p.viewport.YOffset
	viewEnd := viewStart + p.viewport.Height

	if cursorY < viewStart {
		p.viewport.SetYOffset(cursorY)
	} else if cursorY >= viewEnd {
		p.viewport.SetYOffset(cursorY - p.viewport.Height + 1)
	}
}

// GetSelectedPath returns the currently selected path
func (p *ExplorerPanel) GetSelectedPath() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cursor < 0 || p.cursor >= len(p.flatList) {
		return ""
	}
	return p.flatList[p.cursor].Path
}

// GetSelectedRelativePath returns the selected path relative to the project root
func (p *ExplorerPanel) GetSelectedRelativePath() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cursor < 0 || p.cursor >= len(p.flatList) {
		return ""
	}

	absPath := p.flatList[p.cursor].Path
	relPath, err := filepath.Rel(p.initialRoot, absPath)
	if err != nil {
		return absPath
	}
	return relPath
}

// GetSelectedEntry returns the currently selected entry
func (p *ExplorerPanel) GetSelectedEntry() *FileEntry {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cursor < 0 || p.cursor >= len(p.flatList) {
		return nil
	}
	return p.flatList[p.cursor]
}

// ToggleHidden toggles showing hidden files
func (p *ExplorerPanel) ToggleHidden() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.showHidden = !p.showHidden
	_ = p.refresh()
}

// GoUp navigates to parent directory (but not above initialRoot)
func (p *ExplorerPanel) GoUp() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Don't allow going above the initial root
	if p.root == p.initialRoot {
		return
	}

	parent := filepath.Dir(p.root)
	// Extra safety check: ensure parent is still within boundaries
	if !strings.HasPrefix(parent, p.initialRoot) {
		return
	}

	if parent != p.root {
		p.root = parent
		p.cursor = 0
		_ = p.refresh()
	}
}

// Enter enters a directory or returns the file path
func (p *ExplorerPanel) Enter() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cursor < 0 || p.cursor >= len(p.flatList) {
		return ""
	}

	entry := p.flatList[p.cursor]
	if entry.Type == FileTypeDir {
		p.root = entry.Path
		p.cursor = 0
		_ = p.refresh()
		return ""
	}
	return entry.Path
}

// SetSize updates the panel dimensions
func (p *ExplorerPanel) SetSize(width, height int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.width = width
	p.height = height

	// Viewport height calculation:
	// Box content height = height - 2 (borders add 2 outside)
	// Fixed content inside box: header(1) + path(1) + separator(1) + footer(1) = 4 lines
	// Viewport = (height - 2) - 4 = height - 6
	viewportHeight := height - 6
	if viewportHeight < 3 {
		viewportHeight = 3
	}

	if !p.ready {
		p.viewport = viewport.New(width-4, viewportHeight)
		p.ready = true
		_ = p.refresh()
	} else {
		p.viewport.Width = width - 4
		p.viewport.Height = viewportHeight
	}
	p.updateContent()
}

// Width returns the current width of the panel
func (p *ExplorerPanel) Width() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.width
}

// updateContent refreshes the viewport content (must be called with lock held)
func (p *ExplorerPanel) updateContent() {
	if !p.ready {
		return
	}

	var sb strings.Builder
	for i, entry := range p.flatList {
		line := p.formatEntry(entry, i == p.cursor)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	p.viewport.SetContent(sb.String())
}

// formatEntry formats a single entry for display
func (p *ExplorerPanel) formatEntry(entry *FileEntry, selected bool) string {
	// Indentation
	indent := strings.Repeat("  ", entry.Level)

	// Icon and style based on type
	var icon string
	var nameStyle lipgloss.Style

	switch entry.Type {
	case FileTypeDir:
		if entry.Expanded {
			icon = explorerIconFolderOpen
		} else {
			icon = explorerIconFolder
		}
		nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3b82f6")).Bold(true) // blue
	case FileTypeSymlink:
		icon = "⤳"
		nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#06b6d4")) // cyan
	default:
		// File icon based on extension
		ext := strings.ToLower(filepath.Ext(entry.Name))
		baseName := strings.ToLower(entry.Name)

		switch {
		case ext == ".go":
			icon = explorerIconGo
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00add8")) // go blue
		case ext == ".js" || ext == ".jsx":
			icon = explorerIconJs
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f7df1e")) // js yellow
		case ext == ".ts" || ext == ".tsx":
			icon = explorerIconTs
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3178c6")) // ts blue
		case ext == ".py":
			icon = explorerIconPy
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3776ab")) // python blue
		case ext == ".rs":
			icon = explorerIconRust
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#dea584")) // rust orange
		case ext == ".md":
			icon = explorerIconMd
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ca3af")) // gray
		case ext == ".json":
			icon = explorerIconJson
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#a855f7")) // purple
		case ext == ".yaml" || ext == ".yml" || ext == ".toml":
			icon = explorerIconYaml
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#a855f7")) // purple
		case baseName == "dockerfile" || strings.HasPrefix(baseName, "docker-compose"):
			icon = explorerIconDocker
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#2496ed")) // docker blue
		case baseName == ".gitignore" || baseName == ".gitconfig":
			icon = explorerIconGit
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f05032")) // git orange
		case ext == ".sh" || ext == ".bash":
			icon = ""
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e")) // green
		case ext == ".txt":
			icon = ""
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ca3af")) // gray
		default:
			icon = explorerIconFile
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#e5e7eb")) // light gray
		}
	}

	// Hidden files are dimmed
	if entry.IsHidden {
		nameStyle = nameStyle.Faint(true)
	}

	// Cursor indicator
	cursor := " "
	if selected {
		cursor = "›"
		nameStyle = nameStyle.Reverse(true)
	}

	// Truncate name if too long (use lipgloss.Width for Unicode safety)
	maxNameLen := p.width - entry.Level*2 - 6
	if maxNameLen < 10 {
		maxNameLen = 10
	}
	name := entry.Name
	if lipgloss.Width(name) > maxNameLen {
		// Safe truncation for Unicode characters
		truncated := ""
		for _, r := range name {
			if lipgloss.Width(truncated+string(r)+"...") > maxNameLen {
				break
			}
			truncated += string(r)
		}
		name = truncated + "..."
	}

	return fmt.Sprintf("%s%s%s %s", cursor, indent, icon, nameStyle.Render(name))
}

// Nerd Font icons for explorer
const (
	explorerIconFolder     = "" // nf-fa-folder
	explorerIconFolderOpen = "" // nf-fa-folder_open
	explorerIconFile       = "" // nf-fa-file
	explorerIconGo         = "" // nf-seti-go
	explorerIconJs         = "" // nf-seti-javascript
	explorerIconTs         = "" // nf-seti-typescript
	explorerIconPy         = "" // nf-seti-python
	explorerIconRust       = "" // nf-dev-rust
	explorerIconJson       = "" // nf-seti-json
	explorerIconYaml       = "" // nf-seti-yml
	explorerIconMd         = "" // nf-seti-markdown
	explorerIconGit        = "" // nf-dev-git
	explorerIconDocker     = "" // nf-linux-docker
	explorerIconTree       = "" // nf-fa-sitemap
)

// Render renders the explorer panel
func (p *ExplorerPanel) Render() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.ready {
		return ""
	}

	// Header with icon
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a855f7")).
		Bold(true)

	// Display path relative to initial root for cleaner look
	displayPath := p.root
	if p.root == p.initialRoot {
		displayPath = "." // Show "." when at project root
	} else if strings.HasPrefix(p.root, p.initialRoot) {
		// Show relative path from project root
		relPath, err := filepath.Rel(p.initialRoot, p.root)
		if err == nil {
			displayPath = "./" + relPath
		}
	}

	// Truncate path if too long
	maxPathLen := p.width - 10
	if len(displayPath) > maxPathLen {
		displayPath = "..." + displayPath[len(displayPath)-maxPathLen+3:]
	}

	header := headerStyle.Render(explorerIconTree + " Explorer")

	// Help text (keyboard hint)
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true)
	help := helpStyle.Render("^E")

	// Path line with subtle styling
	pathStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ca3af")).
		Italic(true)
	pathLine := pathStyle.Render(displayPath)

	// Header line
	headerWidth := p.width - 4
	gap := headerWidth - lipgloss.Width(header) - lipgloss.Width(help)
	if gap < 1 {
		gap = 1
	}
	headerLine := header + strings.Repeat(" ", gap) + help

	// Content
	content := p.viewport.View()

	// Stats with subtle styling
	statsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	stats := statsStyle.Render(fmt.Sprintf("%d items", len(p.flatList)))

	// Combine
	var sb strings.Builder
	sb.WriteString(headerLine)
	sb.WriteString("\n")
	sb.WriteString(pathLine)
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#374151")).Render(strings.Repeat("─", p.width-4)))
	sb.WriteString("\n")
	sb.WriteString(content)
	sb.WriteString("\n")
	sb.WriteString(stats)

	// Box style with rounded borders - highlighted when focused
	borderColor := lipgloss.Color("#374151") // Default border
	if p.focused {
		borderColor = lipgloss.Color("#7C3AED") // Purple when focused
	}

	// lipgloss Width/Height set CONTENT size, borders are added OUTSIDE.
	// Formula: Width(X-2) + borders(2) = total X
	// DO NOT use MaxWidth/MaxHeight - they truncate AFTER borders, cutting them off.
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(p.width - 2).
		Height(p.height - 2)

	return boxStyle.Render(sb.String())
}

// Count returns the number of visible entries
func (p *ExplorerPanel) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.flatList)
}

// SetFocused sets whether the panel has focus
func (p *ExplorerPanel) SetFocused(focused bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.focused = focused
}

// IsFocused returns whether the panel has focus
func (p *ExplorerPanel) IsFocused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.focused
}
