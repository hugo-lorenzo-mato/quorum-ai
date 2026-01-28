package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileEntry represents a file or directory in the file browser.
type FileEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	IsDir       bool   `json:"is_dir"`
	Size        int64  `json:"size,omitempty"`
	ModTime     string `json:"mod_time,omitempty"`
	Extension   string `json:"extension,omitempty"`
	Permissions string `json:"permissions,omitempty"`
}

// FileContentResponse represents file content response.
type FileContentResponse struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Size     int64  `json:"size"`
	Language string `json:"language,omitempty"`
	Binary   bool   `json:"binary"`
}

// handleListFiles lists files and directories at a given path.
func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	requestedPath := r.URL.Query().Get("path")
	if requestedPath == "" {
		requestedPath = "."
	}

	// Resolve and validate path
	absPath, err := s.resolvePath(requestedPath)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid path")
		return
	}

	// Check if path exists and is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty list for non-existent directories
			respondJSON(w, http.StatusOK, []FileEntry{})
			return
		}
		s.logger.Error("failed to stat path", "path", absPath, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to access path")
		return
	}

	if !info.IsDir() {
		respondError(w, http.StatusBadRequest, "path is not a directory")
		return
	}

	// Read directory contents
	entries, err := os.ReadDir(absPath)
	if err != nil {
		s.logger.Error("failed to read directory", "path", absPath, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to read directory")
		return
	}

	// Convert to FileEntry list
	files := make([]FileEntry, 0, len(entries))
	for _, entry := range entries {
		// Skip hidden files (starting with .)
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		relPath := filepath.Join(requestedPath, entry.Name())
		if requestedPath == "." {
			relPath = entry.Name()
		}

		file := FileEntry{
			Name:        entry.Name(),
			Path:        relPath,
			IsDir:       entry.IsDir(),
			Size:        info.Size(),
			ModTime:     info.ModTime().Format("2006-01-02 15:04:05"),
			Permissions: info.Mode().Perm().String(),
		}

		if !entry.IsDir() {
			file.Extension = strings.TrimPrefix(filepath.Ext(entry.Name()), ".")
		}

		files = append(files, file)
	}

	// Sort: directories first, then alphabetically
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	respondJSON(w, http.StatusOK, files)
}

// handleGetFileContent returns the content of a file.
func (s *Server) handleGetFileContent(w http.ResponseWriter, r *http.Request) {
	requestedPath := r.URL.Query().Get("path")
	if requestedPath == "" {
		respondError(w, http.StatusBadRequest, "path is required")
		return
	}

	// Resolve and validate path
	absPath, err := s.resolvePath(requestedPath)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid path")
		return
	}

	// Check if path exists and is a file
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			respondError(w, http.StatusNotFound, "file not found")
			return
		}
		s.logger.Error("failed to stat file", "path", absPath, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to access file")
		return
	}

	if info.IsDir() {
		respondError(w, http.StatusBadRequest, "path is a directory")
		return
	}

	// Limit file size for reading
	const maxFileSize = 10 * 1024 * 1024 // 10 MB
	if info.Size() > maxFileSize {
		respondError(w, http.StatusBadRequest, "file too large")
		return
	}

	// Read file content
	// #nosec G304 -- absPath is validated to be within the project root
	content, err := os.ReadFile(absPath)
	if err != nil {
		s.logger.Error("failed to read file", "path", absPath, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	// Check if binary
	isBinary := isBinaryContent(content)

	response := FileContentResponse{
		Path:     requestedPath,
		Size:     info.Size(),
		Binary:   isBinary,
		Language: detectLanguage(absPath),
	}

	if !isBinary {
		response.Content = string(content)
	}

	respondJSON(w, http.StatusOK, response)
}

// handleGetFileTree returns a tree structure of the project.
func (s *Server) handleGetFileTree(w http.ResponseWriter, r *http.Request) {
	maxDepth := 3
	if depthStr := r.URL.Query().Get("depth"); depthStr != "" {
		if depth, err := json.Number(depthStr).Int64(); err == nil {
			maxDepth = int(depth)
		}
	}

	tree, err := s.buildFileTree(".", "", 0, maxDepth)
	if err != nil {
		s.logger.Error("failed to build file tree", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to build file tree")
		return
	}

	respondJSON(w, http.StatusOK, tree)
}

// FileTreeNode represents a node in the file tree.
type FileTreeNode struct {
	Name     string         `json:"name"`
	Path     string         `json:"path"`
	IsDir    bool           `json:"is_dir"`
	Children []FileTreeNode `json:"children,omitempty"`
}

// buildFileTree recursively builds a file tree.
func (s *Server) buildFileTree(dir, relPath string, depth, maxDepth int) ([]FileTreeNode, error) {
	if depth >= maxDepth {
		return nil, nil
	}

	absPath, err := s.resolvePath(dir)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}

	nodes := make([]FileTreeNode, 0)
	for _, entry := range entries {
		// Skip hidden files and common ignored directories
		name := entry.Name()
		if strings.HasPrefix(name, ".") || isIgnoredDir(name) {
			continue
		}

		nodePath := name
		if relPath != "" {
			nodePath = filepath.Join(relPath, name)
		}

		node := FileTreeNode{
			Name:  name,
			Path:  nodePath,
			IsDir: entry.IsDir(),
		}

		if entry.IsDir() && depth < maxDepth-1 {
			children, err := s.buildFileTree(filepath.Join(dir, name), nodePath, depth+1, maxDepth)
			if err == nil {
				node.Children = children
			}
		}

		nodes = append(nodes, node)
	}

	// Sort: directories first, then alphabetically
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].IsDir != nodes[j].IsDir {
			return nodes[i].IsDir
		}
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})

	return nodes, nil
}

// resolvePath resolves a relative path and validates it's within the project.
func (s *Server) resolvePath(requestedPath string) (string, error) {
	// Get working directory
	wd := s.root
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	// Clean and resolve the path
	cleanPath := filepath.Clean(requestedPath)
	absPath, err := filepath.Abs(filepath.Join(wd, cleanPath))
	if err != nil {
		return "", err
	}

	// Ensure the resolved path is within the working directory
	rel, err := filepath.Rel(wd, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", os.ErrPermission
	}

	return absPath, nil
}

// isBinaryContent checks if content appears to be binary.
func isBinaryContent(content []byte) bool {
	// Check for null bytes in the first 8000 bytes
	checkLen := len(content)
	if checkLen > 8000 {
		checkLen = 8000
	}

	for i := 0; i < checkLen; i++ {
		if content[i] == 0 {
			return true
		}
	}
	return false
}

// detectLanguage detects the programming language from file extension.
func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	languages := map[string]string{
		".go":         "go",
		".js":         "javascript",
		".jsx":        "javascript",
		".ts":         "typescript",
		".tsx":        "typescript",
		".py":         "python",
		".rb":         "ruby",
		".rs":         "rust",
		".java":       "java",
		".c":          "c",
		".cpp":        "cpp",
		".h":          "c",
		".hpp":        "cpp",
		".css":        "css",
		".scss":       "scss",
		".html":       "html",
		".json":       "json",
		".yaml":       "yaml",
		".yml":        "yaml",
		".toml":       "toml",
		".md":         "markdown",
		".sql":        "sql",
		".sh":         "bash",
		".bash":       "bash",
		".zsh":        "bash",
		".dockerfile": "dockerfile",
	}

	if lang, ok := languages[ext]; ok {
		return lang
	}

	// Check for Dockerfile
	if strings.ToLower(filepath.Base(path)) == "dockerfile" {
		return "dockerfile"
	}

	return ""
}

// isIgnoredDir checks if a directory should be ignored in file listings.
func isIgnoredDir(name string) bool {
	ignored := map[string]bool{
		"node_modules": true,
		"vendor":       true,
		"__pycache__":  true,
		".git":         true,
		".svn":         true,
		".hg":          true,
		"dist":         true,
		"build":        true,
		".cache":       true,
		".vscode":      true,
		".idea":        true,
	}
	return ignored[name]
}
