$ErrorActionPreference = "Stop"

function Wait-BeforeExit {
  if ($env:IOTAEXCEL_NO_PAUSE -eq "1") {
    return
  }
  Write-Host ""
  Read-Host "Press Enter to exit"
}

# 轻量文档同步检查。
# 每次工具能力发生用户可见变化时，至少要保证关键文档存在，并包含核心关键词。
# 这不是完整文档测试，只用于快速发现忘记更新 README/docs 的情况。
$checks = @(
  @{ Path = "README.md"; Terms = @("build", "convert", "decode", "codegen") },
  @{ Path = "docs/format.md"; Terms = @("datetime", "ZigZag", "schemaHash", "fieldNo", "sharedStrings") },
  @{ Path = "docs/codegen.md"; Terms = @("CodegenSchema", "fieldNo", "wireType") },
  @{ Path = "docs/logging.md"; Terms = @("--log-level", "--log-format", "--log-file") },
  @{ Path = "docs/excel-cli-plan_dab03005.plan.md"; Terms = @("convert", "codegen", "binaryVersion") },
  @{ Path = "docs/contributing.md"; Terms = @("Commit Format") }
)

# 逐个文档检查必需关键词；缺文件或缺关键词都会抛错，使 CI/本地脚本返回失败。
try {
  foreach ($check in $checks) {
    if (!(Test-Path $check.Path)) {
      throw "Missing required doc: $($check.Path)"
    }
    $content = Get-Content $check.Path -Raw
    foreach ($term in $check.Terms) {
      if ($content -notmatch [regex]::Escape($term)) {
        throw "Missing term '$term' in $($check.Path)"
      }
    }
  }

  Write-Host "docs check passed"
} finally {
  Wait-BeforeExit
}
