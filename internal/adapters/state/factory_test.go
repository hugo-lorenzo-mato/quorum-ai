package state

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStateManager(t *testing.T) {
	t.Run("creates sqlite manager and normalizes .db extension", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "factory_test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		// Intentionally pass a non-.db path to verify normalization.
		inPath := filepath.Join(tmpDir, "state.legacy")
		wantDBPath := filepath.Join(tmpDir, "state.db")

		sm, err := NewStateManager(inPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer CloseStateManager(sm)

		if gotType := typeName(sm); gotType != "*state.SQLiteStateManager" {
			t.Fatalf("got type %s, want %s", gotType, "*state.SQLiteStateManager")
		}

		if _, err := os.Stat(wantDBPath); err != nil {
			t.Fatalf("expected sqlite db at %s: %v", wantDBPath, err)
		}
	})
}

func TestCloseStateManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "close_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("closes sqlite manager", func(t *testing.T) {
		path := filepath.Join(tmpDir, "close_sqlite.db")
		sm, err := NewStateManager(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := CloseStateManager(sm); err != nil {
			t.Errorf("CloseStateManager failed: %v", err)
		}
	})

	t.Run("handles nil", func(t *testing.T) {
		if err := CloseStateManager(nil); err != nil {
			t.Errorf("CloseStateManager(nil) should not error: %v", err)
		}
	})
}

func typeName(v interface{}) string {
	return fmt.Sprintf("%T", v)
}
