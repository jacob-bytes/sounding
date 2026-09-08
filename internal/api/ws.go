package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true }, // 同源自托管场景
}

// ServeWS 处理 WebSocket JSON-RPC（ink 的实时通道）。
func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}
	defer conn.Close()
	log.Printf("ws client connected: %s", r.RemoteAddr)
	stop := h.startPush(conn)
	defer close(stop)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var req RPCRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			continue
		}
		resp := h.dispatch(req)
		if err := conn.WriteJSON(resp); err != nil {
			return
		}
	}
}

// IsWebSocketRequest 判断是否为 WS 升级请求。
func IsWebSocketRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}
