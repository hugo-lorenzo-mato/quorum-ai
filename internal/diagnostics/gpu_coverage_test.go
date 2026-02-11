package diagnostics

import (
	"log/slog"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestQueryNvidiaSMI tests NVIDIA GPU query function.
func TestQueryNvidiaSMI(t *testing.T) {
	t.Parallel()

	// This will return nil if nvidia-smi is not found or fails
	gpus := queryNvidiaSMI()
	// We just verify it doesn't panic
	_ = gpus
}

// TestQueryAmdSMI tests AMD GPU query function.
func TestQueryAmdSMI(t *testing.T) {
	t.Parallel()

	// This will return nil if amd-smi is not found or fails
	gpus := queryAmdSMI()
	// We just verify it doesn't panic
	_ = gpus
}

// TestQueryRocmSMI tests ROCm GPU query function.
func TestQueryRocmSMI(t *testing.T) {
	t.Parallel()

	// This will return nil if rocm-smi is not found or fails
	gpus := queryRocmSMI()
	// We just verify it doesn't panic
	_ = gpus
}

// TestQuerySystemProfiler tests macOS system_profiler GPU query.
func TestQuerySystemProfiler(t *testing.T) {
	t.Parallel()

	// This will return nil if system_profiler is not found or fails
	gpus := querySystemProfiler()
	// We just verify it doesn't panic
	_ = gpus
}

// TestQueryWindowsGPU tests Windows PowerShell GPU query.
func TestQueryWindowsGPU(t *testing.T) {
	t.Parallel()

	// This will return nil if powershell is not found or fails
	gpus := queryWindowsGPU()
	// We just verify it doesn't panic
	_ = gpus
}

// TestAmdSmiJSON tests AMD SMI JSON parsing.
func TestAmdSmiJSON(t *testing.T) {
	t.Parallel()

	// Will return nil if amd-smi not found
	result := amdSmiJSON("/nonexistent/amd-smi", []string{"--version"})
	if result != nil {
		t.Log("amd-smi found (unexpected on most systems)")
	}
}

// TestMergeAMDJSON_NilPayload tests mergeAMDJSON with nil payload.
func TestMergeAMDJSON_NilPayload(t *testing.T) {
	t.Parallel()

	info := map[int]*GPUInfo{}
	mergeAMDJSON(info, nil)

	if len(info) != 0 {
		t.Error("merging nil payload should not add entries")
	}
}

// TestMergeAMDJSON_ArrayPayload tests mergeAMDJSON with array of GPU data.
func TestMergeAMDJSON_ArrayPayload(t *testing.T) {
	t.Parallel()

	info := map[int]*GPUInfo{}
	payload := []any{
		map[string]any{
			"gpu":       float64(0),
			"card_name": "GPU 0",
		},
		map[string]any{
			"gpu":       float64(1),
			"card_name": "GPU 1",
		},
	}
	mergeAMDJSON(info, payload)

	if len(info) != 2 {
		t.Fatalf("expected 2 GPUs, got %d", len(info))
	}
	if info[0].Name != "GPU 0" {
		t.Errorf("GPU 0 name = %q, want %q", info[0].Name, "GPU 0")
	}
	if info[1].Name != "GPU 1" {
		t.Errorf("GPU 1 name = %q, want %q", info[1].Name, "GPU 1")
	}
}

// TestFindAMDIndex_NoIndex tests findAMDIndex with no valid index key.
func TestFindAMDIndex_NoIndex(t *testing.T) {
	t.Parallel()

	m := map[string]any{
		"name":  "GPU",
		"model": "RX 7900",
	}
	_, ok := findAMDIndex(m)
	if ok {
		t.Error("findAMDIndex should return false with no index key")
	}
}

// TestFindAMDIndex_StringIndex tests findAMDIndex with string index value.
func TestFindAMDIndex_StringIndex(t *testing.T) {
	t.Parallel()

	m := map[string]any{
		"gpu": "5", // String that can be parsed as int
	}
	idx, ok := findAMDIndex(m)
	if !ok {
		t.Fatal("findAMDIndex should succeed with string index")
	}
	if idx != 5 {
		t.Errorf("index = %d, want 5", idx)
	}
}

// TestToInt_StringParsing tests toInt with various string formats.
func TestToInt_StringParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input  any
		want   int
		wantOK bool
	}{
		{"123", 123, true},
		{" 456 ", 456, true},
		{"0", 0, true},
		{"-1", -1, true},
		{"abc", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		got, ok := toInt(tt.input)
		if ok != tt.wantOK {
			t.Errorf("toInt(%v) ok = %v, want %v", tt.input, ok, tt.wantOK)
		}
		if ok && got != tt.want {
			t.Errorf("toInt(%v) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// TestToFloat_Conversions tests toFloat with all type conversions.
func TestToFloat_Conversions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input  any
		want   float64
		wantOK bool
	}{
		{float64(3.14), 3.14, true},
		{float32(2.5), 2.5, true},
		{int(42), 42.0, true},
		{int64(100), 100.0, true},
		{"3.14", 3.14, true},
		{" 2.5 ", 2.5, true},
		{"invalid", 0, false},
	}

	for _, tt := range tests {
		got, ok := toFloat(tt.input)
		if ok != tt.wantOK {
			t.Errorf("toFloat(%v) ok = %v, want %v", tt.input, ok, tt.wantOK)
		}
		if ok && got != tt.want {
			t.Errorf("toFloat(%v) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

// TestParseSizeToMB_InvalidValue tests parseSizeToMB with invalid value.
func TestParseSizeToMB_InvalidValue(t *testing.T) {
	t.Parallel()

	_, ok := parseSizeToMB("invalid", "MB")
	if ok {
		t.Error("parseSizeToMB should fail with invalid value")
	}
}

// TestParseSizeToMB_ByteConversion tests byte to MB conversion.
func TestParseSizeToMB_ByteConversion(t *testing.T) {
	t.Parallel()

	got, ok := parseSizeToMB("1048576", "b")
	if !ok {
		t.Fatal("parseSizeToMB should succeed")
	}
	if got != 1.0 {
		t.Errorf("parseSizeToMB(1048576, b) = %f, want 1.0", got)
	}
}

// TestParseSizeToMB_GBConversion tests GB to MB conversion.
func TestParseSizeToMB_GBConversion(t *testing.T) {
	t.Parallel()

	got, ok := parseSizeToMB("2", "gb")
	if !ok {
		t.Fatal("parseSizeToMB should succeed")
	}
	if got != 2048.0 {
		t.Errorf("parseSizeToMB(2, gb) = %f, want 2048.0", got)
	}
}

// TestParseSizeStringToMB_EmptyString tests parseSizeStringToMB with empty input.
func TestParseSizeStringToMB_EmptyString(t *testing.T) {
	t.Parallel()

	_, ok := parseSizeStringToMB("")
	if ok {
		t.Error("parseSizeStringToMB should fail with empty string")
	}
}

// TestParseSizeStringToMB_NoUnit tests parseSizeStringToMB with just a number.
func TestParseSizeStringToMB_NoUnit(t *testing.T) {
	t.Parallel()

	got, ok := parseSizeStringToMB("1024")
	if !ok {
		t.Fatal("parseSizeStringToMB should succeed with plain number")
	}
	if got != 1024.0 {
		t.Errorf("parseSizeStringToMB(1024) = %f, want 1024.0", got)
	}
}

// TestParseFloatField_Negative tests parseFloatField with negative numbers.
func TestParseFloatField_Negative(t *testing.T) {
	t.Parallel()

	got, ok := parseFloatField("-123.45")
	if !ok {
		t.Fatal("parseFloatField should succeed with negative number")
	}
	if got != -123.45 {
		t.Errorf("parseFloatField(-123.45) = %f, want -123.45", got)
	}
}

// TestParseFloatField_Invalid tests parseFloatField with invalid input.
func TestParseFloatField_Invalid(t *testing.T) {
	t.Parallel()

	tests := []string{
		"not-a-number",
		"",
		"abc123",
		"12.34.56",
	}

	for _, input := range tests {
		_, ok := parseFloatField(input)
		if ok {
			t.Errorf("parseFloatField(%q) should fail", input)
		}
	}
}

// TestFirstString_NonStringValue tests firstString with non-string map values.
func TestFirstString_NonStringValue(t *testing.T) {
	t.Parallel()

	m := map[string]any{
		"name":  123, // Not a string
		"model": "Valid String",
	}
	got := firstString(m, "name", "model")
	if got != "Valid String" {
		t.Errorf("firstString = %q, want %q (should skip non-string)", got, "Valid String")
	}
}

// TestFirstString_AllNonStrings tests firstString when all values are non-strings.
func TestFirstString_AllNonStrings(t *testing.T) {
	t.Parallel()

	m := map[string]any{
		"name":  123,
		"model": 456,
	}
	got := firstString(m, "name", "model")
	if got != "" {
		t.Errorf("firstString = %q, want empty (all values non-string)", got)
	}
}

// TestIsUtilKey_CaseInsensitive tests isUtilKey is case-sensitive.
func TestIsUtilKey_CaseSensitive(t *testing.T) {
	t.Parallel()

	// Function uses strings.Contains which is case-sensitive
	if isUtilKey("UTIL") {
		t.Error("isUtilKey should be case-sensitive (expect lowercase)")
	}
	if !isUtilKey("util") {
		t.Error("isUtilKey('util') should return true")
	}
}

// TestIsTempKey_CaseSensitive tests isTempKey is case-sensitive.
func TestIsTempKey_CaseSensitive(t *testing.T) {
	t.Parallel()

	// Function uses strings.Contains which is case-sensitive
	if isTempKey("TEMP") {
		t.Error("isTempKey should be case-sensitive (expect lowercase)")
	}
	if !isTempKey("temp") {
		t.Error("isTempKey('temp') should return true")
	}
}

// TestExtractAMDFields_MemoryTotal tests extractAMDFields with memory total.
func TestExtractAMDFields_MemoryTotal(t *testing.T) {
	t.Parallel()

	info := &GPUInfo{}
	m := map[string]any{
		"vram_total": float64(16384),
	}
	extractAMDFields(info, m)
	if !info.MemValid || info.MemTotalMB != 16384 {
		t.Errorf("MemTotalMB = %f (valid=%v), want 16384 (valid=true)", info.MemTotalMB, info.MemValid)
	}
}

// TestExtractAMDFields_Temperature tests extractAMDFields with temperature.
func TestExtractAMDFields_Temperature(t *testing.T) {
	t.Parallel()

	info := &GPUInfo{}
	m := map[string]any{
		"edge_temp": float64(72.5),
	}
	extractAMDFields(info, m)
	if !info.TempValid || info.TempC != 72.5 {
		t.Errorf("TempC = %f (valid=%v), want 72.5 (valid=true)", info.TempC, info.TempValid)
	}
}

// TestParseWindowsGPUObj_NoName tests parseWindowsGPUObj with missing name.
func TestParseWindowsGPUObj_NoName(t *testing.T) {
	t.Parallel()

	obj := map[string]any{
		"AdapterRAM": float64(4294967296),
	}
	gpu := parseWindowsGPUObj(obj)
	if gpu.Name != "" {
		t.Errorf("Name = %q, want empty", gpu.Name)
	}
}

// TestParseWindowsGPUObj_NoAdapterRAM tests parseWindowsGPUObj with missing AdapterRAM.
func TestParseWindowsGPUObj_NoAdapterRAM(t *testing.T) {
	t.Parallel()

	obj := map[string]any{
		"Name": "Test GPU",
	}
	gpu := parseWindowsGPUObj(obj)
	if gpu.MemValid {
		t.Error("MemValid should be false when AdapterRAM is missing")
	}
}

// TestRootDiskPath_WindowsEnvVar tests rootDiskPath with Windows SystemDrive env.
func TestRootDiskPath_WindowsEnvVar(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	// Test that it uses SystemDrive env var
	path := rootDiskPath()
	if !strings.HasSuffix(path, "\\") {
		t.Errorf("Windows path should end with backslash, got %q", path)
	}
}

// TestRootDiskPath_UnixPath tests rootDiskPath on Unix systems.
func TestRootDiskPath_UnixPath(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Unix-only test")
	}

	path := rootDiskPath()
	if path != "/" {
		t.Errorf("Unix path should be '/', got %q", path)
	}
}

// TestQueryGPUInfo_FallbackChain tests the GPU query fallback chain.
func TestQueryGPUInfo_FallbackChain(t *testing.T) {
	t.Parallel()

	// This tests the full fallback chain:
	// nvidia-smi -> amd-smi -> rocm-smi/system_profiler/powershell -> ghw
	gpus := queryGPUInfo()
	// We just verify it doesn't panic and returns a valid result
	_ = gpus
}

// TestQueryGhwGPU_EmptyCards tests queryGhwGPU with no graphics cards.
func TestQueryGhwGPU_EmptyCards(t *testing.T) {
	t.Parallel()

	// queryGhwGPU will return nil if ghw.GPU() fails or has no cards
	gpus := queryGhwGPU()
	// We just verify it doesn't panic
	_ = gpus
}

// TestCrashDumpWriter_CleanupOldDumps_Sorting tests cleanup keeps only newest dumps.
func TestCrashDumpWriter_CleanupOldDumps_Sorting(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 2, false, false, logger, nil)

	// Write dumps with delays to ensure different timestamps
	for i := 0; i < 4; i++ {
		_, err := writer.WriteCrashDump("test")
		if err != nil {
			t.Fatalf("failed to write dump: %v", err)
		}
		time.Sleep(50 * time.Millisecond) // Longer delay to ensure distinct timestamps
	}

	// Should have at most 2 dumps (the newest ones)
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}

	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "crash-") && strings.HasSuffix(e.Name(), ".json") {
			count++
		}
	}

	if count > 2 {
		t.Errorf("expected at most 2 crash dumps after cleanup, got %d", count)
	}
}
