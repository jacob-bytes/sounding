#!/usr/bin/env sh
# sounding 一键安装（主控 / Agent）
# 用法：
#   curl -fsSL https://raw.githubusercontent.com/jacob-bytes/sounding/main/scripts/install.sh | sh -s -- server
#   curl -fsSL .../install.sh | sh -s -- agent -server http://control:8080 -token <token>
# 可选环境变量：BIN_DIR（默认 /usr/local/bin）SOUNDING_VERSION（默认 latest）

set -e

REPO="jacob-bytes/sounding"
BIN_DIR="${BIN_DIR:-/usr/local/bin}"
SOUNDING_VERSION="${SOUNDING_VERSION:-latest}"

function log() {
  printf '%s\n' "$1"
}

function die() {
  printf '错误: %s\n' "$1" >&2
  exit 1
}

function detect_os() {
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$os" in
    linux|darwin) printf '%s' "$os" ;;
    *) die "不支持的操作系统: $os（仅支持 linux/darwin）" ;;
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
  [ -n "$tag" ] || die "未找到 Release（请检查网络或指定 SOUNDING_VERSION）"
  printf '%s' "$tag"
}

function check_deps() {
  command -v curl >/dev/null 2>&1 || die "缺少 curl"
  command -v unzip >/dev/null 2>&1 || log "提示: 未找到 unzip——将跳过监控面板安装"
}

function install_bin() {
  role=$1
  shift
  os=$(detect_os)
  arch=$(detect_arch)
  tag=$(resolve_tag)
  bin="sounding-$role-$os-$arch"
  url="https://github.com/$REPO/releases/download/$tag/$bin"

  log "→ 下载 $bin ($tag)"
  tmp="/tmp/$bin.$$"
  curl -fsSL "$url" -o "$tmp" || die "下载失败: $url"
  chmod +x "$tmp"

  if [ -w "$BIN_DIR" ]; then
    mv "$tmp" "$BIN_DIR/sounding-$role"
  else
    sudo mv "$tmp" "$BIN_DIR/sounding-$role"
  fi
  log "✓ 已安装: $BIN_DIR/sounding-$role"
}

function install_ink_dashboard() {
  # 安装 ink 监控面板（sounding-server 的展示前端）
  ink_dir="${SOUNDING_ADMIN_DIR:-/var/lib/sounding/admin}"
  ink_repo="jacob-bytes/komari-theme-ink"
  ink_tag=$(curl -fsSL "https://api.github.com/repos/$ink_repo/releases/latest" \
    | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)
  [ -n "$ink_tag" ] || die "未找到 ink Release"

  log "→ 下载 ink 监控面板 ($ink_tag)"
  tmpdir=$(mktemp -d)
  curl -fsSL "https://github.com/$ink_repo/releases/download/$ink_tag/ink-build-${ink_tag#v}.zip" \
    -o "$tmpdir/ink.zip" 2>/dev/null \
    || curl -fsSL "$(curl -fsSL "https://api.github.com/repos/$ink_repo/releases/latest" \
        | grep -o '"browser_download_url": *"[^"]*\.zip"' | head -1 | cut -d'"' -f4)" -o "$tmpdir/ink.zip" \
    || die "ink 下载失败"
  unzip -q -o "$tmpdir/ink.zip" -d "$tmpdir/ink" || die "解压失败（需要 unzip）"
  rm -rf "$tmpdir/ink.zip"

  mkdir -p "$ink_dir"
  cp -r "$tmpdir/ink/dist/." "$ink_dir/" 2>/dev/null || die "未找到 ink dist 目录"
  rm -rf "$tmpdir"
  log "✓ 监控面板已安装: $ink_dir"
}

function print_usage_hint() {
  role=$1
  shift
  log ""
  log "启动示例:"
  if [ "$role" = "server" ]; then
    log "  sounding-server -addr :8080 -db /var/lib/sounding/sounding.db \\"
    log "    -agent-token <agent-token> -admin-token <admin-token> \\"
    log "    -admin-pass <password> -telegram-token <bot> -telegram-chat-id <id>"
    log ""
    log "管理页:   http://localhost:8080/admin/"
    log "监控面板: http://localhost:8080/  （ink 已自动安装到 /var/lib/sounding/admin）"
  else
    log "  sounding-agent -server http://<主控>:8080 -token <token> -node-uuid \$(hostname) $*"
    log ""
    log "配置探针（名称:类型:目标）:"
    log "  -probe \"上海移动:icmp:223.5.5.5,百度:http:https://baidu.com,解析:dns:baidu.com@8.8.8.8\""
  fi
}

function main() {
  role="${1:-server}"
  shift 2>/dev/null || true
  case "$role" in
    server|agent) ;;
    -h|--help|help) log "用法: install.sh [server|agent] [启动参数...]"; exit 0 ;;
    *) die "未知角色: $role（可选 server / agent）" ;;
  esac
  check_deps
  install_bin "$role"
  if [ "$role" = "server" ]; then
    install_ink_dashboard
  fi
  print_usage_hint "$role" "$@"
}

main "$@"
