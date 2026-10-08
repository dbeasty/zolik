#!/bin/sh
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
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
OUT=${OUT:-${TMPDIR:-/tmp}/zolik-phone-stack}
CLOUD_PORT=${CLOUD_PORT:-18190}
WEB_PORT=${WEB_PORT:-18114}
PATH=/opt/homebrew/bin:$PATH
export PATH

if [ "${1:-}" = stop ]; then
  OUT=${2:-$OUT}
  for f in "$OUT"/*.pid; do [ -f "$f" ] && kill "$(cat "$f")" 2>/dev/null || true; done
  exit 0
fi

mkdir -p "$OUT"
cd "$ROOT/server"
go build -o "$OUT/zolik-server" ./cmd/server
go build -o "$OUT/phonehost" ./cmd/phonehost

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
for _ in $(seq 1 100); do
  curl -sf "http://127.0.0.1:$CLOUD_PORT/healthz" >/dev/null 2>&1 && break
  curl -sf "http://127.0.0.1:$CLOUD_PORT/health" >/dev/null 2>&1 && break
  sleep 0.2
done

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
  npx expo export --platform web --output-dir "$OUT/phoneweb" >"$OUT/export.log" 2>&1

# The phone's owner: a fresh account on the cloud, signed in by email code.
EMAIL="owner-$(date +%s)@phone.test"
CLOUD="http://127.0.0.1:$CLOUD_PORT"
curl -sf -X POST "$CLOUD/auth/email/start" -H 'Content-Type: application/json' -d "{\"email\":\"$EMAIL\"}" >/dev/null
CODE=$(curl -sf "$CLOUD/auth/dev/last-code?email=$EMAIL" | sed -n 's/.*"code":"\([^"]*\)".*/\1/p')
SESSION=$(curl -sf -X POST "$CLOUD/auth/email/verify" -H 'Content-Type: application/json' -d "{\"email\":\"$EMAIL\",\"code\":\"$CODE\"}")
TOKEN=$(printf '%s' "$SESSION" | sed -n 's/.*"accessToken":"\([^"]*\)".*/\1/p')
OWNER=$(printf '%s' "$SESSION" | sed -n 's/.*"userId":"\([^"]*\)".*/\1/p')
PASS=$(printf '%s' "$SESSION" | sed -n 's/.*"offlinePass":"\([^"]*\)".*/\1/p')

# The phone, enrolled as theirs.
rm -rf "$OUT/phone"
"$OUT/phonehost" -data "$OUT/phone" -web "$OUT/phoneweb" -cloud "$CLOUD" -user "$OWNER" -enroll-token "$TOKEN" \
  >"$OUT/phone.json" 2>"$OUT/phone.log" &
echo $! >"$OUT/phone.pid"
for _ in $(seq 1 100); do [ -s "$OUT/phone.json" ] && break; sleep 0.2; done
OWN=$(sed -n 's/.*"baseUrl":"\([^"]*\)".*/\1/p' "$OUT/phone.json")
PORT=$(sed -n 's/.*"lanPort":\([0-9]*\).*/\1/p' "$OUT/phone.json")
LAN=$(ipconfig getifaddr en0 2>/dev/null || echo 127.0.0.1)

for _ in $(seq 1 300); do
  curl -sf "http://127.0.0.1:$WEB_PORT/" >/dev/null 2>&1 && break
  sleep 0.5
done

cat >"$OUT/env" <<ENV
ZOLIK_E2E_API_BASE=$CLOUD
ZOLIK_E2E_WEB_BASE=http://127.0.0.1:$WEB_PORT
ZOLIK_E2E_PHONE_OWN=$OWN
ZOLIK_E2E_PHONE_ROOM=http://$LAN:$PORT
ZOLIK_E2E_PHONE_OWNER=$OWNER
ZOLIK_E2E_PHONE_OWNER_PASS=$PASS
ZOLIK_E2E_PHONE_OWNER_TOKEN=$TOKEN
ENV
cat "$OUT/env"
