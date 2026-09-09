package api

import (
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestWebSocketConcurrentWrites 验证推送协程与 RPC 响应不会并发写同一连接。
// 配合 `go test -race` 可捕获 gorilla/websocket 的并发写问题。
func TestWebSocketConcurrentWrites(t *testing.T) {
	old := pushInterval
	pushInterval = time.Millisecond
	defer func() { pushInterval = old }()

	srv := httptest.NewServer(NewHandler(fakeStore{}))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/rpc2"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// 高频发送 RPC 请求，与 1ms 推送同时写连接
	for i := 0; i < 50; i++ {
		if err := conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": i, "method": "ping"}); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	time.Sleep(50 * time.Millisecond)
	_ = conn.Close()
	<-done
	wg.Wait()
}
