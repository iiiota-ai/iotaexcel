#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

if [ -n "${GO_EXE:-}" ]; then
  GO_CMD=$GO_EXE
elif command -v go >/dev/null 2>&1; then
  GO_CMD=go
else
  echo "Go executable not found. Install Go or set GO_EXE." >&2
  exit 1
fi

cd "$ROOT"
mkdir -p .gocache
GOCACHE="$ROOT/.gocache" "$GO_CMD" test ./...
