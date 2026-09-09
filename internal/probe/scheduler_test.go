package probe

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeRecorder struct {
	mu      sync.Mutex
	clients []string
	tasks   map[string][]Task
	counts  map[string]int
}

func (f *fakeRecorder) InsertProbe(client string, _ int, _ string, _ float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counts == nil {
		f.counts = map[string]int{}
	}
	f.counts[client]++
	return nil
}

func (f *fakeRecorder) ProbeTasks(client string) ([]Task, error) {
	return f.tasks[client], nil
}

func (f *fakeRecorder) ProbeClients() ([]string, error) {
	return f.clients, nil
}

// TestSchedulerPerClientAndInterval 验证调度器覆盖所有 client 且遵守任务间隔。
func TestSchedulerPerClientAndInterval(t *testing.T) {
	old := schedulerTick
	schedulerTick = 5 * time.Millisecond
	defer func() { schedulerTick = old }()

	f := &fakeRecorder{
		clients: []string{"n1", "n2"},
		tasks: map[string][]Task{
			"n1": {{ID: 1, Type: "tcp", Target: "127.0.0.1:1", Interval: 50 * time.Millisecond, Enabled: true}},
			"n2": {{ID: 2, Type: "tcp", Target: "127.0.0.1:1", Interval: 50 * time.Millisecond, Enabled: true}},
		},
	}
	s := NewScheduler(f)
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(300 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)

	f.mu.Lock()
	defer f.mu.Unlock()
	for _, client := range []string{"n1", "n2"} {
		if f.counts[client] < 3 || f.counts[client] > 8 {
			t.Errorf("client %s probe count = %d, want 3..8 (interval honored)", client, f.counts[client])
		}
	}
}

// TestSchedulerSkipsDisabled 验证禁用任务不会被执行。
func TestSchedulerSkipsDisabled(t *testing.T) {
	old := schedulerTick
	schedulerTick = 5 * time.Millisecond
	defer func() { schedulerTick = old }()

	f := &fakeRecorder{
		clients: []string{"n1"},
		tasks: map[string][]Task{
			"n1": {{ID: 1, Type: "tcp", Target: "127.0.0.1:1", Interval: 10 * time.Millisecond, Enabled: false}},
		},
	}
	s := NewScheduler(f)
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(80 * time.Millisecond)
	cancel()

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counts["n1"] != 0 {
		t.Errorf("disabled task ran %d times", f.counts["n1"])
	}
}
