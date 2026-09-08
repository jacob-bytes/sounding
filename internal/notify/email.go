package notify

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

// Email SMTP 邮件通知。
type Email struct {
	Host     string // smtp.example.com:587
	Username string
	Password string
	From     string
	To       []string
}

// NewEmail 创建邮件渠道。
func NewEmail(host, user, pass, from string, to []string) *Email {
	return &Email{Host: host, Username: user, Password: pass, From: from, To: to}
}

// Name 渠道名。
func (*Email) Name() string { return "email" }

// Send 通过 SMTP 发送（STARTTLS 由 net/smtp 自动协商）。
func (e *Email) Send(_ context.Context, msg Message) error {
	if e.Host == "" || e.From == "" || len(e.To) == 0 {
		return fmt.Errorf("email: 配置不完整")
	}
	auth := smtp.PlainAuth("", e.Username, e.Password, strings.Split(e.Host, ":")[0])
	body := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		e.From, strings.Join(e.To, ","), msg.Title, msg.Body)
	return smtp.SendMail(e.Host, auth, e.From, e.To, []byte(body))
}
