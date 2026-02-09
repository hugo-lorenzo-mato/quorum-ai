package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

const (
	// Keep in sync with watchdog defaults; prevents treating tiny/partial files as "done".
	minReconcileConsolidatedSizeBytes int64 = 512
)

// reconcileAnalysisArtifacts repairs checkpoints based on on-disk artifacts.
//
// Problem: resume/retry decisions are checkpoint-based; if an agent wrote the markdown file
// but the process crashed/hung before emitting checkpoints, the workflow may re-run phases.
//
// Policy implemented here:
//   - If `analyze-phase/consolidated.md` exists and is non-trivial, ensure there is a
//     `consolidated_analysis` checkpoint with its content.
//   - If consolidated analysis exists, ensure there is a `phase_complete` checkpoint for Analyze.
//
// This is intentionally narrow (Analyze only) to avoid unintended side effects.
func (r *Runner) reconcileAnalysisArtifacts(ctx context.Context, state *core.WorkflowState) error {
	if r == nil || state == nil {
		return nil
	}

	// Compute expected consolidated analysis path even if ReportPath wasn't persisted (API-created workflows).
	reportPath := state.ReportPath
	if reportPath == "" && r.config != nil && r.config.Report.BaseDir != "" && state.WorkflowID != "" {
		reportPath = filepath.Join(r.config.Report.BaseDir, string(state.WorkflowID))
	}
	if reportPath == "" {
		return nil
	}

	// consolidated.md lives under analyze-phase.
	consolidatedPath := filepath.Join(reportPath, "analyze-phase", "consolidated.md")
	absConsolidatedPath := consolidatedPath
	if !filepath.IsAbs(absConsolidatedPath) && r.projectRoot != "" {
		absConsolidatedPath = filepath.Join(r.projectRoot, absConsolidatedPath)
	}

	info, err := os.Stat(absConsolidatedPath)
	if err != nil || info.IsDir() || info.Size() < minReconcileConsolidatedSizeBytes {
		return nil
	}

	raw, err := os.ReadFile(absConsolidatedPath)
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("artifact reconcile: failed to read consolidated analysis",
				"path", absConsolidatedPath,
				"error", err,
			)
		}
		return nil
	}
	content := strings.TrimSpace(string(raw))
	if content == "" {
		return nil
	}

	if !isValidAnalysisOutput(content) {
		if r.logger != nil {
			r.logger.Warn("artifact reconcile: consolidated file is not valid analysis (skeleton?)",
				"path", absConsolidatedPath, "size", len(content))
		}
		return nil
	}

	// 1) Ensure consolidated_analysis checkpoint exists (or is non-empty).
	// Planner/Executor only need the "content" field.
	if strings.TrimSpace(GetConsolidatedAnalysis(state)) == "" {
		if r.checkpoint == nil {
			return fmt.Errorf("artifact reconcile: checkpoint manager not configured")
		}
		if err := r.checkpoint.CreateCheckpoint(state, "consolidated_analysis", map[string]interface{}{
			"content":     content,
			"reconciled":  true,
			"source_path": consolidatedPath,
			"timestamp":   time.Now().Format(time.RFC3339),
		}); err != nil {
			return fmt.Errorf("artifact reconcile: creating consolidated_analysis checkpoint: %w", err)
		}
		if r.output != nil {
			r.output.Log("info", "workflow", "Recovered consolidated analysis from disk (reconciled checkpoints)")
		}
	}

	// 2) Ensure analyze phase is marked complete so checkpoint-based resume advances.
	if !isPhaseCompleted(state, core.PhaseAnalyze) {
		if r.checkpoint == nil {
			return fmt.Errorf("artifact reconcile: checkpoint manager not configured")
		}
		if err := r.checkpoint.PhaseCheckpoint(state, core.PhaseAnalyze, true); err != nil {
			return fmt.Errorf("artifact reconcile: creating analyze phase_complete checkpoint: %w", err)
		}
	}

	// Persist reconciliation immediately so retries/resumes become deterministic.
	if r.state != nil {
		if saveErr := r.state.Save(ctx, state); saveErr != nil {
			if r.logger != nil {
				r.logger.Warn("artifact reconcile: failed to save reconciled state", "error", saveErr)
			}
		}
	}

	return nil
}
