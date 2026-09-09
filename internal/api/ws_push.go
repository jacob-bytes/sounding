package api

import (
	"time"
)

// pushInterval 状态推送间隔（测试可覆盖）。
var pushInterval = 3 * time.Second

// serveWSPush 连接建立后周期推送 getNodesLatestStatus（管理页/ink 实时化）。
func (h *Handler) serveWSPush(conn *wsWriter, stop <-chan struct{}) {
	t := time.NewTicker(pushInterval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			statuses, err := h.store.LatestStatus()
			if err != nil {
				continue
			}
			nodes, err2 := h.store.Nodes()
			if err2 != nil {
				continue
			}
			msg := map[string]any{
				"jsonrpc": "2.0",
				"method":  "sounding.push",
				"params": map[string]any{
					"statuses": statuses,
					"nodes":    nodes,
					"time":     time.Now().Format(time.RFC3339),
				},
			}
			if err := conn.WriteJSON(msg); err != nil {
				return
			}
		}
	}
}

// startPush 在连接上启动推送协程。
func (h *Handler) startPush(conn *wsWriter) chan struct{} {
	stop := make(chan struct{})
	go h.serveWSPush(conn, stop)
	return stop
}
