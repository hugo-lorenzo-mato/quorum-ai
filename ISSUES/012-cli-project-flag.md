# CLI Project Flag Support

## Summary

Add global `--project` flag to the Quorum CLI enabling project specification without changing directories, with fallback chain supporting environment variables and default project.

## Context

Currently, Quorum CLI operates on the current working directory. For multi-project support, users need to specify which project to operate on without `cd`ing into it:

```bash
# Current approach (requires cd)
cd ~/projects/my-app && quorum start

# Desired approach (with flag)
quorum --project ~/projects/my-app start
quorum -p proj-abc123 start
```

## Implementation Details

### Files to Modify

- `cmd/quorum/cmd/root.go` - Add global `--project` flag
- `cmd/quorum/cmd/serve.go` - Use project flag in serve command
- `internal/project/registry.go` - Add resolution methods
- `cmd/quorum/cmd/completion.go` - Shell completion

### Files to Create

- `cmd/quorum/cmd/projects.go` - Project management subcommand

### Global Flag

```go
var (
  projectFlag     string // -p, --project
  projectIDFlag   string // Internal: resolved project ID
  projectRootFlag string // Internal: resolved project root
)
```

### Project Resolution Chain

```
1. Explicit --project flag (highest priority)
2. QUORUM_PROJECT environment variable
3. Current directory (default)
```

### Projects Subcommand

```bash
quorum projects list                    # List all projects
quorum projects add /path/to/project    # Register project
quorum projects remove proj-abc123      # Unregister project
quorum projects info proj-abc123        # Show project details
quorum projects set-default proj-abc    # Set default
```

### Root Command Modifications

```go
// Root cmd structure
var rootCmd = &cobra.Command{
  Use:   "quorum",
  Short: "Quorum AI Workflow",
  PersistentPreRun: func(cmd *cobra.Command, args []string) {
    // Resolve project before any command runs
    resolveProject()
  },
}

func init() {
  rootCmd.PersistentFlags().StringVarP(
    &projectFlag, "project", "p", "",
    "Project ID or path (or set QUORUM_PROJECT env var)",
  )
}

func resolveProject() {
  // 1. Check explicit flag
  if projectFlag != "" {
    projectRootFlag = resolveProjectPath(projectFlag)
    return
  }

  // 2. Check environment variable
  if env := os.Getenv("QUORUM_PROJECT"); env != "" {
    projectRootFlag = resolveProjectPath(env)
    return
  }

  // 3. Use current directory
  wd, _ := os.Getwd()
  projectRootFlag = wd
}
```

### Projects Command Implementation

```go
// cmd/quorum/cmd/projects.go

var projectsCmd = &cobra.Command{
  Use:   "projects",
  Short: "Manage registered projects",
}

var projectsListCmd = &cobra.Command{
  Use:   "list",
  Short: "List all registered projects",
  RunE: func(cmd *cobra.Command, args []string) error {
    registry, _ := project.NewFileRegistry()
    projects, _ := registry.ListProjects(context.Background())

    // Format and display
    for _, p := range projects {
      status := "●"
      if p.ID == getDefaultProjectID() {
        status = "✓"
      }
      fmt.Printf("%s %-15s %s (%s)\n", status, p.ID, p.Name, p.Path)
    }
    return nil
  },
}

var projectsAddCmd = &cobra.Command{
  Use:   "add <path>",
  Short: "Register a project",
  Args:  cobra.ExactArgs(1),
  RunE: func(cmd *cobra.Command, args []string) error {
    registry, _ := project.NewFileRegistry()
    path, _ := filepath.Abs(args[0])

    proj, err := registry.AddProject(context.Background(), path, nil)
    if err != nil {
      return fmt.Errorf("failed to register: %w", err)
    }

    fmt.Printf("Registered: %s (%s)\n", proj.ID, proj.Name)
    return nil
  },
}

// Similar for remove, info, set-default
```

### Shell Completion

```go
// Bash/Zsh completion for --project flag
func completeProjects(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
  registry, _ := project.NewFileRegistry()
  projects, _ := registry.ListProjects(context.Background())

  var completions []string
  for _, p := range projects {
    completions = append(completions, p.ID+" ("+p.Name+")")
    completions = append(completions, p.Path)
  }

  return completions, cobra.ShellCompDirectiveDefault
}

func init() {
  rootCmd.RegisterFlagCompletionFunc("project", completeProjects)
}
```

## Acceptance Criteria

- [ ] `--project` global flag added to root command
- [ ] Short form `-p` works
- [ ] QUORUM_PROJECT environment variable supported
- [ ] Resolution chain works (flag → env → current dir)
- [ ] `quorum projects list` shows registered projects
- [ ] `quorum projects add <path>` registers new project
- [ ] `quorum projects remove <id>` removes project
- [ ] `quorum projects info <id>` shows details
- [ ] `quorum projects set-default <id>` sets default
- [ ] Shell completion for project IDs and paths
- [ ] All commands work with global flag
- [ ] Error messages clear and helpful
- [ ] Unit tests cover resolution logic
- [ ] Integration tests verify workflows

## Testing Strategy

1. **Unit Tests**:
   - Project resolution chain priority
   - Path → ID conversion
   - Invalid project handling
   - Environment variable parsing

2. **Integration Tests**:
   - `quorum --project X start` works
   - `quorum start` in project directory works
   - `QUORUM_PROJECT=X quorum start` works
   - Projects subcommand CRUD operations

3. **Manual Testing**:
   - Try all resolution methods
   - Test shell completion
   - Verify help text
   - Check error messages

## Definition of Done

- All acceptance criteria met
- Unit test coverage >80%
- Integration tests verify complete workflows
- Clear documentation and help text
- Backward compatible (current dir still works)
