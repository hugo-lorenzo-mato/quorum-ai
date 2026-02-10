package diagnostics

import (
	"testing"
)

func TestNewSystemMetricsCollector(t *testing.T) {
	t.Parallel()
	c := NewSystemMetricsCollector()
	if c == nil {
		t.Fatal("expected non-nil collector")
	}
}

func TestCollect_ReturnsMetrics(t *testing.T) {
	t.Parallel()
	c := NewSystemMetricsCollector()
	m := c.Collect()

	// Memory should be > 0 on any real system
	if m.MemTotalMB <= 0 {
		t.Error("expected MemTotalMB > 0")
	}
	if m.MemPercent < 0 || m.MemPercent > 100 {
		t.Errorf("MemPercent out of range: %f", m.MemPercent)
	}

	// Disk should be > 0 on any real system
	if m.DiskTotalGB <= 0 {
		t.Error("expected DiskTotalGB > 0")
	}
	if m.DiskPercent < 0 || m.DiskPercent > 100 {
		t.Errorf("DiskPercent out of range: %f", m.DiskPercent)
	}
}

func TestCollect_CPUInfoCached(t *testing.T) {
	t.Parallel()
	c := NewSystemMetricsCollector()

	// First call populates CPU info
	m1 := c.Collect()
	// Second call uses cache
	m2 := c.Collect()

	if m1.CPUModel != m2.CPUModel {
		t.Errorf("CPU model changed between calls: %q vs %q", m1.CPUModel, m2.CPUModel)
	}
	if m1.CPUCores != m2.CPUCores {
		t.Errorf("CPU cores changed between calls: %d vs %d", m1.CPUCores, m2.CPUCores)
	}
	if m1.CPUThreads != m2.CPUThreads {
		t.Errorf("CPU threads changed between calls: %d vs %d", m1.CPUThreads, m2.CPUThreads)
	}
}

func TestCollect_GPUCached(t *testing.T) {
	t.Parallel()
	c := NewSystemMetricsCollector()

	// First call queries GPU
	m1 := c.Collect()
	// Second call within 5s should use cache
	m2 := c.Collect()

	// Number of GPUs should be consistent
	if len(m1.GPUInfos) != len(m2.GPUInfos) {
		t.Errorf("GPU count changed between calls: %d vs %d", len(m1.GPUInfos), len(m2.GPUInfos))
	}
}

func TestParseFloatField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    float64
		wantOK  bool
	}{
		{"integer", "42", 42.0, true},
		{"float", "3.14", 3.14, true},
		{"with spaces", "  99.5  ", 99.5, true},
		{"empty", "", 0, false},
		{"not a number", "abc", 0, false},
		{"zero", "0", 0, true},
		{"negative", "-1.5", -1.5, true},
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

func TestParseSizeToMB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		unit    string
		want    float64
		wantOK  bool
	}{
		{"GB to MB", "2", "GB", 2048, true},
		{"MB stays", "512", "MB", 512, true},
		{"KB to MB", "1024", "KB", 1, true},
		{"GiB to MB", "1", "GiB", 1024, true},
		{"MiB stays", "256", "MiB", 256, true},
		{"B to MB", "1048576", "B", 1, true},
		{"unknown unit", "100", "XB", 100, true},
		{"invalid value", "abc", "MB", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseSizeToMB(tt.value, tt.unit)
			if ok != tt.wantOK {
				t.Errorf("parseSizeToMB(%q, %q) ok = %v, want %v", tt.value, tt.unit, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("parseSizeToMB(%q, %q) = %f, want %f", tt.value, tt.unit, got, tt.want)
			}
		})
	}
}

func TestParseSizeStringToMB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		want   float64
		wantOK bool
	}{
		{"GB string", "8 GB", 8192, true},
		{"MB string", "256 MB", 256, true},
		{"KB string", "2048 KB", 2, true},
		{"no unit fallback", "1024", 1024, true},
		{"empty", "", 0, false},
		{"garbage", "not a size", 0, false},
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

func TestToInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  any
		want   int
		wantOK bool
	}{
		{"int", 42, 42, true},
		{"int64", int64(100), 100, true},
		{"float64", float64(3.0), 3, true},
		{"float32", float32(5.0), 5, true},
		{"string", "7", 7, true},
		{"string with spaces", " 10 ", 10, true},
		{"invalid string", "abc", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
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

func TestToFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  any
		want   float64
		wantOK bool
	}{
		{"float64", float64(3.14), 3.14, true},
		{"float32", float32(2.5), 2.5, true},
		{"int", 42, 42.0, true},
		{"int64", int64(100), 100.0, true},
		{"string", "99.5", 99.5, true},
		{"invalid string", "abc", 0, false},
		{"nil", nil, 0, false},
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

func TestFirstString(t *testing.T) {
	t.Parallel()

	m := map[string]any{
		"name":    "GPU 0",
		"vendor":  "NVIDIA",
		"model":   42, // not a string
		"product": "  RTX 4090  ",
	}

	tests := []struct {
		name string
		keys []string
		want string
	}{
		{"first key match", []string{"name"}, "GPU 0"},
		{"second key match", []string{"missing", "vendor"}, "NVIDIA"},
		{"non-string skipped", []string{"model", "vendor"}, "NVIDIA"},
		{"trimmed", []string{"product"}, "RTX 4090"},
		{"no match", []string{"missing1", "missing2"}, ""},
		{"empty keys", []string{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := firstString(m, tt.keys...)
			if got != tt.want {
				t.Errorf("firstString(m, %v) = %q, want %q", tt.keys, got, tt.want)
			}
		})
	}
}

func TestIsUtilKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"gpu_utilization", true},
		{"usage", true},
		{"busy_percent", true},
		{"temperature", false},
		{"memory_total", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			if got := isUtilKey(tt.input); got != tt.want {
				t.Errorf("isUtilKey(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsTempKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"temperature", true},
		{"temp_c", true},
		{"gpu_temp", true},
		{"utilization", false},
		{"memory", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			if got := isTempKey(tt.input); got != tt.want {
				t.Errorf("isTempKey(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestRootDiskPath(t *testing.T) {
	t.Parallel()
	path := rootDiskPath()
	if path == "" {
		t.Error("expected non-empty disk path")
	}
}

func TestExtractAMDFields(t *testing.T) {
	t.Parallel()

	t.Run("extracts name", func(t *testing.T) {
		t.Parallel()
		info := &GPUInfo{}
		m := map[string]any{"card_name": "RX 7900 XTX"}
		extractAMDFields(info, m)
		if info.Name != "RX 7900 XTX" {
			t.Errorf("expected name 'RX 7900 XTX', got %q", info.Name)
		}
	})

	t.Run("extracts utilization", func(t *testing.T) {
		t.Parallel()
		info := &GPUInfo{}
		m := map[string]any{"gpu_utilization": float64(75.5)}
		extractAMDFields(info, m)
		if !info.UtilValid || info.UtilPercent != 75.5 {
			t.Errorf("expected util 75.5, got %f (valid=%v)", info.UtilPercent, info.UtilValid)
		}
	})

	t.Run("extracts memory", func(t *testing.T) {
		t.Parallel()
		info := &GPUInfo{}
		m := map[string]any{
			"vram_total": float64(16384),
			"vram_used":  float64(8000),
		}
		extractAMDFields(info, m)
		if !info.MemValid {
			t.Error("expected MemValid=true")
		}
		if info.MemTotalMB != 16384 {
			t.Errorf("expected MemTotalMB=16384, got %f", info.MemTotalMB)
		}
	})

	t.Run("extracts temperature", func(t *testing.T) {
		t.Parallel()
		info := &GPUInfo{}
		m := map[string]any{"edge_temperature": float64(65)}
		extractAMDFields(info, m)
		if !info.TempValid || info.TempC != 65 {
			t.Errorf("expected temp 65, got %f (valid=%v)", info.TempC, info.TempValid)
		}
	})
}

func TestFindAMDIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		m      map[string]any
		want   int
		wantOK bool
	}{
		{"gpu key", map[string]any{"gpu": float64(0)}, 0, true},
		{"gpu_id key", map[string]any{"gpu_id": float64(1)}, 1, true},
		{"device key", map[string]any{"device": float64(2)}, 2, true},
		{"string index", map[string]any{"gpu": "3"}, 3, true},
		{"gpu_index in name", map[string]any{"my_gpu_index_field": float64(4)}, 4, true},
		{"no match", map[string]any{"name": "GPU"}, 0, false},
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

func TestMergeAMDJSON(t *testing.T) {
	t.Parallel()

	t.Run("nil payload", func(t *testing.T) {
		t.Parallel()
		info := map[int]*GPUInfo{}
		mergeAMDJSON(info, nil)
		if len(info) != 0 {
			t.Error("expected empty map after nil payload")
		}
	})

	t.Run("array payload", func(t *testing.T) {
		t.Parallel()
		info := map[int]*GPUInfo{}
		payload := []any{
			map[string]any{"gpu": float64(0), "card_name": "GPU 0"},
			map[string]any{"gpu": float64(1), "card_name": "GPU 1"},
		}
		mergeAMDJSON(info, payload)
		if len(info) != 2 {
			t.Errorf("expected 2 GPUs, got %d", len(info))
		}
		if info[0].Name != "GPU 0" {
			t.Errorf("expected 'GPU 0', got %q", info[0].Name)
		}
	})
}

func TestParseWindowsGPUObj(t *testing.T) {
	t.Parallel()

	t.Run("with name and memory", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"Name":       "NVIDIA GeForce RTX 4090",
			"AdapterRAM": float64(25769803776), // 24 GB in bytes
		}
		gpu := parseWindowsGPUObj(obj)
		if gpu.Name != "NVIDIA GeForce RTX 4090" {
			t.Errorf("expected name, got %q", gpu.Name)
		}
		if !gpu.MemValid {
			t.Error("expected MemValid=true")
		}
		// 25769803776 / 1024 / 1024 ≈ 24576 MB
		if gpu.MemTotalMB < 24000 || gpu.MemTotalMB > 25000 {
			t.Errorf("expected ~24576 MB, got %f", gpu.MemTotalMB)
		}
	})

	t.Run("no adapter RAM", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{"Name": "Basic Display"}
		gpu := parseWindowsGPUObj(obj)
		if gpu.MemValid {
			t.Error("expected MemValid=false when no AdapterRAM")
		}
	})
}
