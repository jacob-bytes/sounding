package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jacob-bytes/sounding/internal/adminui"
	"github.com/jacob-bytes/sounding/internal/alert"
	"github.com/jacob-bytes/sounding/internal/api"
	"github.com/jacob-bytes/sounding/internal/notify"
	"github.com/jacob-bytes/sounding/internal/probe"
	"github.com/jacob-bytes/sounding/internal/store"
	"github.com/jacob-bytes/sounding/internal/version"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
	dbPath := flag.String("db", "sounding.db", "SQLite 数据库路径")
	seed := flag.Bool("seed", true, "启动时写入演示数据")
	showVersion := flag.Bool("version", false, "显示版本并退出")
	agentToken := flag.String("agent-token", "sounding-demo-token", "Agent 上报认证 token")
	probeClient := flag.String("probe-client", "demo-001", "探针记录挂载的 client uuid")
	adminToken := flag.String("admin-token", "", "管理 API token（为空=不启用认证）")
	alertWebhook := flag.String("alert-webhook", "", "告警 Webhook 地址（为空=关闭该渠道）")
	tgToken := flag.String("telegram-token", "", "Telegram Bot Token（与 chat-id 同时提供则启用）")
	tgChat := flag.String("telegram-chat-id", "", "Telegram Chat ID")
	dingTalk := flag.String("dingtalk-webhook", "", "钉钉机器人 Webhook（可选 -dingtalk-secret 加签）")
	dingTalkSecret := flag.String("dingtalk-secret", "", "钉钉加签密钥")
	feishu := flag.String("feishu-webhook", "", "飞书机器人 Webhook")
	smtpHost := flag.String("smtp-host", "", "SMTP 服务器（如 smtp.example.com:587）")
	smtpUser := flag.String("smtp-user", "", "SMTP 用户名")
	smtpPass := flag.String("smtp-pass", "", "SMTP 密码")
	smtpFrom := flag.String("smtp-from", "", "发件人")
	smtpTo := flag.String("smtp-to", "", "收件人（逗号分隔）")
	alertLatency := flag.Float64("alert-latency-ms", 0, "延迟告警阈值（ms，0=关闭）")
	alertOffline := flag.Bool("alert-offline", true, "离线告警（默认开）")
	retainDays := flag.Int("retain-days", 30, "历史数据保留天数（0=永久）")
	adminUser := flag.String("admin-user", "admin", "管理后台用户名")
	adminPass := flag.String("admin-pass", "", "管理后台密码（为空=不启用 JWT 登录）")
	jwtSecret := flag.String("jwt-secret", "", "JWT 签名密钥（为空则自动生成）")
	staticDir := flag.String("static", "", "前端静态目录（ink 构建产物——可选，提供管理后台）")
	flag.Parse()

	if *showVersion {
		log.Printf("sounding-server %s", version.String())
		return
	}
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
		if *dingTalk != "" {
			notifiers = append(notifiers, notify.NewDingTalk(*dingTalk, *dingTalkSecret))
		}
		if *feishu != "" {
			notifiers = append(notifiers, notify.NewFeishu(*feishu))
		}
		if *smtpHost != "" && *smtpFrom != "" && *smtpTo != "" {
			notifiers = append(notifiers, notify.NewEmail(*smtpHost, *smtpUser, *smtpPass, *smtpFrom, strings.Split(*smtpTo, ",")))
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
	adminH.SetStatusProvider(func() any { m, _ := st.LatestStatus(); return m })
	if alertMgr != nil {
		adminH.SetAlertHooks(
			func() any { return alertMgr.Rules() },
			func(kind, node string, threshold float64, _ string) error {
				alertMgr.SetRule(alert.Rule{Kind: kind, Node: node, Threshold: threshold})
				return nil
			},
			func(kind, node string) error { alertMgr.DeleteRule(kind, node); return nil },
		)
	}
	var loginH *api.LoginEndpoint
	if *adminPass != "" {
		loginH = api.NewLoginEndpoint(*adminUser, *adminPass, *jwtSecret)
	}
	mux := http.NewServeMux()

	// ink 契约：JSON-RPC 2.0 POST <base>/rpc2
	mux.Handle("/rpc2", h)
	// ink 默认 API base 为 /api——同源别名
	mux.Handle("/api/rpc2", h)
	// 内置管理页（Go embed——无需前端构建）
	mux.HandleFunc("/admin/", adminui.Handler())
	// Agent 配置下发（远程拉取探针目标）
	mux.HandleFunc("/agent/config", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.URL.Query().Get("uuid")
		if r.Header.Get("X-Auth-Token") != *agentToken && *agentToken != "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		names, err := st.AdminProbeTaskNames(uuid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tasks, _ := st.ProbeTasks(uuid)
		writeJSON(w, map[string]any{"probe_names": names, "tasks": tasks})
	})
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
	mux.Handle("/api/admin/alerts", adminH)
	// ink 初始化端点（InitManager）
	// ink 初始化端点（根路径 + /api 前缀）
	for _, p := range []string{"/me", "/public", "/version"} {
		path := p
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
			switch path {
			case "/me":
				writeJSON(w, map[string]any{"logged_in": false})
			case "/public":
				publicSettingsHandler(w, nil)
			case "/version":
				writeJSON(w, map[string]any{"version": "0.1.0"})
			}
		})
	}
	// /api/* 与根路径同实现（ink 默认 base=/api）
	mux.HandleFunc("/api/me", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, map[string]any{"logged_in": false}) })
	mux.HandleFunc("/api/public", publicSettingsHandler)
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, map[string]any{"version": "0.1.0"}) })
	// 监控面板（ink 前端构建产物）——-static 指定，或自动探测 ./admin、/var/lib/sounding/admin
	if *staticDir == "" {
		for _, cand := range []string{"./admin", "/var/lib/sounding/admin"} {
			if st, err := os.Stat(cand); err == nil && st.IsDir() {
				*staticDir = cand
				break
			}
		}
	}
	if *staticDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(*staticDir)))
		log.Printf("serving ink dashboard from %s", *staticDir)
	}

	log.Printf("sounding server listening on %s (rpc2: /rpc2)", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

var alertMgr *alert.Manager

// publicSettingsHandler ink 站点公开设置（含 WS 通道开关）。
func publicSettingsHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"status": "success",
		"data": map[string]any{
			"theme_settings": map[string]any{
				"rpcTransportMode": "websocket", // ink 走 WS 实时通道
			},
			"record_enabled":         true,
			"sitename":               "sounding",
			"description":            "sounding",
			"custom_body":            "",
			"custom_head":            "",
			"allow_cors":             false,
			"disable_password_login": true,
			"oauth_enable":           false,
			"oauth_provider":         nil,
			"private_site":           false,
		},
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
