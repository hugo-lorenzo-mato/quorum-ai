package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// newTestServerWithRoot creates a Server with a given root and a discarding logger.
func newTestServerWithRoot(root string) *Server {
	return &Server{
		root:   root,
		logger: slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
	}
}

// --- handleListFiles tests ---

func TestHandleListFiles_DefaultPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Create a non-hidden file and a directory.
	if err := os.WriteFile(filepath.Join(root, "readme.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}

	s := newTestServerWithRoot(root)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/", nil)
	rr := httptest.NewRecorder()

	s.handleListFiles(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusOK)
	}

	var entries []FileEntry
	if err := json.NewDecoder(rr.Body).Decode(&entries); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Directories should come first.
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(entries))
	}
	if !entries[0].IsDir {
		t.Errorf("first entry should be a directory, got %+v", entries[0])
	}
	if entries[0].Name != "src" {
		t.Errorf("expected first entry name 'src', got %q", entries[0].Name)
	}
}

func TestHandleListFiles_ExplicitSubdirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	subdir := filepath.Join(root, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "file.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newTestServerWithRoot(root)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/?path=subdir", nil)
	rr := httptest.NewRecorder()

	s.handleListFiles(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusOK)
	}

	var entries []FileEntry
	if err := json.NewDecoder(rr.Body).Decode(&entries); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "file.go" {
		t.Errorf("expected 'file.go', got %q", entries[0].Name)
	}
	if entries[0].Extension != "go" {
		t.Errorf("expected extension 'go', got %q", entries[0].Extension)
	}
	// Path should be relative to subdirectory.
	if entries[0].Path != filepath.Join("subdir", "file.go") {
		t.Errorf("expected relative path 'subdir/file.go', got %q", entries[0].Path)
	}
}

func TestHandleListFiles_NonExistentPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	s := newTestServerWithRoot(root)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/?path=no-such-dir", nil)
	rr := httptest.NewRecorder()

	s.handleListFiles(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d (empty list for non-existent)", rr.Code, http.StatusOK)
	}

	var entries []FileEntry
	if err := json.NewDecoder(rr.Body).Decode(&entries); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestHandleListFiles_PathIsFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	filePath := filepath.Join(root, "single.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newTestServerWithRoot(root)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/?path=single.txt", nil)
	rr := httptest.NewRecorder()

	s.handleListFiles(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusBadRequest)
	}
	var body map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] != "path is not a directory" {
		t.Errorf("error=%q, want 'path is not a directory'", body["error"])
	}
}

func TestHandleListFiles_InvalidPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	s := newTestServerWithRoot(root)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/?path=../outside", nil)
	rr := httptest.NewRecorder()

	s.handleListFiles(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleListFiles_HiddenFilesSkipped(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".hidden"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newTestServerWithRoot(root)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/", nil)
	rr := httptest.NewRecorder()

	s.handleListFiles(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusOK)
	}

	var entries []FileEntry
	_ = json.NewDecoder(rr.Body).Decode(&entries)

	for _, entry := range entries {
		if entry.Name == ".hidden" {
			t.Errorf("hidden file should be skipped")
		}
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 visible entry, got %d", len(entries))
	}
}

func TestHandleListFiles_SortOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Create files and directories with various names to test sorting.
	if err := os.Mkdir(filepath.Join(root, "zebra"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "middle.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Afile.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newTestServerWithRoot(root)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/", nil)
	rr := httptest.NewRecorder()

	s.handleListFiles(rr, req)

	var entries []FileEntry
	_ = json.NewDecoder(rr.Body).Decode(&entries)

	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	// Directories first, alphabetically.
	if !entries[0].IsDir || entries[0].Name != "alpha" {
		t.Errorf("first should be dir 'alpha', got %+v", entries[0])
	}
	if !entries[1].IsDir || entries[1].Name != "zebra" {
		t.Errorf("second should be dir 'zebra', got %+v", entries[1])
	}
	// Then files, alphabetically (case-insensitive).
	if entries[2].IsDir || entries[2].Name != "Afile.txt" {
		t.Errorf("third should be file 'Afile.txt', got %+v", entries[2])
	}
	if entries[3].IsDir || entries[3].Name != "middle.txt" {
		t.Errorf("fourth should be file 'middle.txt', got %+v", entries[3])
	}
}

// --- handleGetFileContent additional tests ---

func TestHandleGetFileContent_BinaryFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	binaryContent := []byte{0x00, 0x01, 0x02, 0xFF}
	if err := os.WriteFile(filepath.Join(root, "binary.bin"), binaryContent, 0o644); err != nil {
		t.Fatal(err)
	}

	s := newTestServerWithRoot(root)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/content?path=binary.bin", nil)
	rr := httptest.NewRecorder()

	s.handleGetFileContent(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusOK)
	}

	var resp FileContentResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Binary {
		t.Error("expected Binary=true for binary content")
	}
	if resp.Content != "" {
		t.Error("expected empty Content for binary file")
	}
}

func TestHandleGetFileContent_LanguageDetection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newTestServerWithRoot(root)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/content?path=main.go", nil)
	rr := httptest.NewRecorder()

	s.handleGetFileContent(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusOK)
	}

	var resp FileContentResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Language != "go" {
		t.Errorf("expected language 'go', got %q", resp.Language)
	}
}

func TestHandleGetFileContent_ForbiddenPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Create a .env file in root.
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=foo"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newTestServerWithRoot(root)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/content?path=.env", nil)
	rr := httptest.NewRecorder()

	s.handleGetFileContent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusBadRequest)
	}
}

// --- handleGetFileTree tests ---

func TestHandleGetFileTree_Basic(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "app.ts"), []byte("export {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Project"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newTestServerWithRoot(root)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/tree", nil)
	rr := httptest.NewRecorder()

	s.handleGetFileTree(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusOK)
	}

	var tree []FileTreeNode
	if err := json.NewDecoder(rr.Body).Decode(&tree); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(tree) < 2 {
		t.Fatalf("expected at least 2 nodes, got %d", len(tree))
	}
	// Directories first.
	if !tree[0].IsDir || tree[0].Name != "src" {
		t.Errorf("first node should be dir 'src', got %+v", tree[0])
	}
}

func TestHandleGetFileTree_WithDepth(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "deep.txt"), []byte("deep"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newTestServerWithRoot(root)

	// Depth 1 should not descend into children.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/tree?depth=1", nil)
	rr := httptest.NewRecorder()
	s.handleGetFileTree(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusOK)
	}

	var tree []FileTreeNode
	_ = json.NewDecoder(rr.Body).Decode(&tree)
	if len(tree) != 1 {
		t.Fatalf("expected 1 top-level node, got %d", len(tree))
	}
	// With depth=1, children should be nil/empty.
	if len(tree[0].Children) != 0 {
		t.Errorf("expected no children at depth 1, got %d", len(tree[0].Children))
	}
}

func TestHandleGetFileTree_IgnoredDirs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, d := range []string{"node_modules", "vendor", "__pycache__", "src"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	s := newTestServerWithRoot(root)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/tree", nil)
	rr := httptest.NewRecorder()
	s.handleGetFileTree(rr, req)

	var tree []FileTreeNode
	_ = json.NewDecoder(rr.Body).Decode(&tree)

	for _, node := range tree {
		if node.Name == "node_modules" || node.Name == "vendor" || node.Name == "__pycache__" {
			t.Errorf("expected %q to be ignored", node.Name)
		}
	}
	// 'src' should remain.
	found := false
	for _, node := range tree {
		if node.Name == "src" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'src' to be present in tree")
	}
}

// --- Helper function tests ---

func TestIsBinaryContent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		data   []byte
		binary bool
	}{
		{name: "empty", data: []byte{}, binary: false},
		{name: "text", data: []byte("hello world"), binary: false},
		{name: "null byte", data: []byte{0x00}, binary: true},
		{name: "null in middle", data: []byte("hello\x00world"), binary: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isBinaryContent(tc.data)
			if got != tc.binary {
				t.Errorf("isBinaryContent(%q) = %v, want %v", tc.data, got, tc.binary)
			}
		})
	}
}

func TestDetectLanguage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path string
		lang string
	}{
		{"main.go", "go"},
		{"app.js", "javascript"},
		{"app.jsx", "javascript"},
		{"index.ts", "typescript"},
		{"index.tsx", "typescript"},
		{"script.py", "python"},
		{"app.rb", "ruby"},
		{"lib.rs", "rust"},
		{"Main.java", "java"},
		{"main.c", "c"},
		{"main.cpp", "cpp"},
		{"header.h", "c"},
		{"header.hpp", "cpp"},
		{"style.css", "css"},
		{"style.scss", "scss"},
		{"index.html", "html"},
		{"data.json", "json"},
		{"config.yaml", "yaml"},
		{"config.yml", "yaml"},
		{"Cargo.toml", "toml"},
		{"README.md", "markdown"},
		{"schema.sql", "sql"},
		{"run.sh", "bash"},
		{"script.bash", "bash"},
		{"init.zsh", "bash"},
		{"build.dockerfile", "dockerfile"},
		{"Dockerfile", "dockerfile"},
		{"unknown.xyz", ""},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got := detectLanguage(tc.path)
			if got != tc.lang {
				t.Errorf("detectLanguage(%q) = %q, want %q", tc.path, got, tc.lang)
			}
		})
	}
}

func TestIsIgnoredDir(t *testing.T) {
	t.Parallel()
	ignored := []string{"node_modules", "vendor", "__pycache__", ".git", ".svn", ".hg", "dist", "build", ".cache", ".vscode", ".idea"}
	for _, name := range ignored {
		if !isIgnoredDir(name) {
			t.Errorf("expected %q to be ignored", name)
		}
	}
	allowed := []string{"src", "internal", "cmd", "pkg"}
	for _, name := range allowed {
		if isIgnoredDir(name) {
			t.Errorf("expected %q to be allowed", name)
		}
	}
}

func TestIsPathWithinDir(t *testing.T) {
	t.Parallel()
	cases := []struct {
		root   string
		path   string
		within bool
	}{
		{"/home/user/project", "/home/user/project/src/main.go", true},
		{"/home/user/project", "/home/user/project", true},
		{"/home/user/project", "/home/user", false},
		{"/home/user/project", "/tmp/secret", false},
	}
	for _, tc := range cases {
		got := isPathWithinDir(tc.root, tc.path)
		if got != tc.within {
			t.Errorf("isPathWithinDir(%q, %q) = %v, want %v", tc.root, tc.path, got, tc.within)
		}
	}
}

func TestIsForbiddenPathSegment(t *testing.T) {
	t.Parallel()
	forbidden := []string{"..", ".git", ".quorum", ".ssh", ".env", ".env.local", "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519", "server.pem", "server.key", "cert.p12", "cert.pfx"}
	for _, seg := range forbidden {
		if !isForbiddenPathSegment(seg) {
			t.Errorf("expected %q to be forbidden", seg)
		}
	}
	allowed := []string{"main.go", "src", "README.md"}
	for _, seg := range allowed {
		if isForbiddenPathSegment(seg) {
			t.Errorf("expected %q to be allowed", seg)
		}
	}
}

func TestIsForbiddenProjectDir(t *testing.T) {
	t.Parallel()
	if !isForbiddenProjectDir(".git") {
		t.Error("expected .git to be forbidden")
	}
	if !isForbiddenProjectDir(".quorum") {
		t.Error("expected .quorum to be forbidden")
	}
	if !isForbiddenProjectDir(".ssh") {
		t.Error("expected .ssh to be forbidden")
	}
	if isForbiddenProjectDir("src") {
		t.Error("expected src to be allowed")
	}
}

func TestIsForbiddenEnvFilename(t *testing.T) {
	t.Parallel()
	if !isForbiddenEnvFilename(".env") {
		t.Error("expected .env to be forbidden")
	}
	if !isForbiddenEnvFilename(".env.local") {
		t.Error("expected .env.local to be forbidden")
	}
	if isForbiddenEnvFilename("env") {
		t.Error("expected 'env' to be allowed")
	}
}

func TestIsForbiddenPrivateKeyFilename(t *testing.T) {
	t.Parallel()
	for _, f := range []string{"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519"} {
		if !isForbiddenPrivateKeyFilename(f) {
			t.Errorf("expected %q to be forbidden", f)
		}
	}
	if isForbiddenPrivateKeyFilename("id_other") {
		t.Error("expected 'id_other' to be allowed")
	}
}

func TestIsForbiddenKeyMaterialFilename(t *testing.T) {
	t.Parallel()
	for _, f := range []string{"server.pem", "ca.key", "cert.p12", "store.pfx"} {
		if !isForbiddenKeyMaterialFilename(f) {
			t.Errorf("expected %q to be forbidden", f)
		}
	}
	if isForbiddenKeyMaterialFilename("readme.txt") {
		t.Error("expected 'readme.txt' to be allowed")
	}
}

func TestValidateProjectRelativePath(t *testing.T) {
	t.Parallel()
	// Forbidden paths.
	for _, p := range []string{".env", ".git/config", "/etc/passwd"} {
		if err := validateProjectRelativePath(filepath.Clean(p)); err == nil {
			t.Errorf("expected error for path %q", p)
		}
	}
	// Allowed paths.
	for _, p := range []string{"src", "main.go", "docs/api.md"} {
		if err := validateProjectRelativePath(filepath.Clean(p)); err != nil {
			t.Errorf("unexpected error for path %q: %v", p, err)
		}
	}
}

func TestCanonicalizeRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rootAbs, rootReal, err := canonicalizeRoot(root)
	if err != nil {
		t.Fatalf("canonicalizeRoot(%q) error: %v", root, err)
	}
	if rootAbs == "" {
		t.Error("rootAbs should not be empty")
	}
	if rootReal == "" {
		t.Error("rootReal should not be empty")
	}
}

func TestResolvePath_NoContext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "test.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newTestServerWithRoot(root)
	abs, err := s.resolvePath("test.txt")
	if err != nil {
		t.Fatalf("resolvePath error: %v", err)
	}
	if !isPathWithinDir(root, abs) {
		t.Errorf("resolved path %q not within root %q", abs, root)
	}
}
