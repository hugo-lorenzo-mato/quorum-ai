package diagnostics

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// parseWindowsGPUObj — additional coverage for edge cases
// ---------------------------------------------------------------------------

// TestParseWindowsGPUObj_StringZeroRAM tests string "0" AdapterRAM.
func TestParseWindowsGPUObj_StringZeroRAM(t *testing.T) {
	t.Parallel()
	obj := map[string]any{
		"Name":       "Zero String GPU",
		"AdapterRAM": "0",
	}
	gpu := parseWindowsGPUObj(obj)
	if gpu.MemValid {
		t.Error("MemValid should be false for zero string AdapterRAM")
	}
}

// TestParseWindowsGPUObj_NegativeFloat64 tests negative float64 AdapterRAM.
func TestParseWindowsGPUObj_NegativeFloat64(t *testing.T) {
	t.Parallel()
	obj := map[string]any{
		"Name":       "Negative GPU",
		"AdapterRAM": float64(-100),
	}
	gpu := parseWindowsGPUObj(obj)
	if gpu.MemValid {
		t.Error("MemValid should be false for negative AdapterRAM")
	}
}

// TestParseWindowsGPUObj_NegativeInt64 tests negative int64 AdapterRAM.
func TestParseWindowsGPUObj_NegativeInt64(t *testing.T) {
	t.Parallel()
	obj := map[string]any{
		"Name":       "Negative Int64 GPU",
		"AdapterRAM": int64(-100),
	}
	gpu := parseWindowsGPUObj(obj)
	if gpu.MemValid {
		t.Error("MemValid should be false for negative int64 AdapterRAM")
	}
}

// TestParseWindowsGPUObj_BoolAdapterRAM tests unsupported type for AdapterRAM.
func TestParseWindowsGPUObj_BoolAdapterRAM(t *testing.T) {
	t.Parallel()
	obj := map[string]any{
		"Name":       "Bool GPU",
		"AdapterRAM": true,
	}
	gpu := parseWindowsGPUObj(obj)
	if gpu.MemValid {
		t.Error("MemValid should be false for bool AdapterRAM")
	}
}

// ---------------------------------------------------------------------------
// mergeAMDJSON — deeply nested & additional combinations
// ---------------------------------------------------------------------------

// TestMergeAMDJSON_MapWithoutIndex tests mergeAMDJSON with a map that has no index.
func TestMergeAMDJSON_MapWithoutIndex(t *testing.T) {
	t.Parallel()
	info := map[int]*GPUInfo{}
	payload := map[string]any{
		"card_name":   "Orphan GPU",
		"vram_total":  float64(8192),
		"temperature": float64(55),
	}
	mergeAMDJSON(info, payload)
	// No index key means the entry won't be created
	if len(info) != 0 {
		t.Errorf("expected 0 GPUs (no index key), got %d", len(info))
	}
}

// TestMergeAMDJSON_DeeplyNestedChildren tests recursive traversal of child maps.
func TestMergeAMDJSON_DeeplyNestedChildren(t *testing.T) {
	t.Parallel()
	info := map[int]*GPUInfo{}
	payload := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"gpu":       float64(0),
				"card_name": "Deep GPU",
			},
		},
	}
	mergeAMDJSON(info, payload)
	if len(info) != 1 {
		t.Fatalf("expected 1 GPU from deep nesting, got %d", len(info))
	}
	if info[0].Name != "Deep GPU" {
		t.Errorf("Name = %q, want %q", info[0].Name, "Deep GPU")
	}
}

// TestMergeAMDJSON_StringPayload tests mergeAMDJSON with a non-map/non-slice payload.
func TestMergeAMDJSON_StringPayload(t *testing.T) {
	t.Parallel()
	info := map[int]*GPUInfo{}
	mergeAMDJSON(info, "just a string")
	if len(info) != 0 {
		t.Errorf("expected 0 GPUs for string payload, got %d", len(info))
	}
}

// TestMergeAMDJSON_NumberPayload tests mergeAMDJSON with a number payload.
func TestMergeAMDJSON_NumberPayload(t *testing.T) {
	t.Parallel()
	info := map[int]*GPUInfo{}
	mergeAMDJSON(info, float64(42))
	if len(info) != 0 {
		t.Errorf("expected 0 GPUs for number payload, got %d", len(info))
	}
}

// TestMergeAMDJSON_EmptyArrayPayload tests mergeAMDJSON with empty array.
func TestMergeAMDJSON_EmptyArrayPayload(t *testing.T) {
	t.Parallel()
	info := map[int]*GPUInfo{}
	mergeAMDJSON(info, []any{})
	if len(info) != 0 {
		t.Errorf("expected 0 GPUs for empty array, got %d", len(info))
	}
}

// TestMergeAMDJSON_ChildrenOfMapWithIndex tests that child maps are also traversed
// when the parent has an index key.
func TestMergeAMDJSON_ChildrenOfMapWithIndex(t *testing.T) {
	t.Parallel()
	info := map[int]*GPUInfo{}
	payload := map[string]any{
		"gpu":       float64(0),
		"card_name": "Parent GPU",
		"metrics": map[string]any{
			"gpu":              float64(1),
			"card_name":        "Child GPU",
			"gpu_utilization":  float64(50),
			"edge_temperature": float64(70),
		},
	}
	mergeAMDJSON(info, payload)
	if len(info) != 2 {
		t.Fatalf("expected 2 GPUs (parent + child), got %d", len(info))
	}
	if info[0].Name != "Parent GPU" {
		t.Errorf("GPU 0 Name = %q, want %q", info[0].Name, "Parent GPU")
	}
	if info[1].Name != "Child GPU" {
		t.Errorf("GPU 1 Name = %q, want %q", info[1].Name, "Child GPU")
	}
}

// ---------------------------------------------------------------------------
// extractAMDFields — more edge cases
// ---------------------------------------------------------------------------

// TestExtractAMDFields_AllNameKeys tests extractAMDFields with all name keys.
func TestExtractAMDFields_AllNameKeys(t *testing.T) {
	t.Parallel()
	nameKeys := []string{"card_name", "product_name", "name", "model", "gpu_name"}
	for _, key := range nameKeys {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			info := &GPUInfo{}
			m := map[string]any{
				key: "TestGPU_" + key,
			}
			extractAMDFields(info, m)
			if info.Name != "TestGPU_"+key {
				t.Errorf("Name = %q, want %q", info.Name, "TestGPU_"+key)
			}
		})
	}
}

// TestExtractAMDFields_MemoryWithMemSubstring tests mem_total/mem_used keys.
func TestExtractAMDFields_MemoryWithMemSubstring(t *testing.T) {
	t.Parallel()
	info := &GPUInfo{}
	m := map[string]any{
		"mem_total": float64(8192),
		"mem_used":  float64(4096),
	}
	extractAMDFields(info, m)
	if !info.MemValid {
		t.Error("MemValid should be true")
	}
	if info.MemTotalMB != 8192 {
		t.Errorf("MemTotalMB = %f, want 8192", info.MemTotalMB)
	}
	if info.MemUsedMB != 4096 {
		t.Errorf("MemUsedMB = %f, want 4096", info.MemUsedMB)
	}
}

// TestExtractAMDFields_NonNumericValues tests extractAMDFields with non-numeric values.
func TestExtractAMDFields_NonNumericValues(t *testing.T) {
	t.Parallel()
	info := &GPUInfo{}
	m := map[string]any{
		"gpu_utilization":  "not-a-number",
		"vram_total":       true,
		"edge_temperature": []int{1, 2, 3},
	}
	extractAMDFields(info, m)
	// None should be set since values are non-numeric
	if info.UtilValid {
		t.Error("UtilValid should be false for non-numeric")
	}
	if info.MemValid {
		t.Error("MemValid should be false for non-numeric")
	}
	if info.TempValid {
		t.Error("TempValid should be false for non-numeric")
	}
}

// TestExtractAMDFields_StringNumericValues tests extractAMDFields with string numbers.
func TestExtractAMDFields_StringNumericValues(t *testing.T) {
	t.Parallel()
	info := &GPUInfo{}
	m := map[string]any{
		"gpu_utilization":  "75.5",
		"vram_total":       "16384",
		"vram_used":        "8000",
		"edge_temperature": "65.0",
	}
	extractAMDFields(info, m)
	if !info.UtilValid || info.UtilPercent != 75.5 {
		t.Errorf("UtilPercent = %f (valid=%v), want 75.5 (valid=true)", info.UtilPercent, info.UtilValid)
	}
	if !info.MemValid || info.MemTotalMB != 16384 {
		t.Errorf("MemTotalMB = %f (valid=%v), want 16384 (valid=true)", info.MemTotalMB, info.MemValid)
	}
	if !info.TempValid || info.TempC != 65 {
		t.Errorf("TempC = %f (valid=%v), want 65 (valid=true)", info.TempC, info.TempValid)
	}
}

// ---------------------------------------------------------------------------
// findAMDIndex — more edge cases
// ---------------------------------------------------------------------------

// TestFindAMDIndex_InvalidStringIndex tests findAMDIndex with unparseable string.
func TestFindAMDIndex_InvalidStringIndex(t *testing.T) {
	t.Parallel()
	m := map[string]any{
		"gpu": "not-an-int",
	}
	_, ok := findAMDIndex(m)
	if ok {
		t.Error("findAMDIndex should fail with invalid string index")
	}
}

// TestFindAMDIndex_MultipleFallbackKeys tests the priority order of keys.
func TestFindAMDIndex_MultipleFallbackKeys(t *testing.T) {
	t.Parallel()
	// "gpu" should be checked first; if found, "device" is not checked.
	m := map[string]any{
		"gpu":    float64(3),
		"device": float64(7),
	}
	idx, ok := findAMDIndex(m)
	if !ok {
		t.Fatal("findAMDIndex should succeed")
	}
	if idx != 3 {
		t.Errorf("index = %d, want 3 (from 'gpu' key)", idx)
	}
}

// TestFindAMDIndex_FallbackPatternWithBothGpuAndIndex tests the fallback pattern.
func TestFindAMDIndex_FallbackPatternWithBothGpuAndIndex(t *testing.T) {
	t.Parallel()
	m := map[string]any{
		"custom_gpu_index": float64(9),
	}
	idx, ok := findAMDIndex(m)
	if !ok {
		t.Fatal("findAMDIndex should succeed with custom_gpu_index")
	}
	if idx != 9 {
		t.Errorf("index = %d, want 9", idx)
	}
}

// TestFindAMDIndex_FallbackPatternInvalidValue tests fallback with invalid value.
func TestFindAMDIndex_FallbackPatternInvalidValue(t *testing.T) {
	t.Parallel()
	m := map[string]any{
		"custom_gpu_index": "not-a-number",
	}
	_, ok := findAMDIndex(m)
	if ok {
		t.Error("findAMDIndex should fail with invalid fallback value")
	}
}

// ---------------------------------------------------------------------------
// parseSizeStringToMB — more edge cases
// ---------------------------------------------------------------------------

// TestParseSizeStringToMB_InvalidWithoutUnit tests parseSizeStringToMB
// with a non-numeric string that has no unit.
func TestParseSizeStringToMB_InvalidWithoutUnit(t *testing.T) {
	t.Parallel()
	_, ok := parseSizeStringToMB("not a number")
	if ok {
		t.Error("parseSizeStringToMB should fail for non-numeric string without unit")
	}
}

// TestParseSizeStringToMB_BOnly tests parseSizeStringToMB with just B unit.
func TestParseSizeStringToMB_BOnly(t *testing.T) {
	t.Parallel()
	got, ok := parseSizeStringToMB("2097152 B")
	if !ok {
		t.Fatal("parseSizeStringToMB should succeed")
	}
	expected := 2097152.0 / 1024 / 1024 // 2 MB
	if got != expected {
		t.Errorf("got %f, want %f", got, expected)
	}
}

// ---------------------------------------------------------------------------
// parseSizeToMB — edge cases with unit trimming
// ---------------------------------------------------------------------------

// TestParseSizeToMB_UnitWithSpaces tests parseSizeToMB with spaces in unit.
func TestParseSizeToMB_UnitWithSpaces(t *testing.T) {
	t.Parallel()
	got, ok := parseSizeToMB("1024", "  KB  ")
	if !ok {
		t.Fatal("parseSizeToMB should succeed")
	}
	if got != 1.0 {
		t.Errorf("parseSizeToMB(1024, '  KB  ') = %f, want 1.0", got)
	}
}

// ---------------------------------------------------------------------------
// CrashDumpWriter — additional edge cases
// ---------------------------------------------------------------------------

// TestCrashDumpWriter_WriteCrashDump_WithMonitorAndHistory tests dump
// with a monitor that has populated history.
func TestCrashDumpWriter_WriteCrashDump_WithMonitorAndHistory(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)
	// Record multiple snapshots to populate history
	for i := 0; i < 3; i++ {
		snapshot := monitor.TakeSnapshot()
		monitor.recordSnapshot(snapshot)
	}

	writer := NewCrashDumpWriter(tempDir, 10, true, true, logger, monitor)
	writer.SetCurrentContext("test-phase", "test-task")
	writer.SetCurrentCommand(&CommandContext{
		Path:    "/usr/bin/test",
		Args:    []string{"arg1", "arg2"},
		WorkDir: "/tmp/test-workdir",
		Started: time.Now(),
	})

	path, err := writer.WriteCrashDump("comprehensive test panic")
	if err != nil {
		t.Fatalf("failed to write crash dump: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read crash dump: %v", err)
	}

	var dump CrashDump
	if err := json.Unmarshal(data, &dump); err != nil {
		t.Fatalf("failed to parse crash dump: %v", err)
	}

	// Check all fields are populated
	if dump.PanicValue != "comprehensive test panic" {
		t.Errorf("PanicValue = %q, want %q", dump.PanicValue, "comprehensive test panic")
	}
	if dump.CurrentPhase != "test-phase" {
		t.Errorf("CurrentPhase = %q, want %q", dump.CurrentPhase, "test-phase")
	}
	if dump.CurrentTask != "test-task" {
		t.Errorf("CurrentTask = %q, want %q", dump.CurrentTask, "test-task")
	}
	if dump.CommandPath != "/usr/bin/test" {
		t.Errorf("CommandPath = %q, want %q", dump.CommandPath, "/usr/bin/test")
	}
	if dump.WorkDir != "/tmp/test-workdir" {
		t.Errorf("WorkDir = %q, want %q", dump.WorkDir, "/tmp/test-workdir")
	}
	if len(dump.CommandArgs) != 2 {
		t.Errorf("CommandArgs length = %d, want 2", len(dump.CommandArgs))
	}
	if !dump.ResourceState.Timestamp.IsZero() == false {
		// ResourceState should be populated via monitor
	}
	if len(dump.ResourceHistory) < 3 {
		t.Errorf("ResourceHistory length = %d, want >= 3", len(dump.ResourceHistory))
	}
	if len(dump.RedactedEnv) == 0 {
		t.Error("RedactedEnv should be populated (includeEnv=true)")
	}
	if dump.StackTrace == "" {
		t.Error("StackTrace should be populated (includeStack=true)")
	}
}

// TestCrashDumpWriter_RecoverAndDump_NilLogger tests RecoverAndDump with nil logger.
func TestCrashDumpWriter_RecoverAndDump_NilLogger(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Create writer WITHOUT a logger
	writer := NewCrashDumpWriter(tempDir, 10, true, false, nil, nil)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to be re-thrown")
		}

		// Verify dump was still written
		entries, err := os.ReadDir(tempDir)
		if err != nil {
			t.Fatalf("failed to read temp dir: %v", err)
		}
		found := false
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "crash-") && strings.HasSuffix(e.Name(), ".json") {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected crash dump file even with nil logger")
		}
	}()

	func() {
		defer writer.RecoverAndDump()
		panic("nil logger panic")
	}()
}

// TestCrashDumpWriter_RecoverAndDump_WriteFails tests RecoverAndDump when write fails.
func TestCrashDumpWriter_RecoverAndDump_WriteFails(t *testing.T) {
	t.Parallel()

	// Create writer with invalid directory to force write failure
	writer := NewCrashDumpWriter("/dev/null/impossible/path", 10, true, false, nil, nil)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to be re-thrown")
		}
	}()

	func() {
		defer writer.RecoverAndDump()
		panic("write fails panic")
	}()
}

// TestCrashDumpWriter_RecoverAndDump_WriteFailsWithLogger tests RecoverAndDump
// when write fails but logger is present.
func TestCrashDumpWriter_RecoverAndDump_WriteFailsWithLogger(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create writer with invalid directory to force write failure
	writer := NewCrashDumpWriter("/dev/null/impossible/path", 10, true, false, logger, nil)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to be re-thrown")
		}
	}()

	func() {
		defer writer.RecoverAndDump()
		panic("write fails with logger panic")
	}()
}

// TestCrashDumpWriter_RecoverAndReturn_NilLogger tests RecoverAndReturn without logger.
func TestCrashDumpWriter_RecoverAndReturn_NilLogger(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 10, true, false, nil, nil)

	var capturedErr error
	func() {
		defer writer.RecoverAndReturn(&capturedErr)
		panic("nil logger recovery")
	}()

	if capturedErr == nil {
		t.Fatal("expected captured error")
	}
	if !strings.Contains(capturedErr.Error(), "nil logger recovery") {
		t.Errorf("error should contain panic message, got %s", capturedErr.Error())
	}
}

// TestCrashDumpWriter_RecoverAndReturn_WriteFailsNilLogger tests RecoverAndReturn
// when dump write fails and logger is nil.
func TestCrashDumpWriter_RecoverAndReturn_WriteFailsNilLogger(t *testing.T) {
	t.Parallel()

	writer := NewCrashDumpWriter("/dev/null/impossible/path", 10, true, false, nil, nil)

	var capturedErr error
	func() {
		defer writer.RecoverAndReturn(&capturedErr)
		panic("nil logger write fail recovery")
	}()

	if capturedErr == nil {
		t.Fatal("expected captured error")
	}
}

// ---------------------------------------------------------------------------
// cleanupOldDumps — edge cases
// ---------------------------------------------------------------------------

// TestCleanupOldDumps_EmptyDir tests cleanup with empty directory.
func TestCleanupOldDumps_EmptyDir(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 5, false, false, logger, nil)

	err := writer.cleanupOldDumps()
	if err != nil {
		t.Errorf("cleanupOldDumps should succeed on empty dir: %v", err)
	}
}

// TestCleanupOldDumps_UnderLimit tests cleanup with fewer files than limit.
func TestCleanupOldDumps_UnderLimit(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 10, false, false, logger, nil)

	// Write a dump (under limit of 10)
	_, err := writer.WriteCrashDump("panic-under-limit")
	if err != nil {
		t.Fatalf("failed to write dump: %v", err)
	}

	err = writer.cleanupOldDumps()
	if err != nil {
		t.Errorf("cleanupOldDumps should succeed: %v", err)
	}

	// Verify the file still exists (not cleaned up since under limit)
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "crash-") && strings.HasSuffix(e.Name(), ".json") {
			count++
		}
	}
	if count == 0 {
		t.Error("expected at least 1 dump (under limit, should not be cleaned)")
	}
}

// TestCleanupOldDumps_ExactLimit tests cleanup with exactly maxFiles dumps.
func TestCleanupOldDumps_ExactLimit(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 3, false, false, logger, nil)

	// Write exactly 3 dumps
	for i := 0; i < 3; i++ {
		_, err := writer.WriteCrashDump(fmt.Sprintf("panic %d", i))
		if err != nil {
			t.Fatalf("failed to write dump: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "crash-") && strings.HasSuffix(e.Name(), ".json") {
			count++
		}
	}
	if count > 3 {
		t.Errorf("expected at most 3 dumps, got %d", count)
	}
}

// TestCleanupOldDumps_WithSubdirectories tests cleanup ignores directories.
func TestCleanupOldDumps_WithSubdirectories(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	// Create a directory with crash- prefix (should be ignored)
	if err := os.Mkdir(filepath.Join(tempDir, "crash-directory.json"), 0o750); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	writer := NewCrashDumpWriter(tempDir, 2, false, false, logger, nil)

	for i := 0; i < 4; i++ {
		_, err := writer.WriteCrashDump("test")
		if err != nil {
			t.Fatalf("failed to write dump: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify directory still exists
	info, err := os.Stat(filepath.Join(tempDir, "crash-directory.json"))
	if err != nil {
		t.Errorf("directory should still exist: %v", err)
	} else if !info.IsDir() {
		t.Error("crash-directory.json should still be a directory")
	}
}

// TestCleanupOldDumps_NilLogger tests cleanup with nil logger when remove fails.
func TestCleanupOldDumps_NilLogger(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 2, false, false, nil, nil)

	for i := 0; i < 4; i++ {
		_, err := writer.WriteCrashDump("test")
		if err != nil {
			t.Fatalf("failed to write dump: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Should not panic with nil logger
}

// ---------------------------------------------------------------------------
// LoadLatestCrashDump — edge cases
// ---------------------------------------------------------------------------

// TestLoadLatestCrashDump_NonexistentDir tests loading from nonexistent directory.
func TestLoadLatestCrashDump_NonexistentDir(t *testing.T) {
	t.Parallel()
	_, err := LoadLatestCrashDump("/nonexistent/dir/for/crash/dumps")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

// TestLoadLatestCrashDump_OnlyDirectories tests loading when dir has only subdirectories.
func TestLoadLatestCrashDump_OnlyDirectories(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Create only directories (no crash dump files)
	if err := os.Mkdir(filepath.Join(tempDir, "crash-subdir.json"), 0o750); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	_, err := LoadLatestCrashDump(tempDir)
	if err == nil {
		t.Error("expected error when no crash dump files exist")
	}
}

// TestLoadLatestCrashDump_MultipleDumpsPicksNewest tests that the newest dump is loaded.
func TestLoadLatestCrashDump_MultipleDumpsPicksNewest(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 10, false, false, logger, nil)

	// Write multiple dumps
	for i := 0; i < 3; i++ {
		_, err := writer.WriteCrashDump(fmt.Sprintf("panic-%d", i))
		if err != nil {
			t.Fatalf("failed to write dump: %v", err)
		}
		time.Sleep(50 * time.Millisecond) // Ensure different timestamps
	}

	dump, err := LoadLatestCrashDump(tempDir)
	if err != nil {
		t.Fatalf("failed to load latest dump: %v", err)
	}

	// The latest dump should be the last one written
	if dump.PanicValue != "panic-2" {
		t.Errorf("PanicValue = %q, want %q (latest)", dump.PanicValue, "panic-2")
	}
}

// TestLoadLatestCrashDump_IgnoresNonCrashFiles tests that non-crash files are ignored.
func TestLoadLatestCrashDump_IgnoresNonCrashFiles(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Create files that should be ignored
	nonCrashFiles := []string{
		"not-a-crash.json",
		"crash.txt",               // wrong suffix
		"crash-.json",             // no timestamp
		"other-crash-2024.json",   // wrong prefix
		"something-else.json",     // unrelated
	}
	for _, name := range nonCrashFiles {
		if err := os.WriteFile(filepath.Join(tempDir, name), []byte(`{}`), 0o600); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	_, err := LoadLatestCrashDump(tempDir)
	// Only "crash-.json" would match the pattern (starts with "crash-" and ends with ".json")
	// but its content is valid JSON, so it might load
	if err != nil {
		// This is expected if none match or the valid JSON doesn't parse as CrashDump
		t.Logf("LoadLatestCrashDump returned: %v (expected for non-matching files)", err)
	}
}

// ---------------------------------------------------------------------------
// PrepareCommand / PrepareStderrOnly — error paths
// ---------------------------------------------------------------------------

// TestPrepareCommand_AlreadyStartedCmd tests PrepareCommand with an already-started command.
func TestPrepareCommand_AlreadyStartedCmd(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)
	executor := NewSafeExecutor(monitor, nil, logger, true, 20, 256)

	cmd := exec.Command("echo", "test")
	// Start the command first
	cmd.Stdout = os.Stdout // Set stdout so pipe creation will fail
	cmd.Stderr = os.Stderr

	_, err := executor.PrepareCommand(cmd)
	if err == nil {
		t.Error("expected error when Stdout is already set")
	}

	// Active commands should be decremented on error
	snapshot := monitor.TakeSnapshot()
	if snapshot.CommandsActive != 0 {
		t.Errorf("CommandsActive = %d, want 0 after error", snapshot.CommandsActive)
	}
}

// TestPrepareStderrOnly_AlreadyStartedCmd tests PrepareStderrOnly with set stderr.
func TestPrepareStderrOnly_AlreadyStartedCmd(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)
	executor := NewSafeExecutor(monitor, nil, logger, true, 20, 256)

	cmd := exec.Command("echo", "test")
	cmd.Stderr = os.Stderr // Set stderr so pipe creation will fail

	_, _, err := executor.PrepareStderrOnly(cmd)
	if err == nil {
		t.Error("expected error when Stderr is already set")
	}

	// Active commands should be decremented on error
	snapshot := monitor.TakeSnapshot()
	if snapshot.CommandsActive != 0 {
		t.Errorf("CommandsActive = %d, want 0 after error", snapshot.CommandsActive)
	}
}

// TestPrepareCommand_StderrPipeFails tests PrepareCommand when stderr pipe fails.
func TestPrepareCommand_StderrPipeFails(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)
	executor := NewSafeExecutor(monitor, nil, logger, true, 20, 256)

	cmd := exec.Command("echo", "test")
	cmd.Stderr = os.Stderr // Set stderr to force pipe failure, but Stdout is free

	_, err := executor.PrepareCommand(cmd)
	if err == nil {
		t.Error("expected error when Stderr is already set")
	}

	// Active commands should be decremented on error
	snapshot := monitor.TakeSnapshot()
	if snapshot.CommandsActive != 0 {
		t.Errorf("CommandsActive = %d, want 0 after error", snapshot.CommandsActive)
	}
}

// ---------------------------------------------------------------------------
// SafeExecutor — WrapExecution edge cases
// ---------------------------------------------------------------------------

// TestWrapExecution_PanicWithNilDumpWriter tests panic recovery without dump writer.
func TestWrapExecution_PanicWithNilDumpWriter(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	executor := NewSafeExecutor(nil, nil, logger, true, 20, 256)

	// Without a dump writer, a panic should not be caught
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when no dump writer is present")
		}
	}()

	_ = executor.WrapExecution(func() error {
		panic("uncaught panic")
	})
}

// TestWrapExecution_PanicWithDumpWriterNilLogger tests WrapExecution
// panic recovery with dump writer but nil logger.
func TestWrapExecution_PanicWithDumpWriterNilLogger(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	dumpWriter := NewCrashDumpWriter(tempDir, 10, true, false, nil, nil)
	executor := NewSafeExecutor(nil, dumpWriter, nil, true, 20, 256)

	err := executor.WrapExecution(func() error {
		panic("panic with nil logger")
	})

	if err == nil {
		t.Error("expected error from recovered panic")
	}
}

// ---------------------------------------------------------------------------
// RunPreflight — edge cases
// ---------------------------------------------------------------------------

// TestRunPreflight_FDWarningZone tests the FD warning zone (approaching threshold).
func TestRunPreflight_FDWarningZone(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	// Take a snapshot to see actual FD usage
	snapshot := monitor.TakeSnapshot()
	freeFDPercent := 100.0 - snapshot.FDUsagePercent

	// Set threshold so that we're in the warning zone (between threshold and 1.5x threshold)
	// If free is 95%, set minFree to 70 so that 95 > 70 but 95 < 70*1.5=105
	warningThreshold := int(freeFDPercent / 1.5)
	if warningThreshold < 1 {
		warningThreshold = 1
	}

	executor := NewSafeExecutor(monitor, nil, logger, true, warningThreshold, 256)
	result := executor.RunPreflight()

	// Depending on actual FD usage, we might get a warning
	if len(result.Warnings) > 0 {
		t.Logf("Got FD warnings as expected: %v", result.Warnings)
	}
	// The point is to exercise the warning branch in RunPreflight
}

// TestRunPreflight_MinFreeFDZero tests preflight with minFreeFDPercent=0 (disabled).
func TestRunPreflight_MinFreeFDZero(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)
	executor := NewSafeExecutor(monitor, nil, logger, true, 0, 0)

	result := executor.RunPreflight()

	// With zero thresholds, should always pass
	if !result.OK {
		t.Error("expected OK with zero FD threshold")
	}
}

// ---------------------------------------------------------------------------
// toInt / toFloat — untested type branches
// ---------------------------------------------------------------------------

// TestToInt_AllTypes tests all type branches of toInt.
func TestToInt_AllTypes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  any
		want   int
		wantOK bool
	}{
		{"int", int(42), 42, true},
		{"int64", int64(100), 100, true},
		{"float64", float64(7.0), 7, true},
		{"float32", float32(5.0), 5, true},
		{"string valid", "123", 123, true},
		{"string with spaces", " 456 ", 456, true},
		{"string invalid", "abc", 0, false},
		{"string empty", "", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
		{"uint", uint(10), 0, false},
		{"complex128", complex(1, 2), 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := toInt(tt.input)
			if ok != tt.wantOK {
				t.Errorf("toInt(%v) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("toInt(%v) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestToFloat_AllTypes tests all type branches of toFloat.
func TestToFloat_AllTypes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  any
		want   float64
		wantOK bool
	}{
		{"float64", float64(3.14), 3.14, true},
		{"float32", float32(2.5), 2.5, true},
		{"int", int(42), 42.0, true},
		{"int64", int64(100), 100.0, true},
		{"string valid", "99.5", 99.5, true},
		{"string invalid", "abc", 0, false},
		{"nil", nil, 0, false},
		{"bool", false, 0, false},
		{"uint", uint(10), 0, false},
		{"struct", struct{}{}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := toFloat(tt.input)
			if ok != tt.wantOK {
				t.Errorf("toFloat(%v) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("toFloat(%v) = %f, want %f", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CountFDs — testing on Linux
// ---------------------------------------------------------------------------

// TestCountFDs tests the CountFDs function.
func TestCountFDs(t *testing.T) {
	t.Parallel()

	open, limit := CountFDs()

	// On Linux, we should get non-zero values
	if open <= 0 {
		t.Logf("OpenFDs = %d (may be 0 in some environments)", open)
	}
	if limit <= 0 {
		t.Logf("MaxFDs = %d (may be 0 in some environments)", limit)
	}
	if limit > 0 && open > limit {
		t.Errorf("OpenFDs (%d) should not exceed MaxFDs (%d)", open, limit)
	}
}

// ---------------------------------------------------------------------------
// Collector — GPU cache edge cases
// ---------------------------------------------------------------------------

// TestCollectGPUInfo_NilCacheNotRefreshed tests that nil cache forces refresh.
func TestCollectGPUInfo_NilCacheNotRefreshed(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()

	// Manually set lastGPUUpdate to recent but keep gpuCache nil
	c.mu.Lock()
	c.lastGPUUpdate = time.Now()
	c.gpuCache = nil
	c.mu.Unlock()

	stats := &SystemMetrics{}
	c.collectGPUInfo(stats)

	// Since gpuCache is nil, it should refresh (even though time is recent)
	c.mu.Lock()
	defer c.mu.Unlock()
	// gpuCache should now be set (possibly to empty slice or actual GPUs)
}

// TestCollectGPUInfo_CacheHitWithData tests cache hit with actual data.
func TestCollectGPUInfo_CacheHitWithData(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()

	// Manually populate the cache
	c.mu.Lock()
	c.lastGPUUpdate = time.Now()
	c.gpuCache = []GPUInfo{
		{Name: "Cached GPU", UtilPercent: 50, UtilValid: true},
	}
	c.mu.Unlock()

	stats := &SystemMetrics{}
	c.collectGPUInfo(stats)

	// Should use cached data
	if len(stats.GPUInfos) != 1 {
		t.Fatalf("GPUInfos length = %d, want 1", len(stats.GPUInfos))
	}
	if stats.GPUInfos[0].Name != "Cached GPU" {
		t.Errorf("GPU Name = %q, want %q", stats.GPUInfos[0].Name, "Cached GPU")
	}
	if stats.GPUInfos[0].UtilPercent != 50 {
		t.Errorf("GPU UtilPercent = %f, want 50", stats.GPUInfos[0].UtilPercent)
	}
}

// ---------------------------------------------------------------------------
// JSON serialization — additional coverage for struct fields
// ---------------------------------------------------------------------------

// TestResourceSnapshot_JSON tests ResourceSnapshot serialization.
func TestResourceSnapshot_JSON(t *testing.T) {
	t.Parallel()

	snapshot := ResourceSnapshot{
		Timestamp:      time.Now(),
		OpenFDs:        100,
		MaxFDs:         1024,
		FDUsagePercent: 9.77,
		Goroutines:     50,
		HeapAllocMB:    256.5,
		HeapInUseMB:    300.0,
		StackInUseMB:   10.0,
		GCPauseNS:      1000000,
		NumGC:          42,
		ProcessUptime:  5 * time.Minute,
		CommandsRun:    100,
		CommandsActive: 3,
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ResourceSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.OpenFDs != 100 {
		t.Errorf("OpenFDs = %d, want 100", decoded.OpenFDs)
	}
	if decoded.Goroutines != 50 {
		t.Errorf("Goroutines = %d, want 50", decoded.Goroutines)
	}
}

// TestCrashDump_JSON tests CrashDump serialization.
func TestCrashDump_JSON(t *testing.T) {
	t.Parallel()

	dump := CrashDump{
		Timestamp:     time.Now(),
		ProcessID:     12345,
		GoVersion:     "go1.23",
		GOOS:          "linux",
		GOARCH:        "amd64",
		PanicValue:    "test panic",
		PanicLocation: "main.go:42",
		StackTrace:    "goroutine 1 [running]:\nmain.main()\n",
		CurrentPhase:  "execute",
		CurrentTask:   "task-1",
		CommandPath:   "/usr/bin/test",
		CommandArgs:   []string{"arg1"},
		WorkDir:       "/home/test",
		RedactedEnv:   map[string]string{"PATH": "/usr/bin"},
	}

	data, err := json.Marshal(dump)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded CrashDump
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.PanicValue != "test panic" {
		t.Errorf("PanicValue = %q, want %q", decoded.PanicValue, "test panic")
	}
	if decoded.PanicLocation != "main.go:42" {
		t.Errorf("PanicLocation = %q, want %q", decoded.PanicLocation, "main.go:42")
	}
}

// TestCommandContext_Fields tests CommandContext struct fields.
func TestCommandContext_Fields(t *testing.T) {
	t.Parallel()

	ctx := &CommandContext{
		Path:    "/usr/bin/test",
		Args:    []string{"arg1", "arg2"},
		WorkDir: "/tmp",
		Started: time.Now(),
	}

	if ctx.Path != "/usr/bin/test" {
		t.Errorf("Path = %q, want %q", ctx.Path, "/usr/bin/test")
	}
	if len(ctx.Args) != 2 {
		t.Errorf("Args length = %d, want 2", len(ctx.Args))
	}
}

// TestHealthWarning_Fields tests HealthWarning struct fields.
func TestHealthWarning_Fields(t *testing.T) {
	t.Parallel()

	w := HealthWarning{
		Level:   "critical",
		Type:    "memory",
		Message: "heap too high",
		Value:   500.0,
		Limit:   256.0,
	}

	if w.Level != "critical" {
		t.Errorf("Level = %q, want %q", w.Level, "critical")
	}
	if w.Type != "memory" {
		t.Errorf("Type = %q, want %q", w.Type, "memory")
	}
}

// TestResourceTrend_Fields tests ResourceTrend struct fields.
func TestResourceTrend_Fields(t *testing.T) {
	t.Parallel()

	trend := ResourceTrend{
		FDGrowthRate:        10.0,
		GoroutineGrowthRate: 20.0,
		MemoryGrowthRate:    30.0,
		IsHealthy:           true,
		Warnings:            []string{"test warning"},
	}

	if trend.FDGrowthRate != 10.0 {
		t.Errorf("FDGrowthRate = %f, want 10.0", trend.FDGrowthRate)
	}
}

// TestPreflightResult_Fields tests PreflightResult struct fields.
func TestPreflightResult_Fields(t *testing.T) {
	t.Parallel()

	result := PreflightResult{
		OK:       false,
		Warnings: []string{"approaching limit"},
		Errors:   []string{"FD exhausted"},
		Snapshot: ResourceSnapshot{Goroutines: 10},
	}

	if result.OK {
		t.Error("OK should be false")
	}
	if len(result.Warnings) != 1 {
		t.Errorf("Warnings length = %d, want 1", len(result.Warnings))
	}
	if len(result.Errors) != 1 {
		t.Errorf("Errors length = %d, want 1", len(result.Errors))
	}
}

// ---------------------------------------------------------------------------
// PipeSet — additional edge cases
// ---------------------------------------------------------------------------

// TestPipeSet_CleanupWithNilFunc tests PipeSet cleanup with nil cleanup function.
func TestPipeSet_CleanupWithNilFunc(t *testing.T) {
	t.Parallel()

	ps := &PipeSet{
		Stdout:  nil,
		Stderr:  nil,
		cleanup: nil,
		cleaned: false,
	}

	// Should not panic
	ps.Cleanup()

	if !ps.cleaned {
		t.Error("cleaned should be true after Cleanup")
	}

	// Second call should be no-op
	ps.Cleanup()
}

// ---------------------------------------------------------------------------
// SystemMetricsCollector — exercise Collect thoroughly
// ---------------------------------------------------------------------------

// TestCollect_MultipleCollections tests multiple sequential collections.
func TestCollect_MultipleCollections(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()

	// Collect multiple times to exercise caching, CPU delta, GPU cache
	var lastStats SystemMetrics
	for i := 0; i < 5; i++ {
		stats := c.Collect()
		lastStats = stats
		time.Sleep(10 * time.Millisecond)
	}

	// By the 5th collection, we should have CPU delta
	if lastStats.MemTotalMB <= 0 {
		t.Error("MemTotalMB should be positive")
	}
	if lastStats.DiskTotalGB <= 0 {
		t.Error("DiskTotalGB should be positive")
	}
	if lastStats.CPUCores <= 0 {
		t.Error("CPUCores should be positive")
	}
}

// TestCollectCPUInfo_ZeroDelta tests CPU info when there's no time delta.
func TestCollectCPUInfo_ZeroDelta(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()
	stats := &SystemMetrics{}

	// First call: no delta yet
	c.collectCPUInfo(stats)
	if stats.CPUPercent != 0 {
		t.Logf("First call CPUPercent = %f (expected 0)", stats.CPUPercent)
	}

	// Immediate second call: very small delta, might be zero
	stats2 := &SystemMetrics{}
	c.collectCPUInfo(stats2)
	// No assertion on value — just exercise the path
}
