package store

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close(); _ = os.RemoveAll(dir) })
	return s
}

func TestSeedAndNodes(t *testing.T) {
	s := newTestStore(t)
	if err := s.Seed(); err != nil {
		t.Fatalf("seed: %v", err)
	}
	nodes, err := s.Nodes()
	if err != nil {
		t.Fatalf("nodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(nodes))
	}
	if _, ok := nodes["demo-001"]; !ok {
		t.Fatal("demo-001 missing")
	}
}

func TestLatestStatusOnline(t *testing.T) {
	s := newTestStore(t)
	if err := s.Seed(); err != nil {
		t.Fatal(err)
	}
	st, err := s.LatestStatus()
	if err != nil {
		t.Fatal(err)
	}
	if len(st) == 0 {
		t.Fatal("no status")
	}
	for _, v := range st {
		if !v.Online {
			t.Errorf("seed node %s should be online", v.Client)
		}
	}
}

func TestInsertStatusAndRecent(t *testing.T) {
	s := newTestStore(t)
	if err := s.UpsertNode("n1", "test", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertStatus("n1", statusStub()); err != nil {
		t.Fatal(err)
	}
	rec, err := s.RecentStatus("n1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if rec["count"].(int) != 1 {
		t.Fatalf("want count=1, got %v", rec["count"])
	}
}

func TestProbeTaskCRUD(t *testing.T) {
	s := newTestStore(t)
	if err := s.AdminUpsertProbe("*", "223.5.5.5", "阿里", "icmp", true); err != nil {
		t.Fatal(err)
	}
	tasks, err := s.AdminProbeTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Type != "icmp" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
	if err := s.InsertProbe("n1", tasks[0].ID, "2026-01-01T00:00:00Z", 12.5); err != nil {
		t.Fatal(err)
	}
	recs, err := s.PingRecords("n1", tasks[0].ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Value != 12.5 {
		t.Fatalf("unexpected records: %+v", recs)
	}
	if err := s.AdminDeleteProbe("*", "阿里"); err != nil {
		t.Fatal(err)
	}
	tasks, _ = s.AdminProbeTasks()
	if len(tasks) != 0 {
		t.Fatal("probe not deleted")
	}
}

func TestPurge(t *testing.T) {
	s := newTestStore(t)
	if err := s.Seed(); err != nil {
		t.Fatal(err)
	}
	n, err := s.Purge(0) // 保留 30 天——种子数据为近期，应保留
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("recent data should not be purged, got %d", n)
	}
}

func TestNodeDelete(t *testing.T) {
	s := newTestStore(t)
	if err := s.Seed(); err != nil {
		t.Fatal(err)
	}
	if err := s.AdminDeleteNode("demo-001"); err != nil {
		t.Fatal(err)
	}
	nodes, _ := s.Nodes()
	if _, ok := nodes["demo-001"]; ok {
		t.Fatal("node not deleted")
	}
}
