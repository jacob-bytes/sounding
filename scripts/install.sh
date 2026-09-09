#!/usr/bin/env sh
# sounding 一键安装 / 更新 / 部署脚本（Linux / macOS）
#
# 主控一键部署（二进制 + ink 面板 + systemd）：
#   curl -fsSL https://raw.githubusercontent.com/jacob-bytes/sounding/main/scripts/install.sh | sh -s -- deploy
#
# 节点 Agent 一键接入：
#   SOUNDING_AGENT_SERVER=http://主控:8080 SOUNDING_AGENT_TOKEN=xxx \
#     sh -s -- agent < install.sh
#
# 仅安装/更新二进制：
#   sh install.sh install server      # 主控（自动下载 ink 面板）
#   sh install.sh install agent
#   sh install.sh update server
#
# systemd：
#   SOUNDING_AGENT_TOKEN=xxx SOUNDING_ADMIN_TOKEN=yyy sh install.sh systemd server
#   SOUNDING_AGENT_SERVER=http://主控:8080 SOUNDING_AGENT_TOKEN=xxx sh install.sh systemd agent
#
# 其它：
#   sh install.sh status  [server|agent]
#   sh install.sh uninstall [server|agent]     # 保留数据，需删数据加 REMOVE_DATA=1
#   sh install.sh docker                        # 生成 token 并 docker run 主控
#
# 环境变量：
#   BIN_DIR=/usr/local/bin       二进制目录
#   SOUNDING_VERSION=v0.3.0      指定版本（默认 latest）
#   SOUNDING_AGENT_TOKEN / SOUNDING_ADMIN_TOKEN / SOUNDING_ADMIN_USER / SOUNDING_ADMIN_PASS
#   SOUNDING_AGENT_SERVER        Agent 指向的主控地址
#   SOUNDING_ADMIN_DIR           ink 面板目录（默认 /var/lib/sounding/admin）
#   SOUNDING_IMAGE                docker 镜像（默认 ghcr.io/jacob-bytes/sounding:latest）
#   SOUNDING_PORT                监听端口（默认 8080）

set -e

REPO="jacob-bytes/sounding"
BIN_DIR="${BIN_DIR:-/usr/local/bin}"
SOUNDING_VERSION="${SOUNDING_VERSION:-latest}"
SERVICE_USER="sounding"
DATA_DIR="${SOUNDING_DATA_DIR:-/var/lib/sounding}"
SOUNDING_PORT="${SOUNDING_PORT:-8080}"
SOUNDING_IMAGE="${SOUNDING_IMAGE:-ghcr.io/jacob-bytes/sounding:latest}"

function log() { printf '%s\n' "$1"; }
function die() { printf '错误: %s\n' "$1" >&2; exit 1; }
function need_sudo() { [ "$(id -u)" -ne 0 ] && printf 'sudo'; }

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

function gen_token() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 24
  elif [ -r /dev/urandom ]; then
    od -An -N24 -tx1 /dev/urandom | tr -d ' \n'
  else
    die "无法生成随机 token（缺少 openssl/urandom）"
  fi
}

function resolve_tag() {
  if [ "$SOUNDING_VERSION" != "latest" ]; then
    printf '%s' "$SOUNDING_VERSION"
    return
  fi
  tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)
  [ -n "$tag" ] || die "未找到 Release（可用 SOUNDING_VERSION=vX.Y.Z 指定）"
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
    $(need_sudo) mv "$tmp" "$BIN_DIR/sounding-$role"
  fi
  log "✓ 已安装: $BIN_DIR/sounding-$role ($tag)"
}

function install_ink_dashboard() {
  ink_dir="${SOUNDING_ADMIN_DIR:-$DATA_DIR/admin}"
  ink_repo="jacob-bytes/komari-theme-ink"
  command -v unzip >/dev/null 2>&1 || { log "跳过 ink 面板（缺少 unzip，可 apt/yum install unzip）"; return; }
  ink_tag=$(curl -fsSL "https://api.github.com/repos/$ink_repo/releases/latest" \
    | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)
  [ -n "$ink_tag" ] || { log "跳过 ink 面板（未找到 Release）"; return; }

  log "→ 下载 ink 监控面板 ($ink_tag)"
  tmpdir=$(mktemp -d)
  zip_url="https://github.com/$ink_repo/releases/download/$ink_tag/ink-build-${ink_tag#v}.zip"
  curl -fsSL "$zip_url" -o "$tmpdir/ink.zip" || { log "面板下载失败——跳过"; rm -rf "$tmpdir"; return; }
  unzip -q -o "$tmpdir/ink.zip" -d "$tmpdir/ink" || { log "解压失败"; rm -rf "$tmpdir"; return; }
  $(need_sudo) mkdir -p "$ink_dir"
  $(need_sudo) cp -r "$tmpdir/ink/dist/." "$ink_dir/" 2>/dev/null || log "未找到 dist 目录"
  rm -rf "$tmpdir"
  log "✓ 监控面板: $ink_dir"
}

function ensure_service_user() {
  if ! id "$SERVICE_USER" >/dev/null 2>&1; then
    $(need_sudo) useradd --system --home "$DATA_DIR" --shell /usr/sbin/nologin "$SERVICE_USER" 2>/dev/null || true
  fi
  $(need_sudo) mkdir -p "$DATA_DIR"
  $(need_sudo) chown -R "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR" 2>/dev/null || true
}

function setup_systemd() {
  role=$1
  [ "$(detect_os)" = "linux" ] || die "systemd 仅支持 Linux（macOS 请直接运行二进制或使用 docker）"
  ensure_service_user

  unit="/etc/systemd/system/sounding-$role.service"
  if [ "$role" = "server" ]; then
    exec_start="$BIN_DIR/sounding-server -addr :$SOUNDING_PORT -db $DATA_DIR/sounding.db -static $DATA_DIR/admin -retain-days 30"
    [ -n "$SOUNDING_AGENT_TOKEN" ] && exec_start="$exec_start -agent-token $SOUNDING_AGENT_TOKEN"
    [ -n "$SOUNDING_ADMIN_TOKEN" ] && exec_start="$exec_start -admin-token $SOUNDING_ADMIN_TOKEN"
    [ -n "$SOUNDING_ADMIN_USER" ] && exec_start="$exec_start -admin-user $SOUNDING_ADMIN_USER"
    [ -n "$SOUNDING_ADMIN_PASS" ] && exec_start="$exec_start -admin-pass $SOUNDING_ADMIN_PASS"
  else
    [ -n "$SOUNDING_AGENT_SERVER" ] || die "agent 需设置 SOUNDING_AGENT_SERVER=http://主控:$SOUNDING_PORT"
    [ -n "$SOUNDING_AGENT_TOKEN" ] || die "agent 需设置 SOUNDING_AGENT_TOKEN"
    exec_start="$BIN_DIR/sounding-agent -server $SOUNDING_AGENT_SERVER -token $SOUNDING_AGENT_TOKEN -node-uuid \$(hostname)"
  fi

  log "→ 写入 $unit"
  $(need_sudo) tee "$unit" > /dev/null <<UNIT
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

  $(need_sudo) systemctl daemon-reload
  $(need_sudo) systemctl enable --now "sounding-$role"
  sleep 1
  $(need_sudo) systemctl --no-pager status "sounding-$role" | head -8 || true
  log ""
  log "常用命令:"
  log "  sudo systemctl status sounding-$role"
  log "  sudo journalctl -u sounding-$role -f"
  log "  sudo systemctl restart sounding-$role"
}

function health_check() {
  url="http://127.0.0.1:$SOUNDING_PORT/healthz"
  i=0
  while [ $i -lt 10 ]; do
    if curl -fsS "$url" >/dev/null 2>&1; then
      log "✓ 健康检查通过: $url"
      return 0
    fi
    i=$((i + 1))
    sleep 1
  done
  log "⚠ 健康检查未通过（可能仍在启动，或端口被占用）：$url"
  return 1
}

function deploy_server() {
  check_deps
  install_bin server
  install_ink_dashboard
  [ -z "$SOUNDING_AGENT_TOKEN" ] && SOUNDING_AGENT_TOKEN=$(gen_token)
  [ -z "$SOUNDING_ADMIN_TOKEN" ] && SOUNDING_ADMIN_TOKEN=$(gen_token)
  export SOUNDING_AGENT_TOKEN SOUNDING_ADMIN_TOKEN

  log ""
  log "==================== 部署信息（请妥善保存） ===================="
  log "  Agent Token : $SOUNDING_AGENT_TOKEN"
  log "  Admin Token : $SOUNDING_ADMIN_TOKEN"
  log "=============================================================="
  log ""

  if [ "$(detect_os)" = "linux" ]; then
    setup_systemd server
    health_check || true
  else
    log "非 Linux 环境，直接启动："
    log "  $BIN_DIR/sounding-server -addr :$SOUNDING_PORT -db $DATA_DIR/sounding.db -static $DATA_DIR/admin \\"
    log "    -agent-token $SOUNDING_AGENT_TOKEN -admin-token $SOUNDING_ADMIN_TOKEN -retain-days 30"
  fi
  log ""
  log "管理页:   http://<主机>:$SOUNDING_PORT/admin/"
  log "监控面板: http://<主机>:$SOUNDING_PORT/  （ink 主题，零配置）"
  log "健康检查: http://<主机>:$SOUNDING_PORT/healthz"
  log ""
  log "节点接入（在每台被监控机器执行）:"
  log "  curl -fsSL https://raw.githubusercontent.com/$REPO/main/scripts/install.sh | \\"
  log "    SOUNDING_AGENT_SERVER=http://<主控>:$SOUNDING_PORT SOUNDING_AGENT_TOKEN=$SOUNDING_AGENT_TOKEN sh -s -- agent"
}

function deploy_agent() {
  check_deps
  install_bin agent
  if [ -z "$SOUNDING_AGENT_SERVER" ]; then
    log "未设置 SOUNDING_AGENT_SERVER，仅安装二进制。启动示例："
    log "  $BIN_DIR/sounding-agent -server http://<主控>:$SOUNDING_PORT -token <agent-token> -node-uuid \$(hostname)"
    return
  fi
  [ -n "$SOUNDING_AGENT_TOKEN" ] || die "缺少 SOUNDING_AGENT_TOKEN"
  if [ "$(detect_os)" = "linux" ]; then
    setup_systemd agent
  else
    log "非 Linux 环境，直接启动："
    log "  $BIN_DIR/sounding-agent -server $SOUNDING_AGENT_SERVER -token $SOUNDING_AGENT_TOKEN -node-uuid \$(hostname)"
  fi
}

function status_service() {
  role="${1:-server}"
  if [ "$(detect_os)" = "linux" ] && command -v systemctl >/dev/null 2>&1; then
    $(need_sudo) systemctl --no-pager status "sounding-$role" || true
  else
    log "sounding-$role 版本: $(current_version "$role")"
    log "二进制: $BIN_DIR/sounding-$role"
  fi
  if [ "$role" = "server" ]; then
    health_check || true
  fi
}

function uninstall_service() {
  role="${1:-server}"
  log "→ 停止并移除 sounding-$role"
  if [ "$(detect_os)" = "linux" ] && command -v systemctl >/dev/null 2>&1; then
    $(need_sudo) systemctl disable --now "sounding-$role" 2>/dev/null || true
    $(need_sudo) rm -f "/etc/systemd/system/sounding-$role.service"
    $(need_sudo) systemctl daemon-reload || true
  fi
  $(need_sudo) rm -f "$BIN_DIR/sounding-$role"
  if [ "${REMOVE_DATA:-0}" = "1" ]; then
    log "→ 删除数据目录 $DATA_DIR（REMOVE_DATA=1）"
    $(need_sudo) rm -rf "$DATA_DIR"
  else
    log "已保留数据目录: $DATA_DIR（如需删除请设置 REMOVE_DATA=1）"
  fi
  log "✓ 卸载完成"
}

function deploy_docker() {
  check_deps
  command -v docker >/dev/null 2>&1 || die "缺少 docker"
  [ -z "$SOUNDING_AGENT_TOKEN" ] && SOUNDING_AGENT_TOKEN=$(gen_token)
  [ -z "$SOUNDING_ADMIN_TOKEN" ] && SOUNDING_ADMIN_TOKEN=$(gen_token)
  ink_dir="${SOUNDING_ADMIN_DIR:-}"
  args="-d --name sounding --restart unless-stopped -p $SOUNDING_PORT:8080 -v sounding-data:/data"
  [ -n "$ink_dir" ] && args="$args -v $ink_dir:/admin"
  log "→ docker run $SOUNDING_IMAGE"
  # shellcheck disable=SC2086
  docker run $args \
    -e SOUNDING_AGENT_TOKEN="$SOUNDING_AGENT_TOKEN" \
    -e SOUNDING_ADMIN_TOKEN="$SOUNDING_ADMIN_TOKEN" \
    "$SOUNDING_IMAGE" -addr :8080 -db /data/sounding.db -retain-days 30
  sleep 1
  health_check || true
  log ""
  log "Agent Token : $SOUNDING_AGENT_TOKEN"
  log "Admin Token : $SOUNDING_ADMIN_TOKEN"
  log "管理页: http://<主机>:$SOUNDING_PORT/admin/   面板: http://<主机>:$SOUNDING_PORT/"
}

function usage() {
  log "sounding 一键脚本"
  log ""
  log "用法:"
  log "  install.sh deploy [server]        # 主控一键部署（二进制 + ink 面板 + systemd/启动命令）"
  log "  install.sh agent                  # 节点 Agent 一键接入（需 SOUNDING_AGENT_SERVER/TOKEN）"
  log "  install.sh install|update [role]  # 安装/更新二进制（role=server|agent）"
  log "  install.sh systemd [role]         # 生成并启用 systemd 服务"
  log "  install.sh status [role]          # 查看状态 + 健康检查"
  log "  install.sh uninstall [role]       # 卸载（默认保留数据；REMOVE_DATA=1 一并删除）"
  log "  install.sh docker                 # 生成 token 并用 docker 启动主控"
  log ""
  log "环境变量: BIN_DIR · SOUNDING_VERSION · SOUNDING_AGENT_TOKEN · SOUNDING_ADMIN_TOKEN"
  log "          SOUNDING_AGENT_SERVER · SOUNDING_PORT · SOUNDING_ADMIN_DIR · SOUNDING_IMAGE"
}

function main() {
  action="${1:-deploy}"
  shift 2>/dev/null || true
  case "$action" in
    deploy)
      role="${1:-server}"
      if [ "$role" = "agent" ]; then
        deploy_agent
      else
        deploy_server
      fi
      ;;
    agent)
      deploy_agent
      ;;
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
      log ""
      log "提示: 更新只需重跑本命令（配置/数据保留）；systemd 服务: sh install.sh systemd $role"
      ;;
    systemd)
      setup_systemd "${1:-server}"
      ;;
    status)
      status_service "${1:-server}"
      ;;
    uninstall|remove)
      uninstall_service "${1:-server}"
      ;;
    docker)
      deploy_docker
      ;;
    -h|--help|help)
      usage
      ;;
    *) die "未知操作: $action（见 install.sh help）" ;;
  esac
}

main "$@"
