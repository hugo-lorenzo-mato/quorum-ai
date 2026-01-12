package core

import "testing"

func TestArtifact_Builder(t *testing.T) {
	artifact := NewArtifact("a1", ArtifactTypeAnalysis, "t1")
	if artifact.ID != "a1" || artifact.Type != ArtifactTypeAnalysis || artifact.TaskID != "t1" {
		t.Fatalf("unexpected artifact fields: %+v", artifact)
	}
	if artifact.Metadata == nil {
		t.Fatalf("expected metadata map to be initialized")
	}

	artifact.WithContent("data").WithPath("/tmp/file").WithPhase(PhaseAnalyze).WithMetadata("k", "v")
	if artifact.Size != int64(len("data")) {
		t.Fatalf("expected size to match content length")
	}
	if artifact.FileName() != "file" {
		t.Fatalf("expected file name to be derived from path")
	}
	if !artifact.IsFile() {
		t.Fatalf("expected artifact to be file-based")
	}
	if artifact.Metadata["k"] != "v" {
		t.Fatalf("expected metadata to include key")
	}
	if artifact.Phase != PhaseAnalyze {
		t.Fatalf("expected phase to be set")
	}
}

func TestArtifact_Validate(t *testing.T) {
	artifact := NewArtifact("a1", ArtifactTypeAnalysis, "t1")
	if err := artifact.Validate(); err == nil {
		t.Fatalf("expected error when content and path are empty")
	}

	artifact.WithContent("data")
	if err := artifact.Validate(); err != nil {
		t.Fatalf("unexpected error validating artifact: %v", err)
	}

	missingID := NewArtifact("", ArtifactTypeAnalysis, "t1").WithContent("data")
	if err := missingID.Validate(); err == nil {
		t.Fatalf("expected error for missing ID")
	}

	invalidType := NewArtifact("a1", "invalid", "t1").WithContent("data")
	if err := invalidType.Validate(); err == nil {
		t.Fatalf("expected error for invalid artifact type")
	}
}

func TestArtifactType_Valid(t *testing.T) {
	for _, typ := range AllArtifactTypes() {
		if !ValidArtifactType(typ) {
			t.Fatalf("expected valid type %s", typ)
		}
	}
	if ValidArtifactType("invalid") {
		t.Fatalf("expected invalid artifact type to be rejected")
	}
}
