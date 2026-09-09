# 配置手册

## 添加服务器

**① 自动注册（推荐）**：Agent 启动即注册，上报即心跳（超时 60s 判定离线）。

```bash
./sounding-agent -server http://主控:8080 -token <agent-token> -node-uuid n1
```

**② 手动 API**：

```bash
curl -X POST http://主控:8080/api/admin/nodes -H 'X-Admin-Token: <admin-token>' \
  -H 'Content-Type: application/json' \
  -d '{"uuid":"manual-01","name":"东京节点","region":"JP","cpu_cores":4,"mem_total":8589934592}'
```

## 探针配置

**全局 / 节点级**（管理 API）：

```bash
# 全局任务（所有节点执行）
curl -X POST http://主控:8080/api/admin/probes -H 'X-Admin-Token: <token>' \
  -H 'Content-Type: application/json' \
  -d '{"client":"*","target":"223.5.5.5","name":"阿里 DNS","type":"icmp","interval_sec":60}'

# 节点级任务（client=<uuid>）
curl -X POST http://主控:8080/api/admin/probes -H 'X-Admin-Token: <token>' \
  -H 'Content-Type: application/json' \
  -d '{"client":"n1","target":"https://baidu.com","name":"百度","type":"http","interval_sec":60}'
```

**Agent 启动参数声明**（自动 upsert 为 `client=<uuid>` 任务）：

```bash
# 语法：名称[:类型]:主机，逗号分隔；类型可选 icmp/tcp/http/https/dns（默认 icmp）
./sounding-agent -server http://主控:8080 -node-uuid n1 \
  -probe "上海移动:223.5.5.5,腾讯 DNS:119.29.29.29,百度:http:https://baidu.com"
```

> Agent 每 30s 从主控拉取探针配置（`-remote-config`，默认开）；管理页改任务后自动生效。

## 管理 API

| 端点 | 方法 | 用途 |
|---|---|---|
| `/api/admin/nodes` | GET / POST / DELETE | 查看 / 手动添加 / 删除节点 |
| `/api/admin/probes` | GET / POST / DELETE | 查看 / 配置 / 删除探针任务 |
| `/api/admin/alerts` | GET / POST / DELETE | 告警规则 CRUD（kind/node/threshold/silence_until/mute_windows） |
| `/agent/config` | GET | Agent 拉取远程配置（`X-Auth-Token`） |
| `/agent/status` | POST | Agent 上报（`X-Auth-Token`） |
| `/api/login` | POST | JWT 登录（需 `-admin-pass`） |
| `/admin/` | GET | 内置管理页（Go embed） |

认证：管理 API 用 `X-Admin-Token` 或 `Authorization: Bearer <jwt>`；Agent 用 `-agent-token`。

## Agent 配置方式

优先级：**CLI flag > 环境变量 > 配置文件 > 默认值**（server/token/probes 每次上报读取最新值）。

```yaml
# agent.yml（-config 指定；SIGHUP 或编辑保存 30s 内热重载）
server: http://localhost:8080
token: sounding-demo-token
node_uuid: n1
interval: 15s
probes:
  - "上海移动:223.5.5.5"
  - "腾讯 DNS:119.29.29.29"
```

```bash
./sounding-agent -config ./agent.yml
kill -HUP <pid>          # 手动热重载
```

| 环境变量 | 对应 flag | 说明 |
|---|---|---|
| `SOUNDING_SERVER` | `-server` | 主控地址（必配） |
| `SOUNDING_TOKEN` | `-token` | 上报 token |
| `SOUNDING_NODE_UUID` | `-node-uuid` | 节点 ID（默认主机名） |
| `SOUNDING_PROBES` | `-probe` | 本节点延迟目标 |
| `SOUNDING_INTERVAL` | `-interval` | 采集周期 |

主控环境变量：`SOUNDING_AGENT_TOKEN`、`SOUNDING_ADMIN_TOKEN`、`SOUNDING_ADMIN_USER`、`SOUNDING_ADMIN_PASS`、`SOUNDING_JWT_SECRET`。

## 告警

```bash
# 通知渠道（可同时启用，5 分钟去重）
-alert-webhook <url>                       # 通用 JSON POST
-telegram-token <bot> -telegram-chat-id <id>
-dingtalk-webhook <url> [-dingtalk-secret <s>]
-feishu-webhook <url>
-smtp-host <host:587> -smtp-user <u> -smtp-pass <p> -smtp-from <f> -smtp-to <a,b>

# 默认规则（首次启动播种到 SQLite，之后以管理页/API 为准）
-alert-offline=true
-alert-latency-ms 200
```

规则支持 `silence_until`（静默截止）与 `mute_windows`（每日静默时段，如 `02:00-04:00`）。新增通知渠道：实现 `internal/notify.Notifier`（`Name()` + `Send()`）并在 `cmd/server/main.go` 注册。

## 其它主控参数

| flag | 默认 | 说明 |
|---|---|---|
| `-addr` | `:8080` | 监听地址 |
| `-db` | `sounding.db` | SQLite 路径（WAL） |
| `-seed` | `false` | 写入演示数据（演示/联调） |
| `-static` | 自动探测 `./admin`、`/var/lib/sounding/admin` | 前端静态目录 |
| `-retain-days` | `30` | 历史保留天数（6 小时清理 + VACUUM） |
| `-admin-user` / `-admin-pass` | `admin` / 空 | 启用 JWT 登录 |
| `-jwt-secret` | 空 | 为空则随机生成并持久化 |
| `-agent-token` / `-admin-token` | 空 | 为空则随机生成并持久化（重启不变） |
