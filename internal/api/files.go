package api

import (
	"context"
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

	// Resolve and validate path (project-aware)
	absPath, err := s.resolvePathCtx(r.Context(), requestedPath)
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

	// Resolve and validate path (project-aware)
	absPath, err := s.resolvePathCtx(r.Context(), requestedPath)
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

	tree, err := s.buildFileTree(r.Context(), ".", "", 0, maxDepth)
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
func (s *Server) buildFileTree(ctx context.Context, dir, relPath string, depth, maxDepth int) ([]FileTreeNode, error) {
	if depth >= maxDepth {
		return nil, nil
	}

	absPath, err := s.resolvePathCtx(ctx, dir)
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
			children, err := s.buildFileTree(ctx, filepath.Join(dir, name), nodePath, depth+1, maxDepth)
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

// resolvePathCtx resolves a relative path and validates it's within the project.
// It uses the project root from the request context when available (multi-project),
// falling back to s.root (server root) or the process working directory.
func (s *Server) resolvePathCtx(ctx context.Context, requestedPath string) (string, error) {
	// Prefer project root from context (multi-project support), then server root,
	// then process working directory.
	root := s.getProjectRootPath(ctx)
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rootReal := rootAbs
	if rr, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootReal = rr
	}

	cleanPath := filepath.Clean(requestedPath)

	// Prevent accidental exposure of sensitive files if the HTTP server is reachable.
	// This endpoint is intended for project browsing; secrets should not be readable.
	if isForbiddenProjectPath(cleanPath) {
		return "", os.ErrPermission
	}

	// Reject absolute paths (and Windows volume/UNC paths).
	if filepath.IsAbs(cleanPath) || filepath.VolumeName(cleanPath) != "" {
		return "", os.ErrPermission
	}

	absPath, err := filepath.Abs(filepath.Join(rootAbs, cleanPath))
	if err != nil {
		return "", err
	}
	if !isPathWithinDir(rootAbs, absPath) {
		return "", os.ErrPermission
	}

	// If the path exists, resolve symlinks to prevent traversal via in-tree symlinks.
	// For non-existent paths (e.g., browsing a missing directory), preserve legacy
	// behavior and let the caller decide how to handle os.IsNotExist.
	if _, err := os.Lstat(absPath); err == nil {
		realPath, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			return "", err
		}
		if !isPathWithinDir(rootReal, realPath) {
			return "", os.ErrPermission
		}
		return realPath, nil
	} else if !os.IsNotExist(err) {
		// Fail closed on unexpected filesystem errors.
		return "", err
	}

	return absPath, nil
}

func isForbiddenProjectPath(cleanPath string) bool {
	p := filepath.ToSlash(cleanPath)
	p = strings.TrimPrefix(p, "./")
	if p == "" || p == "." {
		return false
	}
	parts := strings.Split(p, "/")
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return true
		}

		// Entire directories to never expose.
		switch part {
		case ".git", ".quorum", ".ssh":
			return true
		}

		// Common secret-bearing filenames.
		if part == ".env" || strings.HasPrefix(part, ".env.") {
			return true
		}

		lower := strings.ToLower(part)
		if lower == "id_rsa" || lower == "id_dsa" || lower == "id_ecdsa" || lower == "id_ed25519" {
			return true
		}
		if strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".key") || strings.HasSuffix(lower, ".p12") || strings.HasSuffix(lower, ".pfx") {
			return true
		}
	}
	return false
}

// resolvePath resolves a relative path using the server root (no request context).
// Prefer resolvePathCtx when a request context is available.
func (s *Server) resolvePath(requestedPath string) (string, error) {
	return s.resolvePathCtx(context.Background(), requestedPath)
}

func isPathWithinDir(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	return true
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
