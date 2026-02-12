package workflow

import (
	"context"
	"os"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// tryRecoverFromOutputFile checks if a previous agent attempt left a valid output file.
// Returns the recovered result if successful, nil otherwise.
func tryRecoverFromOutputFile(absPath, model string, validator func(string) bool) *core.ExecuteResult {
	if absPath == "" {
		return nil
	}
	info, statErr := os.Stat(absPath)
	if statErr != nil || info.Size() <= 1024 {
		return nil
	}
	content, readErr := os.ReadFile(absPath) // #nosec G304 -- path constructed from internal report directory
	if readErr != nil {
		return nil
	}
	if !validator(string(content)) {
		return nil
	}
	return &core.ExecuteResult{Output: string(content), Model: model}
}

// launchWatchdogRecovery sets up a watchdog goroutine for output file monitoring.
// Returns the watchdog and a channel that receives stable content.
// If absOutputPath is empty, returns nil, nil.
func launchWatchdogRecovery(attemptCtx context.Context, absOutputPath string, wctx *Context) (*OutputWatchdog, chan string) {
	if absOutputPath == "" {
		return nil, nil
	}
	watchdog := NewOutputWatchdog(absOutputPath, DefaultWatchdogConfig(), wctx.Logger)
	watchdog.Start()

	stableOutputCh := make(chan string, 1)
	go func() {
		select {
		case content := <-watchdog.StableCh():
			select {
			case stableOutputCh <- content:
			default:
			}
		case <-attemptCtx.Done():
		}
	}()

	return watchdog, stableOutputCh
}

// reapWatchdogOutput checks the watchdog channel for stable output after an execution error.
// Returns recovered result and nil error if successful, otherwise returns nil and the original error.
func reapWatchdogOutput(stableOutputCh chan string, model string, validator func(string) bool, wctx *Context, agentName, absPath string) (*core.ExecuteResult, bool) {
	if stableOutputCh == nil {
		return nil, false
	}
	select {
	case content := <-stableOutputCh:
		if !validator(content) {
			wctx.Logger.Warn("watchdog reap: stable file rejected (unstructured content)",
				"agent", agentName, "path", absPath, "size", len(content))
			return nil, false
		}
		wctx.Logger.Info("watchdog reap: using stable output file",
			"agent", agentName, "path", absPath, "size", len(content))
		return &core.ExecuteResult{Output: content, Model: model}, true
	default:
		return nil, false
	}
}

// recoverOutputFromFile handles post-execution output recovery when stdout is empty
// and quality gate validation when stdout contains unstructured content.
// Returns the potentially updated result.
func recoverOutputFromFile(wctx *Context, result *core.ExecuteResult, absOutputPath, model, agentName string, validator func(string) bool) *core.ExecuteResult {
	// Some CLIs write to file but return empty stdout. Prefer the file content.
	if absOutputPath != "" && (result == nil || strings.TrimSpace(result.Output) == "") {
		content, readErr := os.ReadFile(absOutputPath) // #nosec G304 -- path constructed from internal report directory
		if readErr == nil && strings.TrimSpace(string(content)) != "" && validator(string(content)) {
			if result == nil {
				result = &core.ExecuteResult{}
			}
			result.Output = string(content)
			if result.Model == "" {
				result.Model = model
			}
			wctx.Logger.Info("file enforcement: using output file content (stdout empty)",
				"agent", agentName, "path", absOutputPath, "size", len(content))
		}
	}

	// Quality gate: reject outputs that look like intermediate narration.
	// If stdout fails but the output file contains valid content, prefer the file.
	if result != nil && strings.TrimSpace(result.Output) != "" && !validator(result.Output) {
		if absOutputPath != "" {
			content, readErr := os.ReadFile(absOutputPath) // #nosec G304 -- path constructed from internal report directory
			if readErr == nil {
				fileRaw := string(content)
				if strings.TrimSpace(fileRaw) != "" && validator(fileRaw) {
					wctx.Logger.Warn("stdout rejected; using structured output file content instead",
						"agent", agentName,
						"path", absOutputPath,
						"stdout_size", len(result.Output),
						"file_size", len(fileRaw),
					)
					result.Output = fileRaw
				}
			}
		}
	}

	return result
}

// enforceOutputFile ensures the output file exists on disk via VerifyOrWriteFallback.
func enforceOutputFile(wctx *Context, absOutputPath, output string) {
	if absOutputPath == "" || output == "" {
		return
	}
	enforcement := NewFileEnforcement(wctx.Logger)
	createdByLLM, verifyErr := enforcement.VerifyOrWriteFallback(absOutputPath, output)
	if verifyErr != nil {
		wctx.Logger.Warn("file enforcement failed", "path", absOutputPath, "error", verifyErr)
	} else if !createdByLLM {
		wctx.Logger.Debug("created fallback file from stdout", "path", absOutputPath)
	}
}
