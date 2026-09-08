package alert

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/jacob-bytes/sounding/internal/notify"
)

// Rule 告警规则。
type Rule struct {
	Kind      string  `json:"kind"`      // offline | latency | loss
	Node      string  `json:"node"`      // uuid 或 *（全部）
	Threshold float64 `json:"threshold"` // 阈值（ms / %）
}

// Event 告警事件。
type Event struct {
	Kind      string  `json:"kind"`
	Node      string  `json:"node"`
	NodeName  string  `json:"node_name"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Message   string  `json:"message"`
	Time      string  `json:"time"`
}

// Manager 告警管理器（规则 + 去重 + 多渠道投递）。
type Manager struct {
	mu        sync.Mutex
	rules     []Rule
	lastSent  map[string]time.Time
	cooldown  time.Duration
	notifiers []notify.Notifier
}

// NewManager 创建告警管理器（notifiers 为投递渠道——Webhook/Telegram/…）。
func NewManager(rules []Rule, notifiers []notify.Notifier, cooldown time.Duration) *Manager {
	if cooldown <= 0 {
		cooldown = 5 * time.Minute
	}
	return &Manager{rules: rules, notifiers: notifiers, lastSent: map[string]time.Time{}, cooldown: cooldown}
}

// Rules 返回当前规则。
func (m *Manager) Rules() []Rule {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Rule(nil), m.rules...)
}

// Evaluate 评估一次事件（去重后投递 Webhook）。
func (m *Manager) Evaluate(ev Event) {
	m.mu.Lock()
	key := ev.Kind + "|" + ev.Node
	if t, ok := m.lastSent[key]; ok && time.Since(t) < m.cooldown {
		m.mu.Unlock()
		return
	}
	matched := false
	for _, r := range m.rules {
		if r.Kind == ev.Kind && (r.Node == "*" || r.Node == ev.Node) && ev.Value >= r.Threshold {
			ev.Threshold = r.Threshold
			matched = true
			break
		}
	}
	if !matched || len(m.notifiers) == 0 {
		m.mu.Unlock()
		return
	}
	m.lastSent[key] = time.Now()
	targets := append([]notify.Notifier(nil), m.notifiers...)
	m.mu.Unlock()

	level := "warning"
	if ev.Kind == "offline" {
		level = "critical"
	}
	msg := notify.Message{Title: title(ev), Body: ev.Message, Level: level}
	for _, n := range targets {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := n.Send(ctx, msg); err != nil {
			log.Printf("alert [%s]: %v", n.Name(), err)
		} else {
			log.Printf("alert sent via %s: %s %s=%.1f", n.Name(), ev.Kind, ev.NodeName, ev.Value)
		}
		cancel()
	}
}

// title 生成通知标题。
func title(ev Event) string {
	switch ev.Kind {
	case "offline":
		return "🔴 节点离线 · " + ev.NodeName
	case "latency":
		return "🟡 延迟告警 · " + ev.NodeName
	case "loss":
		return "🔴 探测丢包 · " + ev.NodeName
	}
	return "告警 · " + ev.NodeName
}
