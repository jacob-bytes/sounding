package api

import (
	"encoding/json"
	"net/http"
)

// AdminEndpoint 管理 API（添加服务器 / 探针配置 / 告警规则——带 admin token）。
type AdminEndpoint struct {
	store interface {
		AdminNodes() (map[string]Client, error)
		AdminUpsertNode(n Client) error
		AdminProbeTasks() ([]AdminProbe, error)
		AdminUpsertProbe(client, target, name, typ string, enabled bool) error
		AdminDeleteProbe(client, name string) error
		AdminProbeTaskNames(client string) ([]string, error)
	}
	statuses func() any
	token    string
	alerts   func() any
	setAlert func(kind, node string, threshold float64, webhook string) error
	delAlert func(kind, node string) error
}

// statusProvider 由 main 注入（返回 map[uuid]NodeStatus）。
func (h *AdminEndpoint) statusProvider() any {
	if h.statuses != nil {
		return h.statuses()
	}
	return map[string]any{}
}

// AdminProbe 管理视角探针任务。
type AdminProbe struct {
	ID      int    `json:"id"`
	Client  string `json:"client"`
	Target  string `json:"target"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

// NewAdminEndpoint 构建管理端点。
func NewAdminEndpoint(s interface {
	AdminNodes() (map[string]Client, error)
	AdminUpsertNode(n Client) error
	AdminProbeTasks() ([]AdminProbe, error)
	AdminUpsertProbe(client, target, name, typ string, enabled bool) error
	AdminDeleteProbe(client, name string) error
	AdminProbeTaskNames(client string) ([]string, error)
}, token string) *AdminEndpoint {
	return &AdminEndpoint{store: s, token: token}
}

// SetStatusProvider 注入节点状态读取。
func (h *AdminEndpoint) SetStatusProvider(f func() any) { h.statuses = f }

// SetAlertHooks 注入告警规则读写（避免包循环）。
func (h *AdminEndpoint) SetAlertHooks(list func() any, set func(kind, node string, threshold float64, webhook string) error, del func(kind, node string) error) {
	h.alerts, h.setAlert, h.delAlert = list, set, del
}

func (h *AdminEndpoint) auth(r *http.Request) bool {
	return h.token == "" || r.Header.Get("X-Admin-Token") == h.token
}

// ServeHTTP /api/admin/*。
func (h *AdminEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !h.auth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.URL.Path {
	case "/api/admin/nodes":
		if r.Method == http.MethodGet {
			nodes, err := h.store.AdminNodes()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"nodes": nodes, "statuses": h.statusProvider()})
			return
		}
		if r.Method == http.MethodPost {
			var n Client
			if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
				http.Error(w, "bad payload", http.StatusBadRequest)
				return
			}
			if err := h.store.AdminUpsertNode(n); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	case "/api/admin/alerts":
		if h.alerts == nil {
			_ = json.NewEncoder(w).Encode(map[string]any{"rules": []any{}})
			return
		}
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"rules": h.alerts()})
		case http.MethodPost:
			var p struct {
				Kind      string  `json:"kind"`
				Node      string  `json:"node"`
				Threshold float64 `json:"threshold"`
				Webhook   string  `json:"webhook"`
			}
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				http.Error(w, "bad payload", http.StatusBadRequest)
				return
			}
			if err := h.setAlert(p.Kind, p.Node, p.Threshold, p.Webhook); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		case http.MethodDelete:
			var p struct {
				Kind string `json:"kind"`
				Node string `json:"node"`
			}
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				http.Error(w, "bad payload", http.StatusBadRequest)
				return
			}
			if err := h.delAlert(p.Kind, p.Node); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	case "/api/admin/probes":
		if r.Method == http.MethodGet {
			tasks, err := h.store.AdminProbeTasks()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"probes": tasks})
			return
		}
		if r.Method == http.MethodPost {
			var p struct {
				Client  string `json:"client"`
				Target  string `json:"target"`
				Name    string `json:"name"`
				Type    string `json:"type"`
				Enabled *bool  `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				http.Error(w, "bad payload", http.StatusBadRequest)
				return
			}
			enabled := true
			if p.Enabled != nil {
				enabled = *p.Enabled
			}
			if err := h.store.AdminUpsertProbe(p.Client, p.Target, p.Name, p.Type, enabled); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
			return
		}
		if r.Method == http.MethodDelete {
			var p struct {
				Client string `json:"client"`
				Name   string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				http.Error(w, "bad payload", http.StatusBadRequest)
				return
			}
			if err := h.store.AdminDeleteProbe(p.Client, p.Name); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	default:
		http.NotFound(w, r)
	}
}
