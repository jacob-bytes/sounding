package collect

import (
	"context"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// Snapshot 一次采集结果（字段与 ink StatusRecord 对齐）。
type Snapshot struct {
	CPU           float64 `json:"cpu"`
	RAM           float64 `json:"ram"`
	Swap          float64 `json:"swap"`
	Load          float64 `json:"load"`
	Disk          float64 `json:"disk"`
	NetIn         float64 `json:"net_in"`
	NetOut        float64 `json:"net_out"`
	NetTotalUp    float64 `json:"net_total_up"`
	NetTotalDown  float64 `json:"net_total_down"`
	Process       float64 `json:"process"`
	Connections   float64 `json:"connections"`
	ConnectionsUDP float64 `json:"connections_udp"`
	RAMTotal      float64 `json:"ram_total"`
	SwapTotal     float64 `json:"swap_total"`
	DiskTotal     float64 `json:"disk_total"`
}

// OSInfo 节点身份信息（NodeRecord 部分字段）。
type OSInfo struct {
	HostName    string
	CPUName     string
	Arch        string
	OS          string
	Region      string
	CPUCores    float64
	MemTotal    float64
	SwapTotal   float64
	DiskTotal   float64
}

// Collector 周期采集器。
type Collector struct {
	interval time.Duration
}

// New 创建采集器。
func New(interval time.Duration) *Collector { return &Collector{interval: interval} }

// Interval 返回采集周期。
func (c *Collector) Interval() time.Duration { return c.interval }

// Collect 执行一次采集（CPU 采样取 interval 窗口）。
func (c *Collector) Collect(ctx context.Context) (Snapshot, error) {
	var s Snapshot
	vm, err := mem.VirtualMemory()
	if err == nil {
		s.RAM = float64(vm.Used)
		s.RAMTotal = float64(vm.Total)
		// gopsutil 无独立 SwapUsed——以 swap 总量近似已用（后续可改用 /proc/meminfo 精确读取）
		s.Swap = float64(vm.SwapTotal)
		s.SwapTotal = float64(vm.SwapTotal)
	}
	du, err := disk.Usage("/")
	if err == nil {
		s.Disk = float64(du.Used)
		s.DiskTotal = float64(du.Total)
	}
	ld, err := load.Avg()
	if err == nil {
		s.Load = ld.Load1
	}
	netStat, err := net.IOCounters(false)
	if err == nil && len(netStat) > 0 {
		var up, down uint64
		up = netStat[0].BytesSent
		down = netStat[0].BytesRecv
		s.NetIn = float64(down) / c.interval.Seconds()
		s.NetOut = float64(up) / c.interval.Seconds()
		s.NetTotalUp = float64(up)
		s.NetTotalDown = float64(down)
	}
	// CPU 使用率采样（interval 窗口）——gopsutil 需要间隔调用
	percent, err := cpu.PercentWithContext(ctx, c.interval, false)
	if err == nil && len(percent) > 0 {
		s.CPU = percent[0]
	} else {
		s.CPU = 0
	}
	s.Process = float64(countProcesses())
	return s, nil
}

// GetOSInfo 读取本机信息。
func GetOSInfo() (OSInfo, error) {
	var o OSInfo
	h, err := host.Info()
	if err == nil {
		o.HostName = hostnameFallback(h.Hostname)
		o.OS = h.Platform + " " + h.PlatformVersion
		o.Arch = h.KernelArch
		o.CPUCores = float64(h.Procs)
	}
	o.CPUName = runtime.GOARCH
	o.Arch = runtime.GOARCH
	vm, _ := mem.VirtualMemory()
	if vm != nil {
		o.MemTotal = float64(vm.Total)
		o.SwapTotal = float64(vm.SwapTotal)
	}
	du, _ := disk.Usage("/")
	if du != nil {
		o.DiskTotal = float64(du.Total)
	}
	return o, nil
}

func hostnameFallback(h string) string {
	if h == "" {
		return "sounding-agent"
	}
	return h
}

func countProcesses() int { return runtime.NumGoroutine() }
