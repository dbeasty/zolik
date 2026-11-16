#!/usr/bin/env bash
#
# Runs tests/relay.spec.ts against a fake phonehost that breaks the test
# early, and checks the failed test leaves no phone process behind.
#
#   ZOLIK_E2E_API_BASE=http://127.0.0.1:8090 e2e/test-relay-cleanup.sh
#   ZOLIK_E2E_API_BASE=… e2e/test-relay-cleanup.sh path/to/old-relay.spec.ts
#
# Needs a running server (the spec registers an account before it starts the
# phone); nothing past that is reached, so no web client is needed.
#
#   silent   the phone never prints the line it is up — the spec gives up
#            after 30 s
#   badcode  the phone comes up with a relay code the spec rejects, on the
#            line before its try/finally
#
# Both runs are meant to fail; what is checked is what they leave running.

set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPEC="${1:-$HERE/tests/relay.spec.ts}"
[[ -f "$SPEC" ]] || { echo "no such spec: $SPEC" >&2; exit 2; }
: "${ZOLIK_E2E_API_BASE:?set ZOLIK_E2E_API_BASE to a running server}"
curl -sf -m 5 "$ZOLIK_E2E_API_BASE/healthz" >/dev/null || { echo "no server at $ZOLIK_E2E_API_BASE" >&2; exit 2; }

T="${TMPDIR:-/tmp}"
SCRATCH="$(mktemp -d "${T%/}/relay-cleanup-test.XXXXXX")"
# Beside the real spec, so its relative imports resolve.
COPY="$HERE/tests/relay-under-test-$$.spec.ts"
cp "$SPEC" "$COPY"
trap 'rm -f "$COPY"; pkill -9 -f "$SCRATCH/" 2>/dev/null; rm -rf "$SCRATCH"' EXIT

cat >"$SCRATCH/phone-silent" <<'EOF'
#!/bin/bash
exec -a "$0" sleep 600
EOF
cat >"$SCRATCH/phone-badcode" <<'EOF'
#!/bin/bash
echo '{"baseUrl":"http://127.0.0.1:1","relayCode":"not-a-code","relayUrl":""}'
exec -a "$0" sleep 600
EOF
chmod +x "$SCRATCH"/phone-*

PASS=0
FAIL=0
for c in silent badcode; do
  (cd "$HERE" && PATH="/opt/homebrew/bin:$PATH" ZOLIK_E2E_PHONEHOST="$SCRATCH/phone-$c" \
    npx playwright test "$(basename "$COPY")" --workers=1 --reporter=line >"$SCRATCH/log-$c" 2>&1)
  code=$?
  sleep 1
  left="$(pgrep -f "$SCRATCH/phone-$c" || true)"
  if [[ $code -eq 0 ]]; then
    FAIL=$((FAIL + 1)); printf '  FAIL  %s: the spec passed against a broken phone\n' "$c"
  elif ! grep -q "relay-under-test" "$SCRATCH/log-$c"; then
    FAIL=$((FAIL + 1)); printf '  FAIL  %s: the spec did not run:\n' "$c"; tail -5 "$SCRATCH/log-$c" | sed 's/^/      /'
  elif [[ -n "$left" ]]; then
    FAIL=$((FAIL + 1)); printf '  FAIL  %s: the failed test left the phone running (pid %s)\n' "$c" "$(echo $left)"
  else
    PASS=$((PASS + 1)); printf '  ok    %s: failed, and stopped its phone\n' "$c"
  fi
done

echo
echo "$PASS passed, $FAIL failed"
[[ $FAIL -eq 0 ]]
