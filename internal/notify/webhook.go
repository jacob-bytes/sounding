package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Webhook 通用 HTTP 通知（JSON POST）。
type Webhook struct {
	URL    string
	client *http.Client
}

// NewWebhook 创建 Webhook 渠道。
func NewWebhook(url string) *Webhook {
	return &Webhook{URL: url, client: &http.Client{Timeout: 5 * time.Second}}
}

// Name 渠道名。
func (*Webhook) Name() string { return "webhook" }

// Send POST JSON。
func (w *Webhook) Send(ctx context.Context, msg Message) error {
	body, _ := json.Marshal(map[string]any{
		"title": msg.Title, "body": msg.Body, "level": msg.Level,
		"time": time.Now().Format(time.RFC3339),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
