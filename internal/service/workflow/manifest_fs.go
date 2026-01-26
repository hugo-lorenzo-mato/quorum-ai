package workflow

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	// headerPattern matches "**Header Name**: value" format
	headerPattern = regexp.MustCompile(`^\*\*(.+?)\*\*:\s*(.*)$`)

	// titlePattern matches "# Task: Name" format
	titlePattern = regexp.MustCompile(`^#\s*Task:\s*(.+)$`)
)

// generateManifestFromFilesystem scans a directory for task files and builds
// a ComprehensiveTaskManifest by parsing markdown headers.
func generateManifestFromFilesystem(tasksDir string) (*ComprehensiveTaskManifest, error) {
	pattern := filepath.Join(tasksDir, "task-*.md")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing task files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no task files found in %s", tasksDir)
	}

	// Sort files for deterministic order
	sort.Strings(files)

	var tasks []TaskManifestItem
	var parseErrors []string

	for _, file := range files {
		item, err := parseTaskFile(file)
		if err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("%s: %v", filepath.Base(file), err))
			continue
		}
		tasks = append(tasks, *item)
	}

	if len(tasks) == 0 {
		return nil, fmt.Errorf("all task files failed to parse: %v", parseErrors)
	}

	// Compute execution levels via topological sort
	levels, err := computeExecutionLevels(tasks)
	if err != nil {
		return nil, fmt.Errorf("computing execution levels: %w", err)
	}

	return &ComprehensiveTaskManifest{
		Tasks:           tasks,
		ExecutionLevels: levels,
	}, nil
}

// parseTaskFile reads a task markdown file and extracts metadata from headers.
func parseTaskFile(filePath string) (*TaskManifestItem, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening task file: %w", err)
	}
	defer file.Close()

	item := &TaskManifestItem{
		File:         filePath,
		Dependencies: []string{},
	}

	scanner := bufio.NewScanner(file)
	lineCount := 0
	const maxHeaderLines = 20 // Only scan first 20 lines for headers

	for scanner.Scan() && lineCount < maxHeaderLines {
		line := strings.TrimSpace(scanner.Text())
		lineCount++

		// Try to match task title: # Task: Name
		if matches := titlePattern.FindStringSubmatch(line); len(matches) == 2 {
			item.Name = strings.TrimSpace(matches[1])
			continue
		}

		// Try to match header pattern: **Header**: value
		if matches := headerPattern.FindStringSubmatch(line); len(matches) == 3 {
			headerName := strings.TrimSpace(matches[1])
			headerValue := strings.TrimSpace(matches[2])

			switch headerName {
			case "Task ID":
				item.ID = headerValue
			case "Assigned Agent":
				item.CLI = headerValue
			case "Complexity":
				item.Complexity = headerValue
			case "Dependencies":
				item.Dependencies = parseDependencies(headerValue)
			}
		}

		// Stop at first horizontal rule after headers (end of header section)
		if line == "---" && lineCount > 5 {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning task file: %w", err)
	}

	// Validate required fields
	if item.ID == "" {
		return nil, fmt.Errorf("missing Task ID header")
	}

	// Fallback: extract name from filename if not found in content
	if item.Name == "" {
		item.Name = extractTaskNameFromFilename(filepath.Base(filePath))
	}

	// Default complexity if not specified
	if item.Complexity == "" {
		item.Complexity = "medium"
	}

	return item, nil
}

// parseDependencies parses the Dependencies header value into a slice.
// Handles both "None" and comma-separated task IDs.
func parseDependencies(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "none") {
		return []string{}
	}

	parts := strings.Split(value, ",")
	deps := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" && !strings.EqualFold(trimmed, "none") {
			deps = append(deps, trimmed)
		}
	}
	return deps
}

// extractTaskNameFromFilename extracts task name from filename pattern.
// Handles: "task-1-create-web-server.md" -> "create web server"
func extractTaskNameFromFilename(filename string) string {
	// Remove .md extension
	name := strings.TrimSuffix(filename, ".md")

	// Pattern: task-{id}-{name}
	// Find second hyphen to skip "task-N-"
	parts := strings.SplitN(name, "-", 3)
	if len(parts) >= 3 {
		// Convert kebab-case to title case
		words := strings.Split(parts[2], "-")
		for i, word := range words {
			if word != "" {
				words[i] = strings.ToUpper(word[:1]) + word[1:]
			}
		}
		return strings.Join(words, " ")
	}

	return name
}

// computeExecutionLevels groups tasks into parallel execution levels using
// Kahn's algorithm for topological sorting.
func computeExecutionLevels(tasks []TaskManifestItem) ([][]string, error) {
	if len(tasks) == 0 {
		return nil, nil
	}

	// Build task set and adjacency structures
	taskSet := make(map[string]bool)
	inDegree := make(map[string]int)
	dependents := make(map[string][]string) // task -> tasks that depend on it

	for _, task := range tasks {
		taskSet[task.ID] = true
		inDegree[task.ID] = 0
		dependents[task.ID] = []string{}
	}

	// Calculate in-degrees and build reverse graph
	for _, task := range tasks {
		for _, dep := range task.Dependencies {
			// Skip dependencies that don't exist (graceful degradation)
			if !taskSet[dep] {
				continue
			}
			inDegree[task.ID]++
			dependents[dep] = append(dependents[dep], task.ID)
		}
	}

	// Kahn's algorithm with level tracking
	var levels [][]string
	assigned := make(map[string]bool)

	for len(assigned) < len(tasks) {
		var level []string

		// Find all tasks with zero in-degree that haven't been assigned
		for _, task := range tasks {
			if assigned[task.ID] {
				continue
			}
			if inDegree[task.ID] == 0 {
				level = append(level, task.ID)
			}
		}

		if len(level) == 0 {
			// Circular dependency detected
			var unassigned []string
			for _, task := range tasks {
				if !assigned[task.ID] {
					unassigned = append(unassigned, task.ID)
				}
			}
			return nil, fmt.Errorf("circular dependency detected among tasks: %v", unassigned)
		}

		// Sort level for deterministic order
		sort.Strings(level)

		// Mark level tasks as assigned and decrement in-degrees of dependents
		for _, taskID := range level {
			assigned[taskID] = true
			for _, dependent := range dependents[taskID] {
				inDegree[dependent]--
			}
		}

		levels = append(levels, level)
	}

	return levels, nil
}
