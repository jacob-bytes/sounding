package probe

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// Result 一次探测结果（Value 语义随 Type 变化）。
type Result struct {
	Type   string  `json:"type"`
	Value  float64 `json:"value"` // icmp/tcp: ms · http: ms · dns: ms
	OK     bool    `json:"ok"`
	Detail string  `json:"detail,omitempty"` // http: 状态码 · dns: 解析结果
}

// Run 按类型执行探测（icmp | tcp | http | dns）。
func Run(ctx context.Context, typ, target string) Result {
	switch strings.ToLower(typ) {
	case "http", "https":
		return runHTTP(ctx, target)
	case "dns":
		return runDNS(ctx, target)
	case "tcp":
		return runTCP(ctx, target)
	default:
		ms, kind, err := Ping(ctx, target)
		if err != nil {
			return Result{Type: kind, OK: false, Value: -1, Detail: err.Error()}
		}
		return Result{Type: kind, Value: ms, OK: true}
	}
}

// runTCP 端口连通性 + 延迟。
func runTCP(ctx context.Context, target string) Result {
	t := target
	if _, _, err := net.SplitHostPort(t); err != nil {
		t = net.JoinHostPort(t, "443")
	}
	ms, err := TCPPing(ctx, t)
	if err != nil {
		return Result{Type: "tcp", OK: false, Value: -1, Detail: err.Error()}
	}
	return Result{Type: "tcp", Value: ms, OK: true}
}

// runHTTP HTTP 状态码 + 响应时间（2xx/3xx 视为成功）。
func runHTTP(ctx context.Context, target string) Result {
	url := target
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	client := &http.Client{Timeout: 10 * time.Second}
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{Type: "http", OK: false, Value: -1, Detail: err.Error()}
	}
	resp, err := client.Do(req)
	ms := float64(time.Since(start).Microseconds()) / 1000
	if err != nil {
		return Result{Type: "http", OK: false, Value: -1, Detail: err.Error()}
	}
	defer resp.Body.Close()
	ok := resp.StatusCode < 400
	return Result{Type: "http", Value: ms, OK: ok, Detail: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

// runDNS 解析耗时 + 结果（格式: 域名 或 域名@服务器）。
func runDNS(ctx context.Context, target string) Result {
	host := target
	server := ""
	if i := strings.Index(target, "@"); i > 0 {
		host, server = target[:i], target[i+1:]
	}
	resolver := net.DefaultResolver
	if server != "" {
		if _, _, err := net.SplitHostPort(server); err != nil {
			server = net.JoinHostPort(server, "53")
		}
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				d := net.Dialer{Timeout: 5 * time.Second}
				return d.DialContext(ctx, network, server)
			},
		}
	}
	start := time.Now()
	ips, err := resolver.LookupHost(ctx, host)
	ms := float64(time.Since(start).Microseconds()) / 1000
	if err != nil || len(ips) == 0 {
		return Result{Type: "dns", OK: false, Value: -1, Detail: "解析失败"}
	}
	return Result{Type: "dns", Value: ms, OK: true, Detail: ips[0]}
}
