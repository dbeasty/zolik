#!/usr/bin/env bash
#
# Brings the whole stack up on one command — the same Docker shape we deploy on
# a local server (KDB, one container, ~1 GB cap).
#
#   scripts/dev-stack.sh up      # docker server + web client
#   scripts/dev-stack.sh down    # stop both
#   scripts/dev-stack.sh test    # run every suite: Go, client, e2e
#   scripts/dev-stack.sh logs    # tail the server container
#
# Why a script rather than three commands in a README: Playwright must be
# invoked from e2e/ or it silently loads no config and reports "did not expect
# test.describe() to be called here". This script also pins the API URL the web
# bundle is built with — without that, guest sign-in fails with "Failed to fetch"
# because the browser calls a port nothing is listening on.
#
# Override the compose file with ZOLIK_DEV_COMPOSE=mongo for the full Mongo +
# Redis stack (docker-compose.yml, heavier — two app instances for scaling tests).

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN="${ROOT}/.dev-stack"
SERVER_DIR="${ROOT}/server"
mkdir -p "$RUN"

COMPOSE_MODE="${ZOLIK_DEV_COMPOSE:-kdb}"
case "$COMPOSE_MODE" in
  kdb)   COMPOSE_FILE="docker-compose.kdb.yml" ;;
  mongo) COMPOSE_FILE="docker-compose.yml" ;;
  *)     printf '\033[31mError:\033[0m ZOLIK_DEV_COMPOSE must be kdb or mongo (got: %s)\n' "$COMPOSE_MODE" >&2; exit 1 ;;
esac

PORT="${ZOLIK_DEV_PORT:-8090}"
WEB_PORT="${ZOLIK_DEV_WEB_PORT:-8114}"

# The host the *browser* will use to reach the API, which is not always the
# host this script runs on. The bundle carries this address to whatever machine
# loads it, so a loopback address served to a guest computer names that guest —
# which has no server on it, and the app reports "Failed to fetch".
#
# Overriding client-react-native/.env does not fix that: the EXPO_PUBLIC_* value
# exported below wins over any .env file, so this is the only knob that works.
#
#   ZOLIK_DEV_HOST=192.168.1.7 scripts/dev-stack.sh up
#
HOST="${ZOLIK_DEV_HOST:-127.0.0.1}"
API="http://${HOST}:${PORT}"
WEB="http://${HOST}:${WEB_PORT}"

# The one build identity, shared by the Docker build-args and the web bundle's
# EXPO_PUBLIC_* vars below, so the two footers can never disagree for a reason
# other than actually being different builds. See scripts/version.sh.
eval "$(sh "${ROOT}/scripts/version.sh" --export)"

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

compose() {
  local bind="${ZOLIK_BIND:-}"
  if [[ -z "$bind" && "$HOST" != "127.0.0.1" && "$HOST" != "localhost" ]]; then
    bind="0.0.0.0"
  fi
  # The end-to-end suite seeds mid-round positions through /debug-state and
  # reads sign-in codes back through /auth/dev/last-code. Both are shut in the
  # compose file by default, because that file is also the shape the public
  # host runs; this is the local stack, so this is where they are opened.
  (cd "$SERVER_DIR" && ZOLIK_BIND="${bind:-127.0.0.1}" ZOLIK_TEST_ENDPOINTS=true \
    ZOLIK_BOT_THINK_MIN_MS="${ZOLIK_BOT_THINK_MIN_MS:-}" \
    ZOLIK_BOT_THINK_MAX_MS="${ZOLIK_BOT_THINK_MAX_MS:-}" \
    docker compose -f "$COMPOSE_FILE" "$@")
}

ensure_env() {
  if [[ ! -f "${SERVER_DIR}/.env" ]]; then
    cp "${SERVER_DIR}/.env.example" "${SERVER_DIR}/.env"
    say "created server/.env from .env.example"
  fi
}

stop_web() {
  if [[ -f "${RUN}/web.pid" ]]; then
    kill "$(cat "${RUN}/web.pid")" 2>/dev/null || true
    rm -f "${RUN}/web.pid"
  fi
  pkill -f "expo start --web --port ${WEB_PORT}" 2>/dev/null || true
}

stop_legacy_binary() {
  # Pre-docker dev-stack left a bare binary on :8096; clean it up if still around.
  if [[ -f "${RUN}/server.pid" ]]; then
    kill "$(cat "${RUN}/server.pid")" 2>/dev/null || true
    rm -f "${RUN}/server.pid"
  fi
  pkill -f "${RUN}/zolik-server" 2>/dev/null || true
}

up() {
  ensure_env
  stop_legacy_binary
  stop_web

  if [[ "$COMPOSE_MODE" == "kdb" && ! -f "${ROOT}/../kdb/go/go.mod" ]]; then
    die "KDB stack needs the kdb repo as a sibling of zolik (../kdb). Clone it or use ZOLIK_DEV_COMPOSE=mongo."
  fi

  say "building and starting the server via docker compose (${COMPOSE_FILE}, ${ZOLIK_VERSION}+${ZOLIK_COMMIT})"
  compose up -d --build
  wait_for "${API}/healthz" "the server" 120

  say "starting the web client on ${WEB} (API ${API})"
  (cd "${ROOT}/client-react-native" && \
    EXPO_PUBLIC_ZOLIK_BASE_URL="$API" \
    EXPO_PUBLIC_ZOLIK_VERSION="$ZOLIK_VERSION" EXPO_PUBLIC_ZOLIK_COMMIT="$ZOLIK_COMMIT" \
    nohup npx expo start --web --port "$WEB_PORT" \
      > "${RUN}/web.log" 2>&1 < /dev/null & echo $! > "${RUN}/web.pid" ) > /dev/null 2>&1
  wait_for "$WEB" "the web client" 180

  echo
  say "up"
  printf '   Stack  %s (%s)\n' "$COMPOSE_FILE" "$COMPOSE_MODE"
  printf '   API    %s\n   Web    %s\n' "$API" "$WEB"
  printf '   Version (built): %s+%s\n' "$ZOLIK_VERSION" "$ZOLIK_COMMIT"
  printf '   Version (served): '
  curl -fsS "${API}/version" | python3 -c \
    "import json,sys; b=json.load(sys.stdin); print(f\"{b['version']}+{b['commit']}\")" 2>/dev/null \
    || echo '(could not read /version)'
  echo
  printf '   Games hosted: '
  curl -fsS "${API}/modules" | sed 's/.*/&/' | python3 -c \
    'import json,sys; print(", ".join(m["id"] for m in json.load(sys.stdin)["modules"]))' 2>/dev/null \
    || echo '(could not read /modules)'
  printf '\n   Open %s and press "Play".\n' "$WEB"

  exit 0
}

down() {
  stop_web
  stop_legacy_binary
  compose down 2>/dev/null || true
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

  # Bots answer at the pace they always did here, which is faster than the one
  # the server now defaults to for people to watch.
  #
  # Both directions of that matter. Too slow and the long bot-driven specs
  # (Prsi and the played-out hand in offer-labels) run past their thirty-second
  # timeouts, and the suite ends up measuring the pause rather than the code.
  # Too fast and it gets worse in the other direction: several specs race the
  # bots by construction — drag-and-drop's Canasta lay-off drops a card on a
  # *partnership* meld and asserts it grew by one, which a bot partner melding
  # during the drag turns into two — and every bot action saved is another
  # chance for one to land inside the window. Hurried to nothing, that spec
  # failed two runs in five.
  #
  # So this is deliberately the old pace rather than the fastest one: it keeps
  # the suite's timing exactly where it was before the pause became a thing
  # anybody had chosen, which is the only setting that makes the change to it
  # provably neutral here.
  #
  # Announced, because it recreates the container the developer may have been
  # watching, and leaves it at this pace: `$0 up` restores the watchable one.
  say "setting the bots to the suite's pace (\`$0 up\` restores a watchable one)"
  ZOLIK_BOT_THINK_MIN_MS=400 ZOLIK_BOT_THINK_MAX_MS=1300 compose up -d >/dev/null
  wait_for "${API}/healthz" "the server" 60
  if [[ ! -d "${ROOT}/e2e/node_modules" ]]; then
    say "installing e2e dependencies"
    (cd "${ROOT}/e2e" && npm install)
  fi
  # Idempotent and fast when chromium is already present; without it every
  # e2e test fails with "Executable doesn't exist" on a fresh machine.
  (cd "${ROOT}/e2e" && npx playwright install chromium)
  (cd "${ROOT}/e2e" && ZOLIK_E2E_API_BASE="$API" ZOLIK_E2E_WEB_BASE="$WEB" npx playwright test --workers=2)
}

case "${1:-up}" in
  up)    up ;;
  down)  down ;;
  test)  shift || true; run_tests "$@" ;;
  logs)  compose logs -f app ;;
  *)     die "usage: $0 {up|down|test|logs}" ;;
esac
