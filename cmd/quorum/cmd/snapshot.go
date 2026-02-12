package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/snapshot"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Export and import Quorum project snapshots",
}

var snapshotExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export projects and registry state into a snapshot archive",
	RunE:  runSnapshotExport,
}

var snapshotImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import projects and registry state from a snapshot archive",
	RunE:  runSnapshotImport,
}

var snapshotValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a snapshot archive without importing it",
	RunE:  runSnapshotValidate,
}

var (
	snapshotExportOutputPath       string
	snapshotExportProjectIDs       []string
	snapshotExportIncludeWorktrees bool

	snapshotImportInputPath        string
	snapshotImportMode             string
	snapshotImportDryRun           bool
	snapshotImportConflictPolicy   string
	snapshotImportPathMap          []string
	snapshotImportPreserveIDs      bool
	snapshotImportIncludeWorktrees bool
	snapshotValidateInputPath      string
)

func init() {
	rootCmd.AddCommand(snapshotCmd)
	snapshotCmd.AddCommand(snapshotExportCmd)
	snapshotCmd.AddCommand(snapshotImportCmd)
	snapshotCmd.AddCommand(snapshotValidateCmd)

	snapshotExportCmd.Flags().StringVarP(&snapshotExportOutputPath, "output", "o", "", "Output .tar.gz path (default: ./quorum-snapshot-<timestamp>.tar.gz)")
	snapshotExportCmd.Flags().StringSliceVar(&snapshotExportProjectIDs, "project-id", nil, "Project IDs to export (repeatable). If omitted, exports all projects")
	snapshotExportCmd.Flags().BoolVar(&snapshotExportIncludeWorktrees, "include-worktrees", false, "Include .worktrees directories in the snapshot")

	snapshotImportCmd.Flags().StringVarP(&snapshotImportInputPath, "input", "i", "", "Input .tar.gz snapshot path")
	snapshotImportCmd.Flags().StringVar(&snapshotImportMode, "mode", string(snapshot.ImportModeMerge), "Import mode: merge | replace")
	snapshotImportCmd.Flags().BoolVar(&snapshotImportDryRun, "dry-run", false, "Preview import actions without writing files")
	snapshotImportCmd.Flags().StringVar(&snapshotImportConflictPolicy, "conflict-policy", string(snapshot.ConflictSkip), "Conflict policy: skip | overwrite | fail")
	snapshotImportCmd.Flags().StringArrayVar(&snapshotImportPathMap, "path-map", nil, "Path remap in form old=new (repeatable)")
	snapshotImportCmd.Flags().BoolVar(&snapshotImportPreserveIDs, "preserve-ids", false, "Preserve project IDs from snapshot")
	snapshotImportCmd.Flags().BoolVar(&snapshotImportIncludeWorktrees, "include-worktrees", true, "Restore .worktrees files from the snapshot")
	_ = snapshotImportCmd.MarkFlagRequired("input")

	snapshotValidateCmd.Flags().StringVarP(&snapshotValidateInputPath, "input", "i", "", "Input .tar.gz snapshot path")
	_ = snapshotValidateCmd.MarkFlagRequired("input")
}

func runSnapshotExport(_ *cobra.Command, _ []string) error {
	outputPath := strings.TrimSpace(snapshotExportOutputPath)
	if outputPath == "" {
		outputPath = filepath.Join(".", fmt.Sprintf("quorum-snapshot-%s.tar.gz", time.Now().UTC().Format("20060102-150405")))
	}

	result, err := snapshot.Export(&snapshot.ExportOptions{
		OutputPath:       outputPath,
		IncludeWorktrees: snapshotExportIncludeWorktrees,
		ProjectIDs:       snapshotExportProjectIDs,
		QuorumVersion:    GetVersion(),
	})
	if err != nil {
		return err
	}

	if quiet {
		fmt.Println(result.OutputPath)
		return nil
	}

	fmt.Printf("Snapshot exported to %s\n", result.OutputPath)
	fmt.Printf("Projects: %d\n", result.Manifest.ProjectCount)
	fmt.Printf("Files: %d\n", len(result.Manifest.Files))
	fmt.Printf("Include worktrees: %t\n", result.Manifest.IncludeWorktrees)
	return nil
}

func runSnapshotImport(_ *cobra.Command, _ []string) error {
	pathMap, err := parsePathMapFlags(snapshotImportPathMap)
	if err != nil {
		return err
	}

	report, err := snapshot.Import(&snapshot.ImportOptions{
		InputPath:          snapshotImportInputPath,
		Mode:               snapshot.ImportMode(snapshotImportMode),
		DryRun:             snapshotImportDryRun,
		ConflictPolicy:     snapshot.ConflictPolicy(snapshotImportConflictPolicy),
		PathMap:            pathMap,
		PreserveProjectIDs: snapshotImportPreserveIDs,
		IncludeWorktrees:   snapshotImportIncludeWorktrees,
	})
	if err != nil {
		return err
	}

	if quiet {
		b, marshalErr := json.Marshal(report)
		if marshalErr != nil {
			return marshalErr
		}
		fmt.Println(string(b))
		return nil
	}

	fmt.Printf("Snapshot import complete (mode=%s, dry_run=%t)\n", report.Mode, report.DryRun)
	fmt.Printf("Projects processed: %d\n", len(report.Projects))
	fmt.Printf("Files restored: %d\n", report.RestoredFiles)
	fmt.Printf("Files skipped: %d\n", report.SkippedFiles)
	if len(report.Conflicts) > 0 {
		fmt.Printf("Conflicts: %d\n", len(report.Conflicts))
	}
	if len(report.Warnings) > 0 {
		fmt.Printf("Warnings: %d\n", len(report.Warnings))
	}
	return nil
}

func runSnapshotValidate(_ *cobra.Command, _ []string) error {
	manifest, err := snapshot.ValidateSnapshot(snapshotValidateInputPath)
	if err != nil {
		return err
	}

	if quiet {
		b, marshalErr := json.Marshal(manifest)
		if marshalErr != nil {
			return marshalErr
		}
		fmt.Println(string(b))
		return nil
	}

	fmt.Printf("Snapshot valid: projects=%d files=%d include_worktrees=%t\n", manifest.ProjectCount, len(manifest.Files), manifest.IncludeWorktrees)
	return nil
}

func parsePathMapFlags(raw []string) (map[string]string, error) {
	if len(raw) == 0 {
		return map[string]string{}, nil
	}

	result := make(map[string]string, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --path-map value %q (expected old=new)", item)
		}
		from := strings.TrimSpace(parts[0])
		to := strings.TrimSpace(parts[1])
		if from == "" || to == "" {
			return nil, fmt.Errorf("invalid --path-map value %q (old and new must be non-empty)", item)
		}
		result[from] = to
	}
	return result, nil
}
