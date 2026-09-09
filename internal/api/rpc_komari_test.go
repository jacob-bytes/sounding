package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeKomariStore 实现 Handler 基础接口 + komariStore 扩展接口。
type fakeKomariStore struct{}

func (fakeKomariStore) Nodes() (map[string]Client, error) {
	return map[string]Client{"n1": {UUID: "n1", Name: "test"}}, nil
}

func (fakeKomariStore) LatestStatus() (map[string]NodeStatus, error) {
	return map[string]NodeStatus{"n1": {Client: "n1", Online: true, CPU: 10}}, nil
}

func (fakeKomariStore) RecentStatus(_ string, _ int) (map[string]any, error) {
	return map[string]any{"count": 1, "records": []StatusRecord{{Client: "n1", Time: "2026-01-01T00:00:00Z", CPU: 10}}}, nil
}

func (fakeKomariStore) PingRecords(_ string, _ int, _ int) ([]PingRecord, error) {
	return []PingRecord{{Client: "n1", TaskID: 1, Time: "2026-01-01T00:00:00Z", Value: 12}}, nil
}

func (fakeKomariStore) RecordsSince(_ string, _, _ int) ([]StatusRecord, error) {
	return []StatusRecord{{Client: "n1", Time: "2026-01-01T00:00:00Z", CPU: 10, RAM: 1, RAMTotal: 2}}, nil
}

func (fakeKomariStore) ProbeRecordsSince(_ string, _, _, _ int) ([]PingRecord, error) {
	return []PingRecord{
		{Client: "n1", TaskID: 1, Time: "2026-01-01T00:00:00Z", Value: 10},
		{Client: "n1", TaskID: 1, Time: "2026-01-01T00:01:00Z", Value: -1},
		{Client: "n1", TaskID: 1, Time: "2026-01-01T00:02:00Z", Value: 20},
	}, nil
}

func (fakeKomariStore) AdminProbeTasks() ([]AdminProbe, error) {
	return []AdminProbe{{ID: 1, Client: "n1", Target: "1.1.1.1", Name: "阿里", Type: "icmp", IntervalSec: 60, Enabled: true}}, nil
}

func callRPC(t *testing.T, h *Handler, method, params string) map[string]any {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"` + method + `","params":` + params + `}`
	req := httptest.NewRequest(http.MethodPost, "/rpc2", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode %s: %v (%s)", method, err, w.Body.String())
	}
	if e, ok := resp["error"]; ok && e != nil {
		t.Fatalf("%s error: %v", method, e)
	}
	res, _ := resp["result"].(map[string]any)
	if res == nil {
		// 数组/标量结果：用原始 JSON 返回一个包装
		return map[string]any{"__raw": resp["result"]}
	}
	return res
}

func TestKomariNodeRecentStatusUUIDParam(t *testing.T) {
	h := NewHandler(fakeKomariStore{})
	res := callRPC(t, h, "common:getNodeRecentStatus", `{"uuid":"n1","limit":5}`)
	if res["count"].(float64) != 1 {
		t.Fatalf("want count=1, got %v", res)
	}
}

func TestKomariGetRecordsLoad(t *testing.T) {
	h := NewHandler(fakeKomariStore{})
	res := callRPC(t, h, "common:getRecords", `{"type":"load","uuid":"n1","hours":1,"max_count":10}`)
	records, ok := res["records"].([]any)
	if !ok || len(records) != 1 {
		t.Fatalf("unexpected records: %v", res["records"])
	}
}

func TestKomariGetRecordsPing(t *testing.T) {
	h := NewHandler(fakeKomariStore{})
	res := callRPC(t, h, "common:getRecords", `{"type":"ping","uuid":"n1","hours":1}`)
	if res["count"].(float64) != 3 {
		t.Fatalf("want 3 ping records, got %v", res["count"])
	}
	tasks, ok := res["tasks"].([]any)
	if !ok || len(tasks) != 1 {
		t.Fatalf("unexpected tasks: %v", res["tasks"])
	}
	task := tasks[0].(map[string]any)
	if loss := task["loss"].(float64); loss < 33.3 || loss > 33.4 {
		t.Fatalf("loss = %v, want ~33.3", loss)
	}
}

func TestKomariPublicSettings(t *testing.T) {
	h := NewHandler(fakeKomariStore{})
	res := callRPC(t, h, "public:getPublicSettings", `{}`)
	ts, ok := res["theme_settings"].(map[string]any)
	if !ok {
		t.Fatalf("missing theme_settings: %v", res)
	}
	if ts["rpcTransportMode"] != "websocket" || ts["nodeCardSize"] != "compact" {
		t.Fatalf("unexpected theme settings: %v", ts)
	}
}

func TestKomariVersion(t *testing.T) {
	h := NewHandler(fakeKomariStore{})
	h.SetVersion("v9.9.9")
	res := callRPC(t, h, "rpc.getVersion", `{}`)
	if res["version"] != "v9.9.9" {
		t.Fatalf("version = %v", res["version"])
	}
}

func TestKomariPublicPingTasks(t *testing.T) {
	h := NewHandler(fakeKomariStore{})
	res := callRPC(t, h, "public:getPublicPingTasks", `{}`)
	raw, ok := res["__raw"].([]any)
	if !ok || len(raw) != 1 {
		t.Fatalf("unexpected ping tasks: %v", res)
	}
}

func TestKomariQueryMetrics(t *testing.T) {
	h := NewHandler(fakeKomariStore{})
	res := callRPC(t, h, "public:queryMetrics", `{"metric_keys":["cpu.usage","ping.latency_ms"],"entity_id":"n1","hours":1}`)
	series, ok := res["series"].([]any)
	if !ok || len(series) < 2 {
		t.Fatalf("unexpected series: %v", res["series"])
	}
	first := series[0].(map[string]any)
	if first["metric_key"] != "cpu.usage" {
		t.Fatalf("first metric = %v", first["metric_key"])
	}
}

func TestKomariPingMetricStats(t *testing.T) {
	h := NewHandler(fakeKomariStore{})
	res := callRPC(t, h, "public:getPingMetricStats", `{"entity_id":"n1","hours":1}`)
	stats, ok := res["stats"].([]any)
	if !ok || len(stats) != 1 {
		t.Fatalf("unexpected stats: %v", res["stats"])
	}
	st := stats[0].(map[string]any)
	if st["valid"].(float64) != 2 || st["total"].(float64) != 3 {
		t.Fatalf("valid/total = %v/%v", st["valid"], st["total"])
	}
}
