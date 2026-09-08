package probe

import (
	"context"
	"net"
	"time"
)

// Task 探针任务定义。
type Task struct {
	ID       int    `json:"id"`
	Target   string `json:"target"`   // 主机/IP 或 URL
	Type     string `json:"type"`     // ping | http
	Name     string `json:"name"`     // 运营商标签（上海移动等）
	Interval time.Duration `json:"-"` // 周期
	Enabled  bool   `json:"enabled"`
}

// Record 一次探测结果。
type Record struct {
	Client string  `json:"client"` // 主控伪节点 uuid（探针结果挂该 client）
	TaskID int     `json:"task_id"`
	Time   string  `json:"time"`
	Value  float64 `json:"value"` // 毫秒
}

// Result 单次探测输出。
type Result struct {
	TaskID int     `json:"task_id"`
	Value  float64 `json:"value"`
	OK     bool    `json:"ok"`
}

// TCPPing 通过 TCP 拨号测量往返延迟（毫秒；无 raw socket 依赖，跨平台）。
func TCPPing(ctx context.Context, target string) (float64, error) {
	start := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", target)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	return float64(time.Since(start).Microseconds()) / 1000, nil
}
