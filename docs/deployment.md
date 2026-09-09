# 部署与运维

## 一键部署（推荐）

```bash
# 主控：二进制 + ink 面板 + systemd + 自动生成 token + 健康检查
curl -fsSL https://raw.githubusercontent.com/jacob-bytes/sounding/main/scripts/install.sh | sh -s -- deploy

# 节点 Agent：一条命令接入
curl -fsSL https://raw.githubusercontent.com/jacob-bytes/sounding/main/scripts/install.sh | \
  SOUNDING_AGENT_SERVER=http://<主控>:8080 SOUNDING_AGENT_TOKEN=<agent-token> sh -s -- agent

# Docker 一键启动（自动生成 token）
curl -fsSL https://raw.githubusercontent.com/jacob-bytes/sounding/main/scripts/install.sh | sh -s -- docker
```

`deploy` 会自动：识别 OS/架构 → 下载 `sounding-server` → 下载 ink 面板到 `/var/lib/sounding/admin` → 生成 token 并打印 → Linux 写入并启用 `sounding-server.service` → `curl /healthz` 健康检查 → 打印管理页/面板/Agent 接入命令。

**运维命令**：

```bash
sh scripts/install.sh status server       # 版本 + systemd 状态 + 健康检查
sh scripts/install.sh update server       # 更新（保留数据/配置）
sh scripts/install.sh uninstall server    # 卸载（默认保留数据，REMOVE_DATA=1 一并删除）
sh scripts/install.sh help                # 全部用法
```

**环境变量**：

| 变量 | 说明 |
|---|---|
| `SOUNDING_VERSION` | 锁定版本（默认 `latest`） |
| `SOUNDING_PORT` | 监听端口（默认 `8080`） |
| `SOUNDING_AGENT_TOKEN` / `SOUNDING_ADMIN_TOKEN` | 未设置则自动生成并持久化 |
| `SOUNDING_AGENT_SERVER` | Agent 指向的主控地址 |
| `SOUNDING_ADMIN_DIR` | ink 面板目录（默认 `/var/lib/sounding/admin`） |
| `SOUNDING_DATA_DIR` | 数据目录（默认 `/var/lib/sounding`） |
| `BIN_DIR` | 二进制目录（默认 `/usr/local/bin`） |

## Docker / Compose

```bash
docker compose up -d                      # 主控（healthcheck + ./data 持久化）
docker compose --profile agent up -d      # 同时启动本机 Agent（复用同一镜像）
docker compose logs -f server             # 查看日志
```

镜像同时包含 `sounding-server` 与 `sounding-agent`；token 通过环境变量注入，未设置时主控自动生成并持久化到 `/data`。

手动 `docker run`：

```bash
docker run -d --name sounding \
  -p 8080:8080 -v sounding-data:/data \
  -e SOUNDING_AGENT_TOKEN=changeme-agent \
  -e SOUNDING_ADMIN_TOKEN=changeme-admin \
  -e SOUNDING_ADMIN_USER=admin -e SOUNDING_ADMIN_PASS=changeme-pass \
  ghcr.io/jacob-bytes/sounding:latest
```

## 二进制 / 源码

```bash
go build -o sounding-server ./cmd/server
go build -o sounding-agent  ./cmd/agent

./sounding-server \
  -addr :8080 -db /var/lib/sounding/sounding.db \
  -static /var/lib/sounding/admin \
  -agent-token <agent-token> -admin-token <admin-token> \
  -admin-user admin -admin-pass <password> \
  -retain-days 30
```

## systemd（手动 unit 参考）

```ini
[Unit]
Description=sounding server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=sounding
ExecStart=/usr/local/bin/sounding-server -addr :8080 -db /var/lib/sounding/sounding.db \
  -static /var/lib/sounding/admin -agent-token <agent-token> -admin-token <admin-token> -retain-days 30
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload && sudo systemctl enable --now sounding-server
sudo systemctl status sounding-server
sudo journalctl -u sounding-server -f
```

## 更新

| 部署方式 | 更新方式 |
|---|---|
| 二进制 / systemd | `install.sh update <role>` + `sudo systemctl restart sounding-<role>` |
| Docker | `docker pull ghcr.io/jacob-bytes/sounding:latest && docker restart sounding` |
| 指定版本 | `SOUNDING_VERSION=v0.3.0 sh scripts/install.sh update server` |

## 部署后入口

| 入口 | 地址 |
|---|---|
| 管理页 | `http://<主机>:8080/admin/`（首次输入 Admin Token） |
| ink 监控面板 | `http://<主机>:8080/` |
| 健康检查 | `http://<主机>:8080/healthz` |
| Agent 接入 | `install.sh deploy` 会打印带 token 的命令 |

## 免费演示站

```bash
./scripts/deploy-demo.sh vps     # 自有 VPS + Cloudflare Tunnel（推荐）
./scripts/deploy-demo.sh hf      # Hugging Face Spaces
./scripts/deploy-demo.sh koyeb   # Koyeb 免费实例
./scripts/deploy-demo.sh local   # 本地 + 临时公网
```

## 集群化（水平扩展）

主控是**无状态服务**，唯一状态在 SQLite：

1. **单写多读**：多实例共享同一 SQLite（NFS/云盘）+ 外部 LB（`/healthz` 健康检查）。
2. **换存储**：将 `internal/store` 适配到 PostgreSQL（接口已隔离，替换实现即可）。
3. Agent 无状态：指向 LB 地址即可（任一主控可处理上报）。
