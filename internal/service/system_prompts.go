package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type SystemPromptMeta struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	WorkflowPhase string   `json:"workflow_phase"`
	Step          string   `json:"step"`
	Status        string   `json:"status"`
	UsedBy        []string `json:"used_by"`
	Sha256        string   `json:"sha256"`
}

type SystemPrompt struct {
	SystemPromptMeta
	Content string `json:"content"`
}

type systemPromptFrontmatter struct {
	ID            string   `yaml:"id"`
	Title         string   `yaml:"title"`
	WorkflowPhase string   `yaml:"workflow_phase"`
	Step          string   `yaml:"step"`
	Status        string   `yaml:"status"`
	UsedBy        []string `yaml:"used_by"`
}

func splitSystemPromptFrontmatter(raw string) (frontmatter, body string, ok bool) {
	// Normalize Windows line endings for consistent parsing/hashing.
	s := strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return "", s, false
	}

	rest := s[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end == -1 {
		// No closing delimiter: treat as no frontmatter.
		return "", s, false
	}

	frontmatter = rest[:end]
	body = rest[end+len("\n---\n"):]
	body = strings.TrimLeft(body, "\n") // avoid leading blank line from metadata separation
	return frontmatter, body, true
}

func hashSha256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func validateSystemPromptMeta(meta systemPromptFrontmatter, idFromFilename string) error {
	if strings.TrimSpace(meta.ID) == "" {
		return fmt.Errorf("frontmatter: id is required")
	}
	if meta.ID != idFromFilename {
		return fmt.Errorf("frontmatter: id %q does not match filename %q", meta.ID, idFromFilename)
	}
	if strings.TrimSpace(meta.Title) == "" {
		return fmt.Errorf("frontmatter: title is required (id=%s)", meta.ID)
	}
	switch meta.WorkflowPhase {
	case "refine", "analyze", "plan", "execute":
	default:
		return fmt.Errorf("frontmatter: invalid workflow_phase %q (id=%s)", meta.WorkflowPhase, meta.ID)
	}
	if strings.TrimSpace(meta.Step) == "" {
		return fmt.Errorf("frontmatter: step is required (id=%s)", meta.ID)
	}
	switch meta.Status {
	case "active", "reserved", "deprecated":
	default:
		return fmt.Errorf("frontmatter: invalid status %q (id=%s)", meta.Status, meta.ID)
	}
	if len(meta.UsedBy) == 0 {
		return fmt.Errorf("frontmatter: used_by must not be empty (id=%s)", meta.ID)
	}
	for _, v := range meta.UsedBy {
		switch v {
		case "workflow", "issues":
		default:
			return fmt.Errorf("frontmatter: invalid used_by value %q (id=%s)", v, meta.ID)
		}
	}
	return nil
}

func systemPromptIDFromPath(path string) string {
	name := strings.TrimPrefix(path, "prompts/")
	name = strings.TrimSuffix(name, ".md.tmpl")
	return name
}

// ListSystemPrompts returns metadata for all embedded system prompts.
func ListSystemPrompts() ([]SystemPromptMeta, error) {
	var metas []SystemPromptMeta

	err := fs.WalkDir(promptsFS, "prompts", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md.tmpl") {
			return nil
		}

		content, err := promptsFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		id := systemPromptIDFromPath(path)
		fmRaw, body, ok := splitSystemPromptFrontmatter(string(content))
		if !ok {
			return fmt.Errorf("missing frontmatter (id=%s)", id)
		}

		var fm systemPromptFrontmatter
		if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
			return fmt.Errorf("parsing frontmatter (id=%s): %w", id, err)
		}
		if err := validateSystemPromptMeta(fm, id); err != nil {
			return fmt.Errorf("invalid frontmatter (id=%s): %w", id, err)
		}

		metas = append(metas, SystemPromptMeta{
			ID:            fm.ID,
			Title:         fm.Title,
			WorkflowPhase: fm.WorkflowPhase,
			Step:          fm.Step,
			Status:        fm.Status,
			UsedBy:        append([]string(nil), fm.UsedBy...),
			Sha256:        hashSha256(body),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	phaseOrder := map[string]int{
		"refine":   0,
		"analyze":  1,
		"plan":     2,
		"execute":  3,
		"":         99,
		"unknown":  99,
		"reserved": 99,
	}

	sort.Slice(metas, func(i, j int) bool {
		pi := phaseOrder[metas[i].WorkflowPhase]
		pj := phaseOrder[metas[j].WorkflowPhase]
		if pi != pj {
			return pi < pj
		}
		if metas[i].Step != metas[j].Step {
			return metas[i].Step < metas[j].Step
		}
		return metas[i].ID < metas[j].ID
	})

	return metas, nil
}

// GetSystemPrompt returns a single embedded system prompt with metadata and content.
func GetSystemPrompt(id string) (*SystemPrompt, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	path := "prompts/" + id + ".md.tmpl"
	content, err := promptsFS.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fmRaw, body, ok := splitSystemPromptFrontmatter(string(content))
	if !ok {
		return nil, fmt.Errorf("missing frontmatter (id=%s)", id)
	}

	var fm systemPromptFrontmatter
	if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
		return nil, fmt.Errorf("parsing frontmatter (id=%s): %w", id, err)
	}
	if err := validateSystemPromptMeta(fm, id); err != nil {
		return nil, fmt.Errorf("invalid frontmatter (id=%s): %w", id, err)
	}

	return &SystemPrompt{
		SystemPromptMeta: SystemPromptMeta{
			ID:            fm.ID,
			Title:         fm.Title,
			WorkflowPhase: fm.WorkflowPhase,
			Step:          fm.Step,
			Status:        fm.Status,
			UsedBy:        append([]string(nil), fm.UsedBy...),
			Sha256:        hashSha256(body),
		},
		Content: body,
	}, nil
}
