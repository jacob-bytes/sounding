#!/usr/bin/env sh
# sounding 演示站一键部署（Fly.io）
# 前置：flyctl 已安装并 fly auth login
set -e

function log() { printf '%s\n' "$1"; }
function die() { printf '错误: %s\n' "$1" >&2; exit 1; }

function check_fly() {
  command -v fly >/dev/null 2>&1 || command -v flyctl >/dev/null 2>&1 || die "未安装 flyctl：curl -L https://fly.io/install.sh | sh"
}

function ensure_volume() {
  if fly volumes list 2>/dev/null | grep -q sounding_data; then
    log "✓ 卷 sounding_data 已存在"
  else
    log "→ 创建卷 sounding_data (1GB)"
    fly volumes create sounding_data --size 1 --region hkg --yes
  fi
}

function deploy() {
  log "→ 部署 sounding-demo"
  fly deploy --ha=false
  log ""
  log "✓ 部署完成"
  fly status
  log ""
  log "访问: https://sounding-demo.fly.dev"
  log "管理页: https://sounding-demo.fly.dev/admin/  (Token: demo-admin-token)"
}

function main() {
  check_fly
  ensure_volume
  deploy
}

main "$@"
