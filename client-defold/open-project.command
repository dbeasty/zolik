#!/bin/bash
# Opens this Defold project with the correct project root (the folder containing game.project).
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
DEFOLD_BIN="/Applications/Defold.app/Contents/MacOS/Defold"

# Important: set working directory. Defold appears to resolve game.project relative to the working directory.
cd "$DIR"
exec "$DEFOLD_BIN" .
