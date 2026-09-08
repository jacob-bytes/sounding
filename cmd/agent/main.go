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
)

func main() {
	// 配置优先级：flag > env > 默认（开源友好——Docker/K8s 可用环境变量）
	const (
		envServer = "SOUNDING_SERVER"
		envToken  = "SOUNDING_TOKEN"
		envUUID   = "SOUNDING_NODE_UUID"
		envProbe  = "SOUNDING_PROBES"
		envInterval = "SOUNDING_INTERVAL"
	)
	configFile := flag.String("config", "", "配置文件路径（agent.yml——支持 SIGHUP/修改热重载）")
	server := flag.String("server", envOr(envServer, "http://localhost:8080"), "主控地址（可用 $SOUNDING_SERVER）")
	token := flag.String("token", envOr(envToken, "sounding-demo-token"), "上报认证 token（可用 $SOUNDING_TOKEN）")
	interval := flag.Duration("interval", envOrDuration(envInterval, 15*time.Second), "采集周期（可用 $SOUNDING_INTERVAL）")
	probeTargets := flag.String("probe", envOr(envProbe, ""), `本节点延迟测试目标（格式: "名称:主机,名称:主机"，如 "上海移动:223.5.5.5,腾讯 DNS:119.29.29.29"，可用 $SOUNDING_PROBES）`)
	nodeUUID := flag.String("node-uuid", envOr(envUUID, ""), "节点 UUID（默认主机名；可用 $SOUNDING_NODE_UUID）")
	flag.Parse()

	// 热重载配置（flag/env 为初值；文件配置动态覆盖）
	fc := LoadConfig(*configFile)
	finalServer := fc.Server(*server)
	finalToken := fc.Token(*token)
	intervalVal := fc.Interval(*interval)
	uuid := fc.NodeUUID(*nodeUUID)

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

	log.Printf("sounding-agent %s → %s (every %s, config=%s)", uuid, finalServer, intervalVal, *configFile)
	ticker := time.NewTicker(intervalVal)
	defer ticker.Stop()

	// 首帧立即上报（注册节点）——server/token/probes 每次从热重载配置取
	report(ctx, fc.Server(finalServer), fc.Token(finalToken), uuid, osInfo, c, parseProbes(fc.Probes()+","+*probeTargets))

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			report(ctx, fc.Server(finalServer), fc.Token(finalToken), uuid, osInfo, c, parseProbes(fc.Probes()+","+*probeTargets))
		}
	}
}

// probeTarget 单个探针目标。
type probeTarget struct {
	Name string `json:"name"`
	Host string `json:"host"`
}

// envOr 读取环境变量或返回默认值。
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envOrDuration 读取环境变量持续时间或返回默认值。
func envOrDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func parseProbes(s string) []probeTarget {
	var out []probeTarget
	if s == "" {
		return out
	}
	for _, part := range strings.Split(s, ",") {
		if i := strings.Index(part, ":"); i > 0 {
			out = append(out, probeTarget{Name: strings.TrimSpace(part[:i]), Host: strings.TrimSpace(part[i+1:])})
		}
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
		UUID    string `json:"uuid"`
		Name    string `json:"name"`
		OSInfo  collect.OSInfo `json:"os_info"`
		SNapshot collect.Snapshot `json:"status"`
		Probes  []probeTarget `json:"probes"`
		Time    time.Time `json:"time"`
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
