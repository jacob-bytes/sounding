package probe

import (
	"context"
	"log"
	"sync"
	"time"
)

// Recorder 探测结果存储接口（store 实现）。
type Recorder interface {
	InsertProbe(client string, taskID int, timeStr string, value float64) error
	ProbeTasks(client string) ([]Task, error)
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
		tasks, err := s.store.ProbeTasks(s.client)
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
	ms, kind, err := Ping(probeCtx, t.Target)
	if err != nil {
		log.Printf("probe %s(%d) [%s]: %v", t.Name, t.ID, kind, err)
		// 记录丢包（-1 表示超时/丢包——ink 契约）
		_ = s.store.InsertProbe(s.client, t.ID, time.Now().Format(time.RFC3339), -1)
		return
	}
	if err := s.store.InsertProbe(s.client, t.ID, time.Now().Format(time.RFC3339), ms); err != nil {
		log.Printf("probe insert: %v", err)
	}
}
