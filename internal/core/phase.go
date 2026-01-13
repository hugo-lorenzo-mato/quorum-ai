package core

import "fmt"

// Phase represents a stage in the workflow execution.
type Phase string

const (
	// PhaseOptimize is the first phase where the user prompt is optimized.
	// An LLM enhances the prompt for clarity and effectiveness.
	PhaseOptimize Phase = "optimize"

	// PhaseAnalyze is the second phase where agents analyze the problem.
	// Multiple agents provide independent analyses (V1/V2).
	PhaseAnalyze Phase = "analyze"

	// PhasePlan is the third phase where agents create execution plans.
	// Plans are consolidated based on consensus.
	PhasePlan Phase = "plan"

	// PhaseExecute is the final phase where tasks are executed.
	// Each task runs in isolated git worktrees.
	PhaseExecute Phase = "execute"
)

// AllPhases returns all phases in execution order.
func AllPhases() []Phase {
	return []Phase{PhaseOptimize, PhaseAnalyze, PhasePlan, PhaseExecute}
}

// PhaseOrder returns the numeric order of a phase (0-indexed).
func PhaseOrder(p Phase) int {
	switch p {
	case PhaseOptimize:
		return 0
	case PhaseAnalyze:
		return 1
	case PhasePlan:
		return 2
	case PhaseExecute:
		return 3
	default:
		return -1
	}
}

// NextPhase returns the phase following the given phase.
// Returns empty string if current phase is the last.
func NextPhase(p Phase) Phase {
	switch p {
	case PhaseOptimize:
		return PhaseAnalyze
	case PhaseAnalyze:
		return PhasePlan
	case PhasePlan:
		return PhaseExecute
	default:
		return ""
	}
}

// PrevPhase returns the phase preceding the given phase.
// Returns empty string if current phase is the first.
func PrevPhase(p Phase) Phase {
	switch p {
	case PhaseAnalyze:
		return PhaseOptimize
	case PhasePlan:
		return PhaseAnalyze
	case PhaseExecute:
		return PhasePlan
	default:
		return ""
	}
}

// ValidPhase checks if a phase string is valid.
func ValidPhase(p Phase) bool {
	switch p {
	case PhaseOptimize, PhaseAnalyze, PhasePlan, PhaseExecute:
		return true
	default:
		return false
	}
}

// ParsePhase converts a string to a Phase with validation.
func ParsePhase(s string) (Phase, error) {
	p := Phase(s)
	if !ValidPhase(p) {
		return "", fmt.Errorf("invalid phase: %s", s)
	}
	return p, nil
}

// String returns the string representation of the phase.
func (p Phase) String() string {
	return string(p)
}

// Description returns a human-readable description of the phase.
func (p Phase) Description() string {
	switch p {
	case PhaseOptimize:
		return "Optimize the user prompt for better LLM understanding"
	case PhaseAnalyze:
		return "Analyze the problem with multiple agents"
	case PhasePlan:
		return "Generate and consolidate execution plans"
	case PhaseExecute:
		return "Execute tasks in isolated environments"
	default:
		return "Unknown phase"
	}
}
