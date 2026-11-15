#!/usr/bin/env bash
#
# Breaks e2e/macos/run.mjs's setup on purpose and checks the run leaves no
# process behind. No real app, server or Bluetooth: the app is a fake bundle
# whose executable idles, and `go build` is a stub that writes a fake server.
#
#   e2e/macos/test-cleanup.sh                  # this checkout's run.mjs
#   e2e/macos/test-cleanup.sh path/to/run.mjs  # any other version of it
#
# Cases:
#   hang     the server never answers; the run gives up after 30 s
#   sigint   the server answers, the app never starts, and the run is sent
#            SIGINT while it waits — as Ctrl-C or a harness timeout would
#
# The fake processes name themselves after their own paths (exec -a), which
# are inside the run's work folder, so a survivor is any process with the
# scratch TMPDIR on its command line.

set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
RUNNER="${1:-$HERE/run.mjs}"
[[ -f "$RUNNER" ]] || { echo "no such runner: $RUNNER" >&2; exit 2; }

T="${TMPDIR:-/tmp}"
SCRATCH="$(mktemp -d "${T%/}/macos-cleanup-test.XXXXXX")"
BIN="$SCRATCH/bin"
mkdir -p "$BIN" "$SCRATCH/tmp"

# run.mjs resolves ROOT from its own location, so a copy of it goes where the
# real one lives, beside it, and is removed afterwards.
COPY="$HERE/.run-under-test-$$.mjs"
cp "$RUNNER" "$COPY"
trap 'rm -f "$COPY"; pkill -9 -f "$SCRATCH/" 2>/dev/null; [[ -n "${KEEP:-}" ]] && echo "kept $SCRATCH" || rm -rf "$SCRATCH"' EXIT

APP="$SCRATCH/Jokerless.app"
mkdir -p "$APP/Contents/MacOS"
printf '#!/bin/bash\nexec -a "$0" sleep 600\n' >"$APP/Contents/MacOS/Jokerless"
chmod +x "$APP/Contents/MacOS/Jokerless"
cat >"$APP/Contents/Info.plist" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>com.jokerless.mac.fake</string>
<key>CFBundleExecutable</key><string>Jokerless</string>
</dict></plist>
EOF

# go build -o <out> ./cmd/server → a fake server. $CASE decides what it does.
cat >"$BIN/go" <<'EOF'
#!/bin/bash
out="$3"
if [[ "$CASE" == hang ]]; then
  printf '#!/bin/bash\nexec -a "$0" sleep 600\n' >"$out"
else
  cat >"$out" <<'PY'
#!/usr/bin/env python3
import http.server, os, sys
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200); self.send_header('Content-Type', 'application/json'); self.end_headers()
        self.wfile.write(b'{"version":"0.0.0-fake"}')
    def log_message(self, *a): pass
http.server.HTTPServer(('127.0.0.1', int(os.environ['PORT'])), H).serve_forever()
PY
fi
chmod +x "$out"
EOF
chmod +x "$BIN/go"

PASS=0
FAIL=0
survivors() { pgrep -f "$SCRATCH/tmp/" 2>/dev/null || true; }

check() { # label
  sleep 1
  local left
  left="$(survivors)"
  if [[ -n "$left" ]]; then
    FAIL=$((FAIL + 1))
    printf '  FAIL  %s: left running:\n' "$1"
    ps -o pid=,command= -p $(echo $left | tr ' ' ',') | sed 's/^/      /'
    pkill -9 -f "$SCRATCH/tmp/" 2>/dev/null
  else
    PASS=$((PASS + 1))
    printf '  ok    %s\n' "$1"
  fi
}

launch() { # case → pid in $PID
  CASE="$1" TMPDIR="$SCRATCH/tmp/" PATH="$BIN:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin" \
    ZOLIK_E2E_MACOS_APP="$APP" node "$COPY" >"$SCRATCH/log-$1" 2>&1 &
  PID=$!
}

echo "run.mjs: $RUNNER"

launch hang
wait "$PID"
check "a server that never answers is stopped when the run gives up"

launch sigint
for _ in $(seq 1 100); do grep -q "server http" "$SCRATCH/log-sigint" && break; sleep 0.2; done
sleep 1
kill -INT "$PID"
for _ in $(seq 1 150); do kill -0 "$PID" 2>/dev/null || break; sleep 0.2; done
kill -0 "$PID" 2>/dev/null && { kill -9 "$PID"; echo "  (the run was still going 30 s after SIGINT and was killed)"; }
check "SIGINT while waiting for the app stops the server and the app"

echo
echo "$PASS passed, $FAIL failed"
[[ $FAIL -eq 0 ]]
