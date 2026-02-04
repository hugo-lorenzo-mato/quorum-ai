package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage registered projects",
	Long: `Manage Quorum projects for multi-project support.

Projects allow you to work on multiple codebases simultaneously,
each with their own configuration, state, and workflows.

Use 'quorum project add' to register a new project, or 'quorum project list'
to see all registered projects.`,
}

var addProjectCmd = &cobra.Command{
	Use:   "add [path]",
	Short: "Register a new project",
	Long: `Register a directory as a Quorum project.

The directory must contain a .quorum directory to be registered as a project.
If no path is specified, the current directory is used.

Examples:
  # Register current directory
  quorum project add

  # Register a specific path
  quorum project add /path/to/project

  # Register with a custom name
  quorum project add --name "My Project"

  # Register and set as default
  quorum project add --default`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAddProject,
}

var listProjectsCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered projects",
	Long: `List all registered Quorum projects.

Shows project ID, name, path, status, and whether it's the default project.`,
	Aliases: []string{"ls"},
	RunE:    runListProjects,
}

var removeProjectCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Unregister a project",
	Long: `Remove a project from the registry.

This does not delete any files, it only removes the project from Quorum's registry.
You can re-add the project later with 'quorum project add'.`,
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE:    runRemoveProject,
}

var setDefaultCmd = &cobra.Command{
	Use:   "default <id>",
	Short: "Set the default project",
	Long: `Set a project as the default.

The default project is used when no --project flag is specified
and the current directory is not a registered project.`,
	Args: cobra.ExactArgs(1),
	RunE: runSetDefault,
}

var validateProjectCmd = &cobra.Command{
	Use:   "validate [id]",
	Short: "Validate project configuration",
	Long: `Validate a project's configuration and accessibility.

If no ID is specified, validates all projects.`,
	RunE: runValidateProject,
}

var (
	addProjectName    string
	addProjectColor   string
	addProjectDefault bool
)

func init() {
	rootCmd.AddCommand(projectCmd)

	// Add subcommands
	projectCmd.AddCommand(addProjectCmd)
	projectCmd.AddCommand(listProjectsCmd)
	projectCmd.AddCommand(removeProjectCmd)
	projectCmd.AddCommand(setDefaultCmd)
	projectCmd.AddCommand(validateProjectCmd)

	// Also add 'add' as a top-level shortcut
	rootCmd.AddCommand(&cobra.Command{
		Use:   "add [path]",
		Short: "Register current directory as a project (shortcut for 'project add')",
		Long:  addProjectCmd.Long,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runAddProject,
	})

	// Add flags
	addProjectCmd.Flags().StringVar(&addProjectName, "name", "", "Custom name for the project")
	addProjectCmd.Flags().StringVar(&addProjectColor, "color", "", "Custom color for the project (hex format)")
	addProjectCmd.Flags().BoolVar(&addProjectDefault, "default", false, "Set as default project")
}

func runAddProject(_ *cobra.Command, args []string) error {
	ctx := context.Background()

	// Determine path
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if .quorum directory exists
	quorumDir := filepath.Join(absPath, ".quorum")
	if _, err := os.Stat(quorumDir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("not a Quorum project: %s (missing .quorum directory)\n"+
				"Initialize with 'quorum init' first", absPath)
		}
		return fmt.Errorf("cannot access .quorum directory: %w", err)
	}

	// Create registry
	registry, err := project.NewFileRegistry()
	if err != nil {
		return fmt.Errorf("opening project registry: %w", err)
	}
	defer registry.Close()

	// Add project
	opts := &project.AddProjectOptions{
		Name:  addProjectName,
		Color: addProjectColor,
	}

	p, err := registry.AddProject(ctx, absPath, opts)
	if err != nil {
		if err == project.ErrProjectAlreadyExists {
			return fmt.Errorf("project already registered at %s", absPath)
		}
		return fmt.Errorf("adding project: %w", err)
	}

	// Set as default if requested
	if addProjectDefault {
		if err := registry.SetDefaultProject(ctx, p.ID); err != nil {
			return fmt.Errorf("setting as default: %w", err)
		}
	}

	if quiet {
		fmt.Println(p.ID)
		return nil
	}

	fmt.Printf("Project registered successfully!\n\n")
	fmt.Printf("  ID:    %s\n", p.ID)
	fmt.Printf("  Name:  %s\n", p.Name)
	fmt.Printf("  Path:  %s\n", p.Path)
	if addProjectDefault {
		fmt.Printf("  Default: yes\n")
	}
	fmt.Printf("\nUse 'quorum --project %s <command>' to work with this project.\n", p.ID)

	return nil
}

func runListProjects(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	registry, err := project.NewFileRegistry()
	if err != nil {
		return fmt.Errorf("opening project registry: %w", err)
	}
	defer registry.Close()

	projects, err := registry.ListProjects(ctx)
	if err != nil {
		return fmt.Errorf("listing projects: %w", err)
	}

	if len(projects) == 0 {
		fmt.Println("No projects registered.")
		fmt.Println("\nRegister a project with: quorum add .")
		return nil
	}

	// Get default project
	defaultProject, _ := registry.GetDefaultProject(ctx)
	defaultID := ""
	if defaultProject != nil {
		defaultID = defaultProject.ID
	}

	// Print as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tPATH\tSTATUS\tDEFAULT")
	fmt.Fprintln(w, "──\t────\t────\t──────\t───────")

	for _, p := range projects {
		isDefault := ""
		if p.ID == defaultID {
			isDefault = "*"
		}
		status := string(p.Status)
		if p.StatusMessage != "" {
			status = fmt.Sprintf("%s (%s)", p.Status, p.StatusMessage)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			p.ID, p.Name, p.Path, status, isDefault)
	}
	w.Flush()

	return nil
}

func runRemoveProject(_ *cobra.Command, args []string) error {
	ctx := context.Background()

	registry, err := project.NewFileRegistry()
	if err != nil {
		return fmt.Errorf("opening project registry: %w", err)
	}
	defer registry.Close()

	id := args[0]

	// Try to get project info for display
	p, _ := registry.GetProject(ctx, id)

	if err := registry.RemoveProject(ctx, id); err != nil {
		if err == project.ErrProjectNotFound {
			return fmt.Errorf("project not found: %s", id)
		}
		return fmt.Errorf("removing project: %w", err)
	}

	if quiet {
		return nil
	}

	if p != nil {
		fmt.Printf("Project '%s' (%s) removed from registry.\n", p.Name, p.Path)
	} else {
		fmt.Printf("Project %s removed from registry.\n", id)
	}
	fmt.Println("(No files were deleted)")

	return nil
}

func runSetDefault(_ *cobra.Command, args []string) error {
	ctx := context.Background()

	registry, err := project.NewFileRegistry()
	if err != nil {
		return fmt.Errorf("opening project registry: %w", err)
	}
	defer registry.Close()

	id := args[0]

	// Get project info for display
	p, err := registry.GetProject(ctx, id)
	if err != nil {
		if err == project.ErrProjectNotFound {
			return fmt.Errorf("project not found: %s", id)
		}
		return fmt.Errorf("getting project: %w", err)
	}

	if err := registry.SetDefaultProject(ctx, id); err != nil {
		return fmt.Errorf("setting default: %w", err)
	}

	if !quiet {
		fmt.Printf("Default project set to '%s' (%s)\n", p.Name, p.Path)
	}

	return nil
}

func runValidateProject(_ *cobra.Command, args []string) error {
	ctx := context.Background()

	registry, err := project.NewFileRegistry()
	if err != nil {
		return fmt.Errorf("opening project registry: %w", err)
	}
	defer registry.Close()

	if len(args) == 0 {
		// Validate all
		if err := registry.ValidateAll(ctx); err != nil {
			fmt.Printf("Validation completed with warnings\n")
		} else {
			fmt.Printf("All projects validated successfully\n")
		}
		return nil
	}

	id := args[0]
	if err := registry.ValidateProject(ctx, id); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if !quiet {
		fmt.Printf("Project %s validated successfully\n", id)
	}

	return nil
}

// ResolveProject resolves the project to use based on the --project flag value.
// Resolution order:
// 1. If --project is specified: resolve by ID, name, or path
// 2. If current directory is a registered project: use that
// 3. Fall back to default project
func ResolveProject(ctx context.Context, flagValue string) (*project.Project, error) {
	registry, err := project.NewFileRegistry()
	if err != nil {
		return nil, fmt.Errorf("opening project registry: %w", err)
	}
	defer registry.Close()

	// If explicit project specified via flag
	if flagValue != "" {
		return resolveProjectByValue(ctx, registry, flagValue)
	}

	// Try current directory
	cwd, err := os.Getwd()
	if err == nil {
		p, err := registry.GetProjectByPath(ctx, cwd)
		if err == nil && p != nil {
			return p, nil
		}
	}

	// Fall back to default
	return registry.GetDefaultProject(ctx)
}

// resolveProjectByValue attempts to find a project by ID, name, or path
func resolveProjectByValue(ctx context.Context, registry project.Registry, value string) (*project.Project, error) {
	// Try by ID first
	if p, err := registry.GetProject(ctx, value); err == nil {
		return p, nil
	}

	// Try by path (both absolute and relative)
	path := value
	if !filepath.IsAbs(path) {
		if absPath, err := filepath.Abs(path); err == nil {
			path = absPath
		}
	}
	if p, err := registry.GetProjectByPath(ctx, path); err == nil {
		return p, nil
	}

	// Try by name (partial match)
	projects, err := registry.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	valueLower := strings.ToLower(value)
	var matches []*project.Project
	for _, p := range projects {
		if strings.ToLower(p.Name) == valueLower {
			return p, nil // Exact match
		}
		if strings.Contains(strings.ToLower(p.Name), valueLower) {
			matches = append(matches, p)
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Name
		}
		return nil, fmt.Errorf("ambiguous project name '%s', matches: %s", value, strings.Join(names, ", "))
	}

	return nil, fmt.Errorf("project not found: %s (tried ID, path, and name)", value)
}

// GetProjectID returns the project ID from the --project flag if set
func GetProjectID() string {
	return projectID
}

// GetResolvedProjectPath returns the resolved project path for use in config loading
func GetResolvedProjectPath(ctx context.Context) (string, error) {
	p, err := ResolveProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	if p == nil {
		return "", nil
	}
	return p.Path, nil
}
