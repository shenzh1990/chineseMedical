[CmdletBinding()]
param(
    [ValidateSet("amd64", "arm64")]
    [string]$Arch = "amd64",

    [string]$Version = "",

    [string]$OutputDir = "dist",

    [switch]$SkipTests,

    [switch]$NoArchive
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$root = Resolve-Path (Join-Path $PSScriptRoot "..")
$appName = "chinese-medical"

if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = Get-Date -Format "yyyyMMdd-HHmmss"
}

$distRoot = Join-Path $root $OutputDir
$packageName = "$appName-linux-$Arch-$Version"
$stageDir = Join-Path $distRoot $packageName

function Invoke-Step {
    param(
        [string]$Name,
        [scriptblock]$Action
    )

    Write-Host "==> $Name" -ForegroundColor Cyan
    & $Action
}

function Copy-IfExists {
    param(
        [string]$Path,
        [string]$Destination
    )

    $source = Join-Path $root $Path
    if (Test-Path $source) {
        Copy-Item -Path $source -Destination $Destination -Recurse -Force
    }
}

Invoke-Step "Preparing package directory" {
    if (Test-Path $stageDir) {
        Remove-Item -Path $stageDir -Recurse -Force
    }

    New-Item -ItemType Directory -Path $stageDir | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $stageDir "configs") | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $stageDir "generated") | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $stageDir "logs") | Out-Null
}

Push-Location $root
try {
    if (-not $SkipTests) {
        Invoke-Step "Running tests" {
            go test ./...
        }
    }

    $oldCGO = $env:CGO_ENABLED
    $oldGOOS = $env:GOOS
    $oldGOARCH = $env:GOARCH

    try {
        $env:CGO_ENABLED = "0"
        $env:GOOS = "linux"
        $env:GOARCH = $Arch

        Invoke-Step "Building server for linux/$Arch" {
            go build -trimpath -ldflags="-s -w" -o (Join-Path $stageDir "server") ./cmd/server
        }

        Invoke-Step "Building syncsql for linux/$Arch" {
            go build -trimpath -ldflags="-s -w" -o (Join-Path $stageDir "syncsql") ./cmd/syncsql
        }
    }
    finally {
        $env:CGO_ENABLED = $oldCGO
        $env:GOOS = $oldGOOS
        $env:GOARCH = $oldGOARCH
    }

    Invoke-Step "Copying runtime files" {
        Copy-IfExists "configs\config.yaml.example" (Join-Path $stageDir "configs")
        Copy-IfExists ".env.example" $stageDir
        Copy-IfExists "README.md" $stageDir
        Copy-IfExists "migrations" $stageDir
        Copy-IfExists "sql" $stageDir

        $startScript = @'
#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")"

: "${CONFIG_FILE:=configs/config.yaml}"
: "${SESSION_SECRET:?Please set SESSION_SECRET}"
: "${ADMIN_PASSWORD:?Please set ADMIN_PASSWORD}"

export CONFIG_FILE SESSION_SECRET ADMIN_PASSWORD
exec ./server
'@
        Set-Content -Path (Join-Path $stageDir "start.sh") -Value $startScript -NoNewline -Encoding ascii
    }

    if (-not $NoArchive) {
        Invoke-Step "Creating tar.gz archive" {
            $archivePath = Join-Path $distRoot "$packageName.tar.gz"
            if (Test-Path $archivePath) {
                Remove-Item -Path $archivePath -Force
            }

            Push-Location $distRoot
            try {
                tar -czf "$packageName.tar.gz" $packageName
            }
            finally {
                Pop-Location
            }

            Write-Host "Archive: $archivePath" -ForegroundColor Green
        }
    }

    Write-Host ""
    Write-Host "Release package ready: $stageDir" -ForegroundColor Green
    Write-Host "Upload the directory or tar.gz to Linux, copy configs/config.yaml.example to configs/config.yaml, then run ./start.sh." -ForegroundColor Green
}
finally {
    Pop-Location
}
