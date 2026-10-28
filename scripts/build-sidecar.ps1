$TargetTriple = "x86_64-pc-windows-msvc"
$OutputDir = Join-Path $PSScriptRoot "..\src-tauri\binaries"

if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
}

$BinaryName = "gateway-$TargetTriple.exe"
$TargetPath = Join-Path $OutputDir $BinaryName

Write-Host "Compiling Go Gateway sidecar binary to: $TargetPath" -ForegroundColor Cyan

$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"

go build -ldflags="-s -w" -o "$TargetPath" ./cmd/gateway

if ($LASTEXITCODE -eq 0) {
    Write-Host "Sidecar binary built successfully: $BinaryName" -ForegroundColor Green
    Copy-Item "$TargetPath" (Join-Path $PSScriptRoot "..\gateway.exe") -Force
} else {
    Write-Host "Failed to build Go sidecar binary" -ForegroundColor Red
    exit 1
}
