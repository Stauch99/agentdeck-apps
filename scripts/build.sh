#!/usr/bin/env bash
set -euo pipefail
slug="${1:?usage: build.sh <slug>}"
root="$(cd "$(dirname "$0")/.." && pwd)"
. "$root/scripts/lib/appslib.sh"
manifest_ok "$root/apps/$slug" || { echo "manifest 快检失败: apps/$slug"; exit 1; }
org="${GHCR_ORG:?set GHCR_ORG}"
sha="$(git -C "$root" rev-parse --short=12 HEAD)"
podman build -t "$(image_ref "$org" "$slug" "$sha")" -t "$(image_ref "$org" "$slug" latest)" "$root/apps/$slug"
