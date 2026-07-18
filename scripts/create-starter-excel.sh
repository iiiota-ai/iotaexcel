#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

run_powershell() {
  if command -v pwsh >/dev/null 2>&1; then
    pwsh -NoProfile -ExecutionPolicy Bypass -File "$@"
    return
  fi
  if command -v powershell >/dev/null 2>&1; then
    powershell -NoProfile -ExecutionPolicy Bypass -File "$@"
    return
  fi
  if command -v powershell.exe >/dev/null 2>&1; then
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$@"
    return
  fi
  echo "PowerShell executable not found. Install PowerShell or run scripts/create-starter-excel.ps1 directly on Windows." >&2
  exit 1
}

run_powershell "$ROOT/scripts/create-starter-excel.ps1" "$@"
