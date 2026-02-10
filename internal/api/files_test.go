package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsForbiddenProjectPath(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{in: ".", want: false},
		{in: "", want: false},
		{in: "src/main.go", want: false},
		{in: ".env", want: true},
		{in: ".env.local", want: true},
		{in: ".git/config", want: true},
		{in: ".ssh/id_ed25519", want: true},
		{in: "../secrets.txt", want: true},
		{in: "nested/../..", want: true},
		{in: "keys/server.pem", want: true},
		{in: "keys/server.key", want: true},
		{in: "certs/bundle.p12", want: true},
		// .quorum internals stay blocked
		{in: ".quorum", want: true},
		{in: ".quorum/quorum.db", want: true},
		{in: ".quorum/config", want: true},
		// .quorum/runs/ is allowed (workflow report artifacts)
		{in: ".quorum/runs", want: false},
		{in: ".quorum/runs/wf-123/analyze-phase/v1/claude.md", want: false},
		{in: ".quorum/runs/wf-123/plan-phase/final-plan.md", want: false},
		{in: ".quorum/runs/wf-123/execute-phase/tasks/task-1.md", want: false},
	}

	for _, tc := range cases {
		if got := isForbiddenProjectPath(filepath.Clean(tc.in)); got != tc.want {
			t.Fatalf("isForbiddenProjectPath(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestResolvePathCtx_RejectsTraversalAndForbidden(t *testing.T) {
	root := t.TempDir()
	s := &Server{root: root}

	for _, p := range []string{
		"../outside.txt",
		".git/config",
		".ssh/id_rsa",
		".env",
		".env.production",
	} {
		_, err := s.resolvePathCtx(context.Background(), p)
		if err == nil {
			t.Fatalf("resolvePathCtx(%q) expected error", p)
		}
		if !errors.Is(err, os.ErrPermission) {
			t.Fatalf("resolvePathCtx(%q) error = %v, want os.ErrPermission", p, err)
		}
	}
}

func TestResolvePathCtx_AllowsMissingPathsWithinRoot(t *testing.T) {
	root := t.TempDir()
	s := &Server{root: root}

	abs, err := s.resolvePathCtx(context.Background(), "does-not-exist-yet/dir")
	if err != nil {
		t.Fatalf("resolvePathCtx unexpected error: %v", err)
	}
	if !isPathWithinDir(root, abs) {
		t.Fatalf("resolved path %q is not within root %q", abs, root)
	}
}

func TestResolvePathCtx_BlocksSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Symlink creation often requires elevated privileges on Windows.
		t.Skip("skip symlink test on windows")
	}

	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	linkPath := filepath.Join(root, "link-out")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	s := &Server{root: root}
	_, err := s.resolvePathCtx(context.Background(), "link-out/secret.txt")
	if err == nil {
		t.Fatalf("expected permission error for symlink escape")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("error = %v, want os.ErrPermission", err)
	}
}

