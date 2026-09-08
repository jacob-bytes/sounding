<div align="center">

# sounding

**兼容 Komari RPC 契约的自主分布式探针（Go）**

主控 + Agent 自托管实现——前端直接复用 [komari-theme-ink](https://github.com/jacob-bytes/komari-theme-ink) 主题。

[![License](https://img.shields.io/github/license/jacob-bytes/sounding?style=flat-square)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22-00add8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![PRs](https://img.shields.io/badge/PRs-welcome-brightgreen?style=flat-square)](https://github.com/jacob-bytes/sounding/pulls)

</div>

---

## 架构

```text
┌─────────────┐  HTTPS + token   ┌──────────────────┐  JSON-RPC 2.0  ┌──────────────┐
│ sounding-   │ ───────────────▶ │  sounding-server │ ──────────────▶ │              │
│ agent       │  上报/心跳        │  (主控/存储/API)  │  POST /rpc2    │ ink 前端     │
│ 节点采集     │ ◀─────────────── │  SQLite          │ ◀────────────── │ 仪表盘       │
└─────────────┘   配置/任务       └──────────────────┘                └──────────────┘
```

- **sounding-server**：接收 Agent 上报 + 历史存储 + 暴露 Komari 兼容 RPC（ink 零改动接入）
- **sounding-agent**：节点端采集（CPU/内存/磁盘/网络/系统/uptime）定时上报
- **契约**：`contracts/contracts.md`（字段级——以 ink 为金标准）

## 快速开始（M1+M2：主控 + Agent）

```bash
go build -o sounding-server ./cmd/server
./sounding-server -addr :8080 -db sounding.db   # 自动迁移 + 演示种子
```

验证端点（ink 同款 JSON-RPC 2.0）：

```bash
curl -X POST http://localhost:8080/rpc2 -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"getNodes","params":{}}'
# → result: [{"uuid":"demo-001","name":"Demo 节点 · 本地",...}]
```

### Agent（M2）

```bash
go build -o sounding-agent ./cmd/agent
./sounding-agent -server http://localhost:8080 -token sounding-demo-token -interval 15s
# 首次上报自动注册节点；上报即心跳（离线 = 超时无上报）
```

接入 ink 前端（`.env` 设置 `VITE_API_BASE=http://localhost:8080`）——首页/详情即跑通。

## 路线图

| 里程碑 | 内容 | 状态 |
|---|---|---|
| **M1** | 主控 3 端点（nodes/latest/recent）+ SQLite + 演示数据 | ✅ |
| M2 | Agent 采集器（gopsutil + 上报 + 心跳离线判定） | ⏳ |
| M3 | Ping/HTTP 探针任务（getPingRecords） | ⏳ |
| M4 | 认证（JWT）+ 完整契约（getClient/settings） | ⏳ |
| M5 | Docker 编排 + 跨平台 Release | ⏳ |

## 目录

```text
cmd/server        主控入口
cmd/agent        Agent 入口（M2）
internal/api      JSON-RPC 2.0 处理器（契约实现）
internal/store    SQLite 存储（nodes/status_history/ping_records）
internal/collect  Agent 采集（M2）
contracts/        RPC 契约清单（字段级）
```

## 致谢

- 前端契约金标准：[komari-theme-ink](https://github.com/jacob-bytes/komari-theme-ink)
- 上游平台：[Komari Monitor](https://github.com/elevenhq/komari-monitor)

MIT License——欢迎 Star / Issue / PR 共建。
