#Requires -Version 5.1
# run-playwright.ps1
# Builds the binary and smoke fixtures, starts three binary instances on
# ports 18789/18790/18791, runs the Playwright suite, then tears everything down.
#
# Exit 0 = all tests passed.  Exit 1 = build or test failure.
$ErrorActionPreference = "Stop"

$repoRoot  = (Resolve-Path "$PSScriptRoot\..\..\..").Path
$smokeData = Join-Path $PSScriptRoot "testdata"
$bin       = Join-Path $repoRoot "core\bin\tts-webui-smoke.exe"

# --- build binary -------------------------------------------------------------
Write-Host "`n-- Building tts-webui --"
Push-Location (Join-Path $repoRoot "core")
$env:CGO_ENABLED = "0"
go build -o bin/tts-webui-smoke.exe ./cmd/tts-webui 2>&1
if ($LASTEXITCODE -ne 0) { Write-Host "FAIL: go build failed"; exit 1 }
Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
Pop-Location
Write-Host "PASS: binary built"

# --- build stubs --------------------------------------------------------------
Write-Host "`n-- Building stubs --"
$stubDir = Join-Path ([System.IO.Path]::GetTempPath()) "pw-stubs-$(Get-Random)"
New-Item -ItemType Directory -Path $stubDir | Out-Null
foreach ($stub in @("fakemamba", "fakepython")) {
    Push-Location (Join-Path $smokeData $stub)
    $env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
    go build -o (Join-Path $stubDir "$stub.exe") .
    if ($LASTEXITCODE -ne 0) { Write-Host "FAIL: building $stub"; exit 1 }
    Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
    Write-Host "PASS: stub built: $stub.exe"
}

$fmp = (Join-Path $stubDir "fakemamba.exe").Replace("\", "\\")
$fpp = Join-Path $stubDir "fakepython.exe"

# --- fixtures -----------------------------------------------------------------
Write-Host "`n-- Setting up fixtures --"

# Fixture 1: normal fixture, port 18789.
# init_mamba.bat has a ~5 s ping delay so the reload-mid-step test can observe
# the running state before the bat exits.
$scratch1 = Join-Path ([System.IO.Path]::GetTempPath()) "pw-smoke1-$(Get-Random)"
$s1 = Join-Path $scratch1 "installer_scripts"
$e1 = Join-Path $scratch1 "installer_files\env"
foreach ($d in @($s1, $e1)) { New-Item -ItemType Directory -Path $d | Out-Null }
Copy-Item $fpp (Join-Path $e1 "python.exe")
$bat1 = "@echo off`r`necho mamba: initializing micromamba`r`nping -n 6 127.0.0.1 >nul`r`nmkdir installer_files\mamba 2>nul`r`ncopy /y `"$fmp`" `"installer_files\mamba\micromamba.exe`" >nul`r`necho mamba: micromamba installed successfully"
Set-Content (Join-Path $s1 "init_mamba.bat") $bat1
Set-Content (Join-Path $scratch1 "server.py") "# smoke placeholder"
Write-Host "Fixture 1 (normal): $scratch1"

# Fixture 2: root path with a space, port 18790.
# The preflight SpacesInPath check must detect the space and report fail.
$scratch2 = Join-Path ([System.IO.Path]::GetTempPath()) "pw smoke2 $(Get-Random)"
$s2 = Join-Path $scratch2 "installer_scripts"
New-Item -ItemType Directory -Path $s2 | Out-Null
Set-Content (Join-Path $s2 "init_mamba.bat") "@echo off`r`necho space fixture - should not run"
Set-Content (Join-Path $scratch2 "server.py") "# smoke placeholder"
Write-Host "Fixture 2 (space): $scratch2"

# Fixture 3: preflight pre-completed, init_mamba.bat exits 1, port 18791.
# Tests that a step whose process exits non-zero shows an error in the UI.
$scratch3 = Join-Path ([System.IO.Path]::GetTempPath()) "pw-smoke3-$(Get-Random)"
$s3 = Join-Path $scratch3 "installer_scripts"
New-Item -ItemType Directory -Path $s3 | Out-Null
Set-Content (Join-Path $s3 ".preflight_ok") "ok"
Set-Content (Join-Path $s3 "init_mamba.bat") "@echo off`r`necho mamba: failing stub test`r`nexit /b 1"
Set-Content (Join-Path $scratch3 "server.py") "# smoke placeholder"
Write-Host "Fixture 3 (failing): $scratch3"

# --- start binaries -----------------------------------------------------------
Write-Host "`n-- Starting binaries --"
$proc1 = Start-Process -FilePath $bin -ArgumentList "launch","--no-browser","--root",$scratch1,"--port","18789" -PassThru -WindowStyle Hidden
# Fixture 2 root has a space: wrap in escaped quotes so Start-Process passes it as one arg.
$proc2 = Start-Process -FilePath $bin -ArgumentList @("launch","--no-browser","--root","`"$scratch2`"","--port","18790") -PassThru -WindowStyle Hidden
$proc3 = Start-Process -FilePath $bin -ArgumentList "launch","--no-browser","--root",$scratch3,"--port","18791" -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 3

foreach ($p in @($proc1, $proc2, $proc3)) {
    if ($p.HasExited) {
        Write-Host "FAIL: binary (PID $($p.Id)) exited early"
        exit 1
    }
}
Write-Host "PASS: binaries running on ports 18789, 18790, 18791"

function Cleanup {
    foreach ($p in @($proc1, $proc2, $proc3)) {
        if ($p -and -not $p.HasExited) { try { $p.Kill() } catch {} }
    }
    Remove-Item $stubDir  -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item $scratch1 -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item $scratch2 -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item $scratch3 -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item $bin      -Force   -ErrorAction SilentlyContinue
}

$testResult = 1
try {
    Write-Host "`n-- Installing Playwright dependencies --"
    Push-Location (Join-Path $PSScriptRoot "ui")
    npm install 2>&1 | Out-Null
    npx playwright install chromium 2>&1 | Out-Null

    Write-Host "`n-- Running Playwright tests --"
    npx playwright test
    $testResult = $LASTEXITCODE
    Pop-Location
} finally {
    Cleanup
}

if ($testResult -ne 0) { Write-Host "Playwright tests FAILED"; exit 1 }
Write-Host "Playwright tests PASSED"
exit 0
