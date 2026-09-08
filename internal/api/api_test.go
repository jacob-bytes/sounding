package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeStore struct{}

func (fakeStore) Nodes() (map[string]Client, error) {
	return map[string]Client{"n1": {UUID: "n1", Name: "test"}}, nil
}
func (fakeStore) LatestStatus() (map[string]NodeStatus, error) {
	return map[string]NodeStatus{"n1": {Client: "n1", Online: true, CPU: 10}}, nil
}
func (fakeStore) RecentStatus(_ string, _ int) (map[string]any, error) {
	return map[string]any{"count": 0, "records": []StatusRecord{}}, nil
}
func (fakeStore) PingRecords(_ string, _ int, _ int) ([]PingRecord, error) { return nil, nil }

func TestRPCDispatch(t *testing.T) {
	h := NewHandler(fakeStore{})
	cases := []struct {
		method string
		want   string
	}{
		{"getNodes", `"n1"`},
		{"common:getNodes", `"n1"`},          // 命名空间前缀
		{"rpc.ping", `"pong"`},               // 健康检查
		{"getNodesLatestStatus", `"online"`}, // 契约字段
	}
	for _, c := range cases {
		body := `{"jsonrpc":"2.0","id":1,"method":"` + c.method + `","params":{}}`
		req := httptest.NewRequest(http.MethodPost, "/rpc2", strings.NewReader(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if !strings.Contains(w.Body.String(), c.want) {
			t.Errorf("%s: want %s in %s", c.method, c.want, w.Body.String())
		}
	}
}

func TestRPCUnknownMethod(t *testing.T) {
	h := NewHandler(fakeStore{})
	req := httptest.NewRequest(http.MethodPost, "/rpc2", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"nope"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var resp RPCResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("want -32601, got %+v", resp.Error)
	}
}

func TestHealthEndpoint(t *testing.T) {
	h := &HealthEndpoint{Ready: func() error { return nil }}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "ok") {
		t.Fatalf("health: %d %s", w.Code, w.Body.String())
	}
}
