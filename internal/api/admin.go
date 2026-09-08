package api

import (
	"encoding/json"
	"net/http"
)

// AdminEndpoint 管理 API（添加服务器 / 探针配置——带 admin token）。
type AdminEndpoint struct {
	store interface {
		AdminNodes() (map[string]Client, error)
		AdminUpsertNode(n Client) error
		AdminProbeTasks() ([]AdminProbe, error)
		AdminUpsertProbe(client, target, name string, enabled bool) error
	}
	token string
}

// AdminProbe 管理视角探针任务。
type AdminProbe struct {
	ID      int    `json:"id"`
	Client  string `json:"client"`
	Target  string `json:"target"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// NewAdminEndpoint 构建管理端点。
func NewAdminEndpoint(s interface {
	AdminNodes() (map[string]Client, error)
	AdminUpsertNode(n Client) error
	AdminProbeTasks() ([]AdminProbe, error)
	AdminUpsertProbe(client, target, name string, enabled bool) error
}, token string) *AdminEndpoint {
	return &AdminEndpoint{store: s, token: token}
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
			_ = json.NewEncoder(w).Encode(map[string]any{"nodes": nodes})
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
			if err := h.store.AdminUpsertProbe(p.Client, p.Target, p.Name, enabled); err != nil {
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
