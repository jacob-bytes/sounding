package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DingTalk 钉钉群机器人通知。
type DingTalk struct {
	Webhook string // https://oapi.dingtalk.com/robot/send?access_token=...
	Secret  string // 可选加签密钥
	client  *http.Client
}

// NewDingTalk 创建钉钉渠道。
func NewDingTalk(webhook, secret string) *DingTalk {
	return &DingTalk{Webhook: webhook, Secret: secret, client: &http.Client{Timeout: 8 * time.Second}}
}

// Name 渠道名。
func (*DingTalk) Name() string { return "dingtalk" }

// Send 发送 markdown 消息（加签可选）。
func (d *DingTalk) Send(ctx context.Context, msg Message) error {
	url := d.Webhook
	if d.Secret != "" {
		url = signDingTalk(d.Webhook, d.Secret)
	}
	payload := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": msg.Title,
			"text":  fmt.Sprintf("### %s\n\n%s", msg.Title, msg.Body),
		},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("dingtalk: http %d", resp.StatusCode)
	}
	return nil
}
