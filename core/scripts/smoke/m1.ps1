#Requires -Version 5.1
# M1 smoke: drives every M1 requirement against the real tts-webui binary.
# Requirements from PLAN.md Verification rules M1:
#   1. Bundle has no Neutralino, has /api/steps.
#   2. ALL steps driven via cookie (no X-TTS-Token header).
#   3. Every streaming step: assert log line + event:done (except run: long-lived).
#   4. Marker files compared against golden (legacy-format) values.
#   5. app/start with fake python: alive 5s later; app/stop: no orphan PIDs.
#   6. Every non-2xx is a FAIL.
$ErrorActionPreference = "Stop"
$script:failed = $false

function Fail { param($msg) Write-Host "FAIL: $msg"; $script:failed = $true }
function Pass { param($msg) Write-Host "PASS: $msg" }

$repoRoot  = (Resolve-Path "$PSScriptRoot\..\..\..").Path
$smokeData = Join-Path $PSScriptRoot "testdata"
$port      = 18788
$bin       = Join-Path $repoRoot "core\bin\tts-webui-smoke.exe"

Write-Host "`n-- Building tts-webui --"
Push-Location (Join-Path $repoRoot "core")
go build -o bin/tts-webui-smoke.exe ./cmd/tts-webui 2>&1
if ($LASTEXITCODE -ne 0) { Write-Host "FAIL: go build failed"; exit 1 }
Pop-Location
Pass "binary built"

Write-Host "`n-- Building stubs --"
$stubDir = Join-Path ([System.IO.Path]::GetTempPath()) "m1-stubs-$(Get-Random)"
New-Item -ItemType Directory -Path $stubDir | Out-Null
foreach ($stub in @("fakemamba", "fakepython")) {
    Push-Location (Join-Path $smokeData $stub)
    $env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
    go build -o (Join-Path $stubDir "$stub.exe") . 2>&1
    if ($LASTEXITCODE -ne 0) { Write-Host "FAIL: building $stub"; exit 1 }
    Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
    Pass "stub built: $stub.exe"
}

Write-Host "`n-- Setting up scratch root --"
$scratch  = Join-Path ([System.IO.Path]::GetTempPath()) "m1-smoke-$(Get-Random)"
$scripts  = Join-Path $scratch "installer_scripts"
$ifiles   = Join-Path $scratch "installer_files"
$mambaDir = Join-Path $ifiles "mamba"
$envDir   = Join-Path $ifiles "env"
foreach ($d in @($scripts, $mambaDir, $envDir)) { New-Item -ItemType Directory -Path $d | Out-Null }

Copy-Item (Join-Path $stubDir "fakepython.exe") (Join-Path $envDir "python.exe")

$fakemambaEsc = (Join-Path $stubDir "fakemamba.exe").Replace("\","\\")
$initMambaContent = "@echo off`r`necho mamba: initializing micromamba`r`nmkdir installer_files\mamba 2>nul`r`ncopy /y ""$fakemambaEsc"" ""installer_files\mamba\micromamba.exe"" >nul`r`necho mamba: micromamba installed successfully"
Set-Content (Join-Path $scripts "init_mamba.bat") $initMambaContent

Set-Content (Join-Path $scratch "server.py") "# M1 smoke placeholder"
Write-Host "Scratch root: $scratch"

$goldenGPU        = "CPU"
$goldenGitVersion = "smoke-test-version"

Write-Host "`n-- Starting tts-webui --"
$proc = Start-Process -FilePath $bin `
    -ArgumentList "launch","--no-browser","--root",$scratch,"--port",$port `
    -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 3
if ($proc.HasExited) { Fail "tts-webui exited immediately"; exit 1 }
Pass "tts-webui running (PID $($proc.Id))"

function Cleanup {
    if ($proc -and -not $proc.HasExited) { try { $proc.Kill() } catch {} }
    Remove-Item $stubDir -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item $scratch -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item $bin     -Force   -ErrorAction SilentlyContinue
}

try {

Write-Host "`n-- GET / --"
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$resp    = Invoke-WebRequest -Uri "http://127.0.0.1:$port/" -UseBasicParsing -TimeoutSec 5 -WebSession $session
if ($resp.StatusCode -lt 200 -or $resp.StatusCode -ge 300) { Fail "GET / HTTP $($resp.StatusCode)"; exit 1 }
$html = $resp.Content

if ($html -match "Neutralino") { Fail "bundle contains Neutralino" }
else                           { Pass "bundle has no Neutralino string" }

$jsHref = ($resp.Links | Where-Object { $_.href -match "\.js" } | Select-Object -First 1).href
if (-not $jsHref -and $html -match 'src="([^"]*\.js)"') { $jsHref = $Matches[1] }
if ($jsHref) {
    $jsUri  = if ($jsHref -match "^http") { $jsHref } else { "http://127.0.0.1:$port$jsHref" }
    $jsBody = (Invoke-WebRequest -Uri $jsUri -UseBasicParsing -TimeoutSec 5 -WebSession $session).Content
    if ($jsBody -match "/api/steps") { Pass "JS bundle contains /api/steps" }
    else                             { Fail "JS bundle does not contain /api/steps" }
} else { Fail "no JS script found in index.html" }

$cookie = $session.Cookies.GetCookies("http://127.0.0.1:$port/") |
          Where-Object { $_.Name -eq "tts-token" }
if (-not $cookie) { Fail "no tts-token cookie in GET / response"; exit 1 }
Pass "tts-token cookie received (using cookie for all API calls)"

function Invoke-API {
    param([string]$method, [string]$path, [string]$body = $null)
    $p = @{
        Uri             = "http://127.0.0.1:$port$path"
        Method          = $method
        UseBasicParsing = $true
        TimeoutSec      = 30
        WebSession      = $session
    }
    if ($body) { $p.Body = $body; $p.ContentType = "application/json" }
    $r = Invoke-WebRequest @p
    if ($r.StatusCode -lt 200 -or $r.StatusCode -ge 300) { Fail "$method $path HTTP $($r.StatusCode)" }
    return $r
}

function Test-SSEStream {
    param([string]$path, [string]$label, [int]$timeoutSec = 12, [switch]$LogLineOnly)
    Write-Host "  SSE: $label (timeout=${timeoutSec}s)"
    try {
        $req = [System.Net.HttpWebRequest]::Create("http://127.0.0.1:$port$path")
        $req.CookieContainer = New-Object System.Net.CookieContainer
        $cookieBase = "http://127.0.0.1:$port/"
        foreach ($c in $session.Cookies.GetCookies($cookieBase)) {
            $nc = New-Object System.Net.Cookie($c.Name, $c.Value, $c.Path, $c.Domain)
            $req.CookieContainer.Add($nc)
        }
        $req.Timeout = $timeoutSec * 1000
        $req.ReadWriteTimeout = $timeoutSec * 1000
        $web   = $req.GetResponse()
        $sr    = New-Object System.IO.StreamReader($web.GetResponseStream())
        $lines = [System.Collections.Generic.List[string]]::new()
        $dl    = [DateTime]::UtcNow.AddSeconds($timeoutSec)
        while ([DateTime]::UtcNow -lt $dl) {
            $ln = $sr.ReadLine()
            if ($null -eq $ln) { break }
            if ($ln -ne "") { $lines.Add($ln) }
            if (-not $LogLineOnly -and $ln -match "^event:\s*done") { break }
            if ($LogLineOnly -and ($lines | Where-Object { $_ -match "^data:" })) { break }
        }
        $sr.Close(); $web.Close()
        $logLines  = @($lines | Where-Object { $_ -match "^data:" -and $_ -notmatch '"exitCode"' })
        $doneEvent = @($lines | Where-Object { $_ -match "^event:\s*done" })
        if ($logLines)  { Pass "$label stream: log line: $($logLines[0])" }
        else            { Fail "$label stream: no log data lines" }
        if (-not $LogLineOnly) {
            if ($doneEvent) { Pass "$label stream: event:done received" }
            else            { Fail "$label stream: no event:done received" }
        }
    } catch { Fail "$label stream error: $($_.Exception.Message)" }
}

Write-Host "`n-- Step: status --"
$r = Invoke-API "GET" "/api/steps/status"; Pass "status: $($r.Content)"

Write-Host "`n-- Step: preflight --"
$r  = Invoke-API "POST" "/api/steps/preflight"
$pf = $r.Content | ConvertFrom-Json
if ($pf.checks) { Pass "preflight: $($pf.checks.Count) checks returned" }
else            { Fail "preflight: missing checks field" }
$pfMarker = Join-Path $scripts ".preflight_ok"
if (Test-Path $pfMarker) { Pass ".preflight_ok created" }
else { Write-Host "NOTE: .preflight_ok absent (some env checks may have failed)" }

Write-Host "`n-- Step: mamba --"
$r = Invoke-API "POST" "/api/steps/mamba"; Pass "mamba started"
Test-SSEStream "/api/steps/mamba/stream" "mamba"
$dl = [DateTime]::UtcNow.AddSeconds(12)
while ([DateTime]::UtcNow -lt $dl -and
       -not (Test-Path (Join-Path $mambaDir "micromamba.exe"))) { Start-Sleep -Milliseconds 300 }
if (Test-Path (Join-Path $mambaDir "micromamba.exe")) { Pass "micromamba binary created" }
else { Fail "installer_files/mamba/micromamba.exe not created" }

Write-Host "`n-- Step: gpu --"
$r = Invoke-API "POST" "/api/steps/gpu" '{"label":"CPU"}'
$gpuFile = Join-Path $scripts ".gpu"
if (Test-Path $gpuFile) {
    $v = (Get-Content $gpuFile -Raw).Trim()
    if ($v -eq $goldenGPU) { Pass ".gpu = '$v' (golden match)" }
    else                   { Fail ".gpu = '$v', want '$goldenGPU'" }
} else { Fail ".gpu not created" }

Write-Host "`n-- Step: install --"
$r = Invoke-API "POST" "/api/steps/install"; Pass "install started"
Test-SSEStream "/api/steps/install/stream" "install"
$dl = [DateTime]::UtcNow.AddSeconds(12)
while ([DateTime]::UtcNow -lt $dl -and
       -not (Test-Path (Join-Path $scripts ".git_version"))) { Start-Sleep -Milliseconds 300 }
$gvFile = Join-Path $scripts ".git_version"
if (Test-Path $gvFile) {
    $v = (Get-Content $gvFile -Raw).Trim()
    if ($v -eq $goldenGitVersion) { Pass ".git_version = '$v' (golden match)" }
    else                          { Fail ".git_version = '$v', want '$goldenGitVersion'" }
} else { Fail ".git_version not created by install step" }

Write-Host "`n-- Step: update --"
$r = Invoke-API "POST" "/api/steps/update"; Pass "update: $($r.Content)"

Write-Host "`n-- Step: run --"
$r = Invoke-API "POST" "/api/steps/run"; Pass "run started"
# run starts the long-lived app server; its SSE stream ends only when the
# process exits (intentionally long-lived). Assert only one log line.
Test-SSEStream "/api/steps/run/stream" "run" -timeoutSec 5 -LogLineOnly

Write-Host "`n-- App lifecycle --"
$before = @(Get-CimInstance Win32_Process |
            Where-Object { $_.ParentProcessId -eq $proc.Id -and $_.Name -eq "python.exe" })

$r = Invoke-API "POST" "/api/app/start"; Pass "app/start: $($r.Content)"
Start-Sleep -Seconds 5

$after    = @(Get-CimInstance Win32_Process |
              Where-Object { $_.ParentProcessId -eq $proc.Id -and $_.Name -eq "python.exe" })
$beforePIDs = @($before | ForEach-Object { $_.ProcessId })
$newProcs = @($after | Where-Object { $_.ProcessId -notin $beforePIDs })

if ($newProcs.Count -gt 0) {
    Pass "fake python alive 5 s after app/start (PID $($newProcs[0].ProcessId))"
} else {
    Fail "fake python not alive 5 s after app/start"
}

$r = Invoke-API "POST" "/api/app/stop"; Pass "app/stop: $($r.Content)"
Start-Sleep -Seconds 2

$newPIDs   = @($newProcs | ForEach-Object { $_.ProcessId })
$survivors = @(Get-CimInstance Win32_Process | Where-Object { $_.ProcessId -in $newPIDs })
if ($survivors.Count -eq 0) { Pass "no orphan processes after app/stop" }
else                         { Fail "orphan processes: $($survivors.Name -join ', ')" }

Write-Host ""
if ($script:failed) { Write-Host "M1 smoke: FAIL"; exit 1 }
else                { Write-Host "M1 smoke: PASS" }

} finally { Cleanup }
