#!/usr/bin/env sh
# sounding 演示站部署（纯免费方案）
#   ./scripts/deploy-demo.sh vps       → 自有 VPS（推荐——最真实，含 Cloudflare Tunnel 可选）
#   ./scripts/deploy-demo.sh hf        → Hugging Face Spaces（免费无卡，需独立仓库）
#   ./scripts/deploy-demo.sh koyeb     → Koyeb（免费实例）
#   ./scripts/deploy-demo.sh local     → 本地演示（ngrok/cloudflared 临时公网）
set -e

function log() { printf '%s\n' "$1"; }

function vps_guide() {
  log "=== 自有 VPS 部署（推荐）==="
  log "在你有公网 IP 的服务器上执行："
  log ""
  log "  # 1. Docker 一键启动（含 ink 面板 + 演示数据）"
  log "  docker run -d --name sounding \\"
  log "    -p 8080:8080 -v sounding-data:/data \\"
  log "    ghcr.io/jacob-bytes/sounding:latest \\"
  log "    -addr :8080 -db /data/sounding.db -seed -static /admin \\"
  log "    -agent-token demo-agent-token -admin-token demo-admin-token -retain-days 7"
  log ""
  log "  # 2. 无域名/不想开端口？用 Cloudflare Tunnel（免费 HTTPS）"
  log "  cloudflared tunnel --url http://localhost:8080"
  log "  # → 得到 https://xxx.trycloudflare.com 临时公网地址"
  log ""
  log "  访问: http://<你的IP>:8080/        监控面板（ink）"
  log "        http://<你的IP>:8080/admin/  管理页（Token: demo-admin-token）"
}

function hf_guide() {
  log "=== Hugging Face Spaces（免费、无需卡、无休眠）==="
  log "1. 注册 https://huggingface.co/join （邮箱即可）"
  log "2. New Space → SDK: Docker → 模板 Blank"
  log "3. 上传文件：Dockerfile（本仓库根目录那份）"
  log "4. 在 Space 的 README.md 顶部加 YAML 头："
  log "   ---"
  log "   title: sounding demo"
  log "   sdk: docker"
  log "   app_port: 8080"
  log "   ---"
  log "5. 设置启动命令（Space Settings → Docker command）:"
  log "   sounding-server -addr :8080 -db /tmp/sounding.db -seed -static /admin \\"
  log "     -agent-token demo-agent-token -admin-token demo-admin-token"
  log ""
  log "  访问: https://<用户名>-sounding-demo.hf.space/"
}

function koyeb_guide() {
  log "=== Koyeb（免费 1 实例，无卡）==="
  log "1. 注册 https://app.koyeb.com/auth/signup （GitHub 登录）"
  log "2. Create Service → Docker → ghcr.io/jacob-bytes/sounding:latest"
  log "3. Port 8080 · Health check /healthz · Instance: Free (nano)"
  log "4. Docker command:"
  log "   sounding-server -addr :8080 -db /tmp/sounding.db -seed -static /admin \\"
  log "     -agent-token demo-agent-token -admin-token demo-admin-token"
  log "5. 环境变量 SOUNDING_AGENT_TOKEN=demo-agent-token"
}

function local_guide() {
  log "=== 本地演示（最快）==="
  log "  # 本地跑 + 临时公网（需 cloudflared 或 ngrok）"
  log "  ./sounding-server -addr :8080 -db /tmp/sounding.db -seed -static ./admin"
  log "  cloudflared tunnel --url http://localhost:8080   # 或 ngrok http 8080"
  log ""
  log "  # 模拟节点上报（可选——演示实时数据）"
  log "  ./sounding-agent -server http://localhost:8080 -token demo-agent-token -node-uuid demo-node"
}

case "${1:-vps}" in
  vps) vps_guide ;;
  hf) hf_guide ;;
  koyeb) koyeb_guide ;;
  local) local_guide ;;
  *) log "用法: $0 [vps|hf|koyeb|local]" ;;
esac
