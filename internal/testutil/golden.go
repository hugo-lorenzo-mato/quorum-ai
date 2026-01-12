package testutil

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

// Golden provides golden file testing utilities.
type Golden struct {
	t       *testing.T
	baseDir string
}

// NewGolden creates a new golden file helper.
func NewGolden(t *testing.T, baseDir string) *Golden {
	return &Golden{
		t:       t,
		baseDir: baseDir,
	}
}

// Assert compares actual output against golden file.
func (g *Golden) Assert(name string, actual []byte) {
	g.t.Helper()

	goldenPath := filepath.Join(g.baseDir, name+".golden")

	if *update {
		g.updateGolden(goldenPath, actual)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		g.t.Fatalf("reading golden file %s: %v", goldenPath, err)
	}

	if string(actual) != string(expected) {
		g.t.Errorf("output mismatch for %s:\n--- expected ---\n%s\n--- actual ---\n%s",
			name, expected, actual)
	}
}

// AssertString compares string output against golden file.
func (g *Golden) AssertString(name, actual string) {
	g.Assert(name, []byte(actual))
}

// updateGolden updates the golden file with actual output.
func (g *Golden) updateGolden(path string, actual []byte) {
	g.t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		g.t.Fatalf("creating golden directory: %v", err)
	}

	if err := os.WriteFile(path, actual, 0644); err != nil {
		g.t.Fatalf("writing golden file: %v", err)
	}

	g.t.Logf("updated golden file: %s", path)
}

// Normalize normalizes output for comparison.
func Normalize(s string) string {
	// Normalize line endings
	s = strings.ReplaceAll(s, "\r\n", "\n")

	// Remove trailing whitespace from lines
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}

	// Remove trailing newlines
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// ScrubTimestamps removes timestamps from output.
func ScrubTimestamps(s string) string {
	patterns := []string{
		`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[^\s]*`, // ISO format with timezone
		`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`,       // Standard format
		`\d{2}:\d{2}:\d{2}`,                          // Time only
	}

	result := s
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllString(result, "[TIMESTAMP]")
	}

	return result
}

// ScrubDurations removes durations from output.
func ScrubDurations(s string) string {
	// Duration patterns like "1.234s", "5m30s", "2h15m"
	re := regexp.MustCompile(`\d+(\.\d+)?(ns|us|µs|ms|s|m|h)+`)
	return re.ReplaceAllString(s, "[DURATION]")
}

// ScrubPaths normalizes file paths.
func ScrubPaths(s, basePath string) string {
	return strings.ReplaceAll(s, basePath, "[WORKDIR]")
}

// ScrubUUIDs removes UUIDs from output.
func ScrubUUIDs(s string) string {
	re := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	return re.ReplaceAllString(s, "[UUID]")
}

// ScrubHashes removes git hashes from output.
func ScrubHashes(s string) string {
	re := regexp.MustCompile(`[0-9a-f]{40}`)
	return re.ReplaceAllString(s, "[HASH]")
}

// ScrubAll applies all scrubbing functions.
func ScrubAll(s, basePath string) string {
	result := s
	result = ScrubTimestamps(result)
	result = ScrubDurations(result)
	result = ScrubPaths(result, basePath)
	result = ScrubUUIDs(result)
	result = ScrubHashes(result)
	return Normalize(result)
}
