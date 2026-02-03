package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jaypipes/ghw"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

// GPUInfo holds GPU usage information (best-effort).
type GPUInfo struct {
	Name        string  `json:"name"`
	UtilPercent float64 `json:"util_percent,omitempty"`
	UtilValid   bool    `json:"util_valid"`
	MemTotalMB  float64 `json:"mem_total_mb,omitempty"`
	MemUsedMB   float64 `json:"mem_used_mb,omitempty"`
	MemValid    bool    `json:"mem_valid"`
	TempC       float64 `json:"temp_c,omitempty"`
	TempValid   bool    `json:"temp_valid"`
}

// SystemMetrics holds system-wide resource usage.
type SystemMetrics struct {
	// CPU
	CPUModel   string  `json:"cpu_model"`
	CPUCores   int     `json:"cpu_cores"`
	CPUThreads int     `json:"cpu_threads"`
	CPUPercent float64 `json:"cpu_percent"`

	// Memory (in MB)
	MemTotalMB float64 `json:"mem_total_mb"`
	MemUsedMB  float64 `json:"mem_used_mb"`
	MemPercent float64 `json:"mem_percent"`

	// Disk (in GB)
	DiskTotalGB float64 `json:"disk_total_gb"`
	DiskUsedGB  float64 `json:"disk_used_gb"`
	DiskPercent float64 `json:"disk_percent"`

	// Load Average (Unix)
	LoadAvg1  float64 `json:"load_avg_1"`
	LoadAvg5  float64 `json:"load_avg_5"`
	LoadAvg15 float64 `json:"load_avg_15"`

	// GPU (optional)
	GPUInfos []GPUInfo `json:"gpu_infos,omitempty"`
}

// SystemMetricsCollector collects system-wide statistics.
type SystemMetricsCollector struct {
	mu           sync.Mutex
	lastCPUTotal float64
	lastCPUIdle  float64

	infoCollected bool
	cpuModel      string
	cpuCores      int
	cpuThreads    int

	lastGPUUpdate time.Time
	gpuCache      []GPUInfo
}

// NewSystemMetricsCollector creates a new system metrics collector.
func NewSystemMetricsCollector() *SystemMetricsCollector {
	return &SystemMetricsCollector{}
}

// Collect gathers current system statistics.
func (c *SystemMetricsCollector) Collect() SystemMetrics {
	c.mu.Lock()
	defer c.mu.Unlock()

	stats := SystemMetrics{}

	// Hardware info (cached)
	c.collectHardwareInfo(&stats)

	// Memory info
	c.collectMemoryInfo(&stats)

	// CPU usage
	c.collectCPUInfo(&stats)

	// Disk usage
	c.collectDiskInfo(&stats)

	// Load average
	c.collectLoadAvg(&stats)

	// GPU info (best-effort, cached)
	c.collectGPUInfo(&stats)

	return stats
}

// collectMemoryInfo reads system memory information.
func (c *SystemMetricsCollector) collectMemoryInfo(stats *SystemMetrics) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return
	}

	stats.MemTotalMB = float64(vm.Total) / 1024 / 1024
	stats.MemUsedMB = float64(vm.Used) / 1024 / 1024
	stats.MemPercent = vm.UsedPercent
}

// collectCPUInfo reads system CPU usage.
func (c *SystemMetricsCollector) collectCPUInfo(stats *SystemMetrics) {
	times, err := cpu.Times(false)
	if err != nil || len(times) == 0 {
		return
	}

	t := times[0]
	total := t.User + t.Nice + t.System + t.Idle + t.Iowait + t.Irq + t.Softirq + t.Steal
	idleTime := t.Idle + t.Iowait

	if c.lastCPUTotal > 0 {
		totalDelta := total - c.lastCPUTotal
		idleDelta := idleTime - c.lastCPUIdle
		if totalDelta > 0 {
			stats.CPUPercent = (1 - idleDelta/totalDelta) * 100
		}
	}

	c.lastCPUTotal = total
	c.lastCPUIdle = idleTime
}

// collectDiskInfo reads disk usage for the root filesystem.
func (c *SystemMetricsCollector) collectDiskInfo(stats *SystemMetrics) {
	path := rootDiskPath()
	usage, err := disk.Usage(path)
	if err != nil {
		return
	}
	stats.DiskTotalGB = float64(usage.Total) / 1024 / 1024 / 1024
	stats.DiskUsedGB = float64(usage.Used) / 1024 / 1024 / 1024
	stats.DiskPercent = usage.UsedPercent
}

// collectLoadAvg reads system load averages.
func (c *SystemMetricsCollector) collectLoadAvg(stats *SystemMetrics) {
	avg, err := load.Avg()
	if err != nil {
		return
	}
	stats.LoadAvg1 = avg.Load1
	stats.LoadAvg5 = avg.Load5
	stats.LoadAvg15 = avg.Load15
}

func (c *SystemMetricsCollector) collectHardwareInfo(stats *SystemMetrics) {
	if !c.infoCollected {
		if infos, err := cpu.Info(); err == nil && len(infos) > 0 {
			c.cpuModel = strings.TrimSpace(infos[0].ModelName)
		}
		if cores, err := cpu.Counts(false); err == nil && cores > 0 {
			c.cpuCores = cores
		}
		if threads, err := cpu.Counts(true); err == nil && threads > 0 {
			c.cpuThreads = threads
		}
		c.infoCollected = true
	}
	stats.CPUModel = c.cpuModel
	stats.CPUCores = c.cpuCores
	stats.CPUThreads = c.cpuThreads
}

func (c *SystemMetricsCollector) collectGPUInfo(stats *SystemMetrics) {
	now := time.Now()
	if now.Sub(c.lastGPUUpdate) < 5*time.Second && c.gpuCache != nil {
		stats.GPUInfos = append([]GPUInfo(nil), c.gpuCache...)
		return
	}
	gpus := queryGPUInfo()
	c.gpuCache = gpus
	c.lastGPUUpdate = now
	stats.GPUInfos = append([]GPUInfo(nil), gpus...)
}

func queryGPUInfo() []GPUInfo {
	if gpus := queryNvidiaSMI(); len(gpus) > 0 {
		return gpus
	}
	if gpus := queryAmdSMI(); len(gpus) > 0 {
		return gpus
	}
	switch runtime.GOOS {
	case "linux":
		if gpus := queryRocmSMI(); len(gpus) > 0 {
			return gpus
		}
	case "darwin":
		if gpus := querySystemProfiler(); len(gpus) > 0 {
			return gpus
		}
	case "windows":
		if gpus := queryWindowsGPU(); len(gpus) > 0 {
			return gpus
		}
	}
	if gpus := queryGhwGPU(); len(gpus) > 0 {
		return gpus
	}
	return nil
}

func queryNvidiaSMI() []GPUInfo {
	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=name,utilization.gpu,memory.total,memory.used,temperature.gpu", "--format=csv,noheader,nounits")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	gpus := make([]GPUInfo, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 5 {
			continue
		}
		name := strings.TrimSpace(fields[0])
		util, utilOK := parseFloatField(fields[1])
		memTotal, totalOK := parseFloatField(fields[2])
		memUsed, usedOK := parseFloatField(fields[3])
		temp, tempOK := parseFloatField(fields[4])

		gpus = append(gpus, GPUInfo{
			Name:        name,
			UtilPercent: util,
			UtilValid:   utilOK,
			MemTotalMB:  memTotal,
			MemUsedMB:   memUsed,
			MemValid:    totalOK && usedOK,
			TempC:       temp,
			TempValid:   tempOK,
		})
	}
	return gpus
}

func queryAmdSMI() []GPUInfo {
	path, err := exec.LookPath("amd-smi")
	if err != nil {
		return nil
	}

	staticData := amdSmiJSON(path, []string{"static", "--json"})
	metricData := amdSmiJSON(path, []string{"metric", "--json"})
	if metricData == nil {
		metricData = amdSmiJSON(path, []string{"metric", "--json", "-u", "-m", "-t"})
	}
	listData := amdSmiJSON(path, []string{"list", "--json"})

	info := map[int]*GPUInfo{}
	mergeAMDJSON(info, staticData)
	mergeAMDJSON(info, metricData)
	mergeAMDJSON(info, listData)

	if len(info) == 0 {
		return nil
	}

	idxs := make([]int, 0, len(info))
	for idx := range info {
		idxs = append(idxs, idx)
	}
	sort.Ints(idxs)

	gpus := make([]GPUInfo, 0, len(idxs))
	for _, idx := range idxs {
		gpu := *info[idx]
		if gpu.Name == "" {
			gpu.Name = fmt.Sprintf("AMD GPU %d", idx)
		}
		gpus = append(gpus, gpu)
	}
	return gpus
}

func amdSmiJSON(path string, args []string) any {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var payload any
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil
	}
	return payload
}

func mergeAMDJSON(info map[int]*GPUInfo, payload any) {
	if payload == nil {
		return
	}
	switch v := payload.(type) {
	case []any:
		for _, item := range v {
			mergeAMDJSON(info, item)
		}
	case map[string]any:
		idx, hasIdx := findAMDIndex(v)
		if hasIdx {
			entry, ok := info[idx]
			if !ok {
				entry = &GPUInfo{}
				info[idx] = entry
			}
			extractAMDFields(entry, v)
		}
		for _, child := range v {
			mergeAMDJSON(info, child)
		}
	}
}

func findAMDIndex(m map[string]any) (int, bool) {
	keys := []string{"gpu", "gpu_id", "gpu_index", "device", "device_id", "index"}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if idx, ok := toInt(v); ok {
				return idx, true
			}
		}
	}
	for k, v := range m {
		kl := strings.ToLower(k)
		if strings.Contains(kl, "gpu") && strings.Contains(kl, "index") {
			if idx, ok := toInt(v); ok {
				return idx, true
			}
		}
	}
	return 0, false
}

func extractAMDFields(info *GPUInfo, m map[string]any) {
	if info.Name == "" {
		if name := firstString(m, "card_name", "product_name", "name", "model", "gpu_name"); name != "" {
			info.Name = name
		}
	}

	// Scan fields for utilization, memory and temperature
	for k, v := range m {
		kl := strings.ToLower(k)
		if isTempKey(kl) {
			if val, ok := toFloat(v); ok {
				info.TempC = val
				info.TempValid = true
			}
		}
		if isUtilKey(kl) {
			if val, ok := toFloat(v); ok {
				info.UtilPercent = val
				info.UtilValid = true
			}
		}
		if strings.Contains(kl, "vram") || strings.Contains(kl, "memory") || strings.Contains(kl, "mem") {
			if strings.Contains(kl, "total") {
				if val, ok := toFloat(v); ok {
					info.MemTotalMB = val
					info.MemValid = true
				}
			} else if strings.Contains(kl, "used") {
				if val, ok := toFloat(v); ok {
					info.MemUsedMB = val
					info.MemValid = true
				}
			}
		}
	}
}

func isUtilKey(k string) bool {
	return strings.Contains(k, "util") || strings.Contains(k, "usage") || strings.Contains(k, "busy")
}

func isTempKey(k string) bool {
	return strings.Contains(k, "temp")
}

func toInt(v any) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case float32:
		return int(val), true
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			return n, true
		}
	}
	return 0, false
}

func toFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		return parseFloatField(val)
	}
	return 0, false
}

func queryRocmSMI() []GPUInfo {
	if _, err := exec.LookPath("rocm-smi"); err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "rocm-smi", "--showproductname", "--showuse", "--showmemuse", "--showtemp")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return nil
	}

	reIdx := regexp.MustCompile(`GPU\[(\d+)\]`)
	reName := regexp.MustCompile(`(?i)(Card series|GPU name|Product Name)\s*:\s*(.+)$`)
	reUse := regexp.MustCompile(`(?i)GPU\s*use.*?:\s*([0-9.]+)`)
	reMemPair := regexp.MustCompile(`(?i)VRAM\s*Total.*?:\s*([0-9.]+)\s*([A-Za-z]+).*VRAM\s*Used.*?:\s*([0-9.]+)\s*([A-Za-z]+)`)
	reMemSingle := regexp.MustCompile(`(?i)VRAM\s*(Total|Used).*?:\s*([0-9.]+)\s*([A-Za-z]+)`)
	reTemp := regexp.MustCompile(`(?i)Temperature.*?:\s*([0-9.]+)`)

	byIdx := map[int]*GPUInfo{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := reIdx.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		idx, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		info, ok := byIdx[idx]
		if !ok {
			info = &GPUInfo{}
			byIdx[idx] = info
		}

		if m := reName.FindStringSubmatch(line); len(m) == 3 {
			info.Name = strings.TrimSpace(m[2])
		}
		if m := reUse.FindStringSubmatch(line); len(m) == 2 {
			if v, ok := parseFloatField(m[1]); ok {
				info.UtilPercent = v
				info.UtilValid = true
			}
		}
		if m := reMemPair.FindStringSubmatch(line); len(m) == 5 {
			if total, ok := parseSizeToMB(m[1], m[2]); ok {
				info.MemTotalMB = total
			}
			if used, ok := parseSizeToMB(m[3], m[4]); ok {
				info.MemUsedMB = used
			}
			if info.MemTotalMB > 0 || info.MemUsedMB > 0 {
				info.MemValid = true
			}
		} else if m := reMemSingle.FindStringSubmatch(line); len(m) == 4 {
			if v, ok := parseSizeToMB(m[2], m[3]); ok {
				if strings.EqualFold(m[1], "total") {
					info.MemTotalMB = v
				} else {
					info.MemUsedMB = v
				}
				info.MemValid = true
			}
		}
		if m := reTemp.FindStringSubmatch(line); len(m) == 2 {
			if v, ok := parseFloatField(m[1]); ok {
				info.TempC = v
				info.TempValid = true
			}
		}
	}

	if len(byIdx) == 0 {
		return nil
	}
	idxs := make([]int, 0, len(byIdx))
	for idx := range byIdx {
		idxs = append(idxs, idx)
	}
	sort.Ints(idxs)
	gpus := make([]GPUInfo, 0, len(idxs))
	for _, idx := range idxs {
		gpus = append(gpus, *byIdx[idx])
	}
	return gpus
}

func queryGhwGPU() []GPUInfo {
	info, err := ghw.GPU()
	if err != nil || info == nil || len(info.GraphicsCards) == 0 {
		return nil
	}

	gpus := make([]GPUInfo, 0, len(info.GraphicsCards))
	for _, card := range info.GraphicsCards {
		name := ""
		if card.DeviceInfo != nil {
			if card.DeviceInfo.Vendor != nil && card.DeviceInfo.Product != nil {
				name = strings.TrimSpace(card.DeviceInfo.Vendor.Name + " " + card.DeviceInfo.Product.Name)
			} else if card.DeviceInfo.Product != nil {
				name = strings.TrimSpace(card.DeviceInfo.Product.Name)
			} else if card.DeviceInfo.Vendor != nil {
				name = strings.TrimSpace(card.DeviceInfo.Vendor.Name)
			}
		}
		if name == "" {
			name = fmt.Sprintf("GPU %d", card.Index)
		}
		gpus = append(gpus, GPUInfo{Name: name})
	}
	return gpus
}

func querySystemProfiler() []GPUInfo {
	if _, err := exec.LookPath("system_profiler"); err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "system_profiler", "-json", "SPDisplaysDataType")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil
	}
	raw, ok := payload["SPDisplaysDataType"]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}

	var gpus []GPUInfo
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := firstString(obj, "_name", "sppci_model", "spdisplays_vendor", "spdisplays_device-id")
		if name == "" {
			continue
		}
		vramStr := firstString(obj, "_spdisplays_vram", "spdisplays_vram", "spdisplays_vram_shared")
		memMB, memOK := parseSizeStringToMB(vramStr)
		gpus = append(gpus, GPUInfo{
			Name:       name,
			MemTotalMB: memMB,
			MemValid:   memOK,
		})
	}
	return gpus
}

func queryWindowsGPU() []GPUInfo {
	if _, err := exec.LookPath("powershell"); err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", "Get-CimInstance Win32_VideoController | Select-Object Name, AdapterRAM | ConvertTo-Json")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var raw any
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil
	}

	var gpus []GPUInfo
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			if obj, ok := item.(map[string]any); ok {
				gpus = append(gpus, parseWindowsGPUObj(obj))
			}
		}
	case map[string]any:
		gpus = append(gpus, parseWindowsGPUObj(v))
	}

	filtered := gpus[:0]
	for _, gpu := range gpus {
		if gpu.Name != "" {
			filtered = append(filtered, gpu)
		}
	}
	return filtered
}

func parseWindowsGPUObj(obj map[string]any) GPUInfo {
	name := firstString(obj, "Name")
	var memMB float64
	var memOK bool
	if v, ok := obj["AdapterRAM"]; ok {
		switch val := v.(type) {
		case float64:
			if val > 0 {
				memMB = val / 1024 / 1024
				memOK = true
			}
		case int64:
			if val > 0 {
				memMB = float64(val) / 1024 / 1024
				memOK = true
			}
		case string:
			if parsed, ok := parseFloatField(val); ok && parsed > 0 {
				memMB = parsed / 1024 / 1024
				memOK = true
			}
		}
	}
	return GPUInfo{
		Name:       name,
		MemTotalMB: memMB,
		MemValid:   memOK,
	}
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func parseSizeToMB(value, unit string) (float64, bool) {
	v, ok := parseFloatField(value)
	if !ok {
		return 0, false
	}
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "b":
		return v / 1024 / 1024, true
	case "kb", "kib":
		return v / 1024, true
	case "mb", "mib":
		return v, true
	case "gb", "gib":
		return v * 1024, true
	default:
		return v, true
	}
}

func parseSizeStringToMB(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	re := regexp.MustCompile(`(?i)([0-9.]+)\s*(GB|MB|KB|B)`)
	if m := re.FindStringSubmatch(s); len(m) == 3 {
		return parseSizeToMB(m[1], m[2])
	}
	if v, ok := parseFloatField(s); ok {
		return v, true
	}
	return 0, false
}

func parseFloatField(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func rootDiskPath() string {
	if runtime.GOOS == "windows" {
		drive := os.Getenv("SystemDrive")
		if drive == "" {
			drive = "C:"
		}
		return drive + "\\"
	}
	return "/"
}
