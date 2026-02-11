package clip

import (
	"os"
	"testing"
)

func TestWriteAllOSC52_EmptyText(t *testing.T) {
	err := writeAllOSC52("")
	if err == nil {
		t.Error("should error on empty text")
	}
}

func TestWriteAllOSC52_NotTerminal(t *testing.T) {
	// In tests, stderr is typically not a terminal
	err := writeAllOSC52("test")
	if err == nil {
		t.Skip("stderr is a terminal in this environment")
	}
}

func TestWriteTempFile(t *testing.T) {
	path, err := writeTempFile("test content")
	if err != nil {
		t.Fatalf("writeTempFile error: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != "test content" {
		t.Errorf("got %q, want %q", string(data), "test content")
	}
}

func TestWriteTempFile_Empty(t *testing.T) {
	path, err := writeTempFile("")
	if err != nil {
		t.Fatalf("writeTempFile error: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != "" {
		t.Errorf("got %q, want empty", string(data))
	}
}

func TestMethodConstants(t *testing.T) {
	if MethodNative != "native" {
		t.Errorf("MethodNative = %q", MethodNative)
	}
	if MethodOSC52 != "osc52" {
		t.Errorf("MethodOSC52 = %q", MethodOSC52)
	}
	if MethodFile != "file" {
		t.Errorf("MethodFile = %q", MethodFile)
	}
}
