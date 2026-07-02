assert_eq() { [ "$1" = "$2" ] && { echo "ok: $3"; } || { echo "FAIL: $3 — got [$1] want [$2]"; exit 1; }; }
assert_ok() { if "$@"; then echo "ok: $*"; else echo "FAIL: $*"; exit 1; fi; }
assert_fail() { if "$@"; then echo "FAIL(expected nonzero): $*"; exit 1; else echo "ok(rejected): $*"; fi; }
