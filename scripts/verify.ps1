# GeoPilot verification helper (Windows PowerShell 5.1+ / PowerShell 7+)
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts/verify.ps1
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts/verify.ps1 -Integration
#   pwsh -File scripts/verify.ps1 -Integration
#
# Integration tests require TEST_DB_DSN pointing at a disposable MySQL
# database. They clear application tables in that database.

param(
    [switch]$Integration
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    Write-Host '==> gofmt'
    $goFiles = @(
        Get-ChildItem -Path (Join-Path $root 'core'), (Join-Path $root 'backend') -Recurse -Filter '*.go' |
            ForEach-Object { $_.FullName }
    )
    $unformatted = & gofmt -l $goFiles
    if ($LASTEXITCODE -ne 0) { throw 'gofmt failed' }
    if ($unformatted) {
        throw "Go files need formatting:`n$($unformatted -join "`n")"
    }

    Write-Host '==> go vet'
    & go vet ./core/... ./backend/...
    if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }

    if ($Integration) {
        if (-not $env:TEST_DB_DSN) {
            throw 'TEST_DB_DSN is required for -Integration'
        }
    } elseif (-not $env:TEST_DB_DSN) {
        Write-Host '    TEST_DB_DSN is not set; MySQL integration tests will be skipped.'
    }

    Write-Host '==> go test'
    & go test -count=1 ./core/... ./backend/...
    if ($LASTEXITCODE -ne 0) { throw 'go test failed' }

    Write-Host '==> frontend build'
    Push-Location (Join-Path $root 'frontend')
    try {
        & npm run build
        if ($LASTEXITCODE -ne 0) { throw 'frontend build failed' }
    } finally {
        Pop-Location
    }

    Write-Host ''
    Write-Host 'Verification passed.'
} finally {
    Pop-Location
}
