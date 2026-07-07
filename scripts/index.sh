#!/usr/bin/env bash
# index.sh —— 聚合 apps/*/cartridge.json 成 store/index.json (icon 相对路径 → data URI 内联)。
# 货架 = git repo 的 CI 编译产物: 平台一次 GET 拿全部 manifest, 不 clone 仓。
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
. "$root/scripts/lib/appslib.sh"
out="${1:-$root/store/index.json}"
mkdir -p "$(dirname "$out")"

apps='[]'
for dir in "$root"/apps/*/; do
  slug="$(basename "$dir")"
  [ "$slug" = "_template" ] && continue
  manifest_ok "$dir" || { echo "skip (manifest 快检失败): $slug" >&2; continue; }
  entry="$(cat "$dir/cartridge.json")"
  # icon 内联: meta.icon 若是仓内相对路径且文件存在 → base64 data URI
  icon_rel="$(jq -r '.meta.icon // ""' "$dir/cartridge.json")"
  if [ -n "$icon_rel" ] && [ -f "$dir/$icon_rel" ]; then
    mime="image/svg+xml"
    case "$icon_rel" in *.png) mime="image/png";; *.jpg|*.jpeg) mime="image/jpeg";; esac
    # data URI 经临时文件传给 jq (--rawfile), 不走命令行参数 —— png base64 可达 ~100KB,
    # 超 Linux 单参数上限 MAX_ARG_STRLEN=128KB → --arg 会 "Argument list too long" (Mac 上限大不复现)。
    dfile="$(mktemp)"
    printf 'data:%s;base64,%s' "$mime" "$(base64 < "$dir/$icon_rel" | tr -d '\n')" > "$dfile"
    entry="$(jq --rawfile d "$dfile" '.meta.icon=$d' "$dir/cartridge.json")"
    rm -f "$dfile"
  fi
  apps="$(jq --argjson e "$entry" '. += [$e]' <<<"$apps")"
done

jq -n --argjson apps "$apps" \
  '{store:{id:"official",name:"AgentDeck 官方商店",generatedAt:(now|todate)},apps:$apps}' > "$out"
echo "wrote $out ($(jq '.apps|length' "$out") apps)"
