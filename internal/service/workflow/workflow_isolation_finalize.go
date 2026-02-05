package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// WorkflowIsolationFinalizer performs workflow-level git operations when workflow isolation is enabled.
// This is intentionally best-effort: failures here should not mark the whole workflow as failed,
// because all task results are already present in the workflow branch.
type WorkflowIsolationFinalizer struct {
	Finalization      FinalizationConfig
	GitIsolation      *GitIsolationConfig
	WorkflowWorktrees core.WorkflowWorktreeManager
	Git               core.GitClient
	GitHub            core.GitHubClient
	Logger            *logging.Logger
	Output            OutputNotifier
}

func (f *WorkflowIsolationFinalizer) logWarn(msg string, args ...any) {
	if f != nil && f.Logger != nil {
		f.Logger.Warn(msg, args...)
	}
}

func (f *WorkflowIsolationFinalizer) logInfo(msg string, args ...any) {
	if f != nil && f.Logger != nil {
		f.Logger.Info(msg, args...)
	}
}

func (f *WorkflowIsolationFinalizer) Finalize(ctx context.Context, state *core.WorkflowState) {
	if f == nil || state == nil {
		return
	}

	if f.GitIsolation == nil || !f.GitIsolation.Enabled {
		return
	}
	if f.WorkflowWorktrees == nil {
		return
	}
	if strings.TrimSpace(state.WorkflowBranch) == "" || strings.TrimSpace(string(state.WorkflowID)) == "" {
		return
	}

	cfg := f.Finalization
	remote := cfg.Remote
	if remote == "" {
		remote = "origin"
	}

	workflowID := string(state.WorkflowID)
	workflowBranch := state.WorkflowBranch

	// If configured, push the workflow branch and open a single PR to the base branch.
	// Task-level PRs are disabled under workflow isolation to avoid incorrect targets/noise.
	var prMerged bool
	if cfg.AutoPR {
		if f.Git == nil {
			f.logWarn("workflow isolation: git client not configured, cannot push workflow branch")
		} else {
			if err := f.Git.Push(ctx, remote, workflowBranch); err != nil {
				f.logWarn("workflow isolation: failed to push workflow branch", "branch", workflowBranch, "error", err)
			}
		}

		if f.GitHub == nil {
			f.logWarn("workflow isolation: GitHub client not configured, cannot create workflow PR")
		} else {
			baseBranch := strings.TrimSpace(cfg.PRBaseBranch)
			if baseBranch == "" {
				if b, err := f.GitHub.GetDefaultBranch(ctx); err == nil {
					baseBranch = b
				} else {
					f.logWarn("workflow isolation: failed to detect default branch for PR", "error", err)
				}
			}
			if baseBranch != "" {
				title := fmt.Sprintf("[quorum] Workflow %s", workflowID)
				body := buildWorkflowPRBody(state)
				pr, err := f.GitHub.CreatePR(ctx, core.CreatePROptions{
					Title: title,
					Body:  body,
					Head:  workflowBranch,
					Base:  baseBranch,
				})
				if err != nil {
					f.logWarn("workflow isolation: failed to create workflow PR", "error", err)
				} else {
					f.logInfo("workflow isolation: workflow PR created", "pr_number", pr.Number, "pr_url", pr.HTMLURL)
					if f.Output != nil {
						f.Output.Log("info", "workflow", fmt.Sprintf("Workflow PR created: %s", pr.HTMLURL))
					}

					if cfg.AutoMerge {
						method := cfg.MergeStrategy
						if method == "" {
							method = "squash"
						}
						if err := f.GitHub.MergePR(ctx, pr.Number, core.MergePROptions{
							Method:      method,
							CommitTitle: pr.Title,
						}); err != nil {
							f.logWarn("workflow isolation: auto-merge failed (PR created)", "pr_number", pr.Number, "error", err)
						} else {
							prMerged = true
							f.logInfo("workflow isolation: workflow PR merged", "pr_number", pr.Number)
						}
					}
				}
			}
		}
	}

	// Always cleanup task worktrees/branches under the workflow namespace. Remove the workflow
	// branch too when it has already been merged to base via PR auto-merge.
	if err := f.WorkflowWorktrees.CleanupWorkflow(ctx, workflowID, prMerged); err != nil {
		f.logWarn("workflow isolation: cleanup failed", "workflow_id", workflowID, "error", err)
	}
}

// finalizeWorkflowIsolation wires WorkflowIsolationFinalizer into the Runner lifecycle.
func (r *Runner) finalizeWorkflowIsolation(ctx context.Context, state *core.WorkflowState) {
	if r == nil {
		return
	}
	finalizer := &WorkflowIsolationFinalizer{
		Finalization:      r.config.Finalization,
		GitIsolation:      r.gitIsolation,
		WorkflowWorktrees: r.workflowWorktrees,
		Git:               r.git,
		GitHub:            r.github,
		Logger:            r.logger,
		Output:            r.output,
	}
	finalizer.Finalize(ctx, state)
}

func buildWorkflowPRBody(state *core.WorkflowState) string {
	var b strings.Builder

	b.WriteString("## Prompt\n\n")
	b.WriteString(state.Prompt)
	b.WriteString("\n\n")

	if len(state.TaskOrder) > 0 {
		b.WriteString("## Tasks\n\n")
		for _, id := range state.TaskOrder {
			ts := state.Tasks[id]
			if ts == nil {
				continue
			}
			b.WriteString(fmt.Sprintf("- %s (`%s`)\n", ts.Name, ts.ID))
		}
		b.WriteString("\n")
	}

	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("Workflow ID: `%s`\n", state.WorkflowID))
	b.WriteString("Generated by quorum-ai\n")

	return b.String()
}
