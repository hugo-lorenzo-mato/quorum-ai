package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTraceManifest(t *testing.T, dir string, manifest traceManifestView) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run.json"), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestListTraceEntriesOrdersByStartedAt(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	older := time.Now().Add(-2 * time.Hour).UTC()
	newer := time.Now().Add(-1 * time.Hour).UTC()

	writeTraceManifest(t, filepath.Join(base, "run-1"), traceManifestView{
		RunID:     "run-1",
		StartedAt: older,
		Summary:   traceSummaryView{},
	})
	writeTraceManifest(t, filepath.Join(base, "run-2"), traceManifestView{
		RunID:     "run-2",
		StartedAt: newer,
		Summary:   traceSummaryView{},
	})

	entries, err := listTraceEntries(base)
	if err != nil {
		t.Fatalf("listTraceEntries error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].RunID != "run-2" {
		t.Fatalf("expected latest run first, got %s", entries[0].RunID)
	}
}

func TestResolveTraceManifestUsesLatest(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	older := time.Now().Add(-2 * time.Hour).UTC()
	newer := time.Now().Add(-1 * time.Hour).UTC()

	writeTraceManifest(t, filepath.Join(base, "run-1"), traceManifestView{
		RunID:     "run-1",
		StartedAt: older,
		Summary:   traceSummaryView{},
	})
	writeTraceManifest(t, filepath.Join(base, "run-2"), traceManifestView{
		RunID:     "run-2",
		StartedAt: newer,
		Summary:   traceSummaryView{},
	})

	manifest, runDir, err := resolveTraceManifest(base, "")
	if err != nil {
		t.Fatalf("resolveTraceManifest error: %v", err)
	}
	if manifest.RunID != "run-2" {
		t.Fatalf("expected latest manifest, got %s", manifest.RunID)
	}
	if filepath.Base(runDir) != "run-2" {
		t.Fatalf("expected run dir to match latest run")
	}
}

func TestResolveTraceManifestByID(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	writeTraceManifest(t, filepath.Join(base, "run-1"), traceManifestView{
		RunID:     "run-1",
		StartedAt: time.Now().UTC(),
		Summary:   traceSummaryView{},
	})

	manifest, runDir, err := resolveTraceManifest(base, "run-1")
	if err != nil {
		t.Fatalf("resolveTraceManifest error: %v", err)
	}
	if manifest.RunID != "run-1" {
		t.Fatalf("expected run-1, got %s", manifest.RunID)
	}
	if filepath.Base(runDir) != "run-1" {
		t.Fatalf("expected run dir to match run-1")
	}
}
