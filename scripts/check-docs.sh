#!/usr/bin/env sh
set -eu

# PowerShell 文档检查脚本的 sh 版本。
# 只做轻量关键词检查，目的是在提交前快速发现文档遗漏。
check_file() {
  file="$1"
  shift
  test -f "$file" || {
    echo "Missing required doc: $file" >&2
    exit 1
  }
  for term in "$@"; do
    if ! grep -F -- "$term" "$file" >/dev/null 2>&1; then
      echo "Missing term '$term' in $file" >&2
      exit 1
    fi
  done
}

# 每个文档列出少量必须出现的关键词，覆盖 CLI、格式、代码生成、日志和提交规范。
check_file README.md build convert decode codegen
check_file docs/format.md datetime ZigZag schemaHash fieldNo sharedStrings
check_file docs/codegen.md CodegenSchema fieldNo wireType
check_file docs/logging.md --log-level --log-format --log-file
check_file docs/excel-cli-plan_dab03005.plan.md convert codegen binaryVersion
check_file docs/contributing.md "Commit Format"

echo "docs check passed"
