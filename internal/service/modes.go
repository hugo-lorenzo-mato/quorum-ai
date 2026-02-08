package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// ExecutionMode represents the execution mode.
type ExecutionMode struct {
	DryRun      bool
	Yolo        bool
	DeniedTools []string
	Interactive bool
}

// DefaultMode returns the default execution mode.
func DefaultMode() ExecutionMode {
	return ExecutionMode{
		DryRun:      false,
		Yolo:        false,
		DeniedTools: nil,
		Interactive: true,
	}
}

// Validate validates the mode configuration.
func (m ExecutionMode) Validate() error {
	return nil
}

// ModeEnforcer enforces execution mode constraints.
type ModeEnforcer struct {
	mode         ExecutionMode
	operationLog []string
}

// NewModeEnforcer creates a new mode enforcer.
func NewModeEnforcer(mode ExecutionMode) *ModeEnforcer {
	return &ModeEnforcer{
		mode:         mode,
		operationLog: make([]string, 0),
	}
}

// CanExecute checks if an operation can be executed.
func (e *ModeEnforcer) CanExecute(_ context.Context, op Operation) error {
	// Check dry-run
	if e.mode.DryRun && op.HasSideEffects {
		e.logOperation("BLOCKED (dry-run)", op)
		return ErrDryRunBlocked{Operation: op.Name}
	}

	// Check denied tools
	if e.isToolDenied(op.Tool) {
		e.logOperation("BLOCKED (denied)", op)
		return core.ErrValidation("TOOL_DENIED",
			fmt.Sprintf("tool %s is denied", op.Tool))
	}

	e.logOperation("ALLOWED", op)
	return nil
}

// RequiresConfirmation checks if an operation needs user confirmation.
func (e *ModeEnforcer) RequiresConfirmation(op Operation) bool {
	if e.mode.Yolo {
		return false
	}

	if e.mode.DryRun {
		return false // No confirmation needed in dry-run
	}

	return op.RequiresConfirmation
}

// Mode returns the current execution mode.
func (e *ModeEnforcer) Mode() ExecutionMode {
	return e.mode
}

// isToolDenied checks if a tool is in the deny list.
func (e *ModeEnforcer) isToolDenied(tool string) bool {
	for _, denied := range e.mode.DeniedTools {
		if strings.EqualFold(denied, tool) {
			return true
		}
	}
	return false
}

// logOperation logs an operation check.
func (e *ModeEnforcer) logOperation(status string, op Operation) {
	e.operationLog = append(e.operationLog,
		fmt.Sprintf("[%s] %s: %s", status, op.Type, op.Name))
}

// GetOperationLog returns the operation log.
func (e *ModeEnforcer) GetOperationLog() []string {
	return append([]string{}, e.operationLog...)
}

// Operation represents an operation to be executed.
type Operation struct {
	Name                 string
	Type                 OperationType
	Tool                 string
	HasSideEffects       bool
	RequiresConfirmation bool
	InWorkspace          bool
	IsDestructive        bool
}

// OperationType categorizes operations.
type OperationType string

const (
	OpTypeLLM       OperationType = "llm"
	OpTypeFileRead  OperationType = "file_read"
	OpTypeFileWrite OperationType = "file_write"
	OpTypeGit       OperationType = "git"
	OpTypeNetwork   OperationType = "network"
	OpTypeShell     OperationType = "shell"
)

// ErrDryRunBlocked indicates an operation was blocked by dry-run mode.
type ErrDryRunBlocked struct {
	Operation string
}

func (e ErrDryRunBlocked) Error() string {
	return fmt.Sprintf("operation %s blocked by dry-run mode", e.Operation)
}

// IsDryRunBlocked checks if an error is a dry-run block.
func IsDryRunBlocked(err error) bool {
	_, ok := err.(ErrDryRunBlocked)
	return ok
}
