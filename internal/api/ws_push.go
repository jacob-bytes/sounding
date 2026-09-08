package api

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// pushInterval 状态推送间隔。
const pushInterval = 3 * time.Second

// serveWSPush 连接建立后周期推送 getNodesLatestStatus（管理页/ink 实时化）。
func (h *Handler) serveWSPush(conn *websocket.Conn, stop <-chan struct{}) {
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
func (h *Handler) startPush(conn *websocket.Conn) chan struct{} {
	stop := make(chan struct{})
	go h.serveWSPush(conn, stop)
	return stop
}

var _ = json.Marshal
var _ = log.Printf
