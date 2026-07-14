#!/usr/bin/env sh
set -eu

# tests/generate-fixtures.ps1 的 sh 包装脚本。
# 真正的 fixture 生成逻辑在 PowerShell 版本中维护；这里负责在类 Unix/Git Bash 环境中找到可用的 PowerShell。
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

# 按跨平台可用性依次尝试 pwsh、powershell、powershell.exe。
# 找不到 PowerShell 时给出明确错误，避免静默跳过 fixture 生成。
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
  echo "PowerShell executable not found. Install PowerShell or run tests/generate-fixtures.ps1 directly on Windows." >&2
  exit 1
}

run_powershell "$ROOT/tests/generate-fixtures.ps1"
