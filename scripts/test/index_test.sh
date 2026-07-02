#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
. "$here/assert.sh"

# 临时仓: alpha (带 icon) 入榜, _template 跳过
tmp="$(mktemp -d)"
mkdir -p "$tmp/scripts/lib" "$tmp/scripts/test" "$tmp/apps/alpha" "$tmp/apps/_template"
cp "$here/../lib/appslib.sh" "$tmp/scripts/lib/appslib.sh"
cp "$here/../index.sh" "$tmp/scripts/index.sh"
cat > "$tmp/apps/alpha/cartridge.json" <<'JSON'
{"schemaVersion":2,"name":"Alpha","slug":"alpha","image":"ghcr.io/o/alpha:latest","port":8080,"glyph":"alpha","color":"c","meta":{"icon":"icon.svg","version":"1.0.0","category":"工具"}}
JSON
printf '<svg xmlns="http://www.w3.org/2000/svg"/>' > "$tmp/apps/alpha/icon.svg"
echo '{"schemaVersion":2,"name":"T","slug":"_template","image":"x","port":1}' > "$tmp/apps/_template/cartridge.json"

bash "$tmp/scripts/index.sh" "$tmp/store/index.json"

assert_eq "$(jq '.apps|length' "$tmp/store/index.json")" "1" "只 alpha 入榜 (_template 跳过)"
assert_eq "$(jq -r '.apps[0].slug' "$tmp/store/index.json")" "alpha" "slug 正确"
assert_eq "$(jq -r '.apps[0].meta.icon' "$tmp/store/index.json" | head -c 14)" "data:image/svg" "icon 内联为 data URI"
assert_eq "$(jq -r '.store.id' "$tmp/store/index.json")" "official" "store.id"
echo "ALL PASS"
