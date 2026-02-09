package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

func TestAnalyzer_FinalizeVnRefinementResult_PrefersFileWhenStdoutUnstructured(t *testing.T) {
	t.Parallel()

	// Valid, structured analysis written to disk (simulating tool-written output).
	validFileContent := "# Title\n\n## Section\n\nThis is real analysis content.\n"
	dir := t.TempDir()
	outPath := filepath.Join(dir, "claude.md")
	if err := os.WriteFile(outPath, []byte(validFileContent), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Unstructured stdout (no markdown headers) that would normally be rejected.
	unstructuredStdout := strings.Repeat("log line without headers\n", 120) // > 1KB
	if isValidAnalysisOutput(unstructuredStdout) {
		t.Fatalf("test setup invalid: expected stdout to be rejected by isValidAnalysisOutput")
	}

	analyzer := &Analyzer{}
	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-test",
				Prompt:     "test prompt",
			},
			WorkflowRun: core.WorkflowRun{
				CurrentPhase: core.PhaseAnalyze,
				Tasks:        make(map[core.TaskID]*core.TaskState),
				TaskOrder:    []core.TaskID{},
				Checkpoints:  []core.Checkpoint{},
				Metrics:      &core.StateMetrics{},
			},
		},
		Checkpoint: &mockCheckpointCreator{},
		Logger:     logging.NewNop(),
	}

	setup := &VnRefinementSetup{
		model:          "claude-test-model",
		outputFilePath: outPath,
		absOutputPath:  outPath,
		promptHash:     "prompt-hash",
	}

	result := &core.ExecuteResult{
		Output: unstructuredStdout,
		Model:  "claude-test-model",
	}

	got, err := analyzer.finalizeVnRefinementResult(wctx, "claude", 2, setup, result)
	if err != nil {
		t.Fatalf("finalizeVnRefinementResult() error = %v", err)
	}
	if got.RawOutput != validFileContent {
		t.Fatalf("RawOutput did not come from file fallback\n got: %q\nwant: %q", got.RawOutput, validFileContent)
	}
}
