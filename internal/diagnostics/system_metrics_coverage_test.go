package diagnostics

import (
	"encoding/json"
	"testing"
	"time"
)

// TestSystemMetrics_JSONRoundTrip verifies that SystemMetrics serializes and deserializes correctly.
func TestSystemMetrics_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := SystemMetrics{
		CPUModel:    "Intel Core i9-13900K",
		CPUCores:    24,
		CPUThreads:  32,
		CPUPercent:  45.5,
		MemTotalMB:  65536.0,
		MemUsedMB:   32768.0,
		MemPercent:  50.0,
		DiskTotalGB: 1024.0,
		DiskUsedGB:  512.0,
		DiskPercent: 50.0,
		LoadAvg1:    1.5,
		LoadAvg5:    2.0,
		LoadAvg15:   1.8,
		GPUInfos: []GPUInfo{
			{
				Name:        "NVIDIA RTX 4090",
				UtilPercent: 80.0,
				UtilValid:   true,
				MemTotalMB:  24576.0,
				MemUsedMB:   12288.0,
				MemValid:    true,
				TempC:       72.0,
				TempValid:   true,
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded SystemMetrics
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.CPUModel != original.CPUModel {
		t.Errorf("CPUModel = %q, want %q", decoded.CPUModel, original.CPUModel)
	}
	if decoded.CPUCores != original.CPUCores {
		t.Errorf("CPUCores = %d, want %d", decoded.CPUCores, original.CPUCores)
	}
	if decoded.CPUThreads != original.CPUThreads {
		t.Errorf("CPUThreads = %d, want %d", decoded.CPUThreads, original.CPUThreads)
	}
	if decoded.CPUPercent != original.CPUPercent {
		t.Errorf("CPUPercent = %f, want %f", decoded.CPUPercent, original.CPUPercent)
	}
	if decoded.MemTotalMB != original.MemTotalMB {
		t.Errorf("MemTotalMB = %f, want %f", decoded.MemTotalMB, original.MemTotalMB)
	}
	if decoded.DiskTotalGB != original.DiskTotalGB {
		t.Errorf("DiskTotalGB = %f, want %f", decoded.DiskTotalGB, original.DiskTotalGB)
	}
	if decoded.LoadAvg1 != original.LoadAvg1 {
		t.Errorf("LoadAvg1 = %f, want %f", decoded.LoadAvg1, original.LoadAvg1)
	}
	if len(decoded.GPUInfos) != 1 {
		t.Fatalf("GPUInfos length = %d, want 1", len(decoded.GPUInfos))
	}
	if decoded.GPUInfos[0].Name != "NVIDIA RTX 4090" {
		t.Errorf("GPU Name = %q, want %q", decoded.GPUInfos[0].Name, "NVIDIA RTX 4090")
	}
}

// TestSystemMetrics_JSONOmitEmpty verifies omitempty tags work correctly.
func TestSystemMetrics_JSONOmitEmpty(t *testing.T) {
	t.Parallel()

	m := SystemMetrics{
		CPUModel: "Test CPU",
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// GPUInfos should be omitted when nil/empty
	if _, ok := raw["gpu_infos"]; ok {
		t.Error("gpu_infos should be omitted when empty")
	}
}

// TestGPUInfo_JSONRoundTrip verifies GPUInfo serialization.
func TestGPUInfo_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	gpu := GPUInfo{
		Name:        "AMD RX 7900 XTX",
		UtilPercent: 65.0,
		UtilValid:   true,
		MemTotalMB:  24576.0,
		MemUsedMB:   8000.0,
		MemValid:    true,
		TempC:       68.0,
		TempValid:   true,
	}

	data, err := json.Marshal(gpu)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded GPUInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Name != gpu.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, gpu.Name)
	}
	if decoded.UtilPercent != gpu.UtilPercent {
		t.Errorf("UtilPercent = %f, want %f", decoded.UtilPercent, gpu.UtilPercent)
	}
	if decoded.UtilValid != gpu.UtilValid {
		t.Errorf("UtilValid = %v, want %v", decoded.UtilValid, gpu.UtilValid)
	}
	if decoded.MemTotalMB != gpu.MemTotalMB {
		t.Errorf("MemTotalMB = %f, want %f", decoded.MemTotalMB, gpu.MemTotalMB)
	}
	if decoded.MemUsedMB != gpu.MemUsedMB {
		t.Errorf("MemUsedMB = %f, want %f", decoded.MemUsedMB, gpu.MemUsedMB)
	}
	if decoded.MemValid != gpu.MemValid {
		t.Errorf("MemValid = %v, want %v", decoded.MemValid, gpu.MemValid)
	}
	if decoded.TempC != gpu.TempC {
		t.Errorf("TempC = %f, want %f", decoded.TempC, gpu.TempC)
	}
	if decoded.TempValid != gpu.TempValid {
		t.Errorf("TempValid = %v, want %v", decoded.TempValid, gpu.TempValid)
	}
}

// TestCollect_CPUPercentSecondCall verifies that CPU percent is calculated on second call.
func TestCollect_CPUPercentSecondCall(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()

	// First call sets baseline (no delta yet)
	m1 := c.Collect()
	if m1.CPUPercent != 0 {
		t.Logf("First call CPUPercent = %f (expected 0 on first call)", m1.CPUPercent)
	}

	// Allow some time for CPU time to change
	time.Sleep(50 * time.Millisecond)

	// Second call should compute a delta
	m2 := c.Collect()
	// CPUPercent should be between 0 and 100 (inclusive)
	if m2.CPUPercent < 0 || m2.CPUPercent > 100 {
		t.Errorf("CPUPercent = %f, want between 0 and 100", m2.CPUPercent)
	}
}

// TestCollect_HardwareInfoCached verifies hardware info is only collected once.
func TestCollect_HardwareInfoCachedFlag(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()

	if c.infoCollected {
		t.Error("infoCollected should be false before first Collect")
	}

	c.Collect()
	if !c.infoCollected {
		t.Error("infoCollected should be true after first Collect")
	}

	// Save the collected values
	model := c.cpuModel
	cores := c.cpuCores
	threads := c.cpuThreads

	// Second Collect should not change them
	c.Collect()
	if c.cpuModel != model {
		t.Errorf("cpuModel changed: %q vs %q", c.cpuModel, model)
	}
	if c.cpuCores != cores {
		t.Errorf("cpuCores changed: %d vs %d", c.cpuCores, cores)
	}
	if c.cpuThreads != threads {
		t.Errorf("cpuThreads changed: %d vs %d", c.cpuThreads, threads)
	}
}

// TestCollect_GPUCacheExpiry tests GPU cache time-based expiry logic.
func TestCollect_GPUCacheExpiry(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()

	// First collect
	c.Collect()

	// Force an expired cache by setting lastGPUUpdate to 10 seconds ago
	c.mu.Lock()
	c.lastGPUUpdate = time.Now().Add(-10 * time.Second)
	c.mu.Unlock()

	// This should refresh the GPU cache
	c.Collect()

	c.mu.Lock()
	defer c.mu.Unlock()
	// After refresh, lastGPUUpdate should be recent (within last second)
	if time.Since(c.lastGPUUpdate) > time.Second {
		t.Error("GPU cache was not refreshed after expiry")
	}
}

// TestCollect_GPUCacheHit tests that GPU cache is used within 5 seconds.
func TestCollect_GPUCacheHit(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()

	// First collect populates cache
	c.Collect()

	c.mu.Lock()
	firstUpdate := c.lastGPUUpdate
	gpuCache := c.gpuCache
	// Both conditions must be true for cache to be used: timestamp set AND cache not nil
	if firstUpdate.IsZero() || gpuCache == nil {
		c.mu.Unlock()
		t.Skip("GPU cache was not populated on first collect (no GPU or query failed)")
	}
	c.mu.Unlock()

	// Immediately take second collect (within same millisecond if possible)
	// This ensures we're well within the 5s cache window
	c.Collect()

	c.mu.Lock()
	secondUpdate := c.lastGPUUpdate
	c.mu.Unlock()

	// The cache should be reused (same timestamp) since both conditions were met
	if !firstUpdate.Equal(secondUpdate) {
		timeDiff := secondUpdate.Sub(firstUpdate)
		t.Errorf("GPU cache timestamp changed unexpectedly: first=%v, second=%v (diff=%v)",
			firstUpdate, secondUpdate, timeDiff)
	}
}

// TestCollectHardwareInfo_DirectCall tests collectHardwareInfo directly.
func TestCollectHardwareInfo_DirectCall(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()
	stats := &SystemMetrics{}

	c.collectHardwareInfo(stats)

	// After first call, infoCollected should be true
	if !c.infoCollected {
		t.Error("infoCollected should be true after collectHardwareInfo")
	}

	// Stats should have the hardware info populated
	if stats.CPUModel != c.cpuModel {
		t.Errorf("CPUModel mismatch: stats=%q, cached=%q", stats.CPUModel, c.cpuModel)
	}
	if stats.CPUCores != c.cpuCores {
		t.Errorf("CPUCores mismatch: stats=%d, cached=%d", stats.CPUCores, c.cpuCores)
	}
	if stats.CPUThreads != c.cpuThreads {
		t.Errorf("CPUThreads mismatch: stats=%d, cached=%d", stats.CPUThreads, c.cpuThreads)
	}

	// Second call should use cached values
	stats2 := &SystemMetrics{}
	c.collectHardwareInfo(stats2)
	if stats2.CPUModel != stats.CPUModel {
		t.Errorf("CPUModel changed between calls: %q vs %q", stats2.CPUModel, stats.CPUModel)
	}
}

// TestCollectMemoryInfo_DirectCall tests collectMemoryInfo directly.
func TestCollectMemoryInfo_DirectCall(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()
	stats := &SystemMetrics{}

	c.collectMemoryInfo(stats)

	if stats.MemTotalMB <= 0 {
		t.Errorf("MemTotalMB = %f, want > 0", stats.MemTotalMB)
	}
	if stats.MemUsedMB < 0 {
		t.Errorf("MemUsedMB = %f, want >= 0", stats.MemUsedMB)
	}
	if stats.MemPercent < 0 || stats.MemPercent > 100 {
		t.Errorf("MemPercent = %f, want between 0 and 100", stats.MemPercent)
	}
}

// TestCollectCPUInfo_DirectCall tests collectCPUInfo directly.
func TestCollectCPUInfo_DirectCall(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()
	stats := &SystemMetrics{}

	// First call: baseline only (no delta)
	c.collectCPUInfo(stats)
	if c.lastCPUTotal <= 0 {
		t.Error("lastCPUTotal should be set after first call")
	}

	// Second call: should compute a percent
	time.Sleep(10 * time.Millisecond)
	stats2 := &SystemMetrics{}
	c.collectCPUInfo(stats2)

	if stats2.CPUPercent < 0 || stats2.CPUPercent > 100 {
		t.Errorf("CPUPercent = %f, want between 0 and 100", stats2.CPUPercent)
	}
}

// TestCollectDiskInfo_DirectCall tests collectDiskInfo directly.
func TestCollectDiskInfo_DirectCall(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()
	stats := &SystemMetrics{}

	c.collectDiskInfo(stats)

	if stats.DiskTotalGB <= 0 {
		t.Errorf("DiskTotalGB = %f, want > 0", stats.DiskTotalGB)
	}
	if stats.DiskUsedGB < 0 {
		t.Errorf("DiskUsedGB = %f, want >= 0", stats.DiskUsedGB)
	}
	if stats.DiskPercent < 0 || stats.DiskPercent > 100 {
		t.Errorf("DiskPercent = %f, want between 0 and 100", stats.DiskPercent)
	}
}

// TestCollectLoadAvg_DirectCall tests collectLoadAvg directly.
func TestCollectLoadAvg_DirectCall(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()
	stats := &SystemMetrics{}

	c.collectLoadAvg(stats)

	// Load averages should be >= 0
	if stats.LoadAvg1 < 0 {
		t.Errorf("LoadAvg1 = %f, want >= 0", stats.LoadAvg1)
	}
	if stats.LoadAvg5 < 0 {
		t.Errorf("LoadAvg5 = %f, want >= 0", stats.LoadAvg5)
	}
	if stats.LoadAvg15 < 0 {
		t.Errorf("LoadAvg15 = %f, want >= 0", stats.LoadAvg15)
	}
}

// TestCollectGPUInfo_DirectCall tests collectGPUInfo directly.
func TestCollectGPUInfo_DirectCall(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()
	stats := &SystemMetrics{}

	c.collectGPUInfo(stats)

	// GPU cache should be populated regardless of whether GPUs are found
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastGPUUpdate.IsZero() {
		t.Error("lastGPUUpdate should be set after collectGPUInfo")
	}
}

// TestQueryGPUInfo tests the queryGPUInfo function.
func TestQueryGPUInfo(t *testing.T) {
	t.Parallel()

	// This may return nil or GPUs depending on the system
	gpus := queryGPUInfo()
	// We just verify it doesn't panic
	_ = gpus
}

// TestQueryGhwGPU tests the queryGhwGPU function.
func TestQueryGhwGPU(t *testing.T) {
	t.Parallel()

	// This may return nil or GPUs depending on the system
	gpus := queryGhwGPU()
	// We just verify it doesn't panic
	for _, gpu := range gpus {
		if gpu.Name == "" {
			t.Error("GPU name should not be empty")
		}
	}
}

// TestMergeAMDJSON_MapPayload tests mergeAMDJSON with a map that has a GPU index.
func TestMergeAMDJSON_MapPayload(t *testing.T) {
	t.Parallel()

	info := map[int]*GPUInfo{}
	payload := map[string]any{
		"gpu":              float64(0),
		"card_name":        "Radeon RX 7900 XTX",
		"gpu_utilization":  float64(42.0),
		"vram_total":       float64(24576),
		"vram_used":        float64(12000),
		"edge_temperature": float64(65),
	}
	mergeAMDJSON(info, payload)

	if len(info) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(info))
	}

	gpu := info[0]
	if gpu.Name != "Radeon RX 7900 XTX" {
		t.Errorf("Name = %q, want %q", gpu.Name, "Radeon RX 7900 XTX")
	}
	if !gpu.UtilValid || gpu.UtilPercent != 42.0 {
		t.Errorf("Util = %f (valid=%v), want 42.0 (valid=true)", gpu.UtilPercent, gpu.UtilValid)
	}
	if !gpu.MemValid || gpu.MemTotalMB != 24576 {
		t.Errorf("MemTotalMB = %f (valid=%v), want 24576 (valid=true)", gpu.MemTotalMB, gpu.MemValid)
	}
	if gpu.MemUsedMB != 12000 {
		t.Errorf("MemUsedMB = %f, want 12000", gpu.MemUsedMB)
	}
	if !gpu.TempValid || gpu.TempC != 65 {
		t.Errorf("Temp = %f (valid=%v), want 65 (valid=true)", gpu.TempC, gpu.TempValid)
	}
}

// TestMergeAMDJSON_NestedMap tests mergeAMDJSON with nested maps containing GPU info.
func TestMergeAMDJSON_NestedMap(t *testing.T) {
	t.Parallel()

	info := map[int]*GPUInfo{}
	payload := map[string]any{
		"gpus": []any{
			map[string]any{
				"gpu":       float64(0),
				"card_name": "GPU A",
			},
			map[string]any{
				"gpu":       float64(1),
				"card_name": "GPU B",
			},
		},
	}
	mergeAMDJSON(info, payload)

	if len(info) != 2 {
		t.Fatalf("expected 2 GPUs, got %d", len(info))
	}
	if info[0].Name != "GPU A" {
		t.Errorf("GPU 0 Name = %q, want %q", info[0].Name, "GPU A")
	}
	if info[1].Name != "GPU B" {
		t.Errorf("GPU 1 Name = %q, want %q", info[1].Name, "GPU B")
	}
}

// TestMergeAMDJSON_MergesExistingEntry tests that mergeAMDJSON merges data into existing entries.
func TestMergeAMDJSON_MergesExistingEntry(t *testing.T) {
	t.Parallel()

	info := map[int]*GPUInfo{
		0: {Name: "Existing GPU"},
	}
	payload := map[string]any{
		"gpu":             float64(0),
		"gpu_utilization": float64(55.0),
	}
	mergeAMDJSON(info, payload)

	if len(info) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(info))
	}
	// Name should not change since it was already set
	if info[0].Name != "Existing GPU" {
		t.Errorf("Name should not be overwritten: got %q", info[0].Name)
	}
	if !info[0].UtilValid || info[0].UtilPercent != 55.0 {
		t.Errorf("Util should be merged: %f (valid=%v)", info[0].UtilPercent, info[0].UtilValid)
	}
}

// TestExtractAMDFields_MemoryUsed tests extractAMDFields for memory used field.
func TestExtractAMDFields_MemoryUsed(t *testing.T) {
	t.Parallel()

	info := &GPUInfo{}
	m := map[string]any{
		"memory_used": float64(4096),
	}
	extractAMDFields(info, m)
	if !info.MemValid || info.MemUsedMB != 4096 {
		t.Errorf("MemUsedMB = %f (valid=%v), want 4096 (valid=true)", info.MemUsedMB, info.MemValid)
	}
}

// TestExtractAMDFields_NamePriority tests that extractAMDFields uses first available name key.
func TestExtractAMDFields_NamePriority(t *testing.T) {
	t.Parallel()

	info := &GPUInfo{}
	m := map[string]any{
		"product_name": "Product A",
		"card_name":    "Card A",
	}
	extractAMDFields(info, m)
	// card_name has higher priority in firstString order
	if info.Name != "Card A" {
		t.Errorf("Name = %q, want %q (card_name has priority)", info.Name, "Card A")
	}
}

// TestExtractAMDFields_SkipsNonNameWhenAlreadySet verifies name is not overwritten.
func TestExtractAMDFields_SkipsNonNameWhenAlreadySet(t *testing.T) {
	t.Parallel()

	info := &GPUInfo{Name: "Already Set"}
	m := map[string]any{
		"card_name": "New Name",
	}
	extractAMDFields(info, m)
	if info.Name != "Already Set" {
		t.Errorf("Name should not be overwritten: got %q", info.Name)
	}
}

// TestExtractAMDFields_UsageKey tests that "usage" key is detected as utilization.
func TestExtractAMDFields_UsageKey(t *testing.T) {
	t.Parallel()

	info := &GPUInfo{}
	m := map[string]any{
		"gpu_usage": float64(30.5),
	}
	extractAMDFields(info, m)
	if !info.UtilValid || info.UtilPercent != 30.5 {
		t.Errorf("UtilPercent = %f (valid=%v), want 30.5 (valid=true)", info.UtilPercent, info.UtilValid)
	}
}

// TestExtractAMDFields_BusyKey tests that "busy" key is detected as utilization.
func TestExtractAMDFields_BusyKey(t *testing.T) {
	t.Parallel()

	info := &GPUInfo{}
	m := map[string]any{
		"gpu_busy_percent": float64(88.0),
	}
	extractAMDFields(info, m)
	if !info.UtilValid || info.UtilPercent != 88.0 {
		t.Errorf("UtilPercent = %f (valid=%v), want 88.0 (valid=true)", info.UtilPercent, info.UtilValid)
	}
}

// TestFindAMDIndex_FallbackPattern tests findAMDIndex with keys matching the fallback pattern.
func TestFindAMDIndex_FallbackPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		m      map[string]any
		want   int
		wantOK bool
	}{
		{"gpu_index composite key", map[string]any{"some_gpu_index_key": float64(7)}, 7, true},
		{"GPU in name but no index", map[string]any{"gpu_name": "Test"}, 0, false},
		{"index key", map[string]any{"index": float64(3)}, 3, true},
		{"device_id key", map[string]any{"device_id": float64(2)}, 2, true},
		{"gpu_index key", map[string]any{"gpu_index": float64(5)}, 5, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := findAMDIndex(tt.m)
			if ok != tt.wantOK {
				t.Errorf("findAMDIndex ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("findAMDIndex = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestToInt_EdgeCases tests toInt with additional edge case values.
func TestToInt_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  any
		want   int
		wantOK bool
	}{
		{"float64 zero", float64(0), 0, true},
		{"float64 negative", float64(-1), -1, true},
		{"float32 zero", float32(0), 0, true},
		{"int64 zero", int64(0), 0, true},
		{"int zero", int(0), 0, true},
		{"string with decimal", "42.5", 0, false},
		{"slice input", []int{1, 2}, 0, false},
		{"map input", map[string]int{"a": 1}, 0, false},
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

// TestToFloat_EdgeCases tests toFloat with additional edge case values.
func TestToFloat_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  any
		want   float64
		wantOK bool
	}{
		{"float64 zero", float64(0), 0, true},
		{"float64 negative", float64(-1.5), -1.5, true},
		{"float32 negative", float32(-2.5), -2.5, true},
		{"int zero", int(0), 0, true},
		{"int64 negative", int64(-100), -100, true},
		{"string with spaces", " 42.0 ", 42.0, true},
		{"string zero", "0", 0, true},
		{"bool input", true, 0, false},
		{"slice input", []float64{1.0}, 0, false},
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

// TestParseSizeToMB_KiB tests parseSizeToMB with KiB unit.
func TestParseSizeToMB_KiB(t *testing.T) {
	t.Parallel()

	got, ok := parseSizeToMB("2048", "KiB")
	if !ok {
		t.Fatal("parseSizeToMB should succeed")
	}
	if got != 2 {
		t.Errorf("parseSizeToMB(2048, KiB) = %f, want 2", got)
	}
}

// TestParseSizeToMB_MiB tests parseSizeToMB with MiB unit.
func TestParseSizeToMB_MiB(t *testing.T) {
	t.Parallel()

	got, ok := parseSizeToMB("512", "MiB")
	if !ok {
		t.Fatal("parseSizeToMB should succeed")
	}
	if got != 512 {
		t.Errorf("parseSizeToMB(512, MiB) = %f, want 512", got)
	}
}

// TestParseSizeStringToMB_WithBytes tests parseSizeStringToMB with B unit.
func TestParseSizeStringToMB_WithBytes(t *testing.T) {
	t.Parallel()

	got, ok := parseSizeStringToMB("1048576 B")
	if !ok {
		t.Fatal("parseSizeStringToMB should succeed")
	}
	if got != 1 {
		t.Errorf("parseSizeStringToMB(1048576 B) = %f, want 1", got)
	}
}

// TestParseWindowsGPUObj_Int64AdapterRAM tests parseWindowsGPUObj with int64 AdapterRAM.
func TestParseWindowsGPUObj_Int64AdapterRAM(t *testing.T) {
	t.Parallel()

	obj := map[string]any{
		"Name":       "Test GPU",
		"AdapterRAM": int64(4294967296), // 4 GB
	}
	gpu := parseWindowsGPUObj(obj)
	if gpu.Name != "Test GPU" {
		t.Errorf("Name = %q, want %q", gpu.Name, "Test GPU")
	}
	if !gpu.MemValid {
		t.Error("MemValid should be true")
	}
	expected := float64(4294967296) / 1024 / 1024
	if gpu.MemTotalMB != expected {
		t.Errorf("MemTotalMB = %f, want %f", gpu.MemTotalMB, expected)
	}
}

// TestParseWindowsGPUObj_StringAdapterRAM tests parseWindowsGPUObj with string AdapterRAM.
func TestParseWindowsGPUObj_StringAdapterRAM(t *testing.T) {
	t.Parallel()

	obj := map[string]any{
		"Name":       "String GPU",
		"AdapterRAM": "2147483648", // 2 GB
	}
	gpu := parseWindowsGPUObj(obj)
	if !gpu.MemValid {
		t.Error("MemValid should be true")
	}
	expected := float64(2147483648) / 1024 / 1024
	if gpu.MemTotalMB != expected {
		t.Errorf("MemTotalMB = %f, want %f", gpu.MemTotalMB, expected)
	}
}

// TestParseWindowsGPUObj_ZeroAdapterRAM tests parseWindowsGPUObj with zero AdapterRAM.
func TestParseWindowsGPUObj_ZeroAdapterRAM(t *testing.T) {
	t.Parallel()

	obj := map[string]any{
		"Name":       "Zero GPU",
		"AdapterRAM": float64(0),
	}
	gpu := parseWindowsGPUObj(obj)
	if gpu.MemValid {
		t.Error("MemValid should be false for zero AdapterRAM")
	}
}

// TestParseWindowsGPUObj_Int64ZeroAdapterRAM tests with int64 zero.
func TestParseWindowsGPUObj_Int64ZeroAdapterRAM(t *testing.T) {
	t.Parallel()

	obj := map[string]any{
		"Name":       "Zero Int64 GPU",
		"AdapterRAM": int64(0),
	}
	gpu := parseWindowsGPUObj(obj)
	if gpu.MemValid {
		t.Error("MemValid should be false for zero int64 AdapterRAM")
	}
}

// TestParseWindowsGPUObj_InvalidStringRAM tests with invalid string AdapterRAM.
func TestParseWindowsGPUObj_InvalidStringRAM(t *testing.T) {
	t.Parallel()

	obj := map[string]any{
		"Name":       "Invalid String GPU",
		"AdapterRAM": "not-a-number",
	}
	gpu := parseWindowsGPUObj(obj)
	if gpu.MemValid {
		t.Error("MemValid should be false for invalid string")
	}
}

// TestRootDiskPath_ReturnsNonEmpty verifies rootDiskPath returns a non-empty string.
func TestRootDiskPath_ReturnsNonEmpty(t *testing.T) {
	t.Parallel()

	path := rootDiskPath()
	if path == "" {
		t.Error("rootDiskPath() should return non-empty path")
	}
	// On Linux, it should be "/"
	if path != "/" {
		t.Logf("rootDiskPath() = %q (expected '/' on Linux)", path)
	}
}

// TestIsUtilKey_EdgeCases tests isUtilKey with edge cases.
func TestIsUtilKey_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"UTILIZATION", false}, // case sensitive
		{"util", true},
		{"gpu_util_percent", true},
		{"usage_rate", true},
		{"busy_gpu", true},
		{"compute_usage", true},
		{"name", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := isUtilKey(tt.input)
			if got != tt.want {
				t.Errorf("isUtilKey(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestIsTempKey_EdgeCases tests isTempKey with edge cases.
func TestIsTempKey_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"temp", true},
		{"temperature_edge", true},
		{"gpu_temp_celsius", true},
		{"edge_temp", true},
		{"name", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := isTempKey(tt.input)
			if got != tt.want {
				t.Errorf("isTempKey(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestFirstString_EmptyMap tests firstString with an empty map.
func TestFirstString_EmptyMap(t *testing.T) {
	t.Parallel()

	m := map[string]any{}
	got := firstString(m, "name", "model")
	if got != "" {
		t.Errorf("firstString on empty map = %q, want empty", got)
	}
}

// TestFirstString_NilValueSkipped tests that nil values in map are skipped.
func TestFirstString_NilValueSkipped(t *testing.T) {
	t.Parallel()

	m := map[string]any{
		"name":  nil,
		"model": "Test Model",
	}
	got := firstString(m, "name", "model")
	if got != "Test Model" {
		t.Errorf("firstString = %q, want %q", got, "Test Model")
	}
}

// TestParseSizeToMB_AllUnits tests all supported unit conversions.
func TestParseSizeToMB_AllUnits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		unit  string
		want  float64
	}{
		{"B", "1048576", "b", 1},
		{"KB", "1024", "kb", 1},
		{"KiB", "1024", "kib", 1},
		{"MB", "100", "mb", 100},
		{"MiB", "100", "mib", 100},
		{"GB", "1", "gb", 1024},
		{"GiB", "1", "gib", 1024},
		{"unknown", "500", "TB", 500}, // falls through to default
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseSizeToMB(tt.value, tt.unit)
			if !ok {
				t.Fatal("parseSizeToMB should succeed")
			}
			if got != tt.want {
				t.Errorf("parseSizeToMB(%q, %q) = %f, want %f", tt.value, tt.unit, got, tt.want)
			}
		})
	}
}

// TestParseSizeStringToMB_MixedCase tests parseSizeStringToMB with mixed case units.
func TestParseSizeStringToMB_MixedCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		want   float64
		wantOK bool
	}{
		{"GB mixed case", "4 Gb", 4096, true},
		{"MB mixed case", "256 Mb", 256, true},
		{"KB mixed case", "1024 Kb", 1, true},
		{"No space", "8GB", 8192, true},
		{"Leading text", "VRAM: 16 GB", 16384, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseSizeStringToMB(tt.input)
			if ok != tt.wantOK {
				t.Errorf("parseSizeStringToMB(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("parseSizeStringToMB(%q) = %f, want %f", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseFloatField_ExtraCases tests parseFloatField with additional edge cases.
func TestParseFloatField_ExtraCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		want   float64
		wantOK bool
	}{
		{"scientific notation", "1.5e2", 150, true},
		{"very small", "0.001", 0.001, true},
		{"very large", "999999999.99", 999999999.99, true},
		{"only spaces", "   ", 0, false},
		{"plus sign", "+42", 42, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseFloatField(tt.input)
			if ok != tt.wantOK {
				t.Errorf("parseFloatField(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("parseFloatField(%q) = %f, want %f", tt.input, got, tt.want)
			}
		})
	}
}

// TestCollect_Concurrent verifies Collect is safe for concurrent use.
func TestCollect_Concurrent(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()
	done := make(chan struct{})

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_ = c.Collect()
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
