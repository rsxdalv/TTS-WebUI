#Requires -Version 5.1
$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path "$PSScriptRoot\..\..")
$viteSrc  = Join-Path $repoRoot "launcher\vite-src"
$assets   = Join-Path $repoRoot "core\internal\webui\assets"

Write-Host "Building core entry ..."
Set-Location $viteSrc
npm ci
if ($LASTEXITCODE -ne 0) { throw "npm ci failed (exit $LASTEXITCODE)" }

npm run build:core
if ($LASTEXITCODE -ne 0) { throw "npm run build:core failed (exit $LASTEXITCODE)" }

$distCore = Join-Path $viteSrc "dist-core"

Write-Host "Copying dist-core\ to $assets ..."
if (Test-Path $assets) { Remove-Item $assets -Recurse -Force }
New-Item -ItemType Directory -Path $assets | Out-Null
Copy-Item (Join-Path $distCore "*") $assets -Recurse

# core.html is the entry; rename to index.html for the Go server.
$coreHtml = Join-Path $assets "core.html"
$indexHtml = Join-Path $assets "index.html"
if (Test-Path $coreHtml) { Move-Item $coreHtml $indexHtml -Force }

Write-Host "Done. Core entry built and copied to core\internal\webui\assets\"
