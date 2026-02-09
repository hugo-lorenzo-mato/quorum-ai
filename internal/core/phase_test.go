package core

import "testing"

func TestPhase_Order(t *testing.T) {
	t.Parallel()
	if PhaseOrder(PhaseRefine) != 0 {
		t.Fatalf("expected refine order 0")
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
	if PhaseOrder(PhaseDone) != 4 {
		t.Fatalf("expected done order 4")
	}
	if PhaseOrder("invalid") != -1 {
		t.Fatalf("expected invalid phase order -1")
	}
}

func TestPhase_Navigation(t *testing.T) {
	t.Parallel()
	if NextPhase(PhaseRefine) != PhaseAnalyze {
		t.Fatalf("expected next refine to be analyze")
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

	if PrevPhase(PhaseAnalyze) != PhaseRefine {
		t.Fatalf("expected prev analyze to be refine")
	}
	if PrevPhase(PhasePlan) != PhaseAnalyze {
		t.Fatalf("expected prev plan to be analyze")
	}
	if PrevPhase(PhaseExecute) != PhasePlan {
		t.Fatalf("expected prev execute to be plan")
	}
	if PrevPhase(PhaseDone) != PhaseExecute {
		t.Fatalf("expected prev done to be execute")
	}
	if PrevPhase(PhaseRefine) != "" {
		t.Fatalf("expected no prev phase before refine")
	}
}

func TestPhase_Validation(t *testing.T) {
	t.Parallel()
	for _, phase := range AllPhases() {
		if !ValidPhase(phase) {
			t.Fatalf("expected phase %s to be valid", phase)
		}
	}
	// PhaseDone is valid but not in AllPhases (not an executable phase)
	if !ValidPhase(PhaseDone) {
		t.Fatalf("expected done phase to be valid")
	}
	if ValidPhase("invalid") {
		t.Fatalf("expected invalid phase to be rejected")
	}
}

func TestPhase_Parse(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	tests := []struct {
		phase Phase
		want  string
	}{
		{PhaseRefine, "Refine the user prompt for better LLM understanding"},
		{PhaseAnalyze, "Analyze the problem with multiple agents"},
		{PhasePlan, "Generate and consolidate execution plans"},
		{PhaseExecute, "Execute tasks in isolated environments"},
		{PhaseDone, "All phases completed"},
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
