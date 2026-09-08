package alert

import (
	"context"
	"time"
)

// Snapshot 供 watcher 判定的节点状态。
type Snapshot struct {
	UUID     string
	Name     string
	Online   bool
	LastSeen time.Time
	Ping     map[string]float64 // task name -> latest ms（-1 = 丢包）
	Loss     map[string]float64 // task name -> loss %
}

// Source 提供节点状态快照。
type Source interface {
	AlertSnapshots() ([]Snapshot, error)
}

// Watch 周期评估告警（离线 + 延迟 + 丢包）。
func Watch(ctx context.Context, src Source, mgr *Manager, every time.Duration) {
	if every <= 0 {
		every = 30 * time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			snaps, err := src.AlertSnapshots()
			if err != nil {
				continue
			}
			for _, s := range snaps {
				if !s.Online {
					mgr.Evaluate(Event{Kind: "offline", Node: s.UUID, NodeName: s.Name, Value: 1, Message: "节点离线"})
				}
				for name, ms := range s.Ping {
					if ms < 0 {
						mgr.Evaluate(Event{Kind: "loss", Node: s.UUID, NodeName: s.Name, Value: 100, Message: "探测丢包: " + name})
						continue
					}
					mgr.Evaluate(Event{Kind: "latency", Node: s.UUID, NodeName: s.Name, Value: ms, Message: "延迟偏高: " + name})
				}
			}
		}
	}
}
