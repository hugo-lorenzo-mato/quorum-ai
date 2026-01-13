package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var traceCmd = &cobra.Command{
	Use:   "trace",
	Short: "Show trace summaries",
	Long:  "Display trace summaries from previous workflow runs.",
	RunE:  runTraceCmd,
}

var (
	traceDirFlag string
	traceRunID   string
	traceList    bool
	traceLimit   int
	traceJSON    bool
)

func init() {
	rootCmd.AddCommand(traceCmd)
	traceCmd.Flags().StringVar(&traceDirFlag, "dir", "", "Trace directory (overrides config)")
	traceCmd.Flags().StringVar(&traceRunID, "run-id", "", "Trace run ID to show")
	traceCmd.Flags().BoolVar(&traceList, "list", false, "List available trace runs")
	traceCmd.Flags().IntVar(&traceLimit, "limit", 10, "Limit number of runs in list output")
	traceCmd.Flags().BoolVar(&traceJSON, "json", false, "Output trace manifest as JSON")
}

func runTraceCmd(_ *cobra.Command, _ []string) error {
	traceDir := resolveTraceDir(traceDirFlag)

	if traceList {
		entries, err := listTraceEntries(traceDir)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Printf("No traces found in %s\n", traceDir)
			return nil
		}
		if traceLimit > 0 && len(entries) > traceLimit {
			entries = entries[:traceLimit]
		}
		return renderTraceList(entries)
	}

	manifest, runDir, err := resolveTraceManifest(traceDir, traceRunID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("No traces found in %s\n", traceDir)
			return nil
		}
		return err
	}

	if traceJSON {
		return outputJSON(manifest)
	}

	return renderTraceSummary(manifest, runDir)
}

func resolveTraceDir(override string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}
	traceDir := viper.GetString("trace.dir")
	if strings.TrimSpace(traceDir) == "" {
		traceDir = ".quorum/traces"
	}
	return traceDir
}

func resolveTraceManifest(baseDir, runID string) (*traceManifestView, string, error) {
	if strings.TrimSpace(runID) != "" {
		runDir := filepath.Join(baseDir, runID)
		manifest, err := readTraceManifest(filepath.Join(runDir, "run.json"))
		if err != nil {
			return nil, "", err
		}
		return manifest, runDir, nil
	}

	entries, err := listTraceEntries(baseDir)
	if err != nil {
		return nil, "", err
	}
	if len(entries) == 0 {
		return nil, "", os.ErrNotExist
	}

	selected := entries[0]
	runDir := filepath.Join(baseDir, selected.RunID)
	manifest, err := readTraceManifest(filepath.Join(runDir, "run.json"))
	if err != nil {
		return nil, "", err
	}

	return manifest, runDir, nil
}

func listTraceEntries(baseDir string) ([]traceEntry, error) {
	items, err := os.ReadDir(baseDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	entries := make([]traceEntry, 0)
	for _, item := range items {
		if !item.IsDir() {
			continue
		}
		runID := item.Name()
		runDir := filepath.Join(baseDir, runID)
		manifest, err := readTraceManifest(filepath.Join(runDir, "run.json"))
		if err != nil {
			continue
		}

		entries = append(entries, traceEntry{
			RunID:      manifest.RunID,
			WorkflowID: manifest.WorkflowID,
			StartedAt:  manifest.StartedAt,
			EndedAt:    manifest.EndedAt,
			Summary:    manifest.Summary,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].StartedAt.After(entries[j].StartedAt)
	})

	return entries, nil
}

func readTraceManifest(path string) (*traceManifestView, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest traceManifestView
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func renderTraceList(entries []traceEntry) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RUN ID\tWORKFLOW\tSTARTED\tENDED\tPROMPTS\tTOKENS IN\tTOKENS OUT\tCOST")
	fmt.Fprintln(w, "------\t--------\t-------\t-----\t-------\t---------\t----------\t----")

	for _, entry := range entries {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%d\t%d\t$%.4f\n",
			entry.RunID,
			entry.WorkflowID,
			formatTime(entry.StartedAt),
			formatTime(entry.EndedAt),
			entry.Summary.TotalPrompts,
			entry.Summary.TotalTokensIn,
			entry.Summary.TotalTokensOut,
			entry.Summary.TotalCostUSD,
		)
	}

	return w.Flush()
}

func renderTraceSummary(manifest *traceManifestView, runDir string) error {
	fmt.Printf("Trace run: %s\n", manifest.RunID)
	fmt.Printf("Workflow: %s\n", manifest.WorkflowID)
	fmt.Printf("Started: %s\n", formatTime(manifest.StartedAt))
	fmt.Printf("Ended: %s\n", formatTime(manifest.EndedAt))
	fmt.Printf("Prompt length: %d\n", manifest.PromptLength)
	fmt.Printf("Trace mode: %s\n", manifest.Config.Mode)
	fmt.Printf("Trace dir: %s\n", runDir)
	fmt.Println()
	fmt.Printf("Prompts: %d\n", manifest.Summary.TotalPrompts)
	fmt.Printf("Tokens in: %d\n", manifest.Summary.TotalTokensIn)
	fmt.Printf("Tokens out: %d\n", manifest.Summary.TotalTokensOut)
	fmt.Printf("Cost: $%.4f\n", manifest.Summary.TotalCostUSD)
	fmt.Printf("Files: %d\n", manifest.Summary.TotalFiles)
	fmt.Printf("Bytes: %d\n", manifest.Summary.TotalBytes)
	fmt.Println()

	if manifest.GitCommit != "" {
		dirty := "clean"
		if manifest.GitDirty {
			dirty = "dirty"
		}
		fmt.Printf("Git: %s (%s)\n", manifest.GitCommit, dirty)
	}
	if manifest.AppVersion != "" {
		fmt.Printf("App: %s (%s %s)\n", manifest.AppVersion, manifest.AppCommit, manifest.AppDate)
	}

	return nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(time.RFC3339)
}

type traceEntry struct {
	RunID      string
	WorkflowID string
	StartedAt  time.Time
	EndedAt    time.Time
	Summary    traceSummaryView
}

type traceManifestView struct {
	RunID        string           `json:"run_id"`
	WorkflowID   string           `json:"workflow_id"`
	PromptLength int              `json:"prompt_length"`
	StartedAt    time.Time        `json:"started_at"`
	EndedAt      time.Time        `json:"ended_at"`
	AppVersion   string           `json:"app_version"`
	AppCommit    string           `json:"app_commit"`
	AppDate      string           `json:"app_date"`
	GitCommit    string           `json:"git_commit"`
	GitDirty     bool             `json:"git_dirty"`
	Config       traceConfigView  `json:"config"`
	Summary      traceSummaryView `json:"summary"`
}

type traceConfigView struct {
	Mode string `json:"mode"`
	Dir  string `json:"dir"`
}

type traceSummaryView struct {
	TotalPrompts   int     `json:"total_prompts"`
	TotalTokensIn  int     `json:"total_tokens_in"`
	TotalTokensOut int     `json:"total_tokens_out"`
	TotalCostUSD   float64 `json:"total_cost_usd"`
	TotalFiles     int     `json:"total_files"`
	TotalBytes     int64   `json:"total_bytes"`
}
