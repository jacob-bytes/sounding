package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jacob-bytes/sounding/internal/collect"
	"github.com/jacob-bytes/sounding/internal/version"
)

func main() {
	// 配置优先级：CLI flag > 环境变量 > 配置文件 > 默认值
	const (
		envServer   = "SOUNDING_SERVER"
		envToken    = "SOUNDING_TOKEN"
		envUUID     = "SOUNDING_NODE_UUID"
		envProbe    = "SOUNDING_PROBES"
		envInterval = "SOUNDING_INTERVAL"
	)
	configFile := flag.String("config", "", "配置文件路径（agent.yml——支持 SIGHUP/修改热重载）")
	remoteConfig := flag.Bool("remote-config", true, "从主控拉取探针配置（远程下发）")
	server := flag.String("server", "", "主控地址（可用 $SOUNDING_SERVER）")
	token := flag.String("token", "", "上报认证 token（可用 $SOUNDING_TOKEN）")
	interval := flag.Duration("interval", 0, "采集周期（可用 $SOUNDING_INTERVAL）")
	probeTargets := flag.String("probe", "", `本节点延迟测试目标（格式: "名称[:类型]:主机"，如 "上海移动:223.5.5.5,百度:http:https://baidu.com"，可用 $SOUNDING_PROBES）`)
	nodeUUID := flag.String("node-uuid", "", "节点 UUID（默认主机名；可用 $SOUNDING_NODE_UUID）")
	showVersion := flag.Bool("version", false, "显示版本并退出")
	flag.Parse()
	if *showVersion {
		log.Printf("sounding-agent %s", version.String())
		return
	}

	fc := LoadConfig(*configFile)
	// 每次上报都重新求值——文件热重载后立即生效
	resolveServer := func() string {
		return pick(*server, os.Getenv(envServer), fc.Server(), "http://localhost:8080")
	}
	resolveToken := func() string {
		return pick(*token, os.Getenv(envToken), fc.Token(), "sounding-demo-token")
	}
	resolveProbes := func() string {
		return pick(*probeTargets, os.Getenv(envProbe), fc.Probes(), "")
	}
	intervalVal := pickDuration(*interval, envDuration(envInterval), fc.Interval(), 15*time.Second)
	uuid := pick(*nodeUUID, os.Getenv(envUUID), fc.NodeUUID(), "")

	osInfo, err := collect.GetOSInfo()
	if err != nil {
		log.Printf("osinfo: %v", err)
	}
	if uuid == "" {
		uuid = osInfo.HostName
	}

	c := collect.New(intervalVal)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("sounding-agent %s → %s (every %s, config=%s, remote-config=%v)", uuid, resolveServer(), intervalVal, *configFile, *remoteConfig)
	ticker := time.NewTicker(intervalVal)
	defer ticker.Stop()

	// 远程配置拉取（主控下发的探针目标——每次上报前刷新）
	remoteProbes := ""
	if *remoteConfig {
		remoteProbes = fetchRemoteProbes(ctx, resolveServer(), resolveToken(), uuid)
	}
	// 首帧立即上报（注册节点）
	report(ctx, resolveServer(), resolveToken(), uuid, osInfo, c, parseProbes(remoteProbes+","+resolveProbes()))

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if *remoteConfig {
				remoteProbes = fetchRemoteProbes(ctx, resolveServer(), resolveToken(), uuid)
			}
			report(ctx, resolveServer(), resolveToken(), uuid, osInfo, c, parseProbes(remoteProbes+","+resolveProbes()))
		}
	}
}

// probeTarget 单个探针目标。
type probeTarget struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Type string `json:"type,omitempty"`
}

// probeTypes 支持的探针类型（用于 "名称:类型:目标" 语法判定）。
var probeTypes = map[string]bool{"icmp": true, "ping": true, "tcp": true, "http": true, "https": true, "dns": true}

// fetchRemoteProbes 从主控拉取探针配置（"名称[:类型]:主机, ..."）。
func fetchRemoteProbes(ctx context.Context, server, token, uuid string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server+"/agent/config?uuid="+uuid, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("X-Auth-Token", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	var body struct {
		Tasks []struct {
			Name   string `json:"name"`
			Target string `json:"target"`
			Type   string `json:"type"`
		} `json:"tasks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return ""
	}
	parts := make([]string, 0, len(body.Tasks))
	for _, t := range body.Tasks {
		if t.Name == "" || t.Target == "" {
			continue
		}
		if t.Type != "" {
			parts = append(parts, t.Name+":"+t.Type+":"+t.Target)
		} else {
			parts = append(parts, t.Name+":"+t.Target)
		}
	}
	return strings.Join(parts, ",")
}

// pick 返回第一个非空值（flag > env > file > default）。
func pick(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// pickDuration 返回第一个大于 0 的值。
func pickDuration(vals ...time.Duration) time.Duration {
	for _, v := range vals {
		if v > 0 {
			return v
		}
	}
	return 0
}

// envDuration 读取环境变量持续时间（未设置/非法返回 0）。
func envDuration(key string) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 0
}

// parseProbes 解析 "名称[:类型]:主机" 列表（逗号分隔）。
// 兼容 "上海移动:223.5.5.5" 与 "百度:http:https://baidu.com" 两种写法。
func parseProbes(s string) []probeTarget {
	var out []probeTarget
	if s == "" {
		return out
	}
	seen := map[string]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var name, typ, host string
		segs := strings.SplitN(part, ":", 3)
		if len(segs) >= 3 && probeTypes[strings.ToLower(strings.TrimSpace(segs[1]))] {
			name, typ, host = strings.TrimSpace(segs[0]), strings.ToLower(strings.TrimSpace(segs[1])), strings.TrimSpace(segs[2])
		} else if len(segs) >= 2 {
			name, host = strings.TrimSpace(segs[0]), strings.TrimSpace(strings.Join(segs[1:], ":"))
		} else {
			continue
		}
		if name == "" || host == "" {
			continue
		}
		key := name + "|" + host
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, probeTarget{Name: name, Host: host, Type: typ})
	}
	return out
}

func report(ctx context.Context, server, token, uuid string, info collect.OSInfo, c *collect.Collector, probes []probeTarget) {
	snap, err := c.Collect(ctx)
	if err != nil {
		log.Printf("collect: %v", err)
		return
	}
	payload := struct {
		UUID   string           `json:"uuid"`
		Name   string           `json:"name"`
		OSInfo collect.OSInfo   `json:"os_info"`
		Status collect.Snapshot `json:"status"`
		Probes []probeTarget    `json:"probes"`
		Time   time.Time        `json:"time"`
	}{uuid, info.HostName, info, snap, probes, time.Now()}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server+"/agent/status", bytes.NewReader(body))
	if err != nil {
		log.Printf("req: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("report: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("上报完成: %.1f%% (http %d)", snap.CPU, resp.StatusCode)
}
