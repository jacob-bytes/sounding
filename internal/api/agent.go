package api

import (
	"encoding/json"
	"net/http"
)

// AgentPayload Agent 上报体。
type AgentPayload struct {
	UUID    string  `json:"uuid"`
	Name    string  `json:"name"`
	Time    string  `json:"time"`
	OSInfo  json.RawMessage `json:"os_info"`
	Status  AgentStatus `json:"status"`
	Probes  []ProbeTarget `json:"probes"`
}

// ProbeTarget Agent 上报的探针目标。
type ProbeTarget struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Type string `json:"type,omitempty"`
}

// AgentStatus 上报状态（字段与 collect.Snapshot 对齐）。
type AgentStatus struct {
	CPU           float64 `json:"cpu"`
	RAM           float64 `json:"ram"`
	Swap          float64 `json:"swap"`
	Load          float64 `json:"load"`
	Disk          float64 `json:"disk"`
	NetIn         float64 `json:"net_in"`
	NetOut        float64 `json:"net_out"`
	NetTotalUp    float64 `json:"net_total_up"`
	NetTotalDown  float64 `json:"net_total_down"`
	Process       float64 `json:"process"`
	Connections   float64 `json:"connections"`
	ConnectionsUDP float64 `json:"connections_udp"`
	Temp          float64 `json:"temp"`
	Load5         float64 `json:"load5"`
	Load15        float64 `json:"load15"`
	RAMTotal      float64 `json:"ram_total"`
	SwapTotal     float64 `json:"swap_total"`
	DiskTotal     float64 `json:"disk_total"`
}

// AgentEndpoint 处理 /agent/status（token 校验 + 入库）。
type AgentEndpoint struct {
	store interface {
		UpsertNode(uuid, name string, info string) error
		InsertStatus(uuid string, st AgentStatusForStore) error
		UpsertProbeTask(client, target, name, typ string, intervalSec float64, enabled bool) error
	}
	token string
}

// AgentStatusForStore 存储侧状态（store 包实现）。
type AgentStatusForStore = AgentStatus

// NewAgentEndpoint 构建上报端点。
func NewAgentEndpoint(s interface {
	UpsertNode(uuid, name string, info string) error
	InsertStatus(uuid string, st AgentStatusForStore) error
	UpsertProbeTask(client, target, name, typ string, intervalSec float64, enabled bool) error
}, token string) *AgentEndpoint {
	return &AgentEndpoint{store: s, token: token}
}

// ServeHTTP POST /agent/status。
func (h *AgentEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.token != "" && r.Header.Get("X-Auth-Token") != h.token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var payload AgentPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.store.UpsertNode(payload.UUID, payload.Name, string(payload.OSInfo)); err != nil {
		http.Error(w, "upsert: "+err.Error(), http.StatusInternalServerError)
		return
	}
	st := AgentStatusForStore(payload.Status)
	if err := h.store.InsertStatus(payload.UUID, st); err != nil {
		http.Error(w, "insert: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// per-node 探针目标（自动 upsert）
	for _, pt := range payload.Probes {
		if pt.Name == "" || pt.Host == "" {
			continue
		}
		typ := pt.Type
		if typ == "" {
			typ = "icmp"
		}
		_ = h.store.UpsertProbeTask(payload.UUID, pt.Host, pt.Name, typ, 60, true)
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}
