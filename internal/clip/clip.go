package clip

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	atotto "github.com/atotto/clipboard"
	osc52 "github.com/aymanbagabas/go-osc52/v2"
	"golang.org/x/term"
)

// Method represents the mechanism used to make content copyable.
//
// Note: MethodFile means we couldn't access the clipboard, but we still made the
// content available in a temp file.
type Method string

const (
	MethodNative Method = "native" // OS/native clipboard via github.com/atotto/clipboard
	MethodOSC52  Method = "osc52"  // Terminal clipboard via OSC52 escape sequence
	MethodFile   Method = "file"   // Temp file fallback
)

type Result struct {
	Method   Method
	FilePath string // only set when Method == MethodFile
}

// These vars exist for testability.
var (
	nativeWriteAll = func(text string) error { return atotto.WriteAll(text) }
	osc52WriteAll  = writeAllOSC52
)

// WriteAll tries to copy text to the clipboard.
//
// Strategy:
//  1. Native clipboard (atotto/clipboard)
//  2. OSC52 terminal clipboard (works well in WSL without Windows interop)
//  3. Temp file fallback
func WriteAll(text string) (Result, error) {
	if err := nativeWriteAll(text); err == nil {
		return Result{Method: MethodNative}, nil
	}

	if err := osc52WriteAll(text); err == nil {
		return Result{Method: MethodOSC52}, nil
	}

	path, err := writeTempFile(text)
	if err != nil {
		return Result{}, err
	}

	return Result{Method: MethodFile, FilePath: path}, nil
}

// Conservative default; terminals can have strict OSC52 limits.
const osc52LimitBytes = 100_000

func writeAllOSC52(text string) error {
	if text == "" {
		return errors.New("empty clipboard text")
	}
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return errors.New("stderr is not a terminal")
	}
	// Avoid sending huge OSC52 payloads that terminals may drop or block.
	if len(text) > osc52LimitBytes {
		return fmt.Errorf("text too large for OSC52 (%d bytes > %d)", len(text), osc52LimitBytes)
	}

	seq := osc52.New(text).Limit(osc52LimitBytes)
	if os.Getenv("TMUX") != "" {
		seq = seq.Tmux()
	} else if os.Getenv("STY") != "" {
		seq = seq.Screen()
	}

	// Use stderr to avoid interfering with Bubble Tea's stdout renderer.
	_, err := seq.WriteTo(os.Stderr)
	return err
}

func writeTempFile(text string) (string, error) {
	f, err := os.CreateTemp("", fmt.Sprintf("quorum-ai-clipboard-%d-*.txt", time.Now().UnixNano()))
	if err != nil {
		return "", err
	}
	path := f.Name()
	// Best-effort cleanup on error.
	defer func() {
		_ = f.Close()
		if err != nil {
			_ = os.Remove(path)
		}
	}()

	if _, err = f.WriteString(text); err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}

	// Make the path a bit friendlier to read/copy (no functional changes).
	return filepath.Clean(path), nil
}
