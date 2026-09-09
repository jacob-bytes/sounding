#!/usr/bin/env sh
# 打印 ink 主题基线信息，用于校准 docs/ink-parity.md 与 docs/frontend-porting-plan.md。
#
# 用法:
#   sh scripts/ink-inventory.sh [ink 仓库路径]
#   sh scripts/ink-inventory.sh /path/to/komari-theme-ink
#
# 建议在每次移植阶段开始前跑一次，确认本地/远端基线未漂移。
set -e

INK_DIR="${1:-../komari-theme-ink}"
[ -d "$INK_DIR" ] || { echo "找不到 ink 仓库: $INK_DIR" >&2; exit 1; }
cd "$INK_DIR"

commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)
date=$(git log -1 --format=%ci 2>/dev/null || echo unknown)
branch=$(git status -sb 2>/dev/null | head -1 || echo unknown)
version=$(python3 -c "import json;print(json.load(open('komari-theme.json'))['version'])" 2>/dev/null || echo unknown)
vue=$(find src -name '*.vue' 2>/dev/null | wc -l | tr -d ' ')
ts=$(find src -name '*.ts' 2>/dev/null | wc -l | tr -d ' ')
lines=$(find src \( -name '*.vue' -o -name '*.ts' \) -exec cat {} + 2>/dev/null | wc -l | tr -d ' ')
comp=$(find src/components -name '*.vue' 2>/dev/null | wc -l | tr -d ' ')
ui=$(find src/components/ui -name '*.vue' 2>/dev/null | wc -l | tr -d ' ')
keys=$(python3 -c "import json;d=json.load(open('komari-theme.json'));print(len([i for i in d['configuration']['data'] if i.get('type')!='title']))" 2>/dev/null || echo unknown)
zip=$(ls -t ink-build-*.zip 2>/dev/null | head -1 || echo none)

# 远端 main（HTTPS，无需 SSH）
remote_sha=$(git ls-remote https://github.com/jacob-bytes/komari-theme-ink.git main 2>/dev/null | awk '{print $1}')
local_sha=$(git rev-parse HEAD 2>/dev/null || echo unknown)
if [ -n "$remote_sha" ] && [ "$remote_sha" = "$local_sha" ]; then
  sync="IN SYNC"
elif [ -n "$remote_sha" ]; then
  sync="DRIFT (local HEAD != remote main)"
else
  sync="unknown (network unavailable?)"
fi
remote_short=$(printf '%s' "$remote_sha" | cut -c1-7)

# GitHub 最新 Release（可选）
release=$(curl -fsSL --max-time 10 https://api.github.com/repos/jacob-bytes/komari-theme-ink/releases/latest 2>/dev/null \
  | python3 -c "import sys,json;d=json.load(sys.stdin);a=(d.get('assets') or [{}])[0];print('%s / %s' % (d.get('tag_name','?'), a.get('name','?')))" 2>/dev/null || echo "unknown")

echo "ink baseline"
echo "  path:        $INK_DIR"
echo "  branch:      $branch"
echo "  commit:      $commit ($date)"
echo "  version:     $version"
echo "  remote main: ${remote_short:-unknown} ($sync)"
echo "  release:     $release"
echo "  vue/ts:      $vue / $ts files"
echo "  lines:       $lines"
echo "  components:  $comp (ui atoms: $ui)"
echo "  config keys: $keys"
echo "  latest zip:  $zip"
