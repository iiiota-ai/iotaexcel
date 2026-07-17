$ErrorActionPreference = "Stop"

# 一键验证当前源码版工具的常用导出链路：
# 1. 重新生成测试 Excel；
# 2. 调用 convert 输出自描述 .bytes；
# 3. 调用 decode 把自描述 .bytes 反解析为 CSV/JSON；
# 4. 调用 convert 输出非自描述 .bytes；
# 5. 调用 decode 结合 Excel schema 反解析非自描述 .bytes；
# 6. 调用 codegen 输出 C# reader。
# 该脚本主要给 Windows/PowerShell 环境使用，sh 环境有同名 .sh 版本。
$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $Root

function Wait-BeforeExit {
  if ($env:IOTAEXCEL_NO_PAUSE -eq "1") {
    return
  }
  Write-Host ""
  Read-Host "Press Enter to exit"
}

function Invoke-Checked([scriptblock]$Command, [string]$Description) {
  & $Command
  if ($LASTEXITCODE -ne 0) {
    throw "$Description failed with exit code $LASTEXITCODE"
  }
}

function Invoke-ChildNoPause([scriptblock]$Command, [string]$Description) {
  $oldNoPause = $env:IOTAEXCEL_NO_PAUSE
  try {
    $env:IOTAEXCEL_NO_PAUSE = "1"
    Invoke-Checked $Command $Description
  } finally {
    $env:IOTAEXCEL_NO_PAUSE = $oldNoPause
  }
}

try {
  # 允许外部通过 GO_EXE 指定 Go 路径；未指定时优先使用 PATH 中的 go，
  # 最后兼容 Windows 默认安装路径 C:\Program Files\Go\bin\go.exe。
  $goExe = $env:GO_EXE
  if ([string]::IsNullOrWhiteSpace($goExe)) {
    $goCmd = Get-Command go -ErrorAction SilentlyContinue
    if ($goCmd) {
      $goExe = $goCmd.Source
    } elseif (Test-Path "C:\Program Files\Go\bin\go.exe") {
      $goExe = "C:\Program Files\Go\bin\go.exe"
    } else {
      throw "Go executable not found. Install Go or set GO_EXE."
    }
  }

  # fixture 可能被修改或删除，导出前先重建，保证验证输入可重复。
  Write-Host "[1/13] Generate Excel fixtures"
  Invoke-ChildNoPause { powershell -NoProfile -ExecutionPolicy Bypass -File "tests/generate-fixtures.ps1" } "Generate Excel fixtures"

  # 只导出 valid 目录，避免 invalid fixture 按预期失败导致脚本中断。
  Write-Host "[2/13] Export self-describing .bytes files"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel convert --input tests/testdata/excels/valid --output out/bytes --format bin --check-ref --overwrite --log-level info } "Export .bytes files"

  # 反解析 .bytes，验证当前 CLI 可以不依赖 Excel 源文件恢复可读数据。
  Write-Host "[3/13] Decode self-describing .bytes files to JSON"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel decode --input out/bytes --output out/decoded-json --format json --overwrite --log-level info } "Decode .bytes files to JSON"

  Write-Host "[4/13] Decode self-describing .bytes files to CSV"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel decode --input out/bytes --output out/decoded-csv --format csv --overwrite --log-level info } "Decode .bytes files to CSV"

  # 非自描述 .bytes 不写字段名和类型名，decode 时必须通过 --schema-input 提供 Excel schema。
  Write-Host "[5/13] Export non-self-describing .bytes files"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel convert --input tests/testdata/excels/valid --output out/bytes-compact --format bin --self-describing=false --check-ref --overwrite --log-level info } "Export non-self-describing .bytes files"

  Write-Host "[6/13] Decode non-self-describing .bytes files to JSON"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel decode --input out/bytes-compact --schema-input tests/testdata/excels/valid --output out/decoded-json-compact --format json --self-describing=false --overwrite --log-level info } "Decode non-self-describing .bytes files to JSON"

  Write-Host "[7/13] Decode non-self-describing .bytes files to CSV"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel decode --input out/bytes-compact --schema-input tests/testdata/excels/valid --output out/decoded-csv-compact --format csv --self-describing=false --overwrite --log-level info } "Decode non-self-describing .bytes files to CSV"

  # C# 输出用于验证 codegen 可以基于相同 Excel schema 生成读取代码。
  Write-Host "[8/13] Generate C# code"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel codegen --input tests/testdata/excels/valid --output out/codegen/csharp --lang csharp --check-ref --overwrite --log-level info } "Generate C# code"

  Write-Host "[9/13] Generate Go code"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel codegen --input tests/testdata/excels/valid --output out/codegen/go --lang go --check-ref --overwrite --log-level info } "Generate Go code"

  Write-Host "[10/13] Generate C++ code"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel codegen --input tests/testdata/excels/valid --output out/codegen/cpp --lang cpp --check-ref --overwrite --log-level info } "Generate C++ code"

  Write-Host "[11/13] Generate Java code"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel codegen --input tests/testdata/excels/valid --output out/codegen/java --lang java --check-ref --overwrite --log-level info } "Generate Java code"

  Write-Host "[12/13] Generate JavaScript code"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel codegen --input tests/testdata/excels/valid --output out/codegen/javascript --lang javascript --check-ref --overwrite --log-level info } "Generate JavaScript code"

  Write-Host "[13/13] Generate Python code"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel codegen --input tests/testdata/excels/valid --output out/codegen/python --lang python --check-ref --overwrite --log-level info } "Generate Python code"

  Write-Host "Done."
  Write-Host "Self-describing .bytes output: out/bytes"
  Write-Host "Self-describing decoded JSON output: out/decoded-json"
  Write-Host "Self-describing decoded CSV output: out/decoded-csv"
  Write-Host "Non-self-describing .bytes output: out/bytes-compact"
  Write-Host "Non-self-describing decoded JSON output: out/decoded-json-compact"
  Write-Host "Non-self-describing decoded CSV output: out/decoded-csv-compact"
  Write-Host "C# output: out/codegen/csharp"
  Write-Host "Go output: out/codegen/go"
  Write-Host "C++ output: out/codegen/cpp"
  Write-Host "Java output: out/codegen/java"
  Write-Host "JavaScript output: out/codegen/javascript"
  Write-Host "Python output: out/codegen/python"
} finally {
  Pop-Location
  Wait-BeforeExit
}
