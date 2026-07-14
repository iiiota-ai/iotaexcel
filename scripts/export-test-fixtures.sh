#!/usr/bin/env sh
set -eu

# 一键验证当前源码版工具的常用导出链路。
# 流程包括生成 fixture、导出自描述/非自描述 .bytes、反解析 CSV/JSON，以及生成 C# reader。
# 该脚本是 scripts/export-test-fixtures.ps1 的 sh 版本，便于 Linux/macOS 或 Git Bash 环境使用。
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

# 允许通过 GO_EXE 指定 Go 可执行文件；未指定时从 PATH 查找 go。
if [ -n "${GO_EXE:-}" ]; then
  GO_CMD=$GO_EXE
elif command -v go >/dev/null 2>&1; then
  GO_CMD=go
else
  echo "Go executable not found. Install Go or set GO_EXE." >&2
  exit 1
fi

cd "$ROOT"

# 先重建 fixture，保证后续 convert/codegen 的输入稳定可重复。
echo "[1/8] Generate Excel fixtures"
sh tests/generate-fixtures.sh

# 只处理 valid 目录，invalid fixture 是给失败测试使用的，不应进入一键导出流程。
echo "[2/8] Export self-describing .bytes files"
"$GO_CMD" run ./cmd/iotaexcel convert --input tests/testdata/excels/valid --output out/bytes --format bin --check-ref --overwrite --log-level info

echo "[3/8] Decode self-describing .bytes files to JSON"
"$GO_CMD" run ./cmd/iotaexcel decode --input out/bytes --output out/decoded-json --format json --overwrite --log-level info

echo "[4/8] Decode self-describing .bytes files to CSV"
"$GO_CMD" run ./cmd/iotaexcel decode --input out/bytes --output out/decoded-csv --format csv --overwrite --log-level info

echo "[5/8] Export non-self-describing .bytes files"
"$GO_CMD" run ./cmd/iotaexcel convert --input tests/testdata/excels/valid --output out/bytes-compact --format bin --self-describing=false --check-ref --overwrite --log-level info

echo "[6/8] Decode non-self-describing .bytes files to JSON"
"$GO_CMD" run ./cmd/iotaexcel decode --input out/bytes-compact --schema-input tests/testdata/excels/valid --output out/decoded-json-compact --format json --self-describing=false --overwrite --log-level info

echo "[7/8] Decode non-self-describing .bytes files to CSV"
"$GO_CMD" run ./cmd/iotaexcel decode --input out/bytes-compact --schema-input tests/testdata/excels/valid --output out/decoded-csv-compact --format csv --self-describing=false --overwrite --log-level info

# 使用相同输入生成 C# reader，输出到 out/codegen 供人工检查或后续编译验证使用。
echo "[8/8] Generate C# code"
"$GO_CMD" run ./cmd/iotaexcel codegen --input tests/testdata/excels/valid --output out/codegen --lang csharp --check-ref --overwrite --log-level info

echo "Done."
echo "Self-describing .bytes output: out/bytes"
echo "Self-describing decoded JSON output: out/decoded-json"
echo "Self-describing decoded CSV output: out/decoded-csv"
echo "Non-self-describing .bytes output: out/bytes-compact"
echo "Non-self-describing decoded JSON output: out/decoded-json-compact"
echo "Non-self-describing decoded CSV output: out/decoded-csv-compact"
echo "C# output: out/codegen"
