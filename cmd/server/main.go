package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/jacob-bytes/sounding/internal/api"
	"github.com/jacob-bytes/sounding/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
	dbPath := flag.String("db", "sounding.db", "SQLite 数据库路径")
	seed := flag.Bool("seed", true, "启动时写入演示数据")
	agentToken := flag.String("agent-token", "sounding-demo-token", "Agent 上报认证 token")
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

	h := api.NewHandler(st)
	agentH := api.NewAgentEndpoint(st, *agentToken)
	mux := http.NewServeMux()

	// ink 契约：JSON-RPC 2.0 POST <base>/rpc2
	mux.Handle("/rpc2", h)
	// Agent 上报：POST /agent/status
	mux.Handle("/agent/status", agentH)

	log.Printf("sounding server listening on %s (rpc2: /rpc2)", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
