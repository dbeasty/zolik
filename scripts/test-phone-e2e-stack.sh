#!/usr/bin/env bash
#
# Breaks scripts/phone-e2e-stack.sh's setup on purpose, one step at a time,
# and checks that each break stops the script with a non-zero exit and leaves
# no process behind — and that an unbroken run still writes a complete env.
#
#   scripts/test-phone-e2e-stack.sh                 # this checkout's script
#   scripts/test-phone-e2e-stack.sh path/to/old.sh  # any other version
#
# No server, Metro or network: the script runs from a scratch copy of the
# checkout where go, curl and npx are stubs. `go build` writes fake binaries
# that stand in for the cloud and the phone, and curl answers for the fake
# cloud only once it has said it is up. The script prepends /opt/homebrew/bin
# to PATH, which would put the real tools in front of the stubs, so that one
# line is rewritten in the copy.

set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${1:-${HERE}/phone-e2e-stack.sh}"
[[ -f "$SCRIPT" ]] || { echo "no such script: $SCRIPT" >&2; exit 2; }

WORK="$(mktemp -d "${TMPDIR:-/tmp}/phone-stack-test.XXXXXX")"
ROOT="$WORK/zolik"
BIN="$WORK/bin"
mkdir -p "$ROOT/scripts" "$ROOT/server" "$ROOT/client-react-native" "$BIN"
sed 's|^PATH=/opt/homebrew/bin:\$PATH$|PATH=$PATH|' "$SCRIPT" >"$ROOT/scripts/phone-e2e-stack.sh"
chmod +x "$ROOT/scripts/phone-e2e-stack.sh"
cat >"$ROOT/scripts/version.sh" <<'EOF'
case "$1" in
  --export) echo 'ZOLIK_VERSION=0.0.0-test; ZOLIK_COMMIT=test' ;;
  --exec) shift; exec "$@" ;;
esac
EOF
chmod +x "$ROOT/scripts/version.sh"

stub() { printf '#!/usr/bin/env bash\n%s\n' "$2" >"$BIN/$1"; chmod +x "$BIN/$1"; }

# go build -o <out> ./cmd/<name>: the cloud marks itself up and idles; the
# phone reports an address and idles. $BREAK picks which one misbehaves.
stub go '
[[ "$1" == build ]] || exit 1
out="$3" pkg="$4"
[[ "$BREAK" == build ]] && { echo "build broke" >&2; exit 1; }
case "$pkg" in
  ./cmd/server) body="[[ \"\$BREAK\" == cloud ]] && { echo \"cloud: listen: address in use\" >&2; exit 1; }
touch \"\$STATE/cloud-up\"; exec -a \"\$0\" sleep 300" ;;
  ./cmd/phonehost) body="[[ \"\$BREAK\" == phone ]] && { echo \"phone: enroll refused\" >&2; exit 1; }
echo \"{\\\"baseUrl\\\":\\\"http://127.0.0.1:1\\\",\\\"lanPort\\\":4242}\"; exec -a \"\$0\" sleep 300" ;;
esac
printf "#!/usr/bin/env bash\n%s\n" "$body" >"$out"; chmod +x "$out"'

# curl for the fake cloud: nothing answers until the cloud is up.
stub curl '
url=""; for a in "$@"; do case "$a" in http*) url="$a";; esac; done
case "$url" in
  *:18999/*) [[ "$BREAK" == web ]] && exit 7; exit 0 ;;
esac
[[ -f "$STATE/cloud-up" ]] || exit 7
case "$url" in
  */healthz) echo ok ;;
  */auth/email/start) echo "{}" ;;
  */auth/dev/last-code*) [[ "$BREAK" == code ]] && exit 22; echo "{\"code\":\"123456\"}" ;;
  */auth/email/verify) echo "{\"accessToken\":\"tok\",\"userId\":\"u1\",\"offlinePass\":\"pass\"}" ;;
  *) exit 22 ;;
esac'

stub npx '
case "$*" in
  "expo start"*) exec -a "$0-metro" sleep 300 ;;
  "expo export"*) [[ "$BREAK" == export ]] && { echo "export broke" >&2; exit 1; }
    while [[ $# -gt 0 && "$1" != --output-dir ]]; do shift; done
    mkdir -p "$2" && touch "$2/index.html" ;;
esac'

PASS=0
FAIL=0
LEAKED=()
ok()  { PASS=$((PASS + 1)); printf '  ok    %s\n' "$CASE"; }
bad() { FAIL=$((FAIL + 1)); printf '  FAIL  %s: %s\n' "$CASE" "$*"; }

# Every process the copy started carries $WORK on its command line: the idle
# fakes name themselves after their own path (exec -a).
survivors() { pgrep -f "$WORK/" 2>/dev/null | grep -v "^$$\$" || true; }

# run <label> <BREAK> — returns the script's exit code in $CODE
run() {
  CASE="$1"
  OUT="$WORK/out-$2"
  STATE="$WORK/state-$2"
  mkdir -p "$STATE"
  env -i HOME="$HOME" PATH="$BIN:/usr/bin:/bin:/usr/sbin:/sbin" TMPDIR="$WORK" \
    OUT="$OUT" STATE="$STATE" BREAK="$2" CLOUD_PORT=18998 WEB_PORT=18999 \
    CLOUD_WAIT=3 PHONE_WAIT=3 WEB_WAIT=3 \
    "$ROOT/scripts/phone-e2e-stack.sh" >"$WORK/log-$2" 2>&1
  CODE=$?
}

expect_failed_cleanly() {
  if [[ $CODE -eq 0 ]]; then
    bad "exited 0 (env: $(tr '\n' ' ' <"$OUT/env" 2>/dev/null))"
  elif [[ -n "$(survivors)" ]]; then
    bad "exited $CODE but left processes running: $(survivors | tr '\n' ' ')"
  else
    ok
  fi
  pkill -f "$WORK/" 2>/dev/null || true
}

echo "phone-e2e-stack.sh: $SCRIPT"

run "a failed build stops the script" build;                expect_failed_cleanly
run "a cloud that exits stops the script and nothing leaks" cloud; expect_failed_cleanly
run "a failed web export stops the script" export;          expect_failed_cleanly
run "no sign-in code stops the script" code;                expect_failed_cleanly
run "a phone that exits stops the script" phone;            expect_failed_cleanly
run "a web server that never answers stops the script" web; expect_failed_cleanly

run "an unbroken run writes a complete env and keeps its processes" none
if [[ $CODE -ne 0 ]]; then
  bad "exited $CODE"; tail -5 "$WORK/log-none" | sed 's/^/      /'
elif grep -Eq '=$' "$OUT/env"; then
  bad "env has empty values: $(tr '\n' ' ' <"$OUT/env")"
elif [[ -z "$(survivors)" ]]; then
  bad "nothing left running for the spec to use"
else
  "$ROOT/scripts/phone-e2e-stack.sh" stop "$OUT" >/dev/null 2>&1
  sleep 0.3
  if [[ -n "$(survivors)" ]]; then bad "stop left processes running: $(survivors | tr '\n' ' ')"; else ok; fi
fi

pkill -f "$WORK/" 2>/dev/null || true
[[ -n "${KEEP:-}" ]] && echo "kept $WORK" || rm -rf "$WORK"
echo
echo "$PASS passed, $FAIL failed"
[[ $FAIL -eq 0 ]]
