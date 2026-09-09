package probe

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"
)

// Recorder 探测结果存储接口（store 实现）。
type Recorder interface {
	InsertProbe(client string, taskID int, timeStr string, value float64) error
	ProbeTasks(client string) ([]Task, error)
	ProbeClients() ([]string, error)
}

// Scheduler 探针调度器（per-client 任务 + 按任务间隔调度 + 并发上限）。
type Scheduler struct {
	store Recorder
	extra []string // 额外固定 client（-probe-client，兼容旧用法）
	once  sync.Once
	sem   chan struct{}
}

// NewScheduler 创建调度器。extra 为额外固定 client（可为空）。
func NewScheduler(store Recorder, extra ...string) *Scheduler {
	var cs []string
	for _, c := range extra {
		if c != "" {
			cs = append(cs, c)
		}
	}
	return &Scheduler{store: store, extra: cs, sem: make(chan struct{}, 32)}
}

// Start 启动调度（非阻塞，阻塞直到 ctx 取消由内部 goroutine 负责）。
func (s *Scheduler) Start(ctx context.Context) {
	s.once.Do(func() { go s.loop(ctx) })
}

// schedulerTick 调度检查间隔（任务实际间隔由 interval_sec 控制；测试可覆盖）。
var schedulerTick = 3 * time.Second

func (s *Scheduler) loop(ctx context.Context) {
	lastRun := map[string]time.Time{}
	tick := time.NewTicker(schedulerTick)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			clients := s.clients()
			for _, client := range clients {
				tasks, err := s.store.ProbeTasks(client)
				if err != nil {
					log.Printf("probe: load tasks for %s: %v", client, err)
					continue
				}
				for _, t := range tasks {
					if !t.Enabled {
						continue
					}
					interval := t.Interval
					if interval <= 0 {
						interval = 60 * time.Second
					}
					key := client + "|" + strconv.Itoa(t.ID)
					if last, ok := lastRun[key]; ok && now.Sub(last) < interval {
						continue
					}
					lastRun[key] = now
					s.schedule(ctx, client, t)
				}
			}
		}
	}
}

// clients 返回需要执行探针的 client 列表（节点 + 节点级任务 + 兼容 extra）。
func (s *Scheduler) clients() []string {
	seen := map[string]bool{}
	var out []string
	add := func(c string) {
		if c != "" && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	if cs, err := s.store.ProbeClients(); err == nil {
		for _, c := range cs {
			add(c)
		}
	} else {
		log.Printf("probe: load clients: %v", err)
	}
	for _, c := range s.extra {
		add(c)
	}
	return out
}

// schedule 在并发上限内异步执行一次探测。
func (s *Scheduler) schedule(ctx context.Context, client string, t Task) {
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return
	}
	go func() {
		defer func() { <-s.sem }()
		s.run(ctx, client, t)
	}()
}

func (s *Scheduler) run(ctx context.Context, client string, t Task) {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res := Run(probeCtx, t.Type, t.Target)
	value := res.Value
	if !res.OK {
		log.Printf("probe %s(%d) client=%s [%s]: %s", t.Name, t.ID, client, res.Type, res.Detail)
		value = -1
	}
	if err := s.store.InsertProbe(client, t.ID, time.Now().Format(time.RFC3339), value); err != nil {
		log.Printf("probe insert: %v", err)
	}
}
