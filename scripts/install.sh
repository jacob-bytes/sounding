#!/usr/bin/env sh
# sounding 安装/更新脚本
# 安装：
#   curl -fsSL https://raw.githubusercontent.com/jacob-bytes/sounding/main/scripts/install.sh | sh -s -- server
#   curl -fsSL .../install.sh | sh -s -- agent
# 更新（保留配置）：
#   sh install.sh update server
# systemd 服务：
#   sh install.sh systemd server      # 生成并启用 systemd 服务
#
# 环境变量：BIN_DIR（默认 /usr/local/bin）· SOUNDING_VERSION（默认 latest）
#           SOUNDING_AGENT_TOKEN / SOUNDING_ADMIN_TOKEN / SOUNDING_ADMIN_DIR

set -e

REPO="jacob-bytes/sounding"
BIN_DIR="${BIN_DIR:-/usr/local/bin}"
SOUNDING_VERSION="${SOUNDING_VERSION:-latest}"
SERVICE_USER="sounding"
DATA_DIR="/var/lib/sounding"

function log() { printf '%s\n' "$1"; }
function die() { printf '错误: %s\n' "$1" >&2; exit 1; }

function detect_os() {
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$os" in
    linux|darwin) printf '%s' "$os" ;;
    *) die "不支持的操作系统: $os" ;;
  esac
}

function detect_arch() {
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64) printf 'amd64' ;;
    arm64|aarch64) printf 'arm64' ;;
    *) die "不支持的架构: $arch" ;;
  esac
}

function resolve_tag() {
  if [ "$SOUNDING_VERSION" != "latest" ]; then
    printf '%s' "$SOUNDING_VERSION"
    return
  fi
  tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)
  [ -n "$tag" ] || die "未找到 Release"
  printf '%s' "$tag"
}

function check_deps() {
  command -v curl >/dev/null 2>&1 || die "缺少 curl"
}

function current_version() {
  bin="$BIN_DIR/sounding-$1"
  if [ -x "$bin" ]; then
    "$bin" -version 2>&1 | head -1 | awk '{print $2}'
  else
    printf '未安装'
  fi
}

function install_bin() {
  role=$1
  os=$(detect_os)
  arch=$(detect_arch)
  tag=$(resolve_tag)
  bin="sounding-$role-$os-$arch"
  url="https://github.com/$REPO/releases/download/$tag/$bin"

  old=$(current_version "$role")
  log "当前版本: $old → 目标: $tag"
  log "→ 下载 $bin"
  tmp="/tmp/$bin.$$"
  curl -fsSL "$url" -o "$tmp" || die "下载失败: $url"
  chmod +x "$tmp"
  if [ -w "$BIN_DIR" ]; then
    mv "$tmp" "$BIN_DIR/sounding-$role"
  else
    sudo mv "$tmp" "$BIN_DIR/sounding-$role"
  fi
  log "✓ 已安装: $BIN_DIR/sounding-$role ($tag)"
}

function install_ink_dashboard() {
  ink_dir="${SOUNDING_ADMIN_DIR:-$DATA_DIR/admin}"
  ink_repo="jacob-bytes/komari-theme-ink"
  ink_tag=$(curl -fsSL "https://api.github.com/repos/$ink_repo/releases/latest" \
    | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)
  [ -n "$ink_tag" ] || { log "跳过面板（未找到 ink Release）"; return; }
  command -v unzip >/dev/null 2>&1 || { log "跳过面板（缺少 unzip）"; return; }

  log "→ 下载 ink 监控面板 ($ink_tag)"
  tmpdir=$(mktemp -d)
  zip_url="https://github.com/$ink_repo/releases/download/$ink_tag/ink-build-${ink_tag#v}.zip"
  curl -fsSL "$zip_url" -o "$tmpdir/ink.zip" || { log "面板下载失败——跳过"; rm -rf "$tmpdir"; return; }
  unzip -q -o "$tmpdir/ink.zip" -d "$tmpdir/ink" || { log "解压失败"; rm -rf "$tmpdir"; return; }
  mkdir -p "$ink_dir"
  cp -r "$tmpdir/ink/dist/." "$ink_dir/" 2>/dev/null || log "未找到 dist 目录"
  rm -rf "$tmpdir"
  log "✓ 监控面板: $ink_dir"
}

function setup_systemd() {
  role=$1
  shift
  [ "$(detect_os)" = "linux" ] || die "systemd 仅支持 Linux"

  if ! id "$SERVICE_USER" >/dev/null 2>&1; then
    sudo useradd --system --home "$DATA_DIR" --shell /usr/sbin/nologin "$SERVICE_USER" 2>/dev/null || true
  fi
  sudo mkdir -p "$DATA_DIR" && sudo chown -R "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR"

  unit="/etc/systemd/system/sounding-$role.service"
  if [ "$role" = "server" ]; then
    exec_start="$BIN_DIR/sounding-server -addr :8080 -db $DATA_DIR/sounding.db -static $DATA_DIR/admin -retain-days 30"
    [ -n "$SOUNDING_AGENT_TOKEN" ] && exec_start="$exec_start -agent-token $SOUNDING_AGENT_TOKEN"
    [ -n "$SOUNDING_ADMIN_TOKEN" ] && exec_start="$exec_start -admin-token $SOUNDING_ADMIN_TOKEN"
  else
    [ -n "$SOUNDING_AGENT_SERVER" ] || die "agent 需设置 SOUNDING_AGENT_SERVER=http://主控:8080"
    [ -n "$SOUNDING_AGENT_TOKEN" ] || die "agent 需设置 SOUNDING_AGENT_TOKEN"
    exec_start="$BIN_DIR/sounding-agent -server $SOUNDING_AGENT_SERVER -token $SOUNDING_AGENT_TOKEN -node-uuid \$(hostname)"
  fi

  log "→ 写入 $unit"
  sudo tee "$unit" > /dev/null <<UNIT
[Unit]
Description=sounding $role
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$SERVICE_USER
ExecStart=$exec_start
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
UNIT

  sudo systemctl daemon-reload
  sudo systemctl enable --now "sounding-$role"
  sleep 1
  sudo systemctl --no-pager status "sounding-$role" | head -8 || true
  log ""
  log "常用命令:"
  log "  sudo systemctl status sounding-$role"
  log "  sudo journalctl -u sounding-$role -f"
  log "  sudo systemctl restart sounding-$role"
}

function print_usage_hint() {
  role=$1
  shift
  log ""
  if [ "$role" = "server" ]; then
    log "启动示例:"
    log "  sounding-server -addr :8080 -db $DATA_DIR/sounding.db -static $DATA_DIR/admin \\"
    log "    -agent-token <agent-token> -admin-token <admin-token> -retain-days 30"
    log ""
    log "管理页:   http://localhost:8080/admin/"
    log "监控面板: http://localhost:8080/"
  else
    log "启动示例:"
    log "  sounding-agent -server http://<主控>:8080 -token <token> -node-uuid \$(hostname) $*"
    log ""
    log "探针语法: -probe \"上海移动:icmp:223.5.5.5,百度:http:https://baidu.com\""
  fi
}

function main() {
  action="${1:-install}"
  shift 2>/dev/null || true
  case "$action" in
    install|update)
      role="${1:-server}"
      shift 2>/dev/null || true
      case "$role" in
        server|agent) ;;
        *) die "未知角色: $role（可选 server / agent）" ;;
      esac
      check_deps
      install_bin "$role"
      [ "$role" = "server" ] && install_ink_dashboard
      print_usage_hint "$role" "$@"
      log ""
      log "提示: 更新只需重跑本命令（配置/数据保留）；systemd 服务: sh install.sh systemd $role"
      ;;
    systemd)
      role="${1:-server}"
      shift 2>/dev/null || true
      setup_systemd "$role" "$@"
      ;;
    -h|--help|help)
      log "用法:"
      log "  install.sh [install|update] [server|agent]   # 安装/更新二进制（+ink 面板）"
      log "  install.sh systemd [server|agent]            # 生成 systemd 服务并启动"
      ;;
    *) die "未知操作: $action（install / update / systemd）" ;;
  esac
}

main "$@"
