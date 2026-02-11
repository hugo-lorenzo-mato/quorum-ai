package api

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	msgInvalidPath = "invalid path"
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
		respondError(w, http.StatusBadRequest, msgInvalidPath)
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

	rootReal, fsys, rel, err := s.resolveProjectFileFSPath(r.Context(), requestedPath)
	if err != nil {
		respondError(w, http.StatusBadRequest, msgInvalidPath)
		return
	}

	// Check if path exists and is a file
	info, err := fs.Stat(fsys, rel)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			respondError(w, http.StatusNotFound, "file not found")
			return
		}
		s.logger.Error("failed to stat file", "path", rel, "error", err)
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

	// Best-effort symlink escape protection: ensure the resolved real path stays within rootReal.
	if realPath, err := filepath.EvalSymlinks(filepath.Join(rootReal, filepath.FromSlash(rel))); err == nil {
		if !isPathWithinDir(rootReal, realPath) {
			respondError(w, http.StatusBadRequest, msgInvalidPath)
			return
		}
	}

	// Read file content
	content, err := fs.ReadFile(fsys, rel)
	if err != nil {
		s.logger.Error("failed to read file", "path", rel, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	// Check if binary
	isBinary := isBinaryContent(content)

	response := FileContentResponse{
		Path:     requestedPath,
		Size:     info.Size(),
		Binary:   isBinary,
		Language: detectLanguage(requestedPath),
	}

	if !isBinary {
		response.Content = string(content)
	}

	respondJSON(w, http.StatusOK, response)
}

func (s *Server) resolveProjectFileFSPath(ctx context.Context, requestedPath string) (rootReal string, fsys fs.FS, rel string, err error) {
	// Resolve and validate path (project-aware), but read via an fs rooted at the
	// project directory to avoid path traversal and symlink escapes.
	root, err := s.projectRootForRequest(ctx)
	if err != nil {
		return "", nil, "", err
	}

	_, rootReal, err = canonicalizeRoot(root)
	if err != nil {
		return "", nil, "", err
	}

	cleanPath := filepath.Clean(requestedPath)
	if validateProjectRelativePath(cleanPath) != nil {
		return "", nil, "", os.ErrPermission
	}

	rel = strings.TrimPrefix(filepath.ToSlash(cleanPath), "./")
	if rel == "" || rel == "." || !fs.ValidPath(rel) {
		return "", nil, "", os.ErrPermission
	}

	return rootReal, os.DirFS(rootReal), rel, nil
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
	root, err := s.projectRootForRequest(ctx)
	if err != nil {
		return "", err
	}
	rootAbs, rootReal, err := canonicalizeRoot(root)
	if err != nil {
		return "", err
	}

	cleanPath := filepath.Clean(requestedPath)
	if err := validateProjectRelativePath(cleanPath); err != nil {
		return "", err
	}

	absPath, err := filepath.Abs(filepath.Join(rootAbs, cleanPath))
	if err != nil {
		return "", err
	}
	if !isPathWithinDir(rootAbs, absPath) {
		return "", os.ErrPermission
	}

	return resolveExistingPathWithinRoot(absPath, rootReal)
}

func (s *Server) projectRootForRequest(ctx context.Context) (string, error) {
	root := s.getProjectRootPath(ctx)
	if root != "" {
		return root, nil
	}
	return os.Getwd()
}

func canonicalizeRoot(root string) (rootAbs string, rootReal string, err error) {
	rootAbs, err = filepath.Abs(root)
	if err != nil {
		return "", "", err
	}

	rootReal = rootAbs
	if rr, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootReal = rr
	}
	return rootAbs, rootReal, nil
}

func validateProjectRelativePath(cleanPath string) error {
	// Prevent accidental exposure of sensitive files if the HTTP server is reachable.
	// This endpoint is intended for project browsing; secrets should not be readable.
	if isForbiddenProjectPath(cleanPath) {
		return os.ErrPermission
	}

	// Reject absolute paths (and Windows volume/UNC paths).
	if filepath.IsAbs(cleanPath) || filepath.VolumeName(cleanPath) != "" {
		return os.ErrPermission
	}

	// Also reject Unix-style absolute paths on Windows (e.g., /etc/passwd)
	// These are not considered absolute by filepath.IsAbs on Windows but are clearly external paths
	if len(cleanPath) > 0 && cleanPath[0] == '/' {
		return os.ErrPermission
	}

	return nil
}

func resolveExistingPathWithinRoot(absPath, rootReal string) (string, error) {
	// If the path exists, resolve symlinks to prevent traversal via in-tree symlinks.
	// For non-existent paths (e.g., browsing a missing directory), preserve legacy
	// behavior and let the caller decide how to handle os.IsNotExist.
	_, err := os.Lstat(absPath)
	if err == nil {
		realPath, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			return "", err
		}
		if !isPathWithinDir(rootReal, realPath) {
			return "", os.ErrPermission
		}
		return realPath, nil
	}
	if os.IsNotExist(err) {
		return absPath, nil
	}

	// Fail closed on unexpected filesystem errors.
	return "", err
}

func isForbiddenProjectPath(cleanPath string) bool {
	p := filepath.ToSlash(cleanPath)
	p = strings.TrimPrefix(p, "./")
	if p == "" || p == "." {
		return false
	}
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if isForbiddenPathSegment(part) {
			// Allow .quorum/runs/ (workflow report artifacts are user-facing documents).
			// Everything else inside .quorum/ (db, config, etc.) stays blocked.
			if part == ".quorum" && i+1 < len(parts) && parts[i+1] == "runs" {
				continue
			}
			return true
		}
	}
	return false
}

func isForbiddenPathSegment(part string) bool {
	if part == ".." {
		return true
	}
	if isForbiddenProjectDir(part) {
		return true
	}
	if isForbiddenEnvFilename(part) {
		return true
	}
	lower := strings.ToLower(part)
	return isForbiddenPrivateKeyFilename(lower) || isForbiddenKeyMaterialFilename(lower)
}

func isForbiddenProjectDir(part string) bool {
	switch part {
	case ".git", ".quorum", ".ssh":
		return true
	default:
		return false
	}
}

func isForbiddenEnvFilename(part string) bool {
	return part == ".env" || strings.HasPrefix(part, ".env.")
}

func isForbiddenPrivateKeyFilename(lower string) bool {
	switch lower {
	case "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519":
		return true
	default:
		return false
	}
}

func isForbiddenKeyMaterialFilename(lower string) bool {
	return strings.HasSuffix(lower, ".pem") ||
		strings.HasSuffix(lower, ".key") ||
		strings.HasSuffix(lower, ".p12") ||
		strings.HasSuffix(lower, ".pfx")
}

// resolvePath resolves a relative path using the server root (no request context).
// Prefer resolvePathCtx when a request context is available.
func (s *Server) resolvePath(requestedPath string) (string, error) {
	return s.resolvePathCtx(context.Background(), requestedPath)
}

func isPathWithinDir(root, path string) bool {
	// Normalize root by resolving symlinks (must exist)
	normalizedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		// If we can't resolve the root, fallback to abs path
		normalizedRoot, _ = filepath.Abs(root)
	}

	// For path, resolve symlinks iteratively from the path up to the first existing ancestor
	normalizedPath := normalizePathWithAncestors(path)

	rel, err := filepath.Rel(normalizedRoot, normalizedPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	return true
}

// normalizePathWithAncestors normalizes a path by resolving symlinks in the nearest existing ancestor
func normalizePathWithAncestors(path string) string {
	// Try to resolve the full path first
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}

	// Path doesn't exist - find the first existing ancestor and resolve that
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	// Walk up the directory tree to find the first existing ancestor
	current := absPath
	var nonExistingParts []string

	for {
		if _, err := os.Stat(current); err == nil {
			// Found existing ancestor - resolve its symlinks
			if resolved, err := filepath.EvalSymlinks(current); err == nil {
				// Reconstruct the full path with resolved ancestor
				for i := len(nonExistingParts) - 1; i >= 0; i-- {
					resolved = filepath.Join(resolved, nonExistingParts[i])
				}
				return resolved
			}
			break
		}

		// Move up one level
		parent := filepath.Dir(current)
		if parent == current {
			// Reached root without finding existing path
			break
		}
		nonExistingParts = append(nonExistingParts, filepath.Base(current))
		current = parent
	}

	// Fallback: just return cleaned absolute path
	return filepath.Clean(absPath)
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
