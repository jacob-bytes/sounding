package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true }, // 同源自托管场景
}

// wsWriter 串行化同一连接上的写操作（gorilla/websocket 不允许并发写）。
type wsWriter struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (w *wsWriter) WriteJSON(v any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteJSON(v)
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
	writer := &wsWriter{conn: conn}
	stop := h.startPush(writer)
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
		if err := writer.WriteJSON(resp); err != nil {
			return
		}
	}
}

// IsWebSocketRequest 判断是否为 WS 升级请求。
func IsWebSocketRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}
