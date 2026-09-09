package collect

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/shirou/gopsutil/v4/sensors"
)

// Snapshot 一次采集结果（字段与 ink StatusRecord 对齐）。
type Snapshot struct {
	CPU            float64 `json:"cpu"`
	RAM            float64 `json:"ram"`
	Swap           float64 `json:"swap"`
	Load           float64 `json:"load"`
	Disk           float64 `json:"disk"`
	NetIn          float64 `json:"net_in"`
	NetOut         float64 `json:"net_out"`
	NetTotalUp     float64 `json:"net_total_up"`
	NetTotalDown   float64 `json:"net_total_down"`
	Process        float64 `json:"process"`
	Connections    float64 `json:"connections"`
	ConnectionsUDP float64 `json:"connections_udp"`
	Temp           float64 `json:"temp"`
	Load5          float64 `json:"load5"`
	Load15         float64 `json:"load15"`
	RAMTotal       float64 `json:"ram_total"`
	SwapTotal      float64 `json:"swap_total"`
	DiskTotal      float64 `json:"disk_total"`
	Uptime         float64 `json:"uptime"`
}

// OSInfo 节点身份信息（NodeRecord 部分字段）。
type OSInfo struct {
	HostName       string  `json:"host_name"`
	CPUName        string  `json:"cpu_name"`
	Arch           string  `json:"arch"`
	OS             string  `json:"os"`
	KernelVersion  string  `json:"kernel_version"`
	Virtualization string  `json:"virtualization"`
	Region         string  `json:"region"`
	CPUCores       float64 `json:"cpu_cores"`
	MemTotal       float64 `json:"mem_total"`
	SwapTotal      float64 `json:"swap_total"`
	DiskTotal      float64 `json:"disk_total"`
}

// Collector 周期采集器。
type Collector struct {
	interval time.Duration

	mu          sync.Mutex
	lastNetUp   uint64
	lastNetDown uint64
	lastNetTime time.Time
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
	}
	// Swap 使用真实已用值（SwapTotal-SwapFree），而非总量
	if sw, err := mem.SwapMemory(); err == nil {
		s.Swap = float64(sw.Used)
		s.SwapTotal = float64(sw.Total)
	}
	du, err := disk.Usage("/")
	if err == nil {
		s.Disk = float64(du.Used)
		s.DiskTotal = float64(du.Total)
	}
	ld, err := load.Avg()
	if err == nil {
		s.Load = ld.Load1
		s.Load5 = ld.Load5
		s.Load15 = ld.Load15
	}
	// 温度（部分平台无传感器——静默忽略）
	if temps, err := sensors.SensorsTemperatures(); err == nil {
		for _, t := range temps {
			if t.Temperature > s.Temp {
				s.Temp = t.Temperature
			}
		}
	}
	// 网络速率 = 两次采样差值 / 实际间隔（首次采样无基线时为 0）
	now := time.Now()
	if netStat, err := net.IOCounters(false); err == nil && len(netStat) > 0 {
		up := netStat[0].BytesSent
		down := netStat[0].BytesRecv
		c.mu.Lock()
		if !c.lastNetTime.IsZero() {
			elapsed := now.Sub(c.lastNetTime).Seconds()
			// 计数器回绕/网卡重置时跳过本次速率计算
			if elapsed > 0 && up >= c.lastNetUp && down >= c.lastNetDown {
				s.NetOut = float64(up-c.lastNetUp) / elapsed
				s.NetIn = float64(down-c.lastNetDown) / elapsed
			}
		}
		c.lastNetUp, c.lastNetDown, c.lastNetTime = up, down, now
		c.mu.Unlock()
		s.NetTotalUp = float64(up)
		s.NetTotalDown = float64(down)
	}
	// 进程数（真实进程数，而非 goroutine 数）
	if pids, err := process.Pids(); err == nil {
		s.Process = float64(len(pids))
	}
	// 系统运行时长（秒）
	if up, err := host.Uptime(); err == nil {
		s.Uptime = float64(up)
	}
	// CPU 使用率采样（interval 窗口）——gopsutil 需要间隔调用
	percent, err := cpu.PercentWithContext(ctx, c.interval, false)
	if err == nil && len(percent) > 0 {
		s.CPU = percent[0]
	} else {
		s.CPU = 0
	}
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
		o.KernelVersion = h.KernelVersion
		o.Virtualization = h.VirtualizationSystem
		if h.VirtualizationRole != "" {
			o.Virtualization = h.VirtualizationSystem + "/" + h.VirtualizationRole
		}
	}
	if o.Arch == "" {
		o.Arch = runtime.GOARCH
	}
	// CPU 核心数优先用逻辑核数（host.Procs 在部分平台返回的是进程数）
	if counts, err := cpu.Counts(true); err == nil && counts > 0 {
		o.CPUCores = float64(counts)
	} else if h != nil {
		o.CPUCores = float64(h.Procs)
	}
	if infos, err := cpu.Info(); err == nil && len(infos) > 0 && infos[0].ModelName != "" {
		o.CPUName = infos[0].ModelName
	}
	if o.CPUName == "" {
		o.CPUName = runtime.GOARCH
	}
	if vm, err := mem.VirtualMemory(); err == nil {
		o.MemTotal = float64(vm.Total)
	}
	if sw, err := mem.SwapMemory(); err == nil {
		o.SwapTotal = float64(sw.Total)
	}
	if du, err := disk.Usage("/"); err == nil {
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
