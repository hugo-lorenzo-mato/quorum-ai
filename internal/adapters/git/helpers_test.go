package git

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestParseStatus_BranchInfo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		branch   string
		upstream string
		ahead    int
		behind   int
	}{
		{
			name: "simple branch",
			input: `# branch.head main
# branch.upstream origin/main
# branch.ab +1 -2`,
			branch:   "main",
			upstream: "origin/main",
			ahead:    1,
			behind:   2,
		},
		{
			name:   "no upstream",
			input:  `# branch.head feature`,
			branch: "feature",
		},
		{
			name: "ahead only",
			input: `# branch.head develop
# branch.upstream origin/develop
# branch.ab +5 -0`,
			branch:   "develop",
			upstream: "origin/develop",
			ahead:    5,
			behind:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := parseStatus(tt.input)
			if status.Branch != tt.branch {
				t.Errorf("Branch = %q, want %q", status.Branch, tt.branch)
			}
			if status.Upstream != tt.upstream {
				t.Errorf("Upstream = %q, want %q", status.Upstream, tt.upstream)
			}
			if status.Ahead != tt.ahead {
				t.Errorf("Ahead = %d, want %d", status.Ahead, tt.ahead)
			}
			if status.Behind != tt.behind {
				t.Errorf("Behind = %d, want %d", status.Behind, tt.behind)
			}
		})
	}
}

func TestParseStatus_Untracked(t *testing.T) {
	t.Parallel()
	input := `# branch.head main
? new-file.txt
? another-file.txt`

	status := parseStatus(input)
	if len(status.Untracked) != 2 {
		t.Errorf("len(Untracked) = %d, want 2", len(status.Untracked))
	}
	if status.Untracked[0] != "new-file.txt" {
		t.Errorf("Untracked[0] = %q, want %q", status.Untracked[0], "new-file.txt")
	}
}

func TestParseStatus_Conflicts(t *testing.T) {
	t.Parallel()
	input := `# branch.head main
u UU conflicted.txt`

	status := parseStatus(input)
	if !status.HasConflicts {
		t.Error("HasConflicts should be true")
	}
}

func TestParseStatus_Empty(t *testing.T) {
	t.Parallel()
	status := parseStatus("")
	if status.Branch != "" {
		t.Errorf("Branch = %q, want empty", status.Branch)
	}
	if !status.IsClean() {
		t.Error("Empty status should be clean")
	}
}

func TestStatus_IsClean(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{
			name:   "completely clean",
			status: Status{},
			want:   true,
		},
		{
			name:   "has staged",
			status: Status{Staged: []string{"file.txt"}},
			want:   false,
		},
		{
			name:   "has modified",
			status: Status{Modified: []string{"file.txt"}},
			want:   false,
		},
		{
			name:   "has untracked",
			status: Status{Untracked: []string{"file.txt"}},
			want:   false,
		},
		{
			name:   "has conflicts",
			status: Status{HasConflicts: true},
			want:   false,
		},
		{
			name: "all dirty",
			status: Status{
				Staged:       []string{"a.txt"},
				Modified:     []string{"b.txt"},
				Untracked:    []string{"c.txt"},
				HasConflicts: true,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsClean(); got != tt.want {
				t.Errorf("IsClean() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseStatusToCore(t *testing.T) {
	t.Parallel()
	input := `# branch.head main
# branch.upstream origin/main
# branch.ab +3 -1
? untracked.txt`

	status := parseStatusToCore(input)

	if status.Branch != "main" {
		t.Errorf("Branch = %q, want main", status.Branch)
	}
	if status.Ahead != 3 {
		t.Errorf("Ahead = %d, want 3", status.Ahead)
	}
	if status.Behind != 1 {
		t.Errorf("Behind = %d, want 1", status.Behind)
	}
	if len(status.Untracked) != 1 {
		t.Errorf("len(Untracked) = %d, want 1", len(status.Untracked))
	}
}

func TestParseWorktreesToCore(t *testing.T) {
	t.Parallel()
	input := `worktree /path/to/main
HEAD abc123def456789012345678901234567890abcd
branch refs/heads/main

worktree /path/to/feature
HEAD def456789012345678901234567890abcdef12
branch refs/heads/feature
locked`

	worktrees := parseWorktreesToCore(input, "/path/to/main")

	if len(worktrees) != 2 {
		t.Fatalf("len(worktrees) = %d, want 2", len(worktrees))
	}

	// Check main worktree
	if worktrees[0].Path != "/path/to/main" {
		t.Errorf("worktrees[0].Path = %q, want /path/to/main", worktrees[0].Path)
	}
	if !worktrees[0].IsMain {
		t.Error("worktrees[0].IsMain should be true")
	}
	if worktrees[0].Branch != "main" {
		t.Errorf("worktrees[0].Branch = %q, want main", worktrees[0].Branch)
	}

	// Check feature worktree
	if worktrees[1].Path != "/path/to/feature" {
		t.Errorf("worktrees[1].Path = %q, want /path/to/feature", worktrees[1].Path)
	}
	if worktrees[1].IsMain {
		t.Error("worktrees[1].IsMain should be false")
	}
	if !worktrees[1].IsLocked {
		t.Error("worktrees[1].IsLocked should be true")
	}
}

func TestParseWorktreesToCore_Empty(t *testing.T) {
	t.Parallel()
	worktrees := parseWorktreesToCore("", "/main")
	if len(worktrees) != 0 {
		t.Errorf("len(worktrees) = %d, want 0", len(worktrees))
	}
}

func TestValidateWorktreeBranch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		branch  string
		wantErr bool
	}{
		{"main", false},
		{"feature/test", false},
		{"quorum/task-123", false},
		{"", true},
		{"   ", true},
		{"branch with spaces", true},
		{"branch..invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			err := validateWorktreeBranch(tt.branch)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateWorktreeBranch(%q) error = %v, wantErr %v", tt.branch, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTaskID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		taskID  string
		wantErr bool
	}{
		{"task-123", false},
		{"task_456", false},
		{"simple", false},
		{"Task.789", false},
		{"", true},
		{"   ", true},
		{"task__double", true},    // Contains __
		{"../parent", true},       // Path traversal
		{"task/sub", true},        // Contains /
		{"task\\sub", true},       // Contains \
		{"task with space", true}, // Contains space
		{"task-日本語", true},        // Non-ASCII
	}

	for _, tt := range tests {
		t.Run(tt.taskID, func(t *testing.T) {
			err := validateTaskID(tt.taskID)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTaskID(%q) error = %v, wantErr %v", tt.taskID, err, tt.wantErr)
			}
		})
	}
}

func TestValidateWorktreeName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"valid-name", false},
		{"task__label", false},
		{"", true},
		{"   ", true},
		{"name..invalid", true},
		{"name/with/slashes", true},
		{"name\\backslash", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWorktreeName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateWorktreeName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"Simple Label", 100, "simple-label"},
		{"Feature: Add Support", 100, "feature-add-support"},
		{"CamelCase", 100, "camelcase"},
		{"with--multiple---dashes", 100, "with-multiple-dashes"},
		{"  spaces  around  ", 100, "spaces-around"},
		{"日本語", 100, ""}, // Non-ASCII only
		{"", 100, ""},
		{"test-label-truncated", 10, "test-label"},
		{"Mixed日本語English", 100, "mixed-english"},
		{"123-numbers", 100, "123-numbers"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeLabel(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("normalizeLabel(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestBuildWorktreeName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		task         *core.Task
		wantName     string
		wantFallback bool
		wantErr      bool
	}{
		{
			name:     "with name",
			task:     &core.Task{ID: "task-1", Name: "Feature Setup"},
			wantName: "task-1__feature-setup",
		},
		{
			name:     "with description fallback",
			task:     &core.Task{ID: "task-2", Description: "Database Migration"},
			wantName: "task-2__database-migration",
		},
		{
			name:         "non-ASCII name falls back to ID",
			task:         &core.Task{ID: "task-3", Name: "日本語"},
			wantName:     "task-3",
			wantFallback: true,
		},
		{
			name:    "nil task",
			task:    nil,
			wantErr: true,
		},
		{
			name:    "empty task ID",
			task:    &core.Task{ID: "", Name: "Test"},
			wantErr: true,
		},
		{
			name:    "no name or description",
			task:    &core.Task{ID: "task-4"},
			wantErr: true,
		},
		{
			name:    "invalid task ID",
			task:    &core.Task{ID: "task/invalid", Name: "Test"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, fallback, err := buildWorktreeName(tt.task)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if fallback != tt.wantFallback {
				t.Errorf("fallback = %v, want %v", fallback, tt.wantFallback)
			}
		})
	}
}

func TestResolveWorktreeBranch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		branch  string
		want    string
		wantErr bool
	}{
		{
			name:   "empty branch uses default",
			branch: "",
			want:   "quorum/test-name",
		},
		{
			name:   "explicit branch",
			branch: "feature/test",
			want:   "feature/test",
		},
		{
			name:   "whitespace branch uses default",
			branch: "   ",
			want:   "quorum/test-name",
		},
		{
			name:    "invalid branch",
			branch:  "branch with spaces",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolveWorktreeBranch("test-name", tt.branch)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if result != tt.want {
				t.Errorf("result = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	t.Parallel()
	// Test that resolvePath returns something for regular paths
	result := resolvePath("/tmp")
	if result == "" {
		t.Error("resolvePath should return non-empty for /tmp")
	}

	// Test with non-existent path
	result = resolvePath("/nonexistent/path/that/does/not/exist")
	if result == "" {
		t.Error("resolvePath should return non-empty even for non-existent paths")
	}
}

func TestCommit_Fields(t *testing.T) {
	t.Parallel()
	commit := Commit{
		Hash:        "abc123",
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		Subject:     "Test commit",
	}

	if commit.Hash != "abc123" {
		t.Errorf("Hash = %q, want abc123", commit.Hash)
	}
	if commit.AuthorName != "Test User" {
		t.Errorf("AuthorName = %q, want Test User", commit.AuthorName)
	}
	if commit.AuthorEmail != "test@example.com" {
		t.Errorf("AuthorEmail = %q, want test@example.com", commit.AuthorEmail)
	}
	if commit.Subject != "Test commit" {
		t.Errorf("Subject = %q, want Test commit", commit.Subject)
	}
}

func TestWorktree_Fields(t *testing.T) {
	t.Parallel()
	wt := Worktree{
		Path:     "/path/to/worktree",
		Branch:   "feature",
		Commit:   "abc123",
		Detached: false,
		Locked:   true,
		Prunable: false,
	}

	if wt.Path != "/path/to/worktree" {
		t.Errorf("Path = %q, want /path/to/worktree", wt.Path)
	}
	if wt.Branch != "feature" {
		t.Errorf("Branch = %q, want feature", wt.Branch)
	}
	if wt.Commit != "abc123" {
		t.Errorf("Commit = %q, want abc123", wt.Commit)
	}
	if wt.Detached {
		t.Error("Detached should be false")
	}
	if !wt.Locked {
		t.Error("Locked should be true")
	}
	if wt.Prunable {
		t.Error("Prunable should be false")
	}
}

func TestWorktreeManager_parseWorktreeList(t *testing.T) {
	t.Parallel()
	// Create a minimal WorktreeManager for testing
	manager := &WorktreeManager{prefix: "quorum-"}

	tests := []struct {
		name       string
		output     string
		wantCount  int
		checkFirst func(t *testing.T, wt Worktree)
	}{
		{
			name:      "empty output",
			output:    "",
			wantCount: 0,
		},
		{
			name: "single worktree",
			output: `worktree /path/to/main
HEAD abc123
branch refs/heads/main`,
			wantCount: 1,
			checkFirst: func(t *testing.T, wt Worktree) {
				if wt.Path != "/path/to/main" {
					t.Errorf("Path = %q, want /path/to/main", wt.Path)
				}
				if wt.Branch != "main" {
					t.Errorf("Branch = %q, want main", wt.Branch)
				}
				if wt.Commit != "abc123" {
					t.Errorf("Commit = %q, want abc123", wt.Commit)
				}
			},
		},
		{
			name: "detached worktree",
			output: `worktree /path/to/detached
HEAD def456
detached`,
			wantCount: 1,
			checkFirst: func(t *testing.T, wt Worktree) {
				if !wt.Detached {
					t.Error("Detached should be true")
				}
			},
		},
		{
			name: "locked worktree",
			output: `worktree /path/to/locked
HEAD ghi789
branch refs/heads/feature
locked`,
			wantCount: 1,
			checkFirst: func(t *testing.T, wt Worktree) {
				if !wt.Locked {
					t.Error("Locked should be true")
				}
			},
		},
		{
			name: "prunable worktree",
			output: `worktree /path/to/prunable
HEAD jkl012
branch refs/heads/stale
prunable`,
			wantCount: 1,
			checkFirst: func(t *testing.T, wt Worktree) {
				if !wt.Prunable {
					t.Error("Prunable should be true")
				}
			},
		},
		{
			name: "multiple worktrees",
			output: `worktree /path/to/main
HEAD abc123
branch refs/heads/main

worktree /path/to/feature
HEAD def456
branch refs/heads/feature

worktree /path/to/detached
HEAD ghi789
detached`,
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worktrees := manager.parseWorktreeList(tt.output)
			if len(worktrees) != tt.wantCount {
				t.Errorf("len(worktrees) = %d, want %d", len(worktrees), tt.wantCount)
			}
			if tt.checkFirst != nil && len(worktrees) > 0 {
				tt.checkFirst(t, worktrees[0])
			}
		})
	}
}

func TestStatus_Fields(t *testing.T) {
	t.Parallel()
	status := Status{
		Branch:       "main",
		Upstream:     "origin/main",
		Ahead:        2,
		Behind:       1,
		Staged:       []string{"file1.txt"},
		Modified:     []string{"file2.txt"},
		Untracked:    []string{"file3.txt"},
		HasConflicts: false,
	}

	if status.Branch != "main" {
		t.Errorf("Branch = %q, want main", status.Branch)
	}
	if status.Upstream != "origin/main" {
		t.Errorf("Upstream = %q, want origin/main", status.Upstream)
	}
	if status.Ahead != 2 {
		t.Errorf("Ahead = %d, want 2", status.Ahead)
	}
	if status.Behind != 1 {
		t.Errorf("Behind = %d, want 1", status.Behind)
	}
	if len(status.Staged) != 1 {
		t.Errorf("len(Staged) = %d, want 1", len(status.Staged))
	}
	if len(status.Modified) != 1 {
		t.Errorf("len(Modified) = %d, want 1", len(status.Modified))
	}
	if len(status.Untracked) != 1 {
		t.Errorf("len(Untracked) = %d, want 1", len(status.Untracked))
	}
	if status.HasConflicts {
		t.Error("HasConflicts should be false")
	}
}

func TestParseStatus_OrdinaryChangedEntry(t *testing.T) {
	t.Parallel()
	// Format: 1 XY sub mH mI mW hH hI path
	// The XY is at position 2-3, and path starts at position 113
	// Test with actual git porcelain format
	gitOutput := `# branch.head main
1 M. N... 100644 100644 100644 abcdef123456789012345678901234567890 abcdef123456789012345678901234567890 staged-file.go`

	status := parseStatus(gitOutput)

	// Just verify it doesn't panic and returns a valid status
	if status == nil {
		t.Error("parseStatus should return non-nil status")
	}
}

func TestParseStatus_MultipleBranchInfo(t *testing.T) {
	t.Parallel()
	// Test parsing with malformed branch.ab line (less than 4 parts)
	input := `# branch.head main
# branch.ab +5`

	status := parseStatus(input)
	if status.Ahead != 0 {
		t.Errorf("Ahead should be 0 for malformed line, got %d", status.Ahead)
	}
}
