package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Feishu 飞书自定义机器人通知。
type Feishu struct {
	Webhook string
	client  *http.Client
}

// NewFeishu 创建飞书渠道。
func NewFeishu(webhook string) *Feishu {
	return &Feishu{Webhook: webhook, client: &http.Client{Timeout: 8 * time.Second}}
}

// Name 渠道名。
func (*Feishu) Name() string { return "feishu" }

// Send 发送交互式卡片（简化文本卡片）。
func (f *Feishu) Send(ctx context.Context, msg Message) error {
	payload := map[string]any{
		"msg_type": "post",
		"content": map[string]any{
			"post": map[string]any{
				"zh_cn": map[string]any{
					"title": msg.Title,
					"content": [][]map[string]any{
						{{"tag": "text", "text": msg.Body}},
					},
				},
			},
		},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.Webhook, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("feishu: http %d", resp.StatusCode)
	}
	return nil
}
