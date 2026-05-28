#!/usr/bin/env bash
# Build local-sub2api-lite desktop executable.
# Default: release build (no DevTools, no console).
# Set SUB2API_DESKTOP_DEBUG=1 before running this script to produce a debug build
# (DevTools open on startup, attached Windows console, "(Debug)" in window title).

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

debug_build=0
case "${SUB2API_DESKTOP_DEBUG:-}" in
  1|true|TRUE|yes|y|on) debug_build=1 ;;
esac

echo "==> Building frontend..."
(cd "$ROOT/frontend" && pnpm install && pnpm run build)

echo "==> Desktop shell uses desktop/frontend/dist/index.html (startup loader only)"

echo "==> Building Wails desktop binary..."
mkdir -p "$ROOT/dist"
(cd "$ROOT/desktop" && go mod tidy)

if [ "$debug_build" = "1" ]; then
  OUT_NAME="local-sub2api-lite-debug"
else
  OUT_NAME="local-sub2api-lite"
fi
OUT="$ROOT/dist/$OUT_NAME"
if [ "$(go env GOOS)" = "windows" ]; then
  OUT="${OUT}.exe"
elif [ -z "${GOOS:-}" ] && [[ "$(uname -s 2>/dev/null || true)" == MINGW* ]]; then
  OUT="${OUT}.exe"
fi

if [ "$debug_build" = "1" ]; then
  TAGS="production,debug,embed"
  LDFLAGS=""
else
  TAGS="production,embed"
  LDFLAGS="-s -w"
  if [ "$(go env GOOS)" = "windows" ]; then
    LDFLAGS="$LDFLAGS -H windowsgui"
  fi
fi

echo "==> Tags: $TAGS"
echo "==> ldflags: $LDFLAGS"
(cd "$ROOT/desktop" && go build -tags "$TAGS" -ldflags "$LDFLAGS" -o "$OUT" .)
echo "==> Done: $OUT"
if [ "$debug_build" = "1" ]; then
  echo "    Debug build: DevTools and console auto-enabled. Run it from a terminal to see server logs."
fi
