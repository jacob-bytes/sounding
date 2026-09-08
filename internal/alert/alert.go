package alert

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// Rule 告警规则。
type Rule struct {
	Kind      string  `json:"kind"`      // offline | latency | loss
	Node      string  `json:"node"`      // uuid 或 *（全部）
	Threshold float64 `json:"threshold"` // 阈值（ms / %）
	Webhook   string  `json:"webhook"`   // 通知地址
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

// Manager 告警管理器（规则 + 去重 + Webhook 投递）。
type Manager struct {
	mu       sync.Mutex
	rules    []Rule
	lastSent map[string]time.Time
	cooldown time.Duration
	client   *http.Client
}

// NewManager 创建告警管理器。
func NewManager(rules []Rule, cooldown time.Duration) *Manager {
	if cooldown <= 0 {
		cooldown = 5 * time.Minute
	}
	return &Manager{rules: rules, lastSent: map[string]time.Time{}, cooldown: cooldown, client: &http.Client{Timeout: 5 * time.Second}}
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
	var hook string
	for _, r := range m.rules {
		if r.Kind == ev.Kind && (r.Node == "*" || r.Node == ev.Node) && ev.Value >= r.Threshold {
			hook = r.Webhook
			ev.Threshold = r.Threshold
			break
		}
	}
	if hook == "" {
		m.mu.Unlock()
		return
	}
	m.lastSent[key] = time.Now()
	m.mu.Unlock()

	body, _ := json.Marshal(ev)
	resp, err := m.client.Post(hook, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("alert webhook: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("alert sent: %s %s=%.1f (http %d)", ev.Kind, ev.NodeName, ev.Value, resp.StatusCode)
}
