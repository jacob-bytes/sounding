package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jacob-bytes/sounding/internal/alert"
)

// TestPingSummaryLossAndStats 验证丢包率与统计（不再被 -1 污染）。
func TestPingSummaryLossAndStats(t *testing.T) {
	s := newTestStore(t)
	if err := s.UpsertProbeTask("*", "127.0.0.1", "本地", "icmp", 60, true); err != nil {
		t.Fatal(err)
	}
	tasks, err := s.AdminProbeTasks()
	if err != nil || len(tasks) != 1 {
		t.Fatalf("tasks: %v %v", tasks, err)
	}
	id := tasks[0].ID
	base := time.Now().Add(-time.Minute)
	values := []float64{10, 20, -1, 30} // 1 次丢包 / 4 次 = 25%
	for i, v := range values {
		ts := base.Add(time.Duration(i) * time.Second).Format(time.RFC3339)
		if err := s.InsertProbe("n1", id, ts, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.UpsertNode("n1", "n1", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertStatus("n1", statusStub()); err != nil {
		t.Fatal(err)
	}
	st, err := s.LatestStatus()
	if err != nil {
		t.Fatal(err)
	}
	p, ok := st["n1"].Ping[fmt.Sprint(id)]
	if !ok {
		t.Fatalf("missing ping summary: %+v", st["n1"].Ping)
	}
	if p.Loss != 25 {
		t.Errorf("loss = %v, want 25", p.Loss)
	}
	if p.Latest != 30 {
		t.Errorf("latest = %v, want 30", p.Latest)
	}
	if p.Min != 10 || p.Max != 30 {
		t.Errorf("min/max = %v/%v, want 10/30", p.Min, p.Max)
	}
	if p.Avg != 20 {
		t.Errorf("avg = %v, want 20 (must exclude -1)", p.Avg)
	}
}

// TestPingSummaryAllLoss 全丢包时 loss=100、latest=-1。
func TestPingSummaryAllLoss(t *testing.T) {
	s := newTestStore(t)
	_ = s.UpsertProbeTask("*", "127.0.0.1", "本地", "icmp", 60, true)
	tasks, _ := s.AdminProbeTasks()
	id := tasks[0].ID
	for i := 0; i < 3; i++ {
		_ = s.InsertProbe("n1", id, time.Now().Add(time.Duration(i)*time.Second).Format(time.RFC3339), -1)
	}
	_ = s.UpsertNode("n1", "n1", "")
	_ = s.InsertStatus("n1", statusStub())
	st, _ := s.LatestStatus()
	p := st["n1"].Ping[fmt.Sprint(id)]
	if p.Loss != 100 || p.Latest != -1 {
		t.Errorf("want loss=100 latest=-1, got %+v", p)
	}
}

// TestUpsertNodePersistsOSInfo 验证 Agent 上报的硬件/系统信息会落库，
// 且后续空 info 上报不会清掉已有信息。
func TestUpsertNodePersistsOSInfo(t *testing.T) {
	s := newTestStore(t)
	info := `{"cpu_name":"AMD EPYC 7B13","arch":"amd64","os":"Ubuntu 22.04","kernel_version":"6.1.0","virtualization":"kvm","cpu_cores":8,"mem_total":34359738368,"swap_total":2147483648,"disk_total":107374182400}`
	if err := s.UpsertNode("n1", "东京节点", info); err != nil {
		t.Fatal(err)
	}
	nodes, err := s.Nodes()
	if err != nil {
		t.Fatal(err)
	}
	n := nodes["n1"]
	if n.CPUName != "AMD EPYC 7B13" || n.Arch != "amd64" || n.OS != "Ubuntu 22.04" {
		t.Errorf("os info not persisted: %+v", n)
	}
	if n.CPUCores != 8 || n.MemTotal == 0 || n.SwapTotal == 0 || n.DiskTotal == 0 {
		t.Errorf("hardware totals not persisted: %+v", n)
	}
	if n.KernelVersion != "6.1.0" || n.Virtualization != "kvm" {
		t.Errorf("kernel/virtualization not persisted: %+v", n)
	}
	// 空 info 再次上报：硬件信息保留，name 更新
	if err := s.UpsertNode("n1", "东京节点-2", ""); err != nil {
		t.Fatal(err)
	}
	nodes, _ = s.Nodes()
	n = nodes["n1"]
	if n.Name != "东京节点-2" {
		t.Errorf("name not updated: %q", n.Name)
	}
	if n.CPUName != "AMD EPYC 7B13" || n.MemTotal == 0 {
		t.Errorf("hardware info wiped by empty report: %+v", n)
	}
}

// TestUpsertNodeCamelCaseInfo 兼容旧版 Agent 的 CamelCase 键名。
func TestUpsertNodeCamelCaseInfo(t *testing.T) {
	s := newTestStore(t)
	info := `{"CPUName":"Intel Xeon","Arch":"x86_64","OS":"Debian","CPUCores":4,"MemTotal":1024}`
	if err := s.UpsertNode("n1", "n1", info); err != nil {
		t.Fatal(err)
	}
	nodes, _ := s.Nodes()
	n := nodes["n1"]
	if n.CPUName != "Intel Xeon" || n.Arch != "x86_64" || n.CPUCores != 4 || n.MemTotal != 1024 {
		t.Errorf("camelCase info not parsed: %+v", n)
	}
}

// TestAlertRulePersistence 验证告警规则可持久化/读取/删除。
func TestAlertRulePersistence(t *testing.T) {
	s := newTestStore(t)
	r := alert.Rule{Kind: "latency", Node: "*", Threshold: 250, MuteWindows: []string{"02:00-04:00"}}
	if err := s.SaveAlertRule(r); err != nil {
		t.Fatal(err)
	}
	rules, err := s.LoadAlertRules()
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].Threshold != 250 || len(rules[0].MuteWindows) != 1 {
		t.Fatalf("unexpected rules: %+v", rules)
	}
	if err := s.DeleteAlertRule("latency", "*"); err != nil {
		t.Fatal(err)
	}
	rules, _ = s.LoadAlertRules()
	if len(rules) != 0 {
		t.Fatalf("rule not deleted: %+v", rules)
	}
}

// TestProbeClients 验证调度器能枚举到节点与节点级任务。
func TestProbeClients(t *testing.T) {
	s := newTestStore(t)
	_ = s.UpsertProbeTask("*", "1.1.1.1", "global", "icmp", 60, true)
	_ = s.UpsertProbeTask("n2", "8.8.8.8", "node", "icmp", 60, true)
	_ = s.UpsertNode("n1", "n1", "")
	clients, err := s.ProbeClients()
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, c := range clients {
		set[c] = true
	}
	if !set["n1"] || !set["n2"] {
		t.Fatalf("want n1+n2, got %v", clients)
	}
	if set["*"] {
		t.Fatalf("'*' should not be a probe client: %v", clients)
	}
}

// TestRecentStatusTempLoad 验证历史记录包含温度/负载字段。
func TestRecentStatusTempLoad(t *testing.T) {
	s := newTestStore(t)
	_ = s.UpsertNode("n1", "n1", "")
	_ = s.InsertStatus("n1", statusStub())
	rec, err := s.RecentStatus("n1", 10)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(rec["records"])
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"temp", "load5", "load15"} {
		if !strings.Contains(string(raw), `"`+k+`"`) {
			t.Errorf("missing %s in recent status: %s", k, raw)
		}
	}
}

// TestAgentUpsertKeepsInterval 验证 Agent 回传任务不会覆盖管理页配置的间隔。
func TestAgentUpsertKeepsInterval(t *testing.T) {
	s := newTestStore(t)
	if err := s.AdminUpsertProbeInterval("n1", "1.1.1.1", "x", "icmp", 5, true); err != nil {
		t.Fatal(err)
	}
	// 模拟 Agent 远程配置回传（默认 60s）
	if err := s.UpsertProbeTask("n1", "1.1.1.1", "x", "icmp", 60, true); err != nil {
		t.Fatal(err)
	}
	tasks, err := s.ProbeTasks("n1")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(tasks))
	}
	if tasks[0].Interval != 5*time.Second {
		t.Errorf("interval = %v, want 5s (agent upsert must not clobber)", tasks[0].Interval)
	}
}

// TestSecretsPersist 验证自动生成的 token 持久化，重启后不变。
func TestSecretsPersist(t *testing.T) {
	s := newTestStore(t)
	v1, created, err := s.GetOrCreateSecret("agent_token")
	if err != nil {
		t.Fatal(err)
	}
	if !created || len(v1) < 32 {
		t.Fatalf("first call should create a strong secret, got %q created=%v", v1, created)
	}
	v2, created2, err := s.GetOrCreateSecret("agent_token")
	if err != nil {
		t.Fatal(err)
	}
	if created2 || v2 != v1 {
		t.Fatalf("secret should persist: %q vs %q created=%v", v1, v2, created2)
	}
}
