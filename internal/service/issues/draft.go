package issues

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/report"
	"gopkg.in/yaml.v3"
)

// DraftFrontmatter holds the structured metadata for a draft issue file.
type DraftFrontmatter struct {
	Title       string   `yaml:"title"`
	Labels      []string `yaml:"labels"`
	Assignees   []string `yaml:"assignees"`
	IsMainIssue bool     `yaml:"is_main_issue"`
	TaskID      string   `yaml:"task_id"`
	SourcePath  string   `yaml:"source_path"`
	Status      string   `yaml:"status"`
}

// WriteDraftFile writes a draft issue file with YAML frontmatter to the draft directory.
func (g *Generator) WriteDraftFile(workflowID, fileName string, fm DraftFrontmatter, body string) (string, error) {
	draftDir, err := g.resolveDraftDir(workflowID)
	if err != nil {
		return "", fmt.Errorf(errResolvingDraftDir, err)
	}

	if err := os.MkdirAll(draftDir, 0o755); err != nil {
		return "", fmt.Errorf("creating draft directory: %w", err)
	}

	filePath, err := ValidateOutputPath(draftDir, fileName)
	if err != nil {
		return "", fmt.Errorf("validating draft file path %q: %w", fileName, err)
	}

	content := renderDraftContent(fm, body)
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("writing draft file %s: %w", filePath, err)
	}

	return filePath, nil
}

// ReadDraftFile reads a single draft file and parses its frontmatter.
func (g *Generator) ReadDraftFile(workflowID, fileName string) (*DraftFrontmatter, string, error) {
	draftDir, err := g.resolveDraftDir(workflowID)
	if err != nil {
		return nil, "", fmt.Errorf(errResolvingDraftDir, err)
	}

	filePath, err := ValidateOutputPath(draftDir, fileName)
	if err != nil {
		return nil, "", fmt.Errorf("validating draft file path %q: %w", fileName, err)
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("reading draft file %s: %w", filePath, err)
	}

	fm, body, err := parseDraftContent(string(content))
	if err != nil {
		return nil, "", fmt.Errorf("parsing draft file %s: %w", filePath, err)
	}

	return fm, body, nil
}

// ReadAllDrafts reads all markdown draft files in the draft directory for a workflow.
func (g *Generator) ReadAllDrafts(workflowID string) ([]IssuePreview, error) {
	draftDir, err := g.resolveDraftDir(workflowID)
	if err != nil {
		return nil, fmt.Errorf(errResolvingDraftDir, err)
	}

	entries, err := os.ReadDir(draftDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading draft directory: %w", err)
	}

	// Collect and sort markdown files
	var files []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, entry)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return extractFileNumber(files[i].Name()) < extractFileNumber(files[j].Name())
	})

	baseDir := g.resolveIssuesBaseDir()
	draftRelDir := filepath.Join(baseDir, workflowID, "draft")

	var previews []IssuePreview
	seen := make(map[string]bool)

	for _, entry := range files {
		preview, skip := g.parseDraftEntry(draftDir, draftRelDir, entry, seen)
		if skip {
			continue
		}
		previews = append(previews, preview)
	}

	return previews, nil
}

// parseDraftEntry parses a single draft file entry and returns the preview.
// It returns skip=true if the file should be skipped (read error or duplicate task ID).
func (g *Generator) parseDraftEntry(draftDir, draftRelDir string, entry os.DirEntry, seen map[string]bool) (IssuePreview, bool) {
	filePath := filepath.Join(draftDir, entry.Name())
	content, err := os.ReadFile(filePath)
	if err != nil {
		return IssuePreview{}, true
	}

	fm, body, err := parseDraftContent(string(content))
	if err != nil {
		// Fallback: try parsing as plain markdown (backward compat)
		title, bodyPlain := parseIssueMarkdown(string(content))
		return IssuePreview{
			Title:       title,
			Body:        bodyPlain,
			Labels:      g.config.Labels,
			Assignees:   g.config.Assignees,
			IsMainIssue: strings.Contains(entry.Name(), "consolidated") || strings.HasPrefix(entry.Name(), "00-"),
			FilePath:    filepath.Join(draftRelDir, entry.Name()),
		}, false
	}

	// Deduplicate by task ID
	if fm.TaskID != "" && seen[fm.TaskID] {
		return IssuePreview{}, true
	}
	if fm.TaskID != "" {
		seen[fm.TaskID] = true
	}

	return IssuePreview{
		Title:       fm.Title,
		Body:        body,
		Labels:      fm.Labels,
		Assignees:   fm.Assignees,
		IsMainIssue: fm.IsMainIssue,
		TaskID:      fm.TaskID,
		FilePath:    filepath.Join(draftRelDir, entry.Name()),
	}, false
}

// ReadIssueMapping reads the mapping.json file from the published directory.
func (g *Generator) ReadIssueMapping(workflowID string) (*IssueMapping, error) {
	publishedDir, err := g.resolvePublishedDir(workflowID)
	if err != nil {
		return nil, fmt.Errorf("resolving published directory: %w", err)
	}

	mappingPath := filepath.Join(publishedDir, "mapping.json")
	data, err := os.ReadFile(mappingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading mapping file: %w", err)
	}

	var mapping IssueMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, fmt.Errorf("parsing mapping file: %w", err)
	}

	return &mapping, nil
}

// renderDraftContent renders a draft file with YAML frontmatter followed by the markdown body.
func renderDraftContent(fm DraftFrontmatter, body string) string {
	f := report.NewFrontmatter()
	f.Set("title", fm.Title)
	f.Set("labels", fm.Labels)
	f.Set("assignees", fm.Assignees)
	f.Set("is_main_issue", fm.IsMainIssue)
	f.Set("task_id", fm.TaskID)
	f.Set("source_path", fm.SourcePath)
	f.Set("status", fm.Status)

	var sb strings.Builder
	sb.WriteString(f.Render())
	sb.WriteString(strings.TrimSpace(body))
	sb.WriteString("\n")
	return sb.String()
}

// parseDraftContent parses a draft file's YAML frontmatter and returns the structured metadata and body.
func parseDraftContent(content string) (*DraftFrontmatter, string, error) {
	fmYAML, body, ok := extractFrontmatter(content)
	if !ok {
		return nil, content, fmt.Errorf("no frontmatter found")
	}

	var fm DraftFrontmatter
	if err := yaml.Unmarshal([]byte(fmYAML), &fm); err != nil {
		return nil, body, fmt.Errorf("parsing frontmatter YAML: %w", err)
	}

	return &fm, strings.TrimSpace(body), nil
}

// extractFrontmatter extracts YAML frontmatter delimited by --- from content.
// Returns the YAML content (without delimiters), the remaining body, and whether frontmatter was found.
func extractFrontmatter(content string) (frontmatter, body string, ok bool) {
	re := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)
	match := re.FindStringSubmatchIndex(content)
	if match == nil {
		return "", content, false
	}

	frontmatter = content[match[2]:match[3]]
	body = content[match[1]:]
	return frontmatter, body, true
}
