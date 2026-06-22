#!/usr/bin/env bash
# Run the perfectpixel desktop app in development mode (Wails dev + Vite HMR)
set -euo pipefail

cd "$(dirname "$0")"

# Locate the wails CLI (PATH → GOPATH/bin)
if command -v wails >/dev/null 2>&1; then
  WAILS=wails
elif [ -x "$(go env GOPATH 2>/dev/null || echo "$HOME/go")/bin/wails" ]; then
  WAILS="$(go env GOPATH 2>/dev/null || echo "$HOME/go")/bin/wails"
else
  echo "Error: wails CLI not found." >&2
  echo "Install: go install github.com/wailsapp/wails/v2/cmd/wails@latest" >&2
  exit 1
fi

# Install frontend dependencies (first run only)
if [ ! -d frontend/node_modules ]; then
  echo "Installing frontend dependencies..."
  (cd frontend && npm install)
fi

exec "$WAILS" dev "$@"
