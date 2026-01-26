package clip

import (
	"os"
	"testing"
)

func TestWriteAll_NativeSuccess(t *testing.T) {
	t.Cleanup(resetStubs())
	nativeWriteAll = func(_ string) error { return nil }
	osc52WriteAll = func(_ string) error {
		t.Fatal("osc52 should not be called when native succeeds")
		return nil
	}

	got, err := WriteAll("hello")
	if err != nil {
		t.Fatalf("WriteAll returned error: %v", err)
	}
	if got.Method != MethodNative {
		t.Fatalf("Method=%q, want %q", got.Method, MethodNative)
	}
	if got.FilePath != "" {
		t.Fatalf("FilePath=%q, want empty", got.FilePath)
	}
}

func TestWriteAll_OSC52Fallback(t *testing.T) {
	t.Cleanup(resetStubs())
	nativeWriteAll = func(_ string) error { return errFake("native failed") }
	osc52WriteAll = func(_ string) error { return nil }

	got, err := WriteAll("hello")
	if err != nil {
		t.Fatalf("WriteAll returned error: %v", err)
	}
	if got.Method != MethodOSC52 {
		t.Fatalf("Method=%q, want %q", got.Method, MethodOSC52)
	}
	if got.FilePath != "" {
		t.Fatalf("FilePath=%q, want empty", got.FilePath)
	}
}

func TestWriteAll_FileFallback(t *testing.T) {
	t.Cleanup(resetStubs())
	nativeWriteAll = func(_ string) error { return errFake("native failed") }
	osc52WriteAll = func(_ string) error { return errFake("osc52 failed") }

	got, err := WriteAll("hello")
	if err != nil {
		t.Fatalf("WriteAll returned error: %v", err)
	}
	if got.Method != MethodFile {
		t.Fatalf("Method=%q, want %q", got.Method, MethodFile)
	}
	if got.FilePath == "" {
		t.Fatalf("FilePath is empty")
	}
	t.Cleanup(func() { _ = os.Remove(got.FilePath) })

	b, err := os.ReadFile(got.FilePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(b) != "hello" {
		t.Fatalf("file contents=%q, want %q", string(b), "hello")
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }

func resetStubs() func() {
	origNative := nativeWriteAll
	origOSC52 := osc52WriteAll
	return func() {
		nativeWriteAll = origNative
		osc52WriteAll = origOSC52
	}
}
