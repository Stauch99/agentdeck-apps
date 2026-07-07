#!/usr/bin/env bash
# index.sh —— 聚合 apps/*/cartridge.json 成 store/index.json (icon 相对路径 → data URI 内联)。
# 货架 = git repo 的 CI 编译产物: 平台一次 GET 拿全部 manifest, 不 clone 仓。
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
. "$root/scripts/lib/appslib.sh"
out="${1:-$root/store/index.json}"
mkdir -p "$(dirname "$out")"

# 每个 app 的 (内联后) manifest 写独立文件, 最后 jq -s slurp 成数组组装 ——
# 全程不把 manifest/data URI 塞进命令行参数 (png base64 ~100KB × 7, 超 Linux 单参数
# 上限 MAX_ARG_STRLEN=128KB → --arg/--argjson 会 "Argument list too long"; Mac 上限大不复现)。
work="$(mktemp -d)"; trap 'rm -rf "$work"' EXIT
i=0
for dir in "$root"/apps/*/; do
  slug="$(basename "$dir")"
  [ "$slug" = "_template" ] && continue
  manifest_ok "$dir" || { echo "skip (manifest 快检失败): $slug" >&2; continue; }
  # icon 内联: meta.icon 若是仓内相对路径且文件存在 → base64 data URI (经 --rawfile 文件读入)
  icon_rel="$(jq -r '.meta.icon // ""' "$dir/cartridge.json")"
  if [ -n "$icon_rel" ] && [ -f "$dir/$icon_rel" ]; then
    mime="image/svg+xml"
    case "$icon_rel" in *.png) mime="image/png";; *.jpg|*.jpeg) mime="image/jpeg";; esac
    dfile="$(mktemp)"
    printf 'data:%s;base64,%s' "$mime" "$(base64 < "$dir/$icon_rel" | tr -d '\n')" > "$dfile"
    jq --rawfile d "$dfile" '.meta.icon=$d' "$dir/cartridge.json" > "$work/$(printf '%04d' "$i").json"
    rm -f "$dfile"
  else
    cp "$dir/cartridge.json" "$work/$(printf '%04d' "$i").json"
  fi
  i=$((i + 1))
done

# jq -s: 文件路径作参数 (短), 内容 jq 从文件读 —— 不受命令行参数上限约束。
jq -s '{store:{id:"official",name:"AgentDeck 官方商店",generatedAt:(now|todate)},apps:.}' \
  "$work"/*.json > "$out"
echo "wrote $out ($(jq '.apps|length' "$out") apps)"
