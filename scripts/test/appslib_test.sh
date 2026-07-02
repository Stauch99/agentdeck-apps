#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
. "$here/assert.sh"
. "$here/../lib/appslib.sh"

# image_ref 拼接
assert_eq "$(image_ref myorg openmusic abc123)" "ghcr.io/myorg/openmusic:abc123" "image_ref 拼接"

# manifest_ok: 合法 fixture 过, slug!=dir 拒
tmp="$(mktemp -d)"; mkdir -p "$tmp/apps/foo"
echo '{"schemaVersion":2,"name":"Foo","slug":"foo","image":"ghcr.io/o/foo:latest","port":8080}' > "$tmp/apps/foo/cartridge.json"
assert_ok manifest_ok "$tmp/apps/foo"
echo '{"schemaVersion":2,"name":"Bar","slug":"WRONG","image":"x","port":1}' > "$tmp/apps/foo/cartridge.json"
assert_fail manifest_ok "$tmp/apps/foo"
echo "ALL PASS"
