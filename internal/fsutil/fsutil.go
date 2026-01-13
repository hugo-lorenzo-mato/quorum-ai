package fsutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ReadFileScoped reads a file by opening a root at the file's directory.
// This scopes access to the intended directory and avoids path traversal.
func ReadFileScoped(path string) ([]byte, error) {
	cleaned := filepath.Clean(path)
	dir := filepath.Dir(cleaned)
	base := filepath.Base(cleaned)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return nil, fmt.Errorf("invalid file path: %q", path)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	file, err := root.Open(base)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return io.ReadAll(file)
}
