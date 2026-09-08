// Package notify 提供告警通知渠道抽象（Webhook / Telegram / 后续扩展）。
package notify

import "context"

// Message 通知消息。
type Message struct {
	Title string // 标题（如 "节点离线"）
	Body  string // 正文
	Level string // info | warning | critical
}

// Notifier 通知渠道接口——新增渠道只需实现此接口。
type Notifier interface {
	// Name 渠道名称（用于日志/配置）。
	Name() string
	// Send 发送消息。
	Send(ctx context.Context, msg Message) error
}
