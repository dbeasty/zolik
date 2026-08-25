#!/usr/bin/env bash
#
# Brings the whole stack up on one command, on ports that do not collide with
# a dev server you may already have running.
#
#   scripts/dev-stack.sh up      # start Mongo/Redis (if needed), server, web
#   scripts/dev-stack.sh down    # stop the server and web build
#   scripts/dev-stack.sh test    # run every suite: Go, client, e2e
#   scripts/dev-stack.sh logs    # tail the server log
#
# Why a script rather than three commands in a README: two of the three have a
# trap in them that costs half an hour to diagnose. The server must be a built
# binary rather than a backgrounded `go run` — a reaped `go run` child produces
# a wall of ECONNREFUSED that looks exactly like a code regression — and
# Playwright must be invoked from e2e/ or it silently loads no config and
# reports "did not expect test.describe() to be called here".

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN="${ROOT}/.dev-stack"
mkdir -p "$RUN"

# Deliberately not 8090/8100: those are the ports a normal dev session uses,
# and the point of this script is to be runnable beside one.
PORT="${ZOLIK_DEV_PORT:-8096}"
WEB_PORT="${ZOLIK_DEV_WEB_PORT:-8114}"
MONGO_URI="${ZOLIK_DEV_MONGO_URI:-mongodb://localhost:27018}"
REDIS_URL="${ZOLIK_DEV_REDIS_URL:-redis://localhost:6379/2}"
MONGO_DB="${ZOLIK_DEV_MONGO_DB:-zolik_dev_stack}"

API="http://127.0.0.1:${PORT}"
WEB="http://127.0.0.1:${WEB_PORT}"

say() { printf '\033[1m==>\033[0m %s\n' "$*"; }
die() { printf '\033[31mError:\033[0m %s\n' "$*" >&2; exit 1; }

wait_for() { # url, label, seconds
  local url="$1" label="$2" secs="${3:-60}"
  for _ in $(seq "$secs"); do
    if curl -fsS -m 2 -o /dev/null "$url" 2>/dev/null; then return 0; fi
    sleep 1
  done
  die "$label did not come up at $url — see: $0 logs"
}

ensure_datastores() {
  if ! nc -z localhost 27018 2>/dev/null || ! nc -z localhost 6379 2>/dev/null; then
    say "starting Mongo and Redis via docker compose"
    (cd "${ROOT}/server" && docker compose up -d mongo redis) \
      || die "could not start Mongo/Redis. Start them yourself, or set ZOLIK_DEV_MONGO_URI / ZOLIK_DEV_REDIS_URL."
    sleep 3
  fi
}

up() {
  ensure_datastores

  say "building the server"
  (cd "${ROOT}/server" && go build -o "${RUN}/zolik-server" ./cmd/server)

  say "starting the server on ${API}"
  # The whole launch is wrapped in a subshell whose own stdout and stderr go
  # to /dev/null, not just the daemon's.
  #
  # That is what makes `dev-stack.sh up | tail` terminate. A daemon started
  # here inherits this script's file descriptors, and something in the Expo
  # toolchain keeps a copy of fd 1 alive past its own redirection — so the
  # reader on the other end of the pipe never sees EOF and the command appears
  # to hang, minutes after the stack is up and serving.
  ( APP_ENV=local PORT="$PORT" SSH_ENABLED=false \
      MONGO_URI="$MONGO_URI" MONGO_DB="$MONGO_DB" REDIS_URL="$REDIS_URL" \
      JWT_ACCESS_SECRET=dev_access JWT_REFRESH_SECRET=dev_refresh \
      nohup "${RUN}/zolik-server" > "${RUN}/server.log" 2>&1 < /dev/null &
    echo $! > "${RUN}/server.pid" ) > /dev/null 2>&1
  wait_for "${API}/healthz" "the server" 60

  say "starting the web client on ${WEB}"
  (cd "${ROOT}/client-react-native" && \
    EXPO_PUBLIC_ZOLIK_BASE_URL="$API" nohup npx expo start --web --port "$WEB_PORT" \
      > "${RUN}/web.log" 2>&1 < /dev/null & echo $! > "${RUN}/web.pid" ) > /dev/null 2>&1
  wait_for "$WEB" "the web client" 180

  echo
  say "up"
  printf '   API  %s\n   Web  %s\n\n' "$API" "$WEB"
  printf '   Games hosted: '
  curl -fsS "${API}/modules" | sed 's/.*/&/' | python3 -c \
    'import json,sys; print(", ".join(m["id"] for m in json.load(sys.stdin)["modules"]))' 2>/dev/null \
    || echo '(could not read /modules)'
  printf '\n   Open %s and press "Play".\n' "$WEB"

  # Explicit, because the daemons started above inherit this script's stdout.
  # Without it, `dev-stack.sh up | tail` never sees EOF and appears to hang
  # long after the stack is actually up and serving.
  exit 0
}

down() {
  for name in server web; do
    if [[ -f "${RUN}/${name}.pid" ]]; then
      kill "$(cat "${RUN}/${name}.pid")" 2>/dev/null || true
      rm -f "${RUN}/${name}.pid"
    fi
  done
  # Expo spawns children that outlive the pid we recorded.
  pkill -f "expo start --web --port ${WEB_PORT}" 2>/dev/null || true
  pkill -f "${RUN}/zolik-server" 2>/dev/null || true
  say "down"
}

run_tests() {
  say "Go suites (server)"
  (cd "${ROOT}/server" && go test ./... "$@")

  say "Go suites (terminal client)"
  (cd "${ROOT}/client-tui" && go test ./...)

  say "client typecheck and unit tests"
  (cd "${ROOT}/client-react-native" && ./node_modules/.bin/tsc --noEmit && npm test --silent)

  say "end-to-end (needs the stack up)"
  curl -fsS -m 2 -o /dev/null "${API}/healthz" 2>/dev/null \
    || die "the stack is not running — start it with: $0 up"
  # From e2e/, always: Playwright resolves its config from the working
  # directory and silently runs with none if invoked from the repo root.
  (cd "${ROOT}/e2e" && ZOLIK_E2E_API_BASE="$API" ZOLIK_E2E_WEB_BASE="$WEB" npx playwright test --workers=2)
}

case "${1:-up}" in
  up)    up ;;
  down)  down ;;
  test)  shift || true; run_tests "$@" ;;
  logs)  tail -f "${RUN}/server.log" ;;
  *)     die "usage: $0 {up|down|test|logs}" ;;
esac
