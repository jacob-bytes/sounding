#!/usr/bin/env sh
# sounding 演示站部署（多平台）
#   ./scripts/deploy-demo.sh render   → 打印 Render 部署指引（免费、无需卡）
#   ./scripts/deploy-demo.sh koyeb    → 打印 Koyeb 部署指引（免费、无需卡）
#   ./scripts/deploy-demo.sh fly      → Fly.io（需绑定支付方式）
set -e

function log() { printf '%s\n' "$1"; }

function render_guide() {
  log "=== Render 部署（免费，无需信用卡）==="
  log "1. 注册：https://render.com/  （GitHub 登录即可）"
  log "2. Dashboard → New + → Blueprint"
  log "3. 选择仓库 jacob-bytes/sounding（首次需授权 GitHub）"
  log "4. Render 自动读取 render.yaml → 点 Apply"
  log "5. 等待构建（约 3-5 分钟）→ 获得 https://sounding-demo.onrender.com"
  log ""
  log "管理页: https://<你的域名>/admin/  Token: demo-admin-token"
  log "注意: 免费实例 15 分钟无访问会休眠，首次访问需 ~30s 冷启动"
}

function koyeb_guide() {
  log "=== Koyeb 部署（免费 1 实例，无需卡）==="
  log "1. 注册：https://app.koyeb.com/auth/signup （GitHub 登录）"
  log "2. Create Service → Docker → 镜像: ghcr.io/jacob-bytes/sounding:latest"
  log "3. 端口 8080 · 健康检查 /healthz · 实例类型 Free"
  log "4. 环境变量: SOUNDING_AGENT_TOKEN=demo-agent-token"
  log "5. 启动命令: sounding-server -addr :8080 -db /tmp/sounding.db -seed -static /admin -agent-token demo-agent-token -admin-token demo-admin-token"
}

function fly_guide() {
  log "=== Fly.io（需绑定支付方式——免费额度内不扣费）==="
  log "fly auth login && fly launch --copy-config && fly volumes create sounding_data --size 1 && fly deploy"
}

case "${1:-render}" in
  render) render_guide ;;
  koyeb) koyeb_guide ;;
  fly) fly_guide ;;
  *) log "用法: $0 [render|koyeb|fly]" ;;
esac
