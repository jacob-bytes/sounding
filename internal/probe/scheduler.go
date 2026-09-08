package probe

import (
	"context"
	"log"
	"net"
	"sync"
	"time"
)

// Recorder 探测结果存储接口（store 实现）。
type Recorder interface {
	InsertProbe(client string, taskID int, timeStr string, value float64) error
	ProbeTasks() ([]Task, error)
}

// Scheduler 探针调度器（goroutine 池 + 定时重跑）。
type Scheduler struct {
	store  Recorder
	client string
	once   sync.Once
}

// NewScheduler 创建调度器。
func NewScheduler(store Recorder, client string) *Scheduler {
	return &Scheduler{store: store, client: client}
}

// Start 启动调度（阻塞直到 ctx 取消）。
func (s *Scheduler) Start(ctx context.Context) {
	s.once.Do(func() { go s.loop(ctx) })
}

func (s *Scheduler) loop(ctx context.Context) {
	for {
		tasks, err := s.store.ProbeTasks()
		if err != nil {
			log.Printf("probe: load tasks: %v", err)
		}
		for _, t := range tasks {
			if !t.Enabled {
				continue
			}
			go s.run(ctx, t)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
	}
}

func (s *Scheduler) run(ctx context.Context, t Task) {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// 目标未带端口时补默认端口（TCP 探测用）
	target := t.Target
	if _, _, err := net.SplitHostPort(target); err != nil {
		target = net.JoinHostPort(target, "443")
	}
	ms, err := TCPPing(probeCtx, target)
	if err != nil {
		log.Printf("probe %s(%d): %v", t.Name, t.ID, err)
		return
	}
	if err := s.store.InsertProbe(s.client, t.ID, time.Now().Format(time.RFC3339), ms); err != nil {
		log.Printf("probe insert: %v", err)
	}
}
