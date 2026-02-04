package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

var openCmd = &cobra.Command{
	Use:   "open [path]",
	Short: "Initialize and register a directory as a Quorum project",
	Long: `Open a directory as a Quorum project.

This command combines 'init' and 'project add' into a single step:
1. If the directory doesn't have a .quorum folder, it initializes one
2. Registers the project in the global registry if not already registered

If no path is specified, the current directory is used.

Examples:
  # Open current directory as a Quorum project
  quorum open

  # Open a specific directory
  quorum open /path/to/project

  # Open with a custom project name
  quorum open --name "My Project"

  # Open and set as default project
  quorum open --default`,
	Args: cobra.MaximumNArgs(1),
	RunE: runOpen,
}

var (
	openProjectName    string
	openProjectColor   string
	openProjectDefault bool
	openForce          bool
)

func init() {
	rootCmd.AddCommand(openCmd)

	openCmd.Flags().StringVar(&openProjectName, "name", "", "Custom name for the project")
	openCmd.Flags().StringVar(&openProjectColor, "color", "", "Custom color for the project (hex format)")
	openCmd.Flags().BoolVar(&openProjectDefault, "default", false, "Set as default project")
	openCmd.Flags().BoolVar(&openForce, "force", false, "Overwrite existing configuration during init")
}

func runOpen(_ *cobra.Command, args []string) error {
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

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", absPath)
		}
		return fmt.Errorf("cannot access path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absPath)
	}

	// Step 1: Initialize .quorum if it doesn't exist
	quorumDir := filepath.Join(absPath, ".quorum")
	configPath := filepath.Join(quorumDir, "config.yaml")

	needsInit := false
	if _, err := os.Stat(quorumDir); os.IsNotExist(err) {
		needsInit = true
	} else if _, err := os.Stat(configPath); os.IsNotExist(err) {
		needsInit = true
	}

	if needsInit {
		if err := initializeProject(absPath, openForce); err != nil {
			return fmt.Errorf("initializing project: %w", err)
		}
		if !quiet {
			fmt.Printf("Initialized Quorum project in %s\n", absPath)
		}
	} else if !quiet {
		fmt.Printf("Project already initialized at %s\n", absPath)
	}

	// Step 2: Register in the project registry
	registry, err := project.NewFileRegistry()
	if err != nil {
		return fmt.Errorf("opening project registry: %w", err)
	}
	defer registry.Close()

	// Check if already registered
	existingProject, _ := registry.GetProjectByPath(ctx, absPath)
	if existingProject != nil {
		if !quiet {
			fmt.Printf("Project already registered: %s (%s)\n", existingProject.Name, existingProject.ID)
		}

		// Still set as default if requested
		if openProjectDefault {
			if err := registry.SetDefaultProject(ctx, existingProject.ID); err != nil {
				return fmt.Errorf("setting as default: %w", err)
			}
			if !quiet {
				fmt.Printf("Set as default project\n")
			}
		}
		return nil
	}

	// Add project
	opts := &project.AddProjectOptions{
		Name:  openProjectName,
		Color: openProjectColor,
	}

	p, err := registry.AddProject(ctx, absPath, opts)
	if err != nil {
		return fmt.Errorf("registering project: %w", err)
	}

	// Set as default if requested
	if openProjectDefault {
		if err := registry.SetDefaultProject(ctx, p.ID); err != nil {
			return fmt.Errorf("setting as default: %w", err)
		}
	}

	if quiet {
		fmt.Println(p.ID)
		return nil
	}

	fmt.Printf("\nProject registered successfully!\n")
	fmt.Printf("  ID:    %s\n", p.ID)
	fmt.Printf("  Name:  %s\n", p.Name)
	fmt.Printf("  Path:  %s\n", p.Path)
	if openProjectDefault {
		fmt.Printf("  Default: yes\n")
	}

	return nil
}

// initializeProject creates the .quorum directory structure and config
func initializeProject(absPath string, force bool) error {
	quorumDir := filepath.Join(absPath, ".quorum")
	configPath := filepath.Join(quorumDir, "config.yaml")

	// Check for existing config
	if _, err := os.Stat(configPath); err == nil && !force {
		return nil // Already initialized
	}

	// Create .quorum directory
	if err := os.MkdirAll(quorumDir, 0o750); err != nil {
		return fmt.Errorf("creating .quorum directory: %w", err)
	}

	// Create default config
	if err := os.WriteFile(configPath, []byte(config.DefaultConfigYAML), 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	// Create subdirectories
	dirs := []string{
		"state",
		"logs",
		"runs",
	}

	for _, dir := range dirs {
		dirPath := filepath.Join(quorumDir, dir)
		if err := os.MkdirAll(dirPath, 0o750); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	return nil
}
