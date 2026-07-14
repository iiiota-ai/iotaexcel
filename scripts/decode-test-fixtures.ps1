$ErrorActionPreference = "Stop"

# 专门验证 decode 命令的测试脚本：
# 1. 重新生成合法/非法 Excel fixture；
# 2. 先把 valid fixture 转成自描述和非自描述 .bytes；
# 3. 再把两类 .bytes 反解析为 JSON 和 CSV；
# 4. 检查关键输出文件和内容，确保 decode 命令真实可用。
$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $Root

function Wait-BeforeExit {
  if ($env:IOTAEXCEL_NO_PAUSE -eq "1") {
    return
  }
  Write-Host ""
  Read-Host "Press Enter to exit"
}

function Resolve-GoExe {
  if (![string]::IsNullOrWhiteSpace($env:GO_EXE)) {
    return $env:GO_EXE
  }
  $goCmd = Get-Command go -ErrorAction SilentlyContinue
  if ($goCmd) {
    return $goCmd.Source
  }
  if (Test-Path "C:\Program Files\Go\bin\go.exe") {
    return "C:\Program Files\Go\bin\go.exe"
  }
  throw "Go executable not found. Install Go or set GO_EXE."
}

function Assert-File([string]$Path) {
  if (!(Test-Path $Path -PathType Leaf)) {
    throw "Expected file not found: $Path"
  }
}

function Assert-Contains([string]$Path, [string]$Text) {
  Assert-File $Path
  $content = Get-Content $Path -Raw
  if ($content -notmatch [regex]::Escape($Text)) {
    throw "Expected '$Text' in $Path"
  }
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
  $goExe = Resolve-GoExe
  $bytesOut = "out/decode-test/bytes"
  $jsonOut = "out/decode-test/json"
  $csvOut = "out/decode-test/csv"
  $compactBytesOut = "out/decode-test/bytes-compact"
  $compactJsonOut = "out/decode-test/json-compact"
  $compactCsvOut = "out/decode-test/csv-compact"
  $printLog = "out/decode-test/decode-print.txt"
  Remove-Item "out/decode-test" -Recurse -Force -ErrorAction SilentlyContinue

  Write-Host "[1/8] Generate Excel fixtures"
  Invoke-ChildNoPause { powershell -NoProfile -ExecutionPolicy Bypass -File "tests/generate-fixtures.ps1" } "Generate Excel fixtures"

  Write-Host "[2/8] Convert valid fixtures to self-describing .bytes"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel convert --input tests/testdata/excels/valid --output $bytesOut --format bin --check-ref --overwrite --log-level info } "Convert valid fixtures to .bytes"

  Write-Host "[3/8] Decode self-describing .bytes to JSON"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel decode --input $bytesOut --output $jsonOut --format json --print --print-mode concise --overwrite --log-level info | Tee-Object -FilePath $printLog } "Decode .bytes to JSON"

  Write-Host "[4/8] Decode self-describing .bytes to CSV"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel decode --input $bytesOut --output $csvOut --format csv --overwrite --log-level info } "Decode .bytes to CSV"

  Write-Host "[5/8] Convert valid fixtures to non-self-describing .bytes"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel convert --input tests/testdata/excels/valid --output $compactBytesOut --format bin --self-describing=false --check-ref --overwrite --log-level info } "Convert valid fixtures to non-self-describing .bytes"

  Write-Host "[6/8] Decode non-self-describing .bytes to JSON"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel decode --input $compactBytesOut --schema-input tests/testdata/excels/valid --output $compactJsonOut --format json --self-describing=false --overwrite --log-level info } "Decode non-self-describing .bytes to JSON"

  Write-Host "[7/8] Decode non-self-describing .bytes to CSV"
  Invoke-Checked { & $goExe run ./cmd/iotaexcel decode --input $compactBytesOut --schema-input tests/testdata/excels/valid --output $compactCsvOut --format csv --self-describing=false --overwrite --log-level info } "Decode non-self-describing .bytes to CSV"

  Write-Host "[8/8] Verify decoded outputs"
  Assert-Contains (Join-Path $jsonOut "Config_ItemConfig.json") '"name": "Sword"'
  Assert-Contains (Join-Path $jsonOut "Config_HeroConfig.json") '"itemRef": "1001"'
  Assert-Contains (Join-Path $csvOut "Config_ItemConfig.csv") "Sword"
  Assert-Contains (Join-Path $csvOut "Config_HeroConfig.csv") "hero_001"
  Assert-Contains (Join-Path $compactJsonOut "Config_ItemConfig.json") '"selfDescribing": false'
  Assert-Contains (Join-Path $compactJsonOut "Config_ItemConfig.json") '"name": "Sword"'
  Assert-Contains (Join-Path $compactCsvOut "Config_ItemConfig.csv") "Sword"
  Assert-Contains $printLog "IOTB"
  Assert-Contains $printLog "2`tname`tstring"
  Assert-Contains $printLog "18`t2`t2`tSword"

  Write-Host "decode test passed"
  Write-Host ".bytes output: $bytesOut"
  Write-Host "Decoded JSON output: $jsonOut"
  Write-Host "Decoded CSV output: $csvOut"
  Write-Host "Non-self-describing .bytes output: $compactBytesOut"
  Write-Host "Non-self-describing JSON output: $compactJsonOut"
  Write-Host "Non-self-describing CSV output: $compactCsvOut"
  Write-Host "Decode print log: $printLog"
} finally {
  Pop-Location
  Wait-BeforeExit
}
