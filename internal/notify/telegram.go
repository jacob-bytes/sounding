package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Telegram 通过 Bot API 发送通知。
type Telegram struct {
	BotToken string
	ChatID   string
	client   *http.Client
}

// NewTelegram 创建 Telegram 通知渠道。
func NewTelegram(botToken, chatID string) *Telegram {
	return &Telegram{BotToken: botToken, ChatID: chatID, client: &http.Client{Timeout: 8 * time.Second}}
}

// Name 渠道名。
func (*Telegram) Name() string { return "telegram" }

// Send 调用 sendMessage（MarkdownV2 转义标题/正文关键字符）。
func (t *Telegram) Send(ctx context.Context, msg Message) error {
	if t.BotToken == "" || t.ChatID == "" {
		return fmt.Errorf("telegram: 缺少 bot token 或 chat id")
	}
	text := fmt.Sprintf("*%s*\n%s", escapeMarkdown(msg.Title), escapeMarkdown(msg.Body))
	payload := map[string]any{
		"chat_id":    t.ChatID,
		"text":       text,
		"parse_mode": "MarkdownV2",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram: http %d", resp.StatusCode)
	}
	return nil
}

// escapeMarkdown 转义 MarkdownV2 保留字符。
func escapeMarkdown(s string) string {
	replacer := []string{"_", "\\_", "*", "\\*", "[", "\\[", "]", "\\]", "(", "\\(", ")", "\\)", "~", "\\~", "`", "\\`", ">", "\\>", "#", "\\#", "+", "\\+", "-", "\\-", "=", "\\=", "|", "\\|", "{", "\\{", "}", "\\}", ".", "\\.", "!", "\\!"}
	r := make([]byte, 0, len(s)*2)
	for i := 0; i < len(s); i++ {
		ch := string(s[i])
		escaped := false
		for j := 0; j < len(replacer); j += 2 {
			if ch == replacer[j] {
				r = append(r, []byte(replacer[j+1])...)
				escaped = true
				break
			}
		}
		if !escaped {
			r = append(r, ch...)
		}
	}
	return string(r)
}
