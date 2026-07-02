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
    data="data:$mime;base64,$(base64 < "$dir/$icon_rel" | tr -d '\n')"
    entry="$(jq --arg d "$data" '.meta.icon=$d' "$dir/cartridge.json")"
  fi
  apps="$(jq --argjson e "$entry" '. += [$e]' <<<"$apps")"
done

jq -n --argjson apps "$apps" \
  '{store:{id:"official",name:"AgentDeck 官方商店",generatedAt:(now|todate)},apps:$apps}' > "$out"
echo "wrote $out ($(jq '.apps|length' "$out") apps)"
