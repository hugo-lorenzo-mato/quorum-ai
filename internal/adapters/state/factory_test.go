package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewStateManager(t *testing.T) {
	tests := []struct {
		name        string
		backend     string
		wantType    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "empty backend defaults to json",
			backend:  "",
			wantType: "*state.JSONStateManager",
		},
		{
			name:     "json backend",
			backend:  "json",
			wantType: "*state.JSONStateManager",
		},
		{
			name:     "JSON backend (uppercase)",
			backend:  "JSON",
			wantType: "*state.JSONStateManager",
		},
		{
			name:     "sqlite backend",
			backend:  "sqlite",
			wantType: "*state.SQLiteStateManager",
		},
		{
			name:     "SQLite backend (mixed case)",
			backend:  "SQLite",
			wantType: "*state.SQLiteStateManager",
		},
		{
			name:        "unsupported backend",
			backend:     "unknown",
			wantErr:     true,
			errContains: "unsupported state backend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "factory_test")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmpDir)

			path := filepath.Join(tmpDir, "state.json")

			sm, err := NewStateManager(tt.backend, path)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !containsStr(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Clean up
			CloseStateManager(sm)

			gotType := typeName(sm)
			if gotType != tt.wantType {
				t.Errorf("got type %s, want %s", gotType, tt.wantType)
			}
		})
	}
}

func TestCloseStateManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "close_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("closes sqlite manager", func(t *testing.T) {
		path := filepath.Join(tmpDir, "close_sqlite.db")
		sm, err := NewStateManager("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		if err := CloseStateManager(sm); err != nil {
			t.Errorf("CloseStateManager failed: %v", err)
		}
	})

	t.Run("noop for json manager", func(t *testing.T) {
		path := filepath.Join(tmpDir, "close_json.json")
		sm, err := NewStateManager("json", path)
		if err != nil {
			t.Fatal(err)
		}
		// Should not error
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

func TestNormalizeBackend(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "json"},
		{"json", "json"},
		{"JSON", "json"},
		{"  json  ", "json"},
		{"sqlite", "sqlite"},
		{"SQLite", "sqlite"},
		{"  SQLITE  ", "sqlite"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeBackend(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeBackend(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func typeName(v interface{}) string {
	return fmt.Sprintf("%T", v)
}

func containsStr(s, substr string) bool {
	return strings.Contains(s, substr)
}
