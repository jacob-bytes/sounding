package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	seed := flag.Bool("seed", false, "启动时写入演示数据（演示/联调用）")
	showVersion := flag.Bool("version", false, "显示版本并退出")
	agentToken := flag.String("agent-token", os.Getenv("SOUNDING_AGENT_TOKEN"), "Agent 上报认证 token（为空则随机生成并打印；可用 $SOUNDING_AGENT_TOKEN）")
	probeClient := flag.String("probe-client", "", "额外探针 client uuid（默认自动覆盖所有节点）")
	adminToken := flag.String("admin-token", os.Getenv("SOUNDING_ADMIN_TOKEN"), "管理 API token（为空则随机生成并打印；可用 $SOUNDING_ADMIN_TOKEN）")
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
	adminUser := flag.String("admin-user", envOr("SOUNDING_ADMIN_USER", "admin"), "管理后台用户名")
	adminPass := flag.String("admin-pass", os.Getenv("SOUNDING_ADMIN_PASS"), "管理后台密码（为空=不启用 JWT 登录；可用 $SOUNDING_ADMIN_PASS）")
	jwtSecret := flag.String("jwt-secret", os.Getenv("SOUNDING_JWT_SECRET"), "JWT 签名密钥（为空则自动生成；可用 $SOUNDING_JWT_SECRET）")
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

	// 未显式配置的 token/密钥：持久化到 SQLite，重启后保持不变
	ensureSecret := func(flagVal *string, key, label string, logAlways bool) {
		if *flagVal != "" {
			return
		}
		v, created, err := st.GetOrCreateSecret(key)
		if err != nil {
			log.Fatalf("secret %s: %v", key, err)
		}
		*flagVal = v
		if created {
			log.Printf("已生成 %s: %s", label, v)
		} else if logAlways {
			log.Printf("使用已保存的 %s: %s", label, v)
		}
	}
	ensureSecret(agentToken, "agent_token", "Agent Token", true)
	ensureSecret(adminToken, "admin_token", "Admin Token", true)
	ensureSecret(jwtSecret, "jwt_secret", "JWT Secret", *adminPass != "")
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
		// 告警规则持久化：已存在的以库为准，空库用 CLI 参数播种
		rules, err := st.LoadAlertRules()
		if err != nil {
			log.Printf("load alert rules: %v", err)
		}
		if len(rules) == 0 {
			if *alertOffline {
				rules = append(rules, alert.Rule{Kind: "offline", Node: "*", Threshold: 1})
			}
			if *alertLatency > 0 {
				rules = append(rules, alert.Rule{Kind: "latency", Node: "*", Threshold: *alertLatency})
			}
			rules = append(rules, alert.Rule{Kind: "loss", Node: "*", Threshold: 100})
			for _, r := range rules {
				_ = st.SaveAlertRule(r)
			}
		} else if *alertLatency > 0 {
			// CLI 显式指定延迟阈值时覆盖全局延迟规则
			r := alert.Rule{Kind: "latency", Node: "*", Threshold: *alertLatency}
			_ = st.SaveAlertRule(r)
			replaced := false
			for i := range rules {
				if rules[i].Kind == "latency" && rules[i].Node == "*" {
					rules[i] = r
					replaced = true
				}
			}
			if !replaced {
				rules = append(rules, r)
			}
		}
		alertMgr = alert.NewManager(rules, notifiers, 5*time.Minute)
		log.Printf("alerts enabled: %d 渠道 · %d 规则", len(notifiers), len(rules))
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
		log.Printf("alert watcher started (every 30s)")
	}

	h := api.NewHandler(st)
	h.SetVersion(version.Version)
	h.SetPublicSettings(func() map[string]any { return api.PublicSettings() })
	agentH := api.NewAgentEndpoint(st, *agentToken)
	adminH := api.NewAdminEndpoint(st, *adminToken)
	adminH.SetStatusProvider(func() any { m, _ := st.LatestStatus(); return m })
	adminH.SetJWTAuth(*jwtSecret)
	if alertMgr != nil {
		adminH.SetAlertHooksEx(
			func() any { return alertMgr.Rules() },
			func(kind, node string, threshold float64, _, silenceUntil string, muteWindows []string) error {
				r := alert.Rule{Kind: kind, Node: node, Threshold: threshold, SilenceUntil: silenceUntil, MuteWindows: muteWindows}
				alertMgr.SetRule(r)
				return st.SaveAlertRule(r)
			},
			func(kind, node string) error {
				alertMgr.DeleteRule(kind, node)
				return st.DeleteAlertRule(kind, node)
			},
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
				writeJSON(w, api.PublicSettingsResponse())
			case "/version":
				// ink 的 REST 契约：{status, data:{version, hash}}
				writeJSON(w, map[string]any{"status": "success", "data": map[string]any{"version": version.Version, "hash": version.Commit}})
			}
		})
	}
	// /api/* 与根路径同实现（ink 默认 base=/api）
	mux.HandleFunc("/api/me", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, map[string]any{"logged_in": false}) })
	mux.HandleFunc("/api/public", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, api.PublicSettingsResponse()) })
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"status": "success", "data": map[string]any{"version": version.Version, "hash": version.Commit}})
	})
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
		fs := http.FileServer(http.Dir(*staticDir))
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// SPA fallback：非静态资源路径（如 /node/xxx）回退 index.html
			path := filepath.Join(*staticDir, filepath.Clean(r.URL.Path))
			if _, err := os.Stat(path); err != nil && r.URL.Path != "/" {
				http.ServeFile(w, r, filepath.Join(*staticDir, "index.html"))
				return
			}
			fs.ServeHTTP(w, r)
		}))
		log.Printf("serving dashboard from %s", *staticDir)
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

// envOr 读取环境变量或返回默认值（flag 默认值用，flag 显式传参会覆盖）。
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
