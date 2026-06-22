#!/usr/bin/env bash
# install.sh — installer script for the PerfectPixel skill's headless generator (ppgen).
#
# Install priority:
#   1) If skill/bin/ppgen already works, use it as-is (skip reinstall).
#   2) Download a prebuilt binary matching the OS/arch from GitHub Releases (Go not required).
#   3) (If the download fails) build from Go source: $PERFECTPIXEL_SRC > bundled .src > repo clone.
# On success, print the binary's absolute path as the last line (stdout).
#
# Environment variables:
#   PP_VERSION   Release tag to download (default: latest)
#   PP_BUILD=1   Skip the download and always build from source
#   PERFECTPIXEL_SRC  Local Go source path (containing go.mod)
set -euo pipefail

REPO="gykim80/perfectpixel-studio"
REPO_URL="https://github.com/${REPO}.git"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN_DIR="$SKILL_DIR/bin"
BIN="$BIN_DIR/ppgen"
mkdir -p "$BIN_DIR"

valid() { [ -x "$1" ] && "$1" -dump >/dev/null 2>&1; }

# 1) Reuse an already-working binary
if valid "$BIN"; then
  echo "$BIN"; exit 0
fi

# Map OS/arch → release asset name
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
esac
ext=""
case "$os" in
  msys*|mingw*|cygwin*) os="windows"; ext=".exe" ;;
esac
asset="ppgen-${os}-${arch}${ext}"

# 2) Download a prebuilt binary (skipped if PP_BUILD=1)
if [ "${PP_BUILD:-0}" != "1" ]; then
  ver="${PP_VERSION:-latest}"
  if [ "$ver" = "latest" ]; then
    url="https://github.com/${REPO}/releases/latest/download/${asset}"
  else
    url="https://github.com/${REPO}/releases/download/${ver}/${asset}"
  fi
  echo "Attempting to download prebuilt binary: $url" >&2
  tmp="$(mktemp)"
  if curl -fsSL "$url" -o "$tmp" 2>/dev/null && [ -s "$tmp" ]; then
    chmod +x "$tmp"
    mv "$tmp" "$BIN"
    if valid "$BIN"; then
      echo "$BIN"; exit 0
    fi
    echo "Downloaded binary does not work → falling back to source build" >&2
  else
    rm -f "$tmp"
    echo "No prebuilt binary / download failed → falling back to source build" >&2
  fi
fi

# 3) Build from source
if ! command -v go >/dev/null 2>&1; then
  echo "Error: could not obtain a prebuilt binary, and Go (1.25+) is not installed either. Install it from https://go.dev/dl/ and try again." >&2
  exit 1
fi

SRC=""
if [ -n "${PERFECTPIXEL_SRC:-}" ] && [ -f "${PERFECTPIXEL_SRC}/go.mod" ]; then
  SRC="$PERFECTPIXEL_SRC"
elif [ -f "$SKILL_DIR/.src/go.mod" ]; then
  SRC="$SKILL_DIR/.src"
else
  probe="$SKILL_DIR"
  for _ in 1 2 3 4 5 6; do
    probe="$(dirname "$probe")"
    if [ -f "$probe/go.mod" ] && [ -d "$probe/cmd/ppgen" ]; then
      SRC="$probe"; break
    fi
  done
fi

if [ -z "$SRC" ]; then
  SRC="$SKILL_DIR/.src"
  if [ ! -d "$SRC/.git" ]; then
    echo "Cloning source from public repository: $REPO_URL" >&2
    git clone --depth 1 "$REPO_URL" "$SRC" >&2
  else
    git -C "$SRC" pull --ff-only >&2 || true
  fi
fi

echo "Building ppgen (source: $SRC)" >&2
( cd "$SRC" && go build -o "$BIN" ./cmd/ppgen )
if ! valid "$BIN"; then
  echo "Error: build failed." >&2
  exit 1
fi
echo "$BIN"
