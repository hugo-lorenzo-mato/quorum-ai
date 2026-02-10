package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolvePathCtx_AllowsNonExistentPathsWithinRoot(t *testing.T) {
	root := t.TempDir()
	s := &Server{root: root}

	p, err := s.resolvePathCtx(context.Background(), "does-not-exist/subdir")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("Abs(root): %v", err)
	}
	if !isPathWithinDir(rootAbs, p) {
		t.Fatalf("expected resolved path within root; root=%q path=%q", rootAbs, p)
	}
}

func TestResolvePathCtx_RejectsTraversalOutsideRoot(t *testing.T) {
	root := t.TempDir()
	s := &Server{root: root}

	_, err := s.resolvePathCtx(context.Background(), "../outside")
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected ErrPermission, got %v", err)
	}
}

func TestResolvePathCtx_RejectsAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	s := &Server{root: root}

	abs := filepath.Join(string(os.PathSeparator), "etc", "passwd")
	if runtime.GOOS == "windows" {
		// A rooted path like `\\etc\\passwd` is not considered absolute by Go on Windows.
		// Use a drive-qualified path instead.
		vol := filepath.VolumeName(root)
		if vol == "" {
			t.Fatalf("expected non-empty volume name for root %q", root)
		}
		abs = filepath.Join(vol+string(os.PathSeparator), "not-in-root", "file.txt")
	}
	_, err := s.resolvePathCtx(context.Background(), abs)
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected ErrPermission, got %v", err)
	}
}

func TestResolvePathCtx_RejectsSymlinkEscapes(t *testing.T) {
	// Symlink behavior differs across platforms; skip if unsupported.
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges on Windows")
	}

	root := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o600); err != nil {
		t.Fatalf("WriteFile(outside): %v", err)
	}

	linkPath := filepath.Join(root, "link.txt")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Skipf("os.Symlink not supported here: %v", err)
	}

	s := &Server{root: root}
	_, err := s.resolvePathCtx(context.Background(), "link.txt")
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected ErrPermission, got %v", err)
	}
}

func TestResolvePathCtx_AllowsSymlinksWithinRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges on Windows")
	}

	root := t.TempDir()
	target := filepath.Join(root, "target.txt")
	if err := os.WriteFile(target, []byte("ok"), 0o600); err != nil {
		t.Fatalf("WriteFile(target): %v", err)
	}

	linkPath := filepath.Join(root, "link.txt")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Skipf("os.Symlink not supported here: %v", err)
	}

	s := &Server{root: root}
	got, err := s.resolvePathCtx(context.Background(), "link.txt")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	gotReal, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("EvalSymlinks(got): %v", err)
	}
	targetReal, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatalf("EvalSymlinks(target): %v", err)
	}
	if gotReal != targetReal {
		t.Fatalf("expected resolved symlink target %q, got %q", targetReal, gotReal)
	}
}
