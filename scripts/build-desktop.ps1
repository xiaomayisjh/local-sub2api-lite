# Build local-sub2api-lite desktop executable.
# Default: release build (no DevTools, no console).
# Set $env:SUB2API_DESKTOP_DEBUG = "1" before running this script to produce a debug build
# (DevTools open on startup, attached Windows console, "(Debug)" in window title).

$ErrorActionPreference = "Stop"
$Root = Resolve-Path (Join-Path $PSScriptRoot "..")

$debugBuild = $env:SUB2API_DESKTOP_DEBUG -match '^(1|true|yes|y|on)$'

Write-Host "==> Building frontend..."
Push-Location (Join-Path $Root "frontend")
pnpm install
pnpm run build
Pop-Location

Write-Host "==> Desktop shell uses desktop/frontend/dist/index.html (startup loader only)"

Write-Host "==> Building Wails desktop binary..."
Push-Location (Join-Path $Root "desktop")
go mod tidy
$outDir = Join-Path $Root "dist"
New-Item -ItemType Directory -Force -Path $outDir | Out-Null
$outFileName = if ($debugBuild) { "local-sub2api-lite-debug.exe" } else { "local-sub2api-lite.exe" }
$outFile = Join-Path $outDir $outFileName

# Wails requires tag "production" (or "dev"); backend UI requires tag "embed".
# Debug build adds tag "debug" which flips IsDebugBuild on in desktop/build_debug.go,
# and we deliberately drop "-H windowsgui" so the console window stays attached.
if ($debugBuild) {
    $tags = "production,debug,embed"
    $ldflags = ""  # no -s -w: keep symbols for stack traces; no -H windowsgui: keep console.
} else {
    $tags = "production,embed"
    $ldflags = "-s -w"
    if ($env:GOOS -eq "" -or $env:GOOS -eq "windows") {
        $ldflags += " -H windowsgui"
    }
}

Write-Host ("==> Tags: {0}" -f $tags)
Write-Host ("==> ldflags: {0}" -f $ldflags)
go build -tags $tags -ldflags $ldflags -o $outFile .
Pop-Location

Write-Host "==> Done: $outFile"
if ($debugBuild) {
    Write-Host "    Debug build: DevTools and console auto-enabled. Run it from a terminal to see server logs."
}
