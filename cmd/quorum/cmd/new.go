package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
)

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Reset workflow state and start fresh",
	Long: `Reset the current workflow state to start a new task with a clean slate.

By default, this command deactivates the current workflow without deleting any data.
Previous workflows remain accessible via 'quorum workflows' for reference.

Use flags for more aggressive cleanup:
  --archive   Move completed/failed workflows to archive (removes from list)
  --purge     Delete ALL workflow data permanently (requires confirmation)

Examples:
  quorum new              # Deactivate current workflow
  quorum new --archive    # Archive completed workflows
  quorum new --purge      # Delete all workflow data`,
	RunE: runNew,
}

var (
	newArchive bool
	newPurge   bool
	newForce   bool
)

func init() {
	rootCmd.AddCommand(newCmd)
	newCmd.Flags().BoolVar(&newArchive, "archive", false, "Archive completed/failed workflows")
	newCmd.Flags().BoolVar(&newPurge, "purge", false, "Delete ALL workflow data permanently")
	newCmd.Flags().BoolVarP(&newForce, "force", "f", false, "Skip confirmation prompts")
}

func runNew(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	statePath := viper.GetString("state.path")
	if statePath == "" {
		statePath = ".quorum/state/state.json"
	}
	backend := viper.GetString("state.backend")
	stateManager, err := state.NewStateManager(backend, statePath)
	if err != nil {
		return fmt.Errorf("creating state manager: %w", err)
	}
	defer func() {
		if closeErr := state.CloseStateManager(stateManager); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: closing state manager: %v\n", closeErr)
		}
	}()

	// Handle purge (most destructive)
	if newPurge {
		if !newForce {
			// Get workflow count for confirmation message
			workflows, _ := stateManager.ListWorkflows(ctx)
			count := len(workflows)
			if count == 0 {
				fmt.Println("No workflows to purge.")
				return nil
			}

			fmt.Printf("This will permanently delete %d workflow(s).\n", count)
			fmt.Print("Continue? [y/N] ")

			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))

			if response != "y" && response != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		deleted, err := stateManager.PurgeAllWorkflows(ctx)
		if err != nil {
			return fmt.Errorf("purging workflows: %w", err)
		}

		fmt.Printf("Purged %d workflow(s).\n", deleted)
		fmt.Println("All workflow state deleted. Ready for new task.")
		return nil
	}

	// Handle archive
	if newArchive {
		archived, err := stateManager.ArchiveWorkflows(ctx)
		if err != nil {
			return fmt.Errorf("archiving workflows: %w", err)
		}

		if archived > 0 {
			fmt.Printf("Archived %d completed workflow(s).\n", archived)
		}

		// Also deactivate
		if err := stateManager.DeactivateWorkflow(ctx); err != nil {
			return fmt.Errorf("deactivating workflow: %w", err)
		}

		fmt.Println("Ready for new task.")
		fmt.Println("Use 'quorum analyze <prompt>' to start a new workflow.")
		return nil
	}

	// Default: just deactivate
	activeID, _ := stateManager.GetActiveWorkflowID(ctx)
	if activeID == "" {
		fmt.Println("No active workflow.")
		fmt.Println("Use 'quorum analyze <prompt>' to start a new workflow.")
		return nil
	}

	if err := stateManager.DeactivateWorkflow(ctx); err != nil {
		return fmt.Errorf("deactivating workflow: %w", err)
	}

	fmt.Println("Workflow deactivated. Ready for new task.")
	fmt.Println("Use 'quorum analyze <prompt>' to start a new workflow.")
	fmt.Println("Previous workflows still available via 'quorum workflows'.")

	return nil
}
