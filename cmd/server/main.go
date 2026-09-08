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
	"github.com/jacob-bytes/sounding/internal/notify"
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
	alertWebhook := flag.String("alert-webhook", "", "告警 Webhook 地址（为空=关闭该渠道）")
	tgToken := flag.String("telegram-token", "", "Telegram Bot Token（与 chat-id 同时提供则启用）")
	tgChat := flag.String("telegram-chat-id", "", "Telegram Chat ID")
	alertLatency := flag.Float64("alert-latency-ms", 0, "延迟告警阈值（ms，0=关闭）")
	alertOffline := flag.Bool("alert-offline", true, "离线告警（默认开）")
	retainDays := flag.Int("retain-days", 30, "历史数据保留天数（0=永久）")
	adminUser := flag.String("admin-user", "admin", "管理后台用户名")
	adminPass := flag.String("admin-pass", "", "管理后台密码（为空=不启用 JWT 登录）")
	jwtSecret := flag.String("jwt-secret", "", "JWT 签名密钥（为空则自动生成）")
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
	{
		var notifiers []notify.Notifier
		if *alertWebhook != "" {
			notifiers = append(notifiers, notify.NewWebhook(*alertWebhook))
		}
		if *tgToken != "" && *tgChat != "" {
			notifiers = append(notifiers, notify.NewTelegram(*tgToken, *tgChat))
		}
		if len(notifiers) > 0 {
			rules := []alert.Rule{}
			if *alertOffline {
				rules = append(rules, alert.Rule{Kind: "offline", Node: "*", Threshold: 1})
			}
			if *alertLatency > 0 {
				rules = append(rules, alert.Rule{Kind: "latency", Node: "*", Threshold: *alertLatency})
			}
			rules = append(rules, alert.Rule{Kind: "loss", Node: "*", Threshold: 100})
			alertMgr = alert.NewManager(rules, notifiers, 5*time.Minute)
			log.Printf("alerts enabled: %d 渠道", len(notifiers))
		}
	}

	// 演示探针种子
	if *seed {
		_ = st.SeedProbeTasks()
	}
	sched := probe.NewScheduler(st, *probeClient)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sched.Start(ctx)
	if *retainDays > 0 {
		go st.RetentionLoop(ctx, *retainDays)
	}
	if alertMgr != nil {
		go alert.Watch(ctx, st, alertMgr, 30*time.Second)
		log.Printf("alerts enabled → %s", *alertWebhook)
	}

	h := api.NewHandler(st)
	agentH := api.NewAgentEndpoint(st, *agentToken)
	adminH := api.NewAdminEndpoint(st, *adminToken)
	var loginH *api.LoginEndpoint
	if *adminPass != "" {
		loginH = api.NewLoginEndpoint(*adminUser, *adminPass, *jwtSecret)
	}
	mux := http.NewServeMux()

	// ink 契约：JSON-RPC 2.0 POST <base>/rpc2
	mux.Handle("/rpc2", h)
	// 健康检查（LB/K8s 探针）
	mux.Handle("/healthz", &api.HealthEndpoint{Ready: func() error { _, err := st.Nodes(); return err }})
	// Agent 上报：POST /agent/status
	mux.Handle("/agent/status", agentH)
	// 登录（JWT）
	if loginH != nil {
		mux.Handle("/api/login", loginH)
		mux.HandleFunc("/api/me2", loginH.AuthMiddleware(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]any{"status": "success", "data": map[string]any{"logged_in": true, "username": *adminUser}})
		}))
	}
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
						"theme_settings": map[string]any{
							"rpcTransportMode": "websocket", // ink 走 WS 实时通道
						},
						"record_enabled": true,
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
