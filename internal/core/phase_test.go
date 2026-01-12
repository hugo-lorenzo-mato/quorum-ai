package core

import "testing"

func TestPhase_Order(t *testing.T) {
	if PhaseOrder(PhaseAnalyze) != 0 {
		t.Fatalf("expected analyze order 0")
	}
	if PhaseOrder(PhasePlan) != 1 {
		t.Fatalf("expected plan order 1")
	}
	if PhaseOrder(PhaseExecute) != 2 {
		t.Fatalf("expected execute order 2")
	}
	if PhaseOrder("invalid") != -1 {
		t.Fatalf("expected invalid phase order -1")
	}
}

func TestPhase_Navigation(t *testing.T) {
	if NextPhase(PhaseAnalyze) != PhasePlan {
		t.Fatalf("expected next analyze to be plan")
	}
	if NextPhase(PhasePlan) != PhaseExecute {
		t.Fatalf("expected next plan to be execute")
	}
	if NextPhase(PhaseExecute) != "" {
		t.Fatalf("expected no next phase after execute")
	}

	if PrevPhase(PhasePlan) != PhaseAnalyze {
		t.Fatalf("expected prev plan to be analyze")
	}
	if PrevPhase(PhaseExecute) != PhasePlan {
		t.Fatalf("expected prev execute to be plan")
	}
	if PrevPhase(PhaseAnalyze) != "" {
		t.Fatalf("expected no prev phase before analyze")
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
