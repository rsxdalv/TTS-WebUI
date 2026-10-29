#Requires -Version 5.1
$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path "$PSScriptRoot\..\..")
$viteSrc  = Join-Path $repoRoot "launcher\vite-src"
$assets   = Join-Path $repoRoot "core\internal\webui\assets"

Write-Host "Building launcher with VITE_PLATFORM=core ..."
Set-Location $viteSrc
npm ci
$env:VITE_PLATFORM = "core"
npm run build

Write-Host "Copying dist\ to $assets ..."
if (Test-Path $assets) { Remove-Item $assets -Recurse -Force }
New-Item -ItemType Directory -Path $assets | Out-Null
Copy-Item (Join-Path $viteSrc "dist\*") $assets -Recurse

Write-Host "Done. Launcher built and copied to core\internal\webui\assets\"
