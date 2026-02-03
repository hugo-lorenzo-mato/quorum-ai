package issues

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateOutputPath_ValidFilenames(t *testing.T) {
	baseDir := "/base/issues"

	tests := []struct {
		filename string
		expected string
	}{
		{"01-task.md", filepath.Join(baseDir, "01-task.md")},
		{"consolidated.md", filepath.Join(baseDir, "consolidated.md")},
		{"issue-report.md", filepath.Join(baseDir, "issue-report.md")},
		{"sub/file.md", filepath.Join(baseDir, "sub/file.md")}, // subdirectories allowed
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			result, err := ValidateOutputPath(baseDir, tc.filename)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tc.expected {
				t.Errorf("ValidateOutputPath(%q, %q) = %q, want %q",
					baseDir, tc.filename, result, tc.expected)
			}
		})
	}
}

func TestValidateOutputPath_PathTraversal(t *testing.T) {
	baseDir := "/base/issues"

	tests := []struct {
		filename string
		errType  error
	}{
		{"../etc/passwd", ErrPathTraversal},
		{"../../etc/passwd", ErrPathTraversal},
		{"sub/../../../etc/passwd", ErrPathTraversal},
		{"foo/../../bar", ErrPathTraversal},
		{"..", ErrPathTraversal},
		{"...", ErrPathTraversal}, // Contains ".."
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			_, err := ValidateOutputPath(baseDir, tc.filename)
			if err == nil {
				t.Fatalf("expected error for path traversal attempt: %q", tc.filename)
			}
			if !errors.Is(err, tc.errType) {
				t.Errorf("expected %v error, got: %v", tc.errType, err)
			}
		})
	}
}

func TestValidateOutputPath_AbsolutePath(t *testing.T) {
	baseDir := "/base/issues"

	tests := []string{
		"/etc/passwd",
		"/var/log/test.md",
		"/absolute/path.md",
	}

	for _, filename := range tests {
		t.Run(filename, func(t *testing.T) {
			_, err := ValidateOutputPath(baseDir, filename)
			if err == nil {
				t.Fatalf("expected error for absolute path: %q", filename)
			}
			if !errors.Is(err, ErrAbsolutePath) {
				t.Errorf("expected ErrAbsolutePath, got: %v", err)
			}
		})
	}
}

func TestValidateOutputPath_EmptyFilename(t *testing.T) {
	baseDir := "/base/issues"

	_, err := ValidateOutputPath(baseDir, "")
	if err == nil {
		t.Fatal("expected error for empty filename")
	}
	if !errors.Is(err, ErrInvalidFilename) {
		t.Errorf("expected ErrInvalidFilename, got: %v", err)
	}
}

func TestSanitizeFilename_DangerousCharacters(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Path traversal - filepath.Base extracts just the basename
		{"../etc/passwd", "passwd"},
		// On Linux, backslash is not a path separator so it gets replaced with underscore
		{"..\\windows\\system32", "windows_system32"},

		// Special characters
		{"file:stream", "file_stream"},
		{"file*.md", "file_.md"},
		{"file?.md", "file_.md"},
		{"file<>|.md", "file_.md"},
		{"file\"name\".md", "file_name_.md"},

		// Null byte
		{"file\x00name.md", "file_name.md"},

		// Multiple underscores collapse
		{"file___name.md", "file_name.md"},

		// Leading/trailing underscores trimmed
		{"__file__", "file"},

		// Empty after sanitization
		{"../", "unnamed"},
		{"..", "unnamed"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := SanitizeFilename(tc.input)
			if result != tc.expected {
				t.Errorf("SanitizeFilename(%q) = %q, want %q",
					tc.input, result, tc.expected)
			}
		})
	}
}

func TestSanitizeFilename_PreservesValidNames(t *testing.T) {
	tests := []string{
		"valid-file.md",
		"01-task-name.md",
		"issue_report.md",
		"My Issue File.md",
	}

	for _, filename := range tests {
		t.Run(filename, func(t *testing.T) {
			result := SanitizeFilename(filename)
			// Should be the same (Base only extracts filename)
			expected := filepath.Base(filename)
			if result != expected {
				t.Errorf("SanitizeFilename(%q) = %q, want %q",
					filename, result, expected)
			}
		})
	}
}

func TestSanitizeFilename_ExtractsBasename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/path/to/file.md", "file.md"},
		{"path/to/file.md", "file.md"},
		{"./file.md", "file.md"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := SanitizeFilename(tc.input)
			if result != tc.expected {
				t.Errorf("SanitizeFilename(%q) = %q, want %q",
					tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsValidIssueFilename_Valid(t *testing.T) {
	tests := []string{
		"issue.md",
		"01-task.md",
		"long-issue-name-with-details.md",
		"UPPERCASE.MD",
		"MixedCase.Md",
	}

	for _, filename := range tests {
		t.Run(filename, func(t *testing.T) {
			if !IsValidIssueFilename(filename) {
				t.Errorf("IsValidIssueFilename(%q) = false, want true", filename)
			}
		})
	}
}

func TestIsValidIssueFilename_Invalid(t *testing.T) {
	tests := []struct {
		filename string
		reason   string
	}{
		{"issue.txt", "wrong extension"},
		{"issue", "no extension"},
		{".hidden.md", "hidden file"},
		{"path/to/file.md", "contains path separator"},
		{"path\\to\\file.md", "contains backslash"},
		// Note: "a.md" is 4 chars and valid (minimum is 4)
		// Note: 255 chars is the max valid length
		{strings.Repeat("a", 253) + ".md", "too long (> 255 chars)"}, // 256 chars total
	}

	for _, tc := range tests {
		t.Run(tc.reason, func(t *testing.T) {
			if IsValidIssueFilename(tc.filename) {
				t.Errorf("IsValidIssueFilename(%q) = true, want false (%s)",
					tc.filename, tc.reason)
			}
		})
	}
}

func TestIsValidIssueFilename_LengthBoundaries(t *testing.T) {
	// Exactly 4 characters (minimum valid)
	minValid := "a.md"
	if !IsValidIssueFilename(minValid) {
		t.Errorf("expected %q to be valid (exactly 4 chars)", minValid)
	}

	// 3 characters (too short)
	tooShort := ".md"
	if IsValidIssueFilename(tooShort) {
		t.Errorf("expected %q to be invalid (too short)", tooShort)
	}

	// 255 characters (maximum valid)
	maxValid := strings.Repeat("a", 252) + ".md"
	if !IsValidIssueFilename(maxValid) {
		t.Errorf("expected 255-char filename to be valid")
	}

	// 256 characters (too long)
	tooLong := strings.Repeat("a", 253) + ".md"
	if IsValidIssueFilename(tooLong) {
		t.Errorf("expected 256-char filename to be invalid")
	}
}

func TestValidateAndSanitizeFilename_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"valid-file.md", "valid-file.md"},
		{"01-task.md", "01-task.md"},
		{"file-without-ext", "file-without-ext.md"}, // Adds .md
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result, err := ValidateAndSanitizeFilename(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tc.expected {
				t.Errorf("ValidateAndSanitizeFilename(%q) = %q, want %q",
					tc.input, result, tc.expected)
			}
		})
	}
}

func TestValidateAndSanitizeFilename_SanitizesAndValidates(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"../dangerous", "dangerous.md"},
		{"file*.md", "file_.md"},
		{"path/to/file", "file.md"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result, err := ValidateAndSanitizeFilename(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tc.expected {
				t.Errorf("ValidateAndSanitizeFilename(%q) = %q, want %q",
					tc.input, result, tc.expected)
			}
		})
	}
}

func TestValidateAndSanitizeFilename_InvalidAfterSanitization(t *testing.T) {
	// These inputs become invalid even after sanitization
	tests := []string{
		".", // Becomes "unnamed" which needs .md
		// Note: Most inputs can be made valid, so we mainly test edge cases
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			result, err := ValidateAndSanitizeFilename(input)
			// Should either error or produce valid result
			if err == nil && !IsValidIssueFilename(result) {
				t.Errorf("ValidateAndSanitizeFilename(%q) produced invalid result %q",
					input, result)
			}
		})
	}
}

func TestErrorTypes(t *testing.T) {
	// Test that error types are properly defined and distinguishable
	if ErrPathTraversal == ErrInvalidFilename {
		t.Error("ErrPathTraversal and ErrInvalidFilename should be different")
	}
	if ErrPathTraversal == ErrAbsolutePath {
		t.Error("ErrPathTraversal and ErrAbsolutePath should be different")
	}
	if ErrInvalidFilename == ErrAbsolutePath {
		t.Error("ErrInvalidFilename and ErrAbsolutePath should be different")
	}
}

func TestValidateOutputPath_RealDirectory(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		filename    string
		shouldError bool
	}{
		{"valid.md", false},
		{"subdir/file.md", false},
		{"../escape.md", true},
		{"/absolute/path.md", true},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			_, err := ValidateOutputPath(tempDir, tc.filename)
			if tc.shouldError && err == nil {
				t.Errorf("expected error for %q", tc.filename)
			}
			if !tc.shouldError && err != nil {
				t.Errorf("unexpected error for %q: %v", tc.filename, err)
			}
		})
	}
}

func TestValidateOutputPath_WindowsPaths(t *testing.T) {
	baseDir := "/base/issues"

	// These should be caught regardless of OS
	tests := []string{
		"..\\windows\\system32",
		"C:\\Windows\\System32",
		"file:stream",
	}

	for _, filename := range tests {
		t.Run(filename, func(t *testing.T) {
			_, err := ValidateOutputPath(baseDir, filename)
			// Should either error or be sanitized to safe path
			// The important thing is it doesn't escape baseDir
			if err == nil {
				result, _ := ValidateOutputPath(baseDir, filename)
				if !strings.HasPrefix(result, baseDir) {
					t.Errorf("result %q escapes baseDir %q", result, baseDir)
				}
			}
		})
	}
}

func TestSanitizeFilename_EmptyInput(t *testing.T) {
	// filepath.Base("") returns "." which becomes "unnamed" after sanitization
	result := SanitizeFilename("")
	// On Linux, filepath.Base("") returns ".", which is then preserved
	// The important thing is that empty input doesn't cause panic
	if result == "" {
		t.Error("SanitizeFilename should not return empty string")
	}
}

func TestSanitizeFilename_WhitespaceOnly(t *testing.T) {
	tests := []string{
		"   ",
		"\t\t",
		"\n\n",
		" \t\n ",
	}

	for _, input := range tests {
		t.Run("whitespace", func(t *testing.T) {
			result := SanitizeFilename(input)
			if result == "" {
				t.Errorf("SanitizeFilename should not return empty string")
			}
		})
	}
}

// TestPathTraversalVectors tests various known path traversal attack vectors
func TestPathTraversalVectors(t *testing.T) {
	baseDir := "/base/issues"

	// Common path traversal attack vectors
	vectors := []string{
		"../",
		"..\\",
		"....//",
		"....\\\\",
		"..%2f",
		"..%5c",
		"..%255c",
		"..%c0%af",
		"..%c1%9c",
		"....//....//",
		"..;/",
		"..%00/",
		"..%0d/",
		"..%5c..%5c",
		"..\\..\\..\\",
		"..../",
		"....\\",
		"..%252f",
		"%%32%65%%32%65/",
	}

	for _, vector := range vectors {
		t.Run(vector, func(t *testing.T) {
			result, err := ValidateOutputPath(baseDir, vector)

			// Either should error or result should be safe
			if err == nil {
				// Verify result is within baseDir
				absBase, _ := filepath.Abs(baseDir)
				absResult, _ := filepath.Abs(result)

				if !strings.HasPrefix(absResult, absBase) {
					t.Errorf("path traversal vector %q resulted in escape: %s -> %s",
						vector, baseDir, result)
				}
			}
			// If error, that's also acceptable (blocked)
		})
	}
}
