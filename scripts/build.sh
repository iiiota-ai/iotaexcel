#!/usr/bin/env sh
set -eu

# 构建 iotaexcel 单文件可执行程序。
# go build 只会编译 ./cmd/iotaexcel 及其正常依赖，不会把 *_test.go 或 tests/testdata 打包进可执行文件。
# 默认构建当前平台；传入 --all 时构建常用 Windows/Linux/macOS 目标。
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

build_target() {
  goos="$1"
  goarch="$2"
  name="iotaexcel-$goos-$goarch"
  if [ "$goos" = "windows" ]; then
    name="$name.exe"
  fi
  output="dist/$name"

  echo "Building $output"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 "$GO_CMD" build -trimpath -ldflags="-s -w" -o "$output" ./cmd/iotaexcel
}

write_checksums() {
  checksum_file="dist/sha256sums.txt"
  : > "$checksum_file"
  for artifact in dist/iotaexcel-*; do
    [ -f "$artifact" ] || continue
    name=$(basename "$artifact")
    if command -v sha256sum >/dev/null 2>&1; then
      hash=$(sha256sum "$artifact" | awk '{print $1}')
    else
      hash=$(shasum -a 256 "$artifact" | awk '{print $1}')
    fi
    printf '%s  %s\n' "$hash" "$name" >> "$checksum_file"
  done
  echo "Checksums written to $checksum_file"
}

cd "$ROOT"
mkdir -p dist
mkdir -p .gocache
export GOCACHE="$ROOT/.gocache"

if [ "${1:-}" = "--all" ]; then
  build_target windows amd64
  build_target linux amd64
  build_target darwin amd64
  build_target darwin arm64
else
  build_target "$("$GO_CMD" env GOOS)" "$("$GO_CMD" env GOARCH)"
fi

write_checksums
echo "Build outputs written to dist/"
