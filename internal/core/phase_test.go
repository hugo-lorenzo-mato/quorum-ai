package core

import "testing"

func TestPhase_Order(t *testing.T) {
	if PhaseOrder(PhaseOptimize) != 0 {
		t.Fatalf("expected optimize order 0")
	}
	if PhaseOrder(PhaseAnalyze) != 1 {
		t.Fatalf("expected analyze order 1")
	}
	if PhaseOrder(PhasePlan) != 2 {
		t.Fatalf("expected plan order 2")
	}
	if PhaseOrder(PhaseExecute) != 3 {
		t.Fatalf("expected execute order 3")
	}
	if PhaseOrder("invalid") != -1 {
		t.Fatalf("expected invalid phase order -1")
	}
}

func TestPhase_Navigation(t *testing.T) {
	if NextPhase(PhaseOptimize) != PhaseAnalyze {
		t.Fatalf("expected next optimize to be analyze")
	}
	if NextPhase(PhaseAnalyze) != PhasePlan {
		t.Fatalf("expected next analyze to be plan")
	}
	if NextPhase(PhasePlan) != PhaseExecute {
		t.Fatalf("expected next plan to be execute")
	}
	if NextPhase(PhaseExecute) != "" {
		t.Fatalf("expected no next phase after execute")
	}

	if PrevPhase(PhaseAnalyze) != PhaseOptimize {
		t.Fatalf("expected prev analyze to be optimize")
	}
	if PrevPhase(PhasePlan) != PhaseAnalyze {
		t.Fatalf("expected prev plan to be analyze")
	}
	if PrevPhase(PhaseExecute) != PhasePlan {
		t.Fatalf("expected prev execute to be plan")
	}
	if PrevPhase(PhaseOptimize) != "" {
		t.Fatalf("expected no prev phase before optimize")
	}
}

func TestPhase_Validation(t *testing.T) {
	for _, phase := range AllPhases() {
		if !ValidPhase(phase) {
			t.Fatalf("expected phase %s to be valid", phase)
		}
	}
	if ValidPhase("invalid") {
		t.Fatalf("expected invalid phase to be rejected")
	}
}

func TestPhase_Parse(t *testing.T) {
	p, err := ParsePhase("plan")
	if err != nil {
		t.Fatalf("unexpected error parsing phase: %v", err)
	}
	if p != PhasePlan {
		t.Fatalf("expected plan phase, got %s", p)
	}

	if _, err := ParsePhase("unknown"); err == nil {
		t.Fatalf("expected error parsing invalid phase")
	}
}

func TestPhase_Description(t *testing.T) {
	tests := []struct {
		phase Phase
		want  string
	}{
		{PhaseOptimize, "Optimize the user prompt for better LLM understanding"},
		{PhaseAnalyze, "Analyze the problem with multiple agents"},
		{PhasePlan, "Generate and consolidate execution plans"},
		{PhaseExecute, "Execute tasks in isolated environments"},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			got := tt.phase.Description()
			if got != tt.want {
				t.Errorf("Description() = %q, want %q", got, tt.want)
			}
		})
	}

	// Test unknown phase
	unknown := Phase("unknown")
	if unknown.Description() != "Unknown phase" {
		t.Errorf("Unknown phase description should be 'Unknown phase', got %q", unknown.Description())
	}
}
