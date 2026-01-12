package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show workflow status",
	Long:  "Display the current state of the workflow including task progress.",
	RunE:  runStatus,
}

var (
	statusJSON bool
)

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output as JSON")
}

func runStatus(cmd *cobra.Command, _ []string) error {
	statePath := viper.GetString("state.path")
	if statePath == "" {
		statePath = ".quorum/state/state.json"
	}
	stateManager := state.NewJSONStateManager(statePath)

	if !stateManager.Exists() {
		fmt.Println("No active workflow")
		return nil
	}

	workflowState, err := stateManager.Load(cmd.Context())
	if err != nil {
		return err
	}

	if statusJSON {
		return outputJSON(workflowState)
	}

	// Text output
	fmt.Printf("Workflow ID: %s\n", workflowState.WorkflowID)
	fmt.Printf("Phase: %s\n", workflowState.CurrentPhase)
	fmt.Printf("Status: %s\n", workflowState.Status)
	fmt.Println()

	// Task table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TASK\tPHASE\tSTATUS\tDURATION")
	fmt.Fprintln(w, "----\t-----\t------\t--------")

	for _, task := range workflowState.Tasks {
		duration := "-"
		if task.StartedAt != nil && task.CompletedAt != nil {
			duration = task.CompletedAt.Sub(*task.StartedAt).String()
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			task.Name, task.Phase, task.Status, duration)
	}
	w.Flush()

	return nil
}

func outputJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
