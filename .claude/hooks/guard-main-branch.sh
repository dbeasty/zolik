#!/usr/bin/env bash
#
# Refuse an Edit/Write whose target sits on a repository's main branch.
#
# This exists because a session once did a whole rules change directly in the
# main checkout and left it uncommitted — one `git checkout` from gone, with no
# reflog to recover it, and invisible to the other sessions sharing that tree.
#
# It answers "ask" rather than "deny": the guard is against a session drifting
# onto main on its own, not against a deliberate edit there. Approving the
# prompt is the deliberate act.
set -uo pipefail

payload=$(cat)
file=$(printf '%s' "$payload" | jq -r '.tool_input.file_path // empty')
[ -n "$file" ] || exit 0

# A Write may name a path that does not exist yet, so walk up to the nearest
# directory that does before asking git anything.
dir=$(dirname "$file")
while [ ! -d "$dir" ] && [ "$dir" != "/" ] && [ "$dir" != "." ]; do
  dir=$(dirname "$dir")
done

# Not a repository at all (a scratchpad file, /tmp, somewhere else entirely):
# this guard has no opinion.
branch=$(git -C "$dir" branch --show-current 2>/dev/null) || exit 0
[ -n "$branch" ] || exit 0

case "$branch" in
  main | master) ;;
  # A worktree is already on its own branch — which is the point — so it
  # resolves here and passes untouched.
  *) exit 0 ;;
esac

root=$(git -C "$dir" rev-parse --show-toplevel 2>/dev/null) || exit 0
name=$(basename "$root")

reason="$file is on '$branch' in $name.

This project develops on branches: several Claude sessions share this checkout, so an edit here can collide with another session's work, and anything left uncommitted on main is one 'git checkout' from being lost.

Do this instead, then make the edit inside the worktree:

  git worktree add $root/.claude/worktrees/<slug> -b <branch> origin/$branch

Approve this prompt only if editing '$branch' directly is what is actually wanted."

jq -nc --arg reason "$reason" '{
  hookSpecificOutput: {
    hookEventName: "PreToolUse",
    permissionDecision: "ask",
    permissionDecisionReason: $reason
  }
}'
