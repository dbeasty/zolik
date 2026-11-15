#!/usr/bin/env bash
#
# Checks what scripts/dev-stack.sh hands to docker compose and to go test,
# without Docker, a server or a network: the script runs from a scratch copy
# of the checkout with docker, go, curl, npx and npm replaced by stubs that
# write down what they were asked.
#
#   scripts/test-dev-stack.sh                 # the script in this checkout
#   scripts/test-dev-stack.sh path/to/old.sh  # any other version of it
#
# It pins the defect-review findings on that script: a LAN address must not
# widen the bind by itself, a copy of production must never be served with
# the sign-in-code and debug endpoints, and `test` must name a store engine
# so the store-backed Go tests run instead of skipping.

set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${1:-${HERE}/dev-stack.sh}"
[[ -f "$SCRIPT" ]] || { echo "no such script: $SCRIPT" >&2; exit 2; }

WORK="$(mktemp -d "${TMPDIR:-/tmp}/dev-stack-test.XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

ROOT="$WORK/zolik"
BIN="$WORK/bin"
LOG="$WORK/calls.log"
mkdir -p "$ROOT/scripts" "$ROOT/server" "$ROOT/client-tui" "$ROOT/e2e/node_modules" \
  "$ROOT/client-react-native/node_modules/.bin" "$WORK/kdb/go" "$BIN"
cp "$SCRIPT" "$ROOT/scripts/dev-stack.sh"
touch "$ROOT/server/.env" "$WORK/kdb/go/go.mod"
cat >"$ROOT/scripts/version.sh" <<'EOF'
echo 'export ZOLIK_VERSION=0.0.0-test ZOLIK_COMMIT=test'
EOF

stub() { # name, body
  printf '#!/usr/bin/env bash\n%s\n' "$2" >"$BIN/$1"
  chmod +x "$BIN/$1"
}
stub docker 'echo "docker bind=${ZOLIK_BIND:-} test=${ZOLIK_TEST_ENDPOINTS:-} debug=${ZOLIK_DEBUG_ENDPOINTS:-} args=$*" >>"$CALLS"'
stub go     'echo "go dir=$(basename "$PWD") engine=${ZOLIK_TEST_DB_ENGINE:-} args=$*" >>"$CALLS"'
stub curl   'echo "{}"'
stub npx    'exit 0'
stub npm    'exit 0'
stub pkill  'exit 0'
cp "$BIN/npx" "$ROOT/client-react-native/node_modules/.bin/tsc"

PASS=0
FAIL=0

# run <label> <want-exit: 0|fail> <env...> -- <verb>
run() {
  local label="$1" want="$2"
  shift 2
  local envs=()
  while [[ "$1" != "--" ]]; do envs+=("$1"); shift; done
  shift
  : >"$LOG"
  env -i HOME="$HOME" PATH="$BIN:/usr/bin:/bin" CALLS="$LOG" TMPDIR="$WORK" \
    ZOLIK_DEV_WEB_PORT=59114 ${envs[@]+"${envs[@]}"} \
    bash "$ROOT/scripts/dev-stack.sh" "$@" >"$WORK/out" 2>&1
  local code=$?
  CASE="$label"
  if [[ "$want" == 0 && $code -ne 0 ]] || [[ "$want" == fail && $code -eq 0 ]]; then
    bad "exited $code, want ${want/fail/non-zero}"
    sed 's/^/      /' "$WORK/out" | tail -5
    return 1
  fi
  return 0
}

ok()  { PASS=$((PASS + 1)); printf '  ok    %s\n' "$CASE"; }
bad() { FAIL=$((FAIL + 1)); printf '  FAIL  %s: %s\n' "$CASE" "$*"; }

# expect <pattern> — some call in the log matches
expect()    { grep -Eq -- "$1" "$LOG" || { bad "no call matching /$1/"; sed 's/^/      /' "$LOG"; return 1; }; }
# expect_no <pattern> — no call in the log matches
expect_no() { ! grep -Eq -- "$1" "$LOG" || { bad "unexpected call matching /$1/"; sed 's/^/      /' "$LOG"; return 1; }; }

echo "dev-stack.sh: $SCRIPT"

run "default up binds loopback with the suite's endpoints" 0 -- up &&
  expect '^docker bind=127\.0\.0\.1 test=true debug=true args=.* up -d --build' && ok

run "a LAN ZOLIK_DEV_HOST alone does not widen the bind" fail ZOLIK_DEV_HOST=192.168.1.7 -- up &&
  expect_no 'up -d' && ok

run "ZOLIK_BIND=0.0.0.0 is the explicit ask" 0 ZOLIK_BIND=0.0.0.0 ZOLIK_DEV_HOST=192.168.1.7 -- up &&
  expect '^docker bind=0\.0\.0\.0 .* up -d --build' && ok

run "prodcopy is served with the test and debug endpoints shut" 0 ZOLIK_DEV_DATA=prodcopy -- up &&
  expect '^docker bind=127\.0\.0\.1 test=false debug=false args=.*prodcopy\.yml.* up -d --build' &&
  expect_no 'test=true|debug=true' && ok

run "prodcopy refuses ZOLIK_DEBUG_ENDPOINTS=true" fail ZOLIK_DEV_DATA=prodcopy ZOLIK_DEBUG_ENDPOINTS=true -- up &&
  expect_no 'up -d' && ok

run "prodcopy refuses a LAN bind" fail ZOLIK_DEV_DATA=prodcopy ZOLIK_BIND=0.0.0.0 ZOLIK_DEV_HOST=192.168.1.7 -- up &&
  expect_no 'up -d' && ok

run "test names a store engine for the server's Go tests" 0 -- test &&
  expect '^go dir=server engine=kdb args=test \./\.\.\.' && ok

run "test keeps an engine the caller named" 0 ZOLIK_TEST_DB_ENGINE=mongo -- test &&
  expect '^go dir=server engine=mongo args=test' && ok

run "test refuses prodcopy before running anything" fail ZOLIK_DEV_DATA=prodcopy -- test &&
  expect_no '^go |up -d' && ok

run "down still works whatever the host" 0 ZOLIK_DEV_HOST=192.168.1.7 -- down &&
  expect '^docker .*args=.* down' && ok

echo
echo "$PASS passed, $FAIL failed"
[[ $FAIL -eq 0 ]]
