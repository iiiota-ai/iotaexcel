param(
  [switch]$All
)

$ErrorActionPreference = "Stop"

# 构建 iotaexcel 单文件可执行程序。
# Go build 只会编译 ./cmd/iotaexcel 及其正常依赖，不会把 *_test.go 或 tests/testdata 打包进可执行文件。
# 默认构建当前平台；传入 -All 时构建常用 Windows/Linux/macOS 目标。
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

function Build-Target([string]$GoExe, [string]$Goos, [string]$Goarch) {
  $name = "iotaexcel-$Goos-$Goarch"
  if ($Goos -eq "windows") {
    $name = "$name.exe"
  }
  $output = Join-Path "dist" $name

  Write-Host "Building $output"
  $oldGoos = $env:GOOS
  $oldGoarch = $env:GOARCH
  $oldCgo = $env:CGO_ENABLED
  try {
    $env:GOOS = $Goos
    $env:GOARCH = $Goarch
    $env:CGO_ENABLED = "0"
    Invoke-Checked { & $GoExe build -trimpath -ldflags="-s -w" -o $output ./cmd/iotaexcel } "Build $Goos/$Goarch"
  } finally {
    $env:GOOS = $oldGoos
    $env:GOARCH = $oldGoarch
    $env:CGO_ENABLED = $oldCgo
  }
}

try {
  $goExe = Resolve-GoExe
  New-Item -ItemType Directory -Force -Path "dist" | Out-Null

  if ($All) {
    $targets = @(
      @{ GOOS = "windows"; GOARCH = "amd64" },
      @{ GOOS = "linux"; GOARCH = "amd64" },
      @{ GOOS = "darwin"; GOARCH = "amd64" },
      @{ GOOS = "darwin"; GOARCH = "arm64" }
    )
    foreach ($target in $targets) {
      Build-Target $goExe $target.GOOS $target.GOARCH
    }
  } else {
    $goos = (& $goExe env GOOS).Trim()
    if ($LASTEXITCODE -ne 0) { throw "go env GOOS failed" }
    $goarch = (& $goExe env GOARCH).Trim()
    if ($LASTEXITCODE -ne 0) { throw "go env GOARCH failed" }
    Build-Target $goExe $goos $goarch
  }

  Write-Host "Build outputs written to dist/"
} finally {
  Pop-Location
  Wait-BeforeExit
}
