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
	"syscall"
	"time"

	"github.com/jacob-bytes/sounding/internal/collect"
)

func main() {
	server := flag.String("server", "http://localhost:8080", "主控地址")
	token := flag.String("token", "sounding-demo-token", "上报认证 token")
	interval := flag.Duration("interval", 15*time.Second, "采集周期")
	nodeUUID := flag.String("node-uuid", "", "节点 UUID（默认主机名）")
	flag.Parse()

	osInfo, err := collect.GetOSInfo()
	if err != nil {
		log.Printf("osinfo: %v", err)
	}
	uuid := *nodeUUID
	if uuid == "" {
		uuid = osInfo.HostName
	}
	c := collect.New(*interval)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("sounding-agent %s → %s (every %s)", uuid, *server, *interval)
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	// 首帧立即上报（注册节点）
	report(ctx, *server, *token, uuid, osInfo, c)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			report(ctx, *server, *token, uuid, osInfo, c)
		}
	}
}

func report(ctx context.Context, server, token, uuid string, info collect.OSInfo, c *collect.Collector) {
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
		Time    time.Time `json:"time"`
	}{uuid, info.HostName, info, snap, time.Now()}
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
	log.Printf("上报完成: %s (http %d)", snap.CPU, resp.StatusCode)
}
