package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

func startTraceWriter(t *testing.T, cfg TraceConfig) TraceWriter {
	t.Helper()

	writer := NewTraceWriter(cfg, logging.NewNop())
	if !writer.Enabled() {
		t.Fatalf("trace writer disabled")
	}

	err := writer.StartRun(context.Background(), TraceRunInfo{
		RunID:        "test-run",
		WorkflowID:   "wf-test",
		PromptLength: 5,
		StartedAt:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("StartRun error: %v", err)
	}

	return writer
}

func readTraceRecords(t *testing.T, dir string) []traceRecord {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, "trace.jsonl"))
	if err != nil {
		t.Fatalf("reading trace.jsonl: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	records := make([]traceRecord, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var record traceRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("unmarshal trace record: %v", err)
		}
		records = append(records, record)
	}

	return records
}

func TestTraceWriterSummaryMode(t *testing.T) {
	dir := t.TempDir()
	writer := startTraceWriter(t, TraceConfig{
		Mode:          "summary",
		Dir:           dir,
		Redact:        false,
		MaxBytes:      1024,
		TotalMaxBytes: 4096,
		MaxFiles:      10,
	})

	err := writer.Record(context.Background(), TraceEvent{
		Phase:     "analyze",
		Step:      "v1",
		EventType: "prompt",
		FileExt:   "txt",
		Content:   []byte("hello"),
	})
	if err != nil {
		t.Fatalf("Record error: %v", err)
	}

	writer.EndRun(context.Background())

	entries, err := os.ReadDir(writer.Dir())
	if err != nil {
		t.Fatalf("ReadDir error: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name != "run.json" && name != "trace.jsonl" {
			t.Fatalf("unexpected file in summary trace: %s", name)
		}
	}
}

func TestTraceWriterOffModeNoDir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "traces")
	writer := NewTraceWriter(TraceConfig{
		Mode: "off",
		Dir:  dir,
	}, logging.NewNop())
	if writer.Enabled() {
		t.Fatalf("expected trace writer to be disabled")
	}

	if err := writer.StartRun(context.Background(), TraceRunInfo{
		RunID:     "test-run",
		StartedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("StartRun error: %v", err)
	}

	if _, err := os.Stat(dir); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected trace dir to not exist")
	}
}

func TestTraceWriterFullModeCreatesFiles(t *testing.T) {
	dir := t.TempDir()
	writer := startTraceWriter(t, TraceConfig{
		Mode:          "full",
		Dir:           dir,
		Redact:        false,
		MaxBytes:      1024,
		TotalMaxBytes: 4096,
		MaxFiles:      10,
	})

	err := writer.Record(context.Background(), TraceEvent{
		Phase:     "plan",
		Step:      "generate",
		EventType: "prompt",
		FileExt:   "txt",
		Content:   []byte("plan content"),
	})
	if err != nil {
		t.Fatalf("Record error: %v", err)
	}

	writer.EndRun(context.Background())

	records := readTraceRecords(t, writer.Dir())
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].File == "" {
		t.Fatalf("expected file reference in trace record")
	}

	if _, err := os.Stat(filepath.Join(writer.Dir(), records[0].File)); err != nil {
		t.Fatalf("expected trace file to exist: %v", err)
	}
}

func TestTraceWriterUnwritableDir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "trace-file")
	if err := os.WriteFile(dir, []byte("blocked"), 0o644); err != nil {
		t.Fatalf("writing marker file: %v", err)
	}

	writer := NewTraceWriter(TraceConfig{
		Mode:          "full",
		Dir:           dir,
		Redact:        false,
		MaxBytes:      1024,
		TotalMaxBytes: 4096,
		MaxFiles:      10,
	}, logging.NewNop())

	if err := writer.StartRun(context.Background(), TraceRunInfo{
		RunID:     "test-run",
		StartedAt: time.Now().UTC(),
	}); err == nil {
		t.Fatalf("expected StartRun to fail for unwritable dir")
	}
	if writer.Enabled() {
		t.Fatalf("expected trace writer to disable after StartRun failure")
	}
}

func TestTraceWriterRedactionDefault(t *testing.T) {
	dir := t.TempDir()
	writer := startTraceWriter(t, TraceConfig{
		Mode:          "full",
		Dir:           dir,
		Redact:        true,
		MaxBytes:      2048,
		TotalMaxBytes: 4096,
		MaxFiles:      10,
	})

	secret := "sk-1234567890abcdef1234"
	content := "token=" + secret

	err := writer.Record(context.Background(), TraceEvent{
		Phase:     "execute",
		Step:      "task",
		EventType: "response",
		FileExt:   "txt",
		Content:   []byte(content),
	})
	if err != nil {
		t.Fatalf("Record error: %v", err)
	}

	writer.EndRun(context.Background())

	records := readTraceRecords(t, writer.Dir())
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if !records[0].ContentRedacted {
		t.Fatalf("expected content redaction")
	}
	if records[0].HashRaw == records[0].HashStored {
		t.Fatalf("expected hash_raw and hash_stored to differ")
	}

	filePath := filepath.Join(writer.Dir(), records[0].File)
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("reading trace file: %v", err)
	}
	if strings.Contains(string(data), secret) {
		t.Fatalf("secret should be redacted")
	}
	if !strings.Contains(string(data), "[REDACTED]") {
		t.Fatalf("expected redaction marker")
	}
}

func TestTraceWriterTruncation(t *testing.T) {
	dir := t.TempDir()
	writer := startTraceWriter(t, TraceConfig{
		Mode:          "full",
		Dir:           dir,
		Redact:        false,
		MaxBytes:      40,
		TotalMaxBytes: 4096,
		MaxFiles:      10,
	})

	content := strings.Repeat("a", 200)
	err := writer.Record(context.Background(), TraceEvent{
		Phase:     "analyze",
		Step:      "v1",
		EventType: "response",
		FileExt:   "txt",
		Content:   []byte(content),
	})
	if err != nil {
		t.Fatalf("Record error: %v", err)
	}

	writer.EndRun(context.Background())

	records := readTraceRecords(t, writer.Dir())
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if !records[0].ContentTruncated {
		t.Fatalf("expected content truncated")
	}

	filePath := filepath.Join(writer.Dir(), records[0].File)
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("reading trace file: %v", err)
	}
	if !strings.Contains(string(data), "[trace truncated]") {
		t.Fatalf("expected truncation marker")
	}
}

func TestTraceWriterTotalMaxBytes(t *testing.T) {
	dir := t.TempDir()
	writer := startTraceWriter(t, TraceConfig{
		Mode:          "full",
		Dir:           dir,
		Redact:        false,
		MaxBytes:      20,
		TotalMaxBytes: 25,
		MaxFiles:      10,
	})

	content := strings.Repeat("b", 100)
	ctx := context.Background()
	if err := writer.Record(ctx, TraceEvent{
		Phase:     "plan",
		Step:      "generate",
		EventType: "prompt",
		FileExt:   "txt",
		Content:   []byte(content),
	}); err != nil {
		t.Fatalf("Record error: %v", err)
	}
	if err := writer.Record(ctx, TraceEvent{
		Phase:     "plan",
		Step:      "generate",
		EventType: "response",
		FileExt:   "txt",
		Content:   []byte(content),
	}); err != nil {
		t.Fatalf("Record error: %v", err)
	}

	writer.EndRun(ctx)

	records := readTraceRecords(t, writer.Dir())
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].File == "" {
		t.Fatalf("expected first record file")
	}
	if !records[1].ContentDropped || records[1].File != "" {
		t.Fatalf("expected second record to be dropped due to total_max_bytes")
	}
}

func TestTraceWriterConcurrentSequence(t *testing.T) {
	dir := t.TempDir()
	writer := startTraceWriter(t, TraceConfig{
		Mode:          "summary",
		Dir:           dir,
		Redact:        false,
		MaxBytes:      1024,
		TotalMaxBytes: 4096,
		MaxFiles:      10,
	})

	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = writer.Record(ctx, TraceEvent{
				Phase:     "analyze",
				Step:      "v1",
				EventType: "prompt",
				FileExt:   "txt",
				Content:   []byte("ping"),
			})
		}()
	}
	wg.Wait()

	writer.EndRun(ctx)

	records := readTraceRecords(t, writer.Dir())
	if len(records) != 20 {
		t.Fatalf("expected 20 records, got %d", len(records))
	}

	seen := make(map[int]bool, 20)
	for _, record := range records {
		if seen[record.Seq] {
			t.Fatalf("duplicate seq %d", record.Seq)
		}
		seen[record.Seq] = true
	}
	for i := 1; i <= 20; i++ {
		if !seen[i] {
			t.Fatalf("missing seq %d", i)
		}
	}
}
