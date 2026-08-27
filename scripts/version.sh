#!/bin/sh
# The one place that decides what "version" means. dev-stack.sh, the npm
# scripts, and the Docker build args all come through here, so the RN and TUI
# footers can never disagree for a reason other than actually being different
# builds.
#
#   version.sh                 -> "1.1.1.2 7feb025"
#   version.sh --export        assignments, for: eval "$(scripts/version.sh --export)"
#   version.sh --exec CMD…     export the vars, then exec CMD (used by npm scripts)
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

# First line only, stripped of stray whitespace/CR — a Windows-edited VERSION
# file would otherwise poison every version string in the system.
base=$(sed -n '1p' "$ROOT/VERSION" 2>/dev/null | tr -d ' \t\r\n')
case "$base" in *.*.*) ;; *) base=0.0.0 ;; esac

build=0
commit=unknown
if git -C "$ROOT" rev-parse --git-dir >/dev/null 2>&1; then
  commit=$(git -C "$ROOT" rev-parse --short=7 HEAD 2>/dev/null || echo unknown)
  # The 4th part counts commits since VERSION itself last changed, so bumping
  # the file resets it to 0.
  anchor=$(git -C "$ROOT" log -1 --format=%H -- VERSION 2>/dev/null || true)
  if [ -n "$anchor" ]; then
    build=$(git -C "$ROOT" rev-list --count "$anchor"..HEAD 2>/dev/null || echo 0)
  fi
  # A shallow clone (e.g. CI's default fetch-depth: 1) only knows what it
  # fetched — report 0 rather than a plausible-looking, wrong number.
  gitdir=$(git -C "$ROOT" rev-parse --git-dir 2>/dev/null || true)
  if [ -n "$gitdir" ]; then
    case "$gitdir" in
      /*) shallow_path="$gitdir/shallow" ;;
      *) shallow_path="$ROOT/$gitdir/shallow" ;;
    esac
    [ -f "$shallow_path" ] && build=0
  fi
  [ -z "$(git -C "$ROOT" status --porcelain 2>/dev/null)" ] || commit="$commit-dirty"
fi

ZOLIK_VERSION="$base.$build"
ZOLIK_COMMIT="$commit"

case "${1:-}" in
  --export)
    # `export`, not a bare assignment: this is meant to be eval'd, and a bare
    # ZOLIK_VERSION=... would only ever be visible to the eval'ing shell
    # itself, never to a child process like `docker compose` that reads it
    # from its own inherited environment.
    printf 'export ZOLIK_VERSION=%s\nexport ZOLIK_COMMIT=%s\n' "$ZOLIK_VERSION" "$ZOLIK_COMMIT"
    ;;
  --exec)
    shift
    export ZOLIK_VERSION ZOLIK_COMMIT
    export EXPO_PUBLIC_ZOLIK_VERSION="$ZOLIK_VERSION" EXPO_PUBLIC_ZOLIK_COMMIT="$ZOLIK_COMMIT"
    exec "$@"
    ;;
  *)
    printf '%s %s\n' "$ZOLIK_VERSION" "$ZOLIK_COMMIT"
    ;;
esac
