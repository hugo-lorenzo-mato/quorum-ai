package clip

import (
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// writeAllOSC52: text too large
// ---------------------------------------------------------------------------

func TestWriteAllOSC52_TextTooLarge(t *testing.T) {
	largeText := strings.Repeat("x", osc52LimitBytes+1)
	err := writeAllOSC52(largeText)
	if err == nil {
		t.Error("expected error for text exceeding OSC52 limit")
	}
}

// ---------------------------------------------------------------------------
// WriteAll: all backends fail, temp file works
// ---------------------------------------------------------------------------

func TestWriteAll_AllFail_TempFileCreated(t *testing.T) {
	t.Cleanup(resetStubs())
	nativeWriteAll = func(_ string) error { return errFake("native down") }
	osc52WriteAll = func(_ string) error { return errFake("osc52 down") }

	res, err := WriteAll("fallback content")
	if err != nil {
		t.Fatalf("WriteAll returned error: %v", err)
	}
	if res.Method != MethodFile {
		t.Fatalf("Method = %q, want %q", res.Method, MethodFile)
	}
	if res.FilePath == "" {
		t.Fatal("FilePath is empty")
	}
	t.Cleanup(func() { _ = os.Remove(res.FilePath) })

	data, err := os.ReadFile(res.FilePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "fallback content" {
		t.Errorf("content = %q, want %q", string(data), "fallback content")
	}
}

// ---------------------------------------------------------------------------
// WriteAll: all backends fail, AND temp file creation also fails
// This triggers the error return from WriteAll (line 54-56).
// ---------------------------------------------------------------------------

func TestWriteAll_AllFail_TempFileFails(t *testing.T) {
	t.Cleanup(resetStubs())
	nativeWriteAll = func(_ string) error { return errFake("native down") }
	osc52WriteAll = func(_ string) error { return errFake("osc52 down") }

	// Make TMPDIR point to a non-existent directory so os.CreateTemp fails
	origTmpdir := os.Getenv("TMPDIR")
	os.Setenv("TMPDIR", "/nonexistent-temp-dir-for-test")
	t.Cleanup(func() {
		if origTmpdir == "" {
			os.Unsetenv("TMPDIR")
		} else {
			os.Setenv("TMPDIR", origTmpdir)
		}
	})

	_, err := WriteAll("should fail")
	if err == nil {
		t.Error("expected error when all backends fail including temp file")
	}
}

// ---------------------------------------------------------------------------
// writeTempFile: CreateTemp fails
// ---------------------------------------------------------------------------

func TestWriteTempFile_CreateTempFails(t *testing.T) {
	origTmpdir := os.Getenv("TMPDIR")
	os.Setenv("TMPDIR", "/nonexistent-temp-dir-for-test")
	t.Cleanup(func() {
		if origTmpdir == "" {
			os.Unsetenv("TMPDIR")
		} else {
			os.Setenv("TMPDIR", origTmpdir)
		}
	})

	_, err := writeTempFile("should fail")
	if err == nil {
		t.Error("expected error when TMPDIR is invalid")
	}
}

// ---------------------------------------------------------------------------
// WriteAll: empty string
// ---------------------------------------------------------------------------

func TestWriteAll_EmptyString(t *testing.T) {
	t.Cleanup(resetStubs())
	nativeWriteAll = func(_ string) error { return errFake("native down") }
	osc52WriteAll = func(_ string) error { return errFake("osc52 down") }

	res, err := WriteAll("")
	if err != nil {
		t.Fatalf("WriteAll returned error: %v", err)
	}
	if res.Method != MethodFile {
		t.Fatalf("Method = %q, want %q", res.Method, MethodFile)
	}
	t.Cleanup(func() { _ = os.Remove(res.FilePath) })
}

// ---------------------------------------------------------------------------
// WriteAll: large text that exceeds OSC52 limit but native works
// ---------------------------------------------------------------------------

func TestWriteAll_LargeText_NativeSucceeds(t *testing.T) {
	t.Cleanup(resetStubs())
	nativeWriteAll = func(_ string) error { return nil }
	osc52WriteAll = func(_ string) error {
		t.Fatal("osc52 should not be called when native succeeds")
		return nil
	}

	large := strings.Repeat("x", osc52LimitBytes+100)
	res, err := WriteAll(large)
	if err != nil {
		t.Fatalf("WriteAll returned error: %v", err)
	}
	if res.Method != MethodNative {
		t.Errorf("Method = %q, want %q", res.Method, MethodNative)
	}
}

// ---------------------------------------------------------------------------
// writeTempFile: various content
// ---------------------------------------------------------------------------

func TestWriteTempFile_LargeContent(t *testing.T) {
	content := strings.Repeat("abcde", 10000) // 50KB
	path, err := writeTempFile(content)
	if err != nil {
		t.Fatalf("writeTempFile: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != content {
		t.Error("content mismatch")
	}
}

func TestWriteTempFile_SpecialCharacters(t *testing.T) {
	content := "line1\nline2\ttab\r\n\x00null"
	path, err := writeTempFile(content)
	if err != nil {
		t.Fatalf("writeTempFile: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != content {
		t.Errorf("content = %q, want %q", string(data), content)
	}
}

func TestWriteTempFile_UnicodeContent(t *testing.T) {
	content := "Hello, world! Hola, mundo! Unicode: \u2603\u2764\u2728"
	path, err := writeTempFile(content)
	if err != nil {
		t.Fatalf("writeTempFile: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != content {
		t.Errorf("content = %q, want %q", string(data), content)
	}
}

// ---------------------------------------------------------------------------
// Result struct
// ---------------------------------------------------------------------------

func TestResult_FieldAccess(t *testing.T) {
	r := Result{
		Method:   MethodFile,
		FilePath: "/tmp/test.txt",
	}
	if r.Method != MethodFile {
		t.Errorf("Method = %q", r.Method)
	}
	if r.FilePath != "/tmp/test.txt" {
		t.Errorf("FilePath = %q", r.FilePath)
	}

	r2 := Result{Method: MethodNative}
	if r2.FilePath != "" {
		t.Errorf("FilePath should be empty for native, got %q", r2.FilePath)
	}
}

// ---------------------------------------------------------------------------
// WriteAll: native fails, OSC52 succeeds with various content
// ---------------------------------------------------------------------------

func TestWriteAll_OSC52_WithContent(t *testing.T) {
	t.Cleanup(resetStubs())
	var captured string
	nativeWriteAll = func(_ string) error { return errFake("native down") }
	osc52WriteAll = func(text string) error {
		captured = text
		return nil
	}

	res, err := WriteAll("test content for osc52")
	if err != nil {
		t.Fatalf("WriteAll: %v", err)
	}
	if res.Method != MethodOSC52 {
		t.Errorf("Method = %q, want %q", res.Method, MethodOSC52)
	}
	if captured != "test content for osc52" {
		t.Errorf("captured = %q", captured)
	}
}
