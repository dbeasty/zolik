#!/usr/bin/env bash
# Brings up what e2e/tests/phone-table.spec.ts runs against: a cloud and a
# "phone" on this machine, with no docker and no device.
#
#   cloud   the ordinary server binary as the sync hub, on $CLOUD_PORT, with
#           test endpoints, and an Expo web dev server for it on $WEB_PORT
#   phone   cmd/phonehost - the very zolikcore package the apps embed -
#           serving a web export of the client to its room listener, enrolled
#           with the cloud as a fresh account
#
# It prints the environment the spec reads, and writes it to $OUT/env. Stop
# everything with:  scripts/phone-e2e-stack.sh stop $OUT
#
#   OUT=/tmp/phone-stack scripts/phone-e2e-stack.sh
#
# Every step either works or stops the script with a reason and the tail of
# the log that has it, and a stop takes down whatever had already started.
# The spec skips itself when this environment is missing, so a step that
# failed quietly used to turn into a suite that passed by running nothing,
# with a cloud and a Metro server left behind on their ports.
set -euo pipefail
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
OUT=${OUT:-${TMPDIR:-/tmp}/zolik-phone-stack}
CLOUD_PORT=${CLOUD_PORT:-18190}
WEB_PORT=${WEB_PORT:-18114}
# Seconds to wait for each listener. Metro's first bundle is the slow one.
CLOUD_WAIT=${CLOUD_WAIT:-30}
PHONE_WAIT=${PHONE_WAIT:-30}
WEB_WAIT=${WEB_WAIT:-180}
PATH=/opt/homebrew/bin:$PATH
export PATH

# A process and everything under it, children first. The web server is
# `sh version.sh --exec npx expo …`, whose node and its workers outlive a kill
# of the pid that was recorded.
kill_tree() {
  local child
  for child in $(pgrep -P "$1" 2>/dev/null); do kill_tree "$child"; done
  kill "$1" 2>/dev/null || true
}

stop_all() {
  local f
  for f in "$OUT"/*.pid; do
    [ -f "$f" ] || continue
    kill_tree "$(cat "$f")"
    rm -f "$f"
  done
}

if [ "${1:-}" = stop ]; then
  OUT=${2:-$OUT}
  stop_all
  exit 0
fi

die() { # message, [log file]
  printf '\033[31mphone-e2e-stack: %s\033[0m\n' "$1" >&2
  if [ -n "${2:-}" ] && [ -f "$2" ]; then
    printf -- '--- last lines of %s ---\n' "$2" >&2
    tail -n 20 "$2" >&2
  fi
  exit 1
}

# Anything that stops this script before the environment is written stops
# what it started, too. Once the env is out, the processes are the point.
STARTED=0
cleanup() {
  local code=$?
  if [ "$STARTED" = 0 ]; then
    stop_all
    # Not finished is failed, whatever $? says: bash 3.2 (macOS's /bin/bash)
    # reaches this trap with $? = 0 after a `set -u` abort.
    [ $code -eq 0 ] && code=1
    printf 'phone-e2e-stack: failed (exit %d); stopped what had started.\n' "$code" >&2
  fi
  exit $code
}
trap cleanup EXIT
trap 'exit 130' INT TERM

alive() { kill -0 "$(cat "$1")" 2>/dev/null; }

wait_http() { # url, seconds, pid file, label, log
  local i
  for i in $(seq 1 $(($2 * 5))); do
    curl -sf -m 2 "$1" >/dev/null 2>&1 && return 0
    alive "$3" || die "$4 exited before answering at $1" "$5"
    sleep 0.2
  done
  die "$4 did not answer at $1 within $2s" "$5"
}

mkdir -p "$OUT"
# A previous run's processes would answer the health checks below for us.
stop_all
cd "$ROOT/server"
go build -o "$OUT/zolik-server" ./cmd/server || die "building cmd/server failed"
go build -o "$OUT/phonehost" ./cmd/phonehost || die "building cmd/phonehost failed"

if curl -sf -m 2 "http://127.0.0.1:$CLOUD_PORT/healthz" >/dev/null 2>&1; then
  die "something is already answering on CLOUD_PORT=$CLOUD_PORT; stop it or pick another port"
fi

# The cloud.
rm -rf "$OUT/cloud"
mkdir -p "$OUT/cloud"
(
  cd "$OUT/cloud"
  APP_ENV=local PORT=$CLOUD_PORT REDIS_URL= FEATURE_FLAG_DB_ENGINE=kdb KDB_PATH="$OUT/cloud/kdb" \
    ENABLE_TEST_ENDPOINTS=true FEATURE_FLAG_SYNC=hub SYNC_ADVERTISE=hub \
    JWT_SIGNING_KEY_FILE="$OUT/cloud/signing.pem" PUBLIC_BASE_URL="http://127.0.0.1:$CLOUD_PORT" \
    JWT_ACCESS_SECRET=dev_access_secret_change_me JWT_REFRESH_SECRET=dev_refresh_secret_change_me \
    "$OUT/zolik-server" >"$OUT/cloud.log" 2>&1 &
  echo $! >"$OUT/cloud.pid"
)
wait_http "http://127.0.0.1:$CLOUD_PORT/healthz" "$CLOUD_WAIT" "$OUT/cloud.pid" "the cloud server" "$OUT/cloud.log"

# Its web client, as a dev server.
cd "$ROOT/client-react-native"
mkdir -p "$OUT/metro-tmp"
(
  TMPDIR="$OUT/metro-tmp/" EXPO_PUBLIC_ZOLIK_BASE_URL="http://127.0.0.1:$CLOUD_PORT" \
    sh ../scripts/version.sh --exec npx expo start --web --port "$WEB_PORT" >"$OUT/web.log" 2>&1 &
  echo $! >"$OUT/web.pid"
)

# The web client the phone serves: same-origin, so it talks to whichever
# phone served it, and with claim links that name the cloud above.
rm -rf "$OUT/phoneweb"
eval "$(../scripts/version.sh --export)"
TMPDIR="$OUT/metro-tmp/" EXPO_PUBLIC_ZOLIK_API_SAME_ORIGIN=1 \
  EXPO_PUBLIC_ZOLIK_CLOUD_URL="http://127.0.0.1:$WEB_PORT" \
  EXPO_PUBLIC_ZOLIK_VERSION="$ZOLIK_VERSION" EXPO_PUBLIC_ZOLIK_COMMIT="$ZOLIK_COMMIT" \
  npx expo export --platform web --output-dir "$OUT/phoneweb" >"$OUT/export.log" 2>&1 \
  || die "the phone's web export failed" "$OUT/export.log"
[ -f "$OUT/phoneweb/index.html" ] || die "the phone's web export wrote no index.html" "$OUT/export.log"

# The phone's owner: a fresh account on the cloud, signed in by email code.
EMAIL="owner-$(date +%s)@phone.test"
CLOUD="http://127.0.0.1:$CLOUD_PORT"
field() { sed -n "s/.*\"$1\":\"\\([^\"]*\\)\".*/\\1/p"; }
curl -sf -X POST "$CLOUD/auth/email/start" -H 'Content-Type: application/json' -d "{\"email\":\"$EMAIL\"}" >/dev/null \
  || die "the cloud refused to start an email sign-in" "$OUT/cloud.log"
CODE=$(curl -sf "$CLOUD/auth/dev/last-code?email=$EMAIL" | field code) \
  || die "the cloud would not hand back the sign-in code (are its test endpoints on?)" "$OUT/cloud.log"
[ -n "$CODE" ] || die "the cloud handed back no sign-in code" "$OUT/cloud.log"
SESSION=$(curl -sf -X POST "$CLOUD/auth/email/verify" -H 'Content-Type: application/json' -d "{\"email\":\"$EMAIL\",\"code\":\"$CODE\"}") \
  || die "the cloud refused the sign-in code" "$OUT/cloud.log"
TOKEN=$(printf '%s' "$SESSION" | field accessToken)
OWNER=$(printf '%s' "$SESSION" | field userId)
PASS=$(printf '%s' "$SESSION" | field offlinePass)
[ -n "$TOKEN" ] && [ -n "$OWNER" ] && [ -n "$PASS" ] \
  || die "the sign-in answered without an access token, user id and offline pass: $SESSION"

# The phone, enrolled as theirs.
rm -rf "$OUT/phone"
"$OUT/phonehost" -data "$OUT/phone" -web "$OUT/phoneweb" -cloud "$CLOUD" -user "$OWNER" -enroll-token "$TOKEN" \
  >"$OUT/phone.json" 2>"$OUT/phone.log" &
echo $! >"$OUT/phone.pid"
for _ in $(seq 1 $((PHONE_WAIT * 5))); do
  [ -s "$OUT/phone.json" ] && break
  alive "$OUT/phone.pid" || die "the phone host exited before reporting its address" "$OUT/phone.log"
  sleep 0.2
done
[ -s "$OUT/phone.json" ] || die "the phone host reported no address within ${PHONE_WAIT}s" "$OUT/phone.log"
OWN=$(sed -n 's/.*"baseUrl":"\([^"]*\)".*/\1/p' "$OUT/phone.json")
PORT=$(sed -n 's/.*"lanPort":\([0-9]*\).*/\1/p' "$OUT/phone.json")
[ -n "$OWN" ] && [ -n "$PORT" ] || die "the phone host's report has no baseUrl and lanPort" "$OUT/phone.json"
LAN=$(ipconfig getifaddr en0 2>/dev/null || echo 127.0.0.1)

wait_http "http://127.0.0.1:$WEB_PORT/" "$WEB_WAIT" "$OUT/web.pid" "the web dev server" "$OUT/web.log"

cat >"$OUT/env" <<ENV
ZOLIK_E2E_API_BASE=$CLOUD
ZOLIK_E2E_WEB_BASE=http://127.0.0.1:$WEB_PORT
ZOLIK_E2E_PHONE_OWN=$OWN
ZOLIK_E2E_PHONE_ROOM=http://$LAN:$PORT
ZOLIK_E2E_PHONE_OWNER=$OWNER
ZOLIK_E2E_PHONE_OWNER_PASS=$PASS
ZOLIK_E2E_PHONE_OWNER_TOKEN=$TOKEN
ENV
STARTED=1
cat "$OUT/env"
