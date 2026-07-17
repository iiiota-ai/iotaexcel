$ErrorActionPreference = "Stop"

[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

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

function Invoke-Checked([scriptblock]$Command, [string]$Description) {
  & $Command
  if ($LASTEXITCODE -ne 0) {
    throw "$Description failed with exit code $LASTEXITCODE"
  }
}

try {
  $goExe = Resolve-GoExe
  Invoke-Checked { & $goExe test ./... } "go test"
} finally {
  Pop-Location
  Wait-BeforeExit
}
