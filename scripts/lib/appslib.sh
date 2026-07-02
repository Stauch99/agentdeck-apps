# image_ref <org> <slug> <tag> -> ghcr 全名
image_ref() { printf 'ghcr.io/%s/%s:%s' "$1" "$2" "$3"; }

# manifest_ok <app-dir>: cartridge.json 存在 + JSON 有效 + slug==目录名 + 必填字段在
manifest_ok() {
  local dir="$1" mf="$1/cartridge.json" slug
  slug="$(basename "$dir")"
  [ -f "$mf" ] || return 1
  jq -e --arg s "$slug" \
    '.slug==$s and (.name|length>0) and (.image|length>0) and (.port|numbers)' \
    "$mf" >/dev/null 2>&1
}
