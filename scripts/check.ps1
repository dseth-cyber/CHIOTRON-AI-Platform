# Runs the same checks as .github/workflows/ci.yml on this machine.
#
# Everything runs in Docker because the development host has no Go or Node
# toolchain installed. Module and npm caches live in named volumes so repeat
# runs do not re-download the world.
#
#   powershell -File scripts/check.ps1

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$failures = @()

function Invoke-Check {
    param([string]$Name, [scriptblock]$Body)

    Write-Host ""
    Write-Host "==> $Name" -ForegroundColor Cyan
    & $Body
    if ($LASTEXITCODE -ne 0) {
        $script:failures += $Name
        Write-Host "FAILED: $Name" -ForegroundColor Red
    }
}

# The script is normalised to LF (sh rejects CRLF) and then base64-encoded:
# Windows PowerShell 5.1 does not escape embedded double quotes when it calls a
# native executable, so passing this text to `docker` verbatim would let the C
# runtime re-split it into separate arguments.
$goChecks = (@'
set -e
apk add --no-cache git gcc musl-dev >/dev/null
cp go.mod /tmp/go.mod.before && cp go.sum /tmp/go.sum.before
go mod tidy
diff -q /tmp/go.mod.before go.mod || { echo "go.mod is not tidy"; exit 1; }
diff -q /tmp/go.sum.before go.sum || { echo "go.sum is not tidy"; exit 1; }
unformatted=$(gofmt -l .)
[ -z "$unformatted" ] || { echo "not gofmt-formatted:"; echo "$unformatted"; exit 1; }
go vet ./...
go test -race ./...
go build ./...
'@) -replace "`r", ""

$goChecksB64 = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($goChecks))

Invoke-Check "control-plane: tidy, format, vet, test, build" {
    # gcc and musl-dev are required because -race needs cgo.
    docker run --rm `
        -v ceap-gomodcache:/go/pkg/mod `
        -v "${root}\control-plane:/src" -w /src `
        -e GOTOOLCHAIN=local -e CGO_ENABLED=1 `
        golang:1.25-alpine sh -c "echo $goChecksB64 | base64 -d | sh"
}

# Built through the image so npm never writes node_modules into the working tree.
Invoke-Check "portal: build" {
    docker build -q -t chiotron-ai-portal:check "$root\web" | Out-Null
}

Invoke-Check "compose: definition" {
    docker compose --project-directory $root config --quiet
}

Write-Host ""
if ($failures.Count -gt 0) {
    Write-Host "FAILED checks: $($failures -join ', ')" -ForegroundColor Red
    exit 1
}
Write-Host "All checks passed." -ForegroundColor Green
