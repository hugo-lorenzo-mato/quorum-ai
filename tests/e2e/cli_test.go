//go:build e2e

package e2e_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

var goldenDir = filepath.Join("..", "..", "testdata", "golden")

func TestCLI_Help(t *testing.T) {
	binary := buildBinary(t)

	cmd := exec.Command(binary, "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v\noutput: %s", err, output)
	}

	golden := testutil.NewGolden(t, goldenDir)
	golden.AssertString("help", testutil.Normalize(string(output)))
}

func TestCLI_Version(t *testing.T) {
	binary := buildBinary(t)

	cmd := exec.Command(binary, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v\noutput: %s", err, output)
	}

	// Scrub version-specific info
	scrubbed := testutil.ScrubTimestamps(string(output))
	scrubbed = regexp.MustCompile(`version \S+`).ReplaceAllString(scrubbed, "version [VERSION]")
	scrubbed = regexp.MustCompile(`commit: \S+`).ReplaceAllString(scrubbed, "commit: [COMMIT]")
	scrubbed = regexp.MustCompile(`Version:\s+\S+`).ReplaceAllString(scrubbed, "Version: [VERSION]")
	scrubbed = regexp.MustCompile(`Commit:\s+\S+`).ReplaceAllString(scrubbed, "Commit: [COMMIT]")
	scrubbed = regexp.MustCompile(`Date:\s+\S+`).ReplaceAllString(scrubbed, "Date: [DATE]")

	golden := testutil.NewGolden(t, goldenDir)
	golden.AssertString("version", testutil.Normalize(scrubbed))
}

func TestCLI_Init(t *testing.T) {
	binary := buildBinary(t)
	dir := testutil.TempDir(t)

	cmd := exec.Command(binary, "init")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v\noutput: %s", err, output)
	}

	// Verify files created
	configPath := filepath.Join(dir, ".quorum", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file not created")
	}

	stateDir := filepath.Join(dir, ".quorum", "state")
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		t.Fatal("state directory not created")
	}

	scrubbed := testutil.ScrubPaths(string(output), dir)
	golden := testutil.NewGolden(t, goldenDir)
	golden.AssertString("init", testutil.Normalize(scrubbed))
}

func TestCLI_Doctor(t *testing.T) {
	binary := buildBinary(t)

	cmd := exec.Command(binary, "doctor")
	output, _ := cmd.CombinedOutput() // May fail if deps missing, that's ok

	golden := testutil.NewGolden(t, goldenDir)
	golden.AssertString("doctor", testutil.Normalize(string(output)))
}

func TestCLI_Status_NoWorkflow(t *testing.T) {
	binary := buildBinary(t)
	dir := testutil.TempDir(t)

	// Init project first
	initCmd := exec.Command(binary, "init")
	initCmd.Dir = dir
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\n%s", err, out)
	}

	// Check status with no workflow
	cmd := exec.Command(binary, "status")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v\noutput: %s", err, output)
	}

	golden := testutil.NewGolden(t, goldenDir)
	golden.AssertString("status_no_workflow", testutil.Normalize(string(output)))
}

func TestCLI_Run_DryRun(t *testing.T) {
	t.Skip("Skipping hanging test due to configuration validation issues in CI environment")
	binary := buildBinary(t)
	dir := testutil.TempDir(t)

	// Init project first
	initCmd := exec.Command(binary, "init")
	initCmd.Dir = dir
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\n%s", err, out)
	}

	// Run with dry-run flag
	cmd := exec.Command(binary, "run", "--dry-run", "--output", "plain", "test prompt")
	cmd.Dir = dir
	output, _ := cmd.CombinedOutput() // May have errors without LLM providers

	scrubbed := testutil.ScrubAll(string(output), dir)
	golden := testutil.NewGolden(t, goldenDir)
	golden.AssertString("run_dryrun", scrubbed)
}

// buildBinary builds the CLI binary for testing.
func buildBinary(t *testing.T) string {
	t.Helper()

	// Build to a temp location
	binary := filepath.Join(t.TempDir(), "quorum")

	cmd := exec.Command("go", "build", "-o", binary, "../../cmd/quorum")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, stderr.String())
	}

	return binary
}
