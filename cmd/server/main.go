package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/jacob-bytes/sounding/internal/alert"
	"github.com/jacob-bytes/sounding/internal/api"
	"github.com/jacob-bytes/sounding/internal/probe"
	"github.com/jacob-bytes/sounding/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
	dbPath := flag.String("db", "sounding.db", "SQLite 数据库路径")
	seed := flag.Bool("seed", true, "启动时写入演示数据")
	agentToken := flag.String("agent-token", "sounding-demo-token", "Agent 上报认证 token")
	probeClient := flag.String("probe-client", "demo-001", "探针记录挂载的 client uuid")
	adminToken := flag.String("admin-token", "", "管理 API token（为空=不启用认证）")
	alertWebhook := flag.String("alert-webhook", "", "告警 Webhook 地址（为空=关闭告警）")
	alertLatency := flag.Float64("alert-latency-ms", 0, "延迟告警阈值（ms，0=关闭）")
	alertOffline := flag.Bool("alert-offline", true, "离线告警（默认开）")
	staticDir := flag.String("static", "", "前端静态目录（ink 构建产物——可选，提供管理后台）")
	flag.Parse()

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if *seed {
		if err := st.Seed(); err != nil {
			log.Printf("seed: %v", err)
		}
	}

	// 告警（可选）
	if *alertWebhook != "" {
		rules := []alert.Rule{}
		if *alertOffline {
			rules = append(rules, alert.Rule{Kind: "offline", Node: "*", Threshold: 1, Webhook: *alertWebhook})
		}
		if *alertLatency > 0 {
			rules = append(rules, alert.Rule{Kind: "latency", Node: "*", Threshold: *alertLatency, Webhook: *alertWebhook})
		}
		rules = append(rules, alert.Rule{Kind: "loss", Node: "*", Threshold: 100, Webhook: *alertWebhook})
		alertMgr = alert.NewManager(rules, 5*time.Minute)
	}

	// 演示探针种子
	if *seed {
		_ = st.SeedProbeTasks()
	}
	sched := probe.NewScheduler(st, *probeClient)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sched.Start(ctx)
	if alertMgr != nil {
		go alert.Watch(ctx, st, alertMgr, 30*time.Second)
		log.Printf("alerts enabled → %s", *alertWebhook)
	}

	h := api.NewHandler(st)
	agentH := api.NewAgentEndpoint(st, *agentToken)
	adminH := api.NewAdminEndpoint(st, *adminToken)
	mux := http.NewServeMux()

	// ink 契约：JSON-RPC 2.0 POST <base>/rpc2
	mux.Handle("/rpc2", h)
	// Agent 上报：POST /agent/status
	mux.Handle("/agent/status", agentH)
	// 管理 API：/api/admin/*
	mux.Handle("/api/admin/nodes", adminH)
	mux.Handle("/api/admin/probes", adminH)
	// ink 初始化端点（InitManager）
	// ink 初始化端点（VITE_API_BASE 即 base——根路径）
	for _, p := range []string{"/me", "/public", "/version"} {
		mux.HandleFunc(p, func(w http.ResponseWriter, _ *http.Request) {
			switch p {
			case "/me":
				writeJSON(w, map[string]any{"logged_in": false})
			case "/public":
				writeJSON(w, map[string]any{
					"status": "success",
					"data": map[string]any{
						"theme_settings": map[string]any{},
						"record_enabled":  true,
						"sitename":       "sounding",
						"description":    "sounding",
						"custom_body":    "",
						"custom_head":    "",
						"allow_cors":     false,
						"disable_password_login": true,
						"oauth_enable":   false,
						"oauth_provider": nil,
						"private_site":   false,
					},
				})
			case "/version":
				writeJSON(w, map[string]any{"version": "0.1.0"})
			}
		})
	}
	mux.HandleFunc("/api/me", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, map[string]any{"logged_in": false}) })
	mux.HandleFunc("/api/public", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"theme_settings": map[string]any{}, "record_enabled": true})
	})
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, map[string]any{"version": "0.1.0"}) })
	// 管理后台（ink 前端构建产物）
	if *staticDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(*staticDir)))
		log.Printf("serving admin UI from %s", *staticDir)
	}

	log.Printf("sounding server listening on %s (rpc2: /rpc2)", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

var alertMgr *alert.Manager

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
