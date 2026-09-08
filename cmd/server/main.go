package main

import (
	"context"
	"flag"
	"log"
	"net/http"

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

	// 演示探针种子
	if *seed {
		_ = st.SeedProbeTasks()
	}
	sched := probe.NewScheduler(st, *probeClient)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sched.Start(ctx)

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
