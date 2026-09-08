#!/usr/bin/env sh
# sounding 一键安装（主控 + Agent）
#   curl -fsSL https://raw.githubusercontent.com/jacob-bytes/sounding/main/scripts/install.sh | sh -s -- server
#   curl -fsSL ... | sh -s -- agent -server http://my-control:8080 -token xxx
set -e

ROLE="${1:-server}"
shift 2>/dev/null || true
REPO="jacob-bytes/sounding"
BIN_DIR="${BIN_DIR:-/usr/local/bin}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "不支持的架构: $ARCH"; exit 1 ;;
esac

TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)
[ -z "$TAG" ] && { echo "未找到 Release"; exit 1; }

BIN="sounding-$ROLE-$OS-$ARCH"
URL="https://github.com/$REPO/releases/download/$TAG/$BIN"
echo "→ 下载 $BIN ($TAG)"
curl -fsSL "$URL" -o "/tmp/$BIN"
chmod +x "/tmp/$BIN"
sudo mv "/tmp/$BIN" "$BIN_DIR/sounding-$ROLE"
echo "✓ 已安装: $BIN_DIR/sounding-$ROLE"
echo ""
echo "启动示例:"
if [ "$ROLE" = "server" ]; then
  echo "  sounding-server -addr :8080 -db /var/lib/sounding/sounding.db -agent-token <token> -admin-token <token>"
else
  echo "  sounding-agent -server http://<主控>:8080 -token <token> -node-uuid \$(hostname) $*"
fi
