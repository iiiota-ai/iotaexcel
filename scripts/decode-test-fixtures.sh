#!/usr/bin/env sh
set -eu

# scripts/decode-test-fixtures.ps1 的 sh 版本。
# 用于在 Linux/macOS 或 Git Bash 环境中验证 decode 命令可以把 .bytes 反解析为 JSON/CSV。
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

assert_file() {
  file="$1"
  test -f "$file" || {
    echo "Expected file not found: $file" >&2
    exit 1
  }
}

assert_contains() {
  file="$1"
  text="$2"
  assert_file "$file"
  if ! grep -F -- "$text" "$file" >/dev/null 2>&1; then
    echo "Expected '$text' in $file" >&2
    exit 1
  fi
}

cd "$ROOT"

BYTES_OUT="out/decode-test/bytes"
JSON_OUT="out/decode-test/json"
CSV_OUT="out/decode-test/csv"
COMPACT_BYTES_OUT="out/decode-test/bytes-compact"
COMPACT_JSON_OUT="out/decode-test/json-compact"
COMPACT_CSV_OUT="out/decode-test/csv-compact"
PRINT_LOG="out/decode-test/decode-print.txt"

echo "[1/8] Generate Excel fixtures"
sh tests/generate-fixtures.sh

echo "[2/8] Convert valid fixtures to self-describing .bytes"
"$GO_CMD" run ./cmd/iotaexcel convert --input tests/testdata/excels/valid --output "$BYTES_OUT" --format bin --check-ref --overwrite --log-level info

echo "[3/8] Decode self-describing .bytes to JSON"
"$GO_CMD" run ./cmd/iotaexcel decode --input "$BYTES_OUT" --output "$JSON_OUT" --format json --print --print-mode concise --overwrite --log-level info | tee "$PRINT_LOG"

echo "[4/8] Decode self-describing .bytes to CSV"
"$GO_CMD" run ./cmd/iotaexcel decode --input "$BYTES_OUT" --output "$CSV_OUT" --format csv --overwrite --log-level info

echo "[5/8] Convert valid fixtures to non-self-describing .bytes"
"$GO_CMD" run ./cmd/iotaexcel convert --input tests/testdata/excels/valid --output "$COMPACT_BYTES_OUT" --format bin --self-describing=false --check-ref --overwrite --log-level info

echo "[6/8] Decode non-self-describing .bytes to JSON"
"$GO_CMD" run ./cmd/iotaexcel decode --input "$COMPACT_BYTES_OUT" --schema-input tests/testdata/excels/valid --output "$COMPACT_JSON_OUT" --format json --self-describing=false --overwrite --log-level info

echo "[7/8] Decode non-self-describing .bytes to CSV"
"$GO_CMD" run ./cmd/iotaexcel decode --input "$COMPACT_BYTES_OUT" --schema-input tests/testdata/excels/valid --output "$COMPACT_CSV_OUT" --format csv --self-describing=false --overwrite --log-level info

echo "[8/8] Verify decoded outputs"
assert_contains "$JSON_OUT/Config_ItemConfig.json" '"name": "Sword"'
assert_contains "$JSON_OUT/Config_HeroConfig.json" '"itemRef": "1001"'
assert_contains "$CSV_OUT/Config_ItemConfig.csv" "Sword"
assert_contains "$CSV_OUT/Config_HeroConfig.csv" "hero_001"
assert_contains "$COMPACT_JSON_OUT/Config_ItemConfig.json" '"selfDescribing": false'
assert_contains "$COMPACT_JSON_OUT/Config_ItemConfig.json" '"name": "Sword"'
assert_contains "$COMPACT_CSV_OUT/Config_ItemConfig.csv" "Sword"
assert_contains "$PRINT_LOG" "IOTB"
assert_contains "$PRINT_LOG" "2	name	string"
assert_contains "$PRINT_LOG" "18	2	2	Sword"

echo "decode test passed"
echo ".bytes output: $BYTES_OUT"
echo "Decoded JSON output: $JSON_OUT"
echo "Decoded CSV output: $CSV_OUT"
echo "Non-self-describing .bytes output: $COMPACT_BYTES_OUT"
echo "Non-self-describing JSON output: $COMPACT_JSON_OUT"
echo "Non-self-describing CSV output: $COMPACT_CSV_OUT"
echo "Decode print log: $PRINT_LOG"
