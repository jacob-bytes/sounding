package api

import "net/http"

// HealthEndpoint 健康检查（负载均衡/K8s 探针用）。
type HealthEndpoint struct {
	Ready func() error
}

// ServeHTTP GET /healthz。
func (h *HealthEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Ready != nil {
		if err := h.Ready(); err != nil {
			http.Error(w, "not ready: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
	}
	writeJSON(w, map[string]any{"status": "ok", "service": "sounding"})
}
