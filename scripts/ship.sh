#!/usr/bin/env bash
set -euo pipefail
slug="${1:?usage: ship.sh <slug>}"
root="$(cd "$(dirname "$0")/.." && pwd)"
. "$root/scripts/lib/appslib.sh"
org="${GHCR_ORG:?set GHCR_ORG}"
sha="$(git -C "$root" rev-parse --short=12 HEAD)"
"$root/scripts/build.sh" "$slug"
podman push "$(image_ref "$org" "$slug" "$sha")"
podman push "$(image_ref "$org" "$slug" latest)"
