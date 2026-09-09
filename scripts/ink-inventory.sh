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

echo "ink baseline"
echo "  path:        $INK_DIR"
echo "  branch:      $branch"
echo "  commit:      $commit ($date)"
echo "  version:     $version"
echo "  vue/ts:      $vue / $ts files"
echo "  lines:       $lines"
echo "  components:  $comp (ui atoms: $ui)"
echo "  config keys: $keys"
echo "  latest zip:  $zip"
