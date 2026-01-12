package core

import (
	"fmt"
	"path/filepath"
	"time"
)

// ArtifactType categorizes the kind of output.
type ArtifactType string

const (
	ArtifactTypeAnalysis      ArtifactType = "analysis"
	ArtifactTypePlan          ArtifactType = "plan"
	ArtifactTypeCode          ArtifactType = "code"
	ArtifactTypeTest          ArtifactType = "test"
	ArtifactTypeDocumentation ArtifactType = "documentation"
	ArtifactTypeLog           ArtifactType = "log"
	ArtifactTypeConsensus     ArtifactType = "consensus"
)

// Artifact represents an output produced by a task.
type Artifact struct {
	ID        string
	Type      ArtifactType
	TaskID    TaskID
	Phase     Phase
	Path      string            // File path if persisted
	Content   string            // Raw content if in-memory
	Metadata  map[string]string // Additional metadata
	Size      int64
	Checksum  string
	CreatedAt time.Time
}

// NewArtifact creates a new artifact.
func NewArtifact(id string, artifactType ArtifactType, taskID TaskID) *Artifact {
	return &Artifact{
		ID:        id,
		Type:      artifactType,
		TaskID:    taskID,
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
	}
}

// WithContent sets the artifact content.
func (a *Artifact) WithContent(content string) *Artifact {
	a.Content = content
	a.Size = int64(len(content))
	return a
}

// WithPath sets the artifact file path.
func (a *Artifact) WithPath(path string) *Artifact {
	a.Path = path
	return a
}

// WithPhase sets the artifact phase.
func (a *Artifact) WithPhase(phase Phase) *Artifact {
	a.Phase = phase
	return a
}

// WithMetadata adds metadata to the artifact.
func (a *Artifact) WithMetadata(key, value string) *Artifact {
	a.Metadata[key] = value
	return a
}

// FileName returns the base name if path is set.
func (a *Artifact) FileName() string {
	if a.Path == "" {
		return ""
	}
	return filepath.Base(a.Path)
}

// IsFile returns true if the artifact is file-based.
func (a *Artifact) IsFile() bool {
	return a.Path != ""
}

// Validate checks artifact invariants.
func (a *Artifact) Validate() error {
	if a.ID == "" {
		return &DomainError{
			Category: ErrCatValidation,
			Code:     "ARTIFACT_ID_REQUIRED",
			Message:  "artifact ID cannot be empty",
		}
	}
	if !ValidArtifactType(a.Type) {
		return &DomainError{
			Category: ErrCatValidation,
			Code:     "INVALID_ARTIFACT_TYPE",
			Message:  fmt.Sprintf("invalid artifact type: %s", a.Type),
		}
	}
	if a.Content == "" && a.Path == "" {
		return &DomainError{
			Category: ErrCatValidation,
			Code:     "ARTIFACT_EMPTY",
			Message:  "artifact must have content or path",
		}
	}
	return nil
}

// ValidArtifactType checks if an artifact type is valid.
func ValidArtifactType(t ArtifactType) bool {
	switch t {
	case ArtifactTypeAnalysis, ArtifactTypePlan, ArtifactTypeCode,
		ArtifactTypeTest, ArtifactTypeDocumentation, ArtifactTypeLog,
		ArtifactTypeConsensus:
		return true
	default:
		return false
	}
}

// AllArtifactTypes returns all valid artifact types.
func AllArtifactTypes() []ArtifactType {
	return []ArtifactType{
		ArtifactTypeAnalysis,
		ArtifactTypePlan,
		ArtifactTypeCode,
		ArtifactTypeTest,
		ArtifactTypeDocumentation,
		ArtifactTypeLog,
		ArtifactTypeConsensus,
	}
}
