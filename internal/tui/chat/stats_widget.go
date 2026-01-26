package chat

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

	"github.com/charmbracelet/lipgloss"
	"github.com/jaypipes/ghw"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

// Icons for stats widget (unique, not used elsewhere)
const (
	iconClock  = "◷" // Clock face
	iconMemory = "⬢" // Hexagon (memory chip)
	iconCPU    = "⚙" // Gear (processor)
)

// ProcessStats holds current process statistics
type ProcessStats struct {
	MemoryMB      float64
	MemoryAlloc   uint64
	CPUPercent    float64 // Normalized (0-100%, relative to total system capacity)
	CPURawPercent float64 // Raw (can exceed 100% on multi-core, like top/htop)
	Goroutines    int
	Uptime        time.Duration
}

// StatsWidget displays process statistics
type StatsWidget struct {
	mu        sync.Mutex
	stats     ProcessStats
	startTime time.Time
	visible   bool
	width     int
	height    int

	// CPU calculation
	lastCPUTime  float64
	lastWallTime time.Time

	proc     *process.Process
	cpuCount int
}

// NewStatsWidget creates a new stats widget
func NewStatsWidget() *StatsWidget {
	pid := os.Getpid()
	// #nosec G115 - PID is guaranteed to fit in int32 on all supported platforms
	proc, _ := process.NewProcess(int32(pid))
	cpuCount, err := cpu.Counts(true)
	if err != nil || cpuCount <= 0 {
		cpuCount = runtime.NumCPU()
	}
	w := &StatsWidget{
		startTime:    time.Now(),
		visible:      false, // Hidden by default, toggle with Ctrl+5
		lastWallTime: time.Now(),
		proc:         proc,
		cpuCount:     cpuCount,
	}
	w.Update() // Initial stats
	return w
}

// Update refreshes the statistics
func (w *StatsWidget) Update() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	w.stats.MemoryAlloc = memStats.Alloc
	if w.proc != nil {
		if memInfo, err := w.proc.MemoryInfo(); err == nil {
			w.stats.MemoryMB = float64(memInfo.RSS) / 1024 / 1024
		} else {
			w.stats.MemoryMB = float64(memStats.Alloc) / 1024 / 1024
		}
	} else {
		w.stats.MemoryMB = float64(memStats.Alloc) / 1024 / 1024
	}

	// Goroutines
	w.stats.Goroutines = runtime.NumGoroutine()

	// Uptime
	w.stats.Uptime = time.Since(w.startTime)

	// CPU percentage (process-level, cross-platform)
	rawCPU := w.calculateCPURaw()
	w.stats.CPURawPercent = rawCPU
	denom := float64(w.cpuCount)
	if denom <= 0 {
		denom = float64(runtime.NumCPU())
	}
	if denom <= 0 {
		denom = 1
	}
	w.stats.CPUPercent = rawCPU / denom // Normalized to system capacity
}

// calculateCPURaw calculates raw CPU usage for the current process (can exceed 100% on multi-core)
func (w *StatsWidget) calculateCPURaw() float64 {
	if w.proc == nil {
		return 0
	}

	// Process CPU time in seconds
	times, err := w.proc.Times()
	if err != nil {
		return 0
	}

	currentCPUTime := times.User + times.System
	currentWallTime := time.Now()

	// Calculate CPU percentage
	if w.lastCPUTime > 0 {
		cpuDelta := currentCPUTime - w.lastCPUTime
		wallDelta := currentWallTime.Sub(w.lastWallTime).Seconds()

		if wallDelta > 0 {
			cpuPercent := (cpuDelta / wallDelta) * 100

			w.lastCPUTime = currentCPUTime
			w.lastWallTime = currentWallTime

			return cpuPercent
		}
	}

	w.lastCPUTime = currentCPUTime
	w.lastWallTime = currentWallTime
	return 0
}

// Toggle toggles visibility
func (w *StatsWidget) Toggle() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.visible = !w.visible
}

// IsVisible returns visibility
func (w *StatsWidget) IsVisible() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.visible
}

// SetSize sets widget dimensions
func (w *StatsWidget) SetSize(width, height int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.width = width
	w.height = height
}

// GetStats returns current stats
func (w *StatsWidget) GetStats() ProcessStats {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stats
}

// Render renders the stats widget
func (w *StatsWidget) Render() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.visible {
		return ""
	}

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#38bdf8")). // Sky blue
		Bold(true)

	iconStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a78bfa")) // Purple

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ca3af")) // Gray

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f0fdf4")). // Light
		Bold(true)

	timeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#fbbf24")). // Amber
		Bold(true)

	// Current time
	now := time.Now()
	timeStr := now.Format("15:04:05")

	// Memory bar (0-100MB scale, adjust as needed)
	memPercent := w.stats.MemoryMB / 100 * 100
	if memPercent > 100 {
		memPercent = 100
	}
	memBar := w.renderBar(memPercent, 8, lipgloss.Color("#22c55e")) // Green

	// CPU bar
	cpuPercent := w.stats.CPUPercent
	if cpuPercent > 100 {
		cpuPercent = 100
	}
	cpuBar := w.renderBar(cpuPercent, 8, lipgloss.Color("#f97316")) // Orange

	var sb strings.Builder

	// Header with time
	header := headerStyle.Render(iconClock + " " + timeStr)
	sb.WriteString(header)
	sb.WriteString("\n")

	// Separator
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#374151")).Render("────────────"))
	sb.WriteString("\n")

	// Memory line
	memIcon := iconStyle.Render(iconMemory)
	memLabel := labelStyle.Render(" RAM ")
	memValue := valueStyle.Render(fmt.Sprintf("%5.1fMB", w.stats.MemoryMB))
	sb.WriteString(memIcon + memLabel + memBar + " " + memValue)
	sb.WriteString("\n")

	// CPU line (normalized % / raw %)
	cpuIcon := iconStyle.Render(iconCPU)
	cpuLabel := labelStyle.Render(" CPU ")
	// Show both: normalized (for bar) and raw (actual cores usage)
	cpuValue := valueStyle.Render(fmt.Sprintf("%4.1f%%", w.stats.CPUPercent))
	cpuRaw := labelStyle.Render(fmt.Sprintf("/%3.0f%%", w.stats.CPURawPercent))
	cpuHint := labelStyle.Render(" t/c")
	sb.WriteString(cpuIcon + cpuLabel + cpuBar + " " + cpuValue + cpuRaw + cpuHint)
	sb.WriteString("\n")

	// Goroutines (small detail)
	goLabel := labelStyle.Render("  ∴ ")
	goValue := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render(fmt.Sprintf("%d goroutines", w.stats.Goroutines))
	sb.WriteString(goLabel + goValue)
	sb.WriteString("\n")

	// Uptime
	uptimeLabel := labelStyle.Render("  ↑ ")
	uptimeValue := timeStyle.Render(w.formatUptime(w.stats.Uptime))
	sb.WriteString(uptimeLabel + uptimeValue)

	// Box style - no fixed background to work with any terminal color
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#38bdf8")). // Sky blue border
		Padding(0, 1)

	return boxStyle.Render(sb.String())
}

// renderBar renders a progress bar
func (w *StatsWidget) renderBar(percent float64, width int, color lipgloss.Color) string {
	filled := int(percent * float64(width) / 100)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	filledStyle := lipgloss.NewStyle().Foreground(color)
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))

	return filledStyle.Render(strings.Repeat("▓", filled)) +
		emptyStyle.Render(strings.Repeat("░", width-filled))
}

// formatUptime formats duration as human-readable
func (w *StatsWidget) formatUptime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm%02ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// ============================================================
// Machine Stats Collection (System-wide metrics)
// ============================================================

// MachineStatsCollector collects system-wide statistics
type MachineStatsCollector struct {
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

// NewMachineStatsCollector creates a new machine stats collector
func NewMachineStatsCollector() *MachineStatsCollector {
	return &MachineStatsCollector{}
}

// Collect gathers current machine statistics
func (c *MachineStatsCollector) Collect() MachineStats {
	c.mu.Lock()
	defer c.mu.Unlock()

	stats := MachineStats{}

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
func (c *MachineStatsCollector) collectMemoryInfo(stats *MachineStats) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return
	}

	stats.MemTotalMB = float64(vm.Total) / 1024 / 1024
	stats.MemUsedMB = float64(vm.Used) / 1024 / 1024
	stats.MemPercent = vm.UsedPercent
}

// collectCPUInfo reads system CPU usage.
func (c *MachineStatsCollector) collectCPUInfo(stats *MachineStats) {
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
func (c *MachineStatsCollector) collectDiskInfo(stats *MachineStats) {
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
func (c *MachineStatsCollector) collectLoadAvg(stats *MachineStats) {
	avg, err := load.Avg()
	if err != nil {
		return
	}
	stats.LoadAvg1 = avg.Load1
	stats.LoadAvg5 = avg.Load5
	stats.LoadAvg15 = avg.Load15
}

func (c *MachineStatsCollector) collectHardwareInfo(stats *MachineStats) {
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

func (c *MachineStatsCollector) collectGPUInfo(stats *MachineStats) {
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
	reMemPair := regexp.MustCompile(`(?i)VRAM\s*Total.*?:\s*([0-9.]+)\\s*([A-Za-z]+).*VRAM\s*Used.*?:\s*([0-9.]+)\\s*([A-Za-z]+)`)
	reMemSingle := regexp.MustCompile(`(?i)VRAM\s*(Total|Used).*?:\s*([0-9.]+)\\s*([A-Za-z]+)`)
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
	re := regexp.MustCompile(`(?i)([0-9.]+)\\s*(GB|MB|KB|B)`)
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
