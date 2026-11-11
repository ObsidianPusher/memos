#!/bin/sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ -f "$SCRIPT_DIR/go.mod" ]; then
  ROOT="$SCRIPT_DIR"
else
  ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
fi
cd "$ROOT"

BUILD_DIR="$ROOT/build"
OUTPUT="$BUILD_DIR/memos-linux-arm64-musl"
DIST_DIR="$ROOT/server/router/frontend/dist"
COREPACK_HOME="$ROOT/.corepack"
GO_CACHE_ROOT="$ROOT/.cache/go-mod"
GO_MODULE_CACHE="$GO_CACHE_ROOT/pkg/mod"
MARKER_FILE="$GO_CACHE_ROOT/.initialized"
PNPM_CACHE_DIR="$ROOT/.cache/pnpm"
PNPM_MARKER="$PNPM_CACHE_DIR/node_modules_installed"
NPM_REGISTRY="https://registry.npmmirror.com"

command -v go >/dev/null 2>&1 || {
  echo "go command is not available; please install Go before building" >&2
  exit 1
}

mkdir -p "$COREPACK_HOME"
mkdir -p "$GO_MODULE_CACHE"
mkdir -p "$PNPM_CACHE_DIR"

GOPROXY=${GOPROXY:-https://goproxy.cn,direct}
GOSUMDB=${GOSUMDB:-off}
export GOPROXY GOSUMDB
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=arm64
export GO111MODULE=on
export GOMODCACHE="$GO_MODULE_CACHE"
export PNPM_HOME="$PNPM_CACHE_DIR/home"
export PNPM_STORE_PATH="$PNPM_CACHE_DIR/store"
export NPM_CONFIG_REGISTRY="$NPM_REGISTRY"
mkdir -p "$PNPM_HOME" "$PNPM_STORE_PATH"

SKIP_DOWNLOAD="${MUSL_BUILD_SKIP_MODULE_DOWNLOAD:-0}"
if [ "$SKIP_DOWNLOAD" = "1" ]; then
  echo "Skipping Go module download because MUSL_BUILD_SKIP_MODULE_DOWNLOAD=1"
elif [ ! -f "$MARKER_FILE" ]; then
  echo "Downloading Go modules (cached under $GO_MODULE_CACHE)..."
  go mod download
  touch "$MARKER_FILE"
else
  echo "Go module cache ready at $GO_MODULE_CACHE"
fi

BACKUP_DIR="$(mktemp -d)"
if [ -d "$DIST_DIR" ]; then
  cp -R "$DIST_DIR" "$BACKUP_DIR/dist"
else
  mkdir -p "$DIST_DIR"
fi

restore_dist() {
  if [ -z "$BACKUP_DIR" ]; then
    return
  fi
  rm -rf "$DIST_DIR"
  if [ -d "$BACKUP_DIR/dist" ]; then
    cp -R "$BACKUP_DIR/dist" "$DIST_DIR"
  fi
  rm -rf "$BACKUP_DIR"
  BACKUP_DIR=""
}

cleanup() {
  exit_code=$?
  if [ "$exit_code" -ne 0 ]; then
    restore_dist
  else
    if [ -d "$BACKUP_DIR" ]; then
      rm -rf "$BACKUP_DIR"
    fi
  fi
}

trap 'cleanup' EXIT

echo "Installing frontend dependencies..."
if [ -f "$PNPM_MARKER" ] && [ "$PNPM_MARKER" -nt "$ROOT/web/pnpm-lock.yaml" ]; then
  echo "Frontend dependencies already installed (marker: $PNPM_MARKER)"
else
  COREPACK_HOME="$COREPACK_HOME" pnpm --dir "$ROOT/web" install --frozen-lockfile
  touch "$PNPM_MARKER"
fi

echo "Building frontend bundle..."
COREPACK_HOME="$COREPACK_HOME" pnpm --dir "$ROOT/web" release

mkdir -p "$BUILD_DIR"
echo "Building musl/arm64 binary..."
go build -ldflags='-s -w' -o "$OUTPUT" ./bin/memos/main.go

echo "Build successful: $OUTPUT"
