package collect

import (
	"context"
	"testing"
	"time"
)

// TestCollectBasic 验证采集返回真实进程数、uptime 与总量，且 swap 已用不超过总量。
func TestCollectBasic(t *testing.T) {
	c := New(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := c.Collect(ctx)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if s.RAMTotal <= 0 || s.DiskTotal <= 0 {
		t.Errorf("totals should be > 0: ram=%v disk=%v", s.RAMTotal, s.DiskTotal)
	}
	if s.Process <= 0 {
		t.Errorf("process count should be > 0, got %v", s.Process)
	}
	if s.Uptime <= 0 {
		t.Errorf("uptime should be > 0, got %v", s.Uptime)
	}
	// 修复前 swap 恒等于总量（100%）；现在应为已用值
	if s.SwapTotal > 0 && s.Swap > s.SwapTotal {
		t.Errorf("swap used %v > total %v", s.Swap, s.SwapTotal)
	}
}

// TestCollectNetRateNonNegative 验证网络速率不会出现负值。
func TestCollectNetRateNonNegative(t *testing.T) {
	c := New(50 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.Collect(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	s, err := c.Collect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.NetIn < 0 || s.NetOut < 0 {
		t.Errorf("negative net rate: in=%v out=%v", s.NetIn, s.NetOut)
	}
}

// TestGetOSInfo 验证 OS 信息包含真实 CPU 型号与架构。
func TestGetOSInfo(t *testing.T) {
	o, err := GetOSInfo()
	if err != nil {
		t.Fatalf("osinfo: %v", err)
	}
	if o.CPUName == "" || o.Arch == "" {
		t.Errorf("cpu_name/arch empty: %+v", o)
	}
	if o.MemTotal <= 0 {
		t.Errorf("mem_total should be > 0: %+v", o)
	}
	if o.CPUCores <= 0 || o.CPUCores > 4096 {
		t.Errorf("cpu_cores looks wrong: %v", o.CPUCores)
	}
}
