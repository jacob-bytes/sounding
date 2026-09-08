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
- **探针调度器**：Ping（TCP 拨号延迟）/HTTP 探针任务——结果经 `getPingRecords` 暴露给 ink 前端
- **管理后台**：`-static <ink-dist>` 直接挂载 [komari-theme-ink](https://github.com/jacob-bytes/komari-theme-ink) 构建产物——**ink 前端即管理后台**
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

接入 ink 前端（`.env` 设置 `VITE_API_BASE=http://localhost:8080`）——首页/详情即跑通：

```bash
# 完整运行（主控 + 探针 + 管理后台）
./sounding-server -addr :8080 -db sounding.db -static ./ink-dist
# → http://localhost:8080 即 ink 管理后台（图表/探针/节点全可看）
```

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



## 配置手册（M4）

### 添加服务器（两种方式）

**① 一键自动**：Agent 启动即自动注册（推荐）：

```bash
./sounding-agent -server http://主控:8080 -token <agent-token> -node-uuid n1
```

**② 手动 API**：

```bash
curl -X POST http://主控:8080/api/admin/nodes -H 'X-Admin-Token: <admin-token>' \
  -H 'Content-Type: application/json' \
  -d '{"uuid":"manual-01","name":"东京节点","region":"JP","cpu_cores":4,"mem_total":8589934592}'
```

### 不同服务器不同延迟目标（per-node）

```bash
# Agent 启动参数声明本节点测试目标（"名称:主机" 逗号分隔）
./sounding-agent -server http://主控:8080 -node-uuid n1 \
  -probe "上海移动:223.5.5.5,腾讯 DNS:119.29.29.29"
```
→ 主控自动创建 `client=n1` 的探针任务（与全局任务共存，结果经 getPingRecords 返回）

**管理 API 一览**：

| 端点 | 方法 | 用途 |
|---|---|---|
| `/api/admin/nodes` | GET / POST | 查看 / 手动添加服务器 |
| `/api/admin/probes` | GET / POST / DELETE | 查看 / 配置 / 删除探针任务（`client=*` 全局 / `client=n1` 节点级） |
| `/api/admin/alerts` | GET / POST / DELETE | 告警规则运行时 CRUD（kind/node/threshold） |
| `/agent/config` | GET | Agent 拉取远程配置（探针目标下发） |
| `/admin/` | GET | **内置管理页**（节点/探针/告警可视化） |
| `/agent/status` | POST | Agent 上报（`X-Auth-Token`） |

> 认证：管理 API 用 `-admin-token`；Agent 上报用 `-agent-token`。



## 配置方式（CLI / 环境变量 / 配置文件）

纯 CLI 不够灵活？支持**配置文件 + 热重载**（改配置无需重启）：

```yaml
# agent.yml（-config 指定；SIGHUP 或编辑保存即热重载）
server: http://localhost:8080
token: sounding-demo-token
node_uuid: n1
interval: 15s
probes:
  - "上海移动:223.5.5.5"
  - "腾讯 DNS:119.29.29.29"
```

```bash
./sounding-agent -config ./agent.yml          # 文件配置（推荐）
kill -HUP <pid>                                # 手动热重载
# 或直接编辑保存 agent.yml（30s 内自动感知）
```

**优先级**：`CLI flag > 环境变量 > 配置文件 > 默认值`（server/token/probes 每次上报都读最新值——**修改即刻生效**）。


## 部署模型（开源使用说明）

**每个使用者自托管自己的主控**——Agent 只需指向自己的主控地址：

```bash
# 使用者 A（推荐容器/环境变量方式）
SOUNDING_SERVER=https://a.example.com \
SOUNDING_TOKEN=<a-token> \
SOUNDING_PROBES="上海移动:223.5.5.5,腾讯 DNS:119.29.29.29" \
  ./sounding-agent -node-uuid a-n1

# 使用者 B（CLI 方式指自己的主控）
./sounding-agent -server http://b.example.com:8080 -token <b-token> -node-uuid b-n1
```

**配置优先级**：`CLI flag > 环境变量 > 默认值`

| 环境变量 | 对应 flag | 说明 |
|---|---|---|
| `SOUNDING_SERVER` | `-server` | 主控地址（**必配——指到自己的主控**） |
| `SOUNDING_TOKEN` | `-token` | 上报 token |
| `SOUNDING_NODE_UUID` | `-node-uuid` | 节点 ID（默认主机名） |
| `SOUNDING_PROBES` | `-probe` | 本节点延迟目标 |
| `SOUNDING_INTERVAL` | `-interval` | 采集周期 |

> 主控侧：`-agent-token` 与 `-admin-token` 由部署者自行设定——**不同部署者互不干扰**（各自的 server 管各自的 agents）。

## Docker / Release（M5）

```bash
docker compose up -d   # 主控（挂载 ./admin 为 ink 管理后台）
# 发布：打 tag vX.Y.Z → Actions 自动构建 4 平台二进制 → Release
```

## 与 ink 前端联调（已验证）

```bash
# 构建 ink（指向 sounding）
cd komari-theme-ink && VITE_API_BASE=http://localhost:8080 bun run build
# 主控挂载管理后台
./sounding-server -addr :8080 -db sounding.db -static ./komari-theme-ink/dist
# → http://localhost:8080 即完整面板（初始化/设置/健康检查/数据新鲜度全通）
```

## 运维与安全（M7-M8）

| 能力 | 用法 |
|---|---|
| **ICMP 真探测** | 默认启用（非特权 ICMP，失败自动回退 TCP） |
| **离线判定** | 60s 无上报标记离线（前端卡片灰显） |
| **告警通知** | **Telegram**：`-telegram-token <bot> -telegram-chat-id <id>`；**Webhook**：`-alert-webhook <url>`（可同时启用，5 分钟去重） |
| **JWT 登录** | `-admin-user admin -admin-pass <pw> -jwt-secret <secret>` → `POST /api/login` |
| **历史保留** | `-retain-days 30`（6 小时清理 + VACUUM） |
| **健康检查** | `GET /healthz`（LB/K8s 探针） |
| **实时通道** | `/public` 返回 `theme_settings.rpcTransportMode=websocket` → ink 自动走 WS（秒级推送） |
| **一键安装** | `curl -fsSL .../scripts/install.sh \| sh -s -- server` |
| **内置管理页** | `http://<主控>:8080/admin/`（Go embed 单文件——节点/探针/告警规则可视化配置） |
| **Agent 远程配置** | 主控改探针 → Agent 30s 内自动拉取（`-remote-config`，默认开） |

### 集群化（P2-3）

主控为**无状态服务**（唯一状态在 SQLite）——水平扩展方案：
1. **单写多读**：主控实例共享同一 SQLite（NFS/云盘）+ 外部 LB（`/healthz` 健康检查）
2. **换存储**：将 `internal/store` 适配到 PostgreSQL（接口已隔离，替换实现即可）
3. Agent 侧无状态——指向 LB 地址即可（任一主控可处理上报）

## 通知渠道扩展（Notifier 接口）

新增渠道只需实现 `internal/notify.Notifier`（`Name()` + `Send(ctx, Message)`）——当前已内置：

| 渠道 | 参数 |
|---|---|
| **Telegram** | `-telegram-token` + `-telegram-chat-id` |
| Webhook | `-alert-webhook <url>`（JSON POST） |

后续可加钉钉/飞书/邮件（同一接口，注册即用）。

## 致谢

- 前端契约金标准：[komari-theme-ink](https://github.com/jacob-bytes/komari-theme-ink)
- 上游平台：[Komari Monitor](https://github.com/elevenhq/komari-monitor)

MIT License——欢迎 Star / Issue / PR 共建。
