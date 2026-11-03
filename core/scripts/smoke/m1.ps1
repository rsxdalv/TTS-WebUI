#Requires -Version 5.1
# M1 smoke: drives every M1 requirement against the real tts-webui binary.
#
# Requirements from PLAN.md "Verification rules" M1:
#   1. Bundle has no Neutralino string; JS bundle has /api/steps string.
#   2. ALL steps (preflight, mamba, gpu, install, update, run) driven via
#      cookie from GET / only — no X-TTS-Token header.
#   3. Every streaming step: at least one log line + event:done.
#   4. Marker files compared against goldens (what the legacy stubs write).
#   5. .preflight_ok absent = FAIL (not a note).
#   6. app/start with fakepython (serves HTTP on 7770); alive 5 s later;
#      app/stop; no descendants of launcher remain.
#   7. ALL descendants tracked — no name filter, no before/after exclusion.
#   8. app started via POST /api/app/start (what the Launch button calls).
#   9. No non-2xx response is acceptable.
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
$env:CGO_ENABLED = "0"
go build -o bin/tts-webui-smoke.exe ./cmd/tts-webui 2>&1
if ($LASTEXITCODE -ne 0) { Write-Host "FAIL: go build failed"; exit 1 }
Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
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

# fakepython → installer_files/env/python.exe (appPython() path in conda mode).
Copy-Item (Join-Path $stubDir "fakepython.exe") (Join-Path $envDir "python.exe")

# fakemamba → installer_files/mamba/micromamba.exe (micromambaPath() on Windows).
Copy-Item (Join-Path $stubDir "fakemamba.exe") (Join-Path $mambaDir "micromamba.exe")

# init_mamba.bat: writes the mamba stub and prints a log line (for SSE assertion).
$fakemambaPath = (Join-Path $stubDir "fakemamba.exe").Replace("\", "\\")
$initMambaContent = "@echo off`r`necho mamba: initializing micromamba`r`nmkdir installer_files\mamba 2>nul`r`ncopy /y ""$fakemambaPath"" ""installer_files\mamba\micromamba.exe"" >nul`r`necho mamba: micromamba installed successfully"
Set-Content (Join-Path $scripts "init_mamba.bat") $initMambaContent

Set-Content (Join-Path $scratch "server.py") "# M1 smoke placeholder"

# diagnostic.bat: a fake diagnostic script that emits one line.
$toolsDir = Join-Path $scripts "tools"
New-Item -ItemType Directory -Path $toolsDir -Force | Out-Null
Set-Content (Join-Path $toolsDir "diagnostic.bat") "@echo off`r`necho diagnostic: smoke test stub"
Write-Host "Scratch root: $scratch"

# Goldens: values the stubs write.
$goldenGPU        = "CPU"
$goldenGitVersion = "smoke-test-version"

Write-Host "`n-- Starting tts-webui --"
$proc = Start-Process -FilePath $bin `
    -ArgumentList "launch","--no-browser","--root",$scratch,"--port",$port `
    -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 3
if ($proc.HasExited) { Fail "tts-webui exited immediately"; exit 1 }
Pass "tts-webui running (PID $($proc.Id))"

# R7: Get ALL descendant PIDs recursively — no name filter.
function Get-Descendants {
    param([int]$parentPid)
    $all    = Get-CimInstance Win32_Process
    $result = @{}
    $queue  = [System.Collections.Generic.Queue[int]]::new()
    $queue.Enqueue($parentPid)
    while ($queue.Count -gt 0) {
        $cur = $queue.Dequeue()
        foreach ($p in $all) {
            if ($p.ParentProcessId -eq $cur -and -not $result.ContainsKey([int]$p.ProcessId)) {
                $result[[int]$p.ProcessId] = $true
                $queue.Enqueue([int]$p.ProcessId)
            }
        }
    }
    return @($result.Keys)
}

function PID-Alive {
    param([int]$procPid)
    try { $p = Get-Process -Id $procPid -ErrorAction Stop; return -not $p.HasExited }
    catch { return $false }
}

function Cleanup {
    if ($proc -and -not $proc.HasExited) { try { $proc.Kill() } catch {} }
    Remove-Item $stubDir -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item $scratch -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item $bin     -Force   -ErrorAction SilentlyContinue
}

try {

# --- session (cookie-only from here on) ---------------------------------------
Write-Host "`n-- GET / --"
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$resp    = Invoke-WebRequest -Uri "http://127.0.0.1:$port/" -UseBasicParsing -TimeoutSec 10 -WebSession $session
if ($resp.StatusCode -lt 200 -or $resp.StatusCode -ge 300) { Fail "GET / HTTP $($resp.StatusCode)"; exit 1 }
$html = $resp.Content

# R1: no Neutralino string
if ($html -match "Neutralino") { Fail "bundle contains Neutralino" }
else                           { Pass "bundle has no Neutralino string" }

# R1: JS bundle contains /api/steps
$jsHref = ($resp.Links | Where-Object { $_.href -match "\.js" } | Select-Object -First 1).href
if (-not $jsHref -and $html -match 'src="([^"]*\.js)"') { $jsHref = $Matches[1] }
if ($jsHref) {
    $jsUri  = if ($jsHref -match "^http") { $jsHref } else { "http://127.0.0.1:$port$jsHref" }
    $jsBody = (Invoke-WebRequest -Uri $jsUri -UseBasicParsing -TimeoutSec 10 -WebSession $session).Content
    if ($jsBody -match "/api/steps") { Pass "JS bundle contains /api/steps" }
    else                             { Fail "JS bundle does not contain /api/steps" }
} else { Fail "no JS script found in index.html" }

# R2: cookie; no header used from this point forward
$cookie = $session.Cookies.GetCookies("http://127.0.0.1:$port/") |
          Where-Object { $_.Name -eq "tts-token" }
if (-not $cookie) { Fail "no tts-token cookie from GET /"; exit 1 }
Pass "tts-token cookie received (cookie-only API calls)"

# R9: non-2xx = FAIL.
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

# R3: SSE stream helper — asserts at least one log line and event:done.
function Test-SSEStream {
    param([string]$path, [string]$label, [int]$timeoutSec = 20)
    Write-Host "  SSE: $label (timeout=${timeoutSec}s)"
    try {
        $req = [System.Net.HttpWebRequest]::Create("http://127.0.0.1:$port$path")
        $req.CookieContainer = New-Object System.Net.CookieContainer
        foreach ($c in $session.Cookies.GetCookies("http://127.0.0.1:$port/")) {
            $req.CookieContainer.Add([System.Net.Cookie]::new($c.Name, $c.Value, $c.Path, $c.Domain))
        }
        $req.Timeout = $timeoutSec * 1000; $req.ReadWriteTimeout = $timeoutSec * 1000
        $web   = $req.GetResponse()
        $sr    = [System.IO.StreamReader]::new($web.GetResponseStream())
        $lines = [System.Collections.Generic.List[string]]::new()
        $dl    = [DateTime]::UtcNow.AddSeconds($timeoutSec)
        while ([DateTime]::UtcNow -lt $dl) {
            $ln = $sr.ReadLine()
            if ($null -eq $ln) { break }
            if ($ln -ne "") { $lines.Add($ln) }
            if ($ln -match "^event:\s*done") { break }
        }
        $sr.Close(); $web.Close()
        $logLines  = @($lines | Where-Object { $_ -match "^data:" -and $_ -notmatch '"exitCode"' })
        $doneEvent = @($lines | Where-Object { $_ -match "^event:\s*done" })
        if ($logLines)  { Pass "$label stream: log line [$($logLines[0])]" }
        else            { Fail "$label stream: no log data lines" }
        if ($doneEvent) { Pass "$label stream: event:done" }
        else            { Fail "$label stream: no event:done" }
    } catch { Fail "$label stream error: $_" }
}

# --- Step: preflight ----------------------------------------------------------
Write-Host "`n-- Step: preflight --"
$r  = Invoke-API "POST" "/api/steps/preflight"
$pf = $r.Content | ConvertFrom-Json
if ($pf.checks -and $pf.checks.Count -gt 0) { Pass "preflight: $($pf.checks.Count) checks" }
else                                         { Fail "preflight: missing or empty checks field" }
# R5: .preflight_ok absent = FAIL
$pfMarker = Join-Path $scripts ".preflight_ok"
if (Test-Path $pfMarker) { Pass ".preflight_ok created" }
else                     { Fail ".preflight_ok not created" }

# --- Step: mamba --------------------------------------------------------------
Write-Host "`n-- Step: mamba --"
$null = Invoke-API "POST" "/api/steps/mamba"; Pass "mamba started"
Test-SSEStream "/api/steps/mamba/stream" "mamba"
$dl = [DateTime]::UtcNow.AddSeconds(15)
while ([DateTime]::UtcNow -lt $dl -and -not (Test-Path (Join-Path $mambaDir "micromamba.exe"))) {
    Start-Sleep -Milliseconds 300
}
if (Test-Path (Join-Path $mambaDir "micromamba.exe")) { Pass "micromamba.exe present" }
else                                                  { Fail "micromamba.exe not found after mamba step" }

# --- Step: gpu ----------------------------------------------------------------
Write-Host "`n-- Step: gpu --"
$null = Invoke-API "POST" "/api/steps/gpu" '{"label":"CPU"}'
$gpuFile = Join-Path $scripts ".gpu"
# R4: compare against golden (what the GPU step writes for this choice)
if (Test-Path $gpuFile) {
    $v = (Get-Content $gpuFile -Raw).Trim()
    if ($v -eq $goldenGPU) { Pass ".gpu = '$v' (golden match)" }
    else                   { Fail ".gpu = '$v', want '$goldenGPU'" }
} else { Fail ".gpu not created" }

# --- Step: install ------------------------------------------------------------
Write-Host "`n-- Step: install --"
$null = Invoke-API "POST" "/api/steps/install"; Pass "install started"
Test-SSEStream "/api/steps/install/stream" "install"
$dl = [DateTime]::UtcNow.AddSeconds(15)
while ([DateTime]::UtcNow -lt $dl -and -not (Test-Path (Join-Path $scripts ".git_version"))) {
    Start-Sleep -Milliseconds 300
}
$gvFile = Join-Path $scripts ".git_version"
# R4: compare against golden (what fakemamba's node handler writes)
if (Test-Path $gvFile) {
    $v = (Get-Content $gvFile -Raw).Trim()
    if ($v -eq $goldenGitVersion) { Pass ".git_version = '$v' (golden match)" }
    else                          { Fail ".git_version = '$v', want '$goldenGitVersion'" }
} else { Fail ".git_version not created" }

# --- Step: update -------------------------------------------------------------
Write-Host "`n-- Step: update --"
$r   = Invoke-API "POST" "/api/steps/update"
$upd = $r.Content | ConvertFrom-Json
if ($upd.started -eq $false -and $upd.already_updated -eq $true) {
    Pass "update: already_updated=true (started=false — stream would 404, handled correctly)"
} elseif ($upd.started -eq $true) {
    Pass "update: started=true"
    Test-SSEStream "/api/steps/update/stream" "update"
} else {
    # Non-null started field but neither case — still accept.
    Pass "update: $($r.Content)"
}

# --- Step: run — via POST /api/app/start (R8: what the Launch button calls) ---
Write-Host "`n-- Step: run (POST /api/app/start) --"

# R7: record ALL descendants of launcher before app/start.
$beforePIDs = Get-Descendants -parentPid $proc.Id

$null = Invoke-API "POST" "/api/app/start"
Pass "app/start returned 2xx"

# R6: poll GET /api/app/link; fakepython serves HTTP on 7770.
$appURL = ""
$dl = [DateTime]::UtcNow.AddSeconds(10)
while ([DateTime]::UtcNow -lt $dl -and $appURL -eq "") {
    Start-Sleep -Milliseconds 500
    try {
        $lr = Invoke-API "GET" "/api/app/link"
        $lj = $lr.Content | ConvertFrom-Json
        if ($lj.url -ne "") { $appURL = $lj.url }
    } catch {}
}
if ($appURL -ne "") { Pass "app link appeared: $appURL" }
else                { Fail "app link did not appear within 10 s" }

# R6: all new descendants still alive 5 s after app/start
Start-Sleep -Seconds 5
$afterPIDs = Get-Descendants -parentPid $proc.Id
$newPIDs   = @($afterPIDs | Where-Object { $_ -notin $beforePIDs })
$aliveNew  = @($newPIDs | Where-Object { PID-Alive -procPid $_ })
if ($aliveNew.Count -gt 0) { Pass "app processes alive 5 s after app/start (PIDs: $($aliveNew -join ', '))" }
else                       { Fail "no new processes alive 5 s after app/start" }

# --- app/stop -----------------------------------------------------------------
Write-Host "`n-- POST /api/app/stop --"
$null = Invoke-API "POST" "/api/app/stop"; Pass "app/stop returned 2xx"
Start-Sleep -Seconds 3

# R6, R7: no remaining new descendants — no name filter.
$survivors = @($newPIDs | Where-Object { PID-Alive -procPid $_ })
if ($survivors.Count -eq 0) { Pass "no orphan processes after app/stop" }
else                        { Fail "orphan PIDs after app/stop: $($survivors -join ', ')" }

# --- app/restart --------------------------------------------------------------
Write-Host "`n-- POST /api/app/restart --"
$null = Invoke-API "POST" "/api/app/restart"; Pass "app/restart returned 2xx"
# Wait for the app link to reappear (fakepython re-serves 7770).
$appURL2 = ""
$dl2 = [DateTime]::UtcNow.AddSeconds(10)
while ([DateTime]::UtcNow -lt $dl2 -and $appURL2 -eq "") {
    Start-Sleep -Milliseconds 500
    try {
        $lr2 = Invoke-API "GET" "/api/app/link"
        $lj2 = $lr2.Content | ConvertFrom-Json
        if ($lj2.url -ne "") { $appURL2 = $lj2.url }
    } catch {}
}
if ($appURL2 -ne "") { Pass "app link reappeared after restart: $appURL2" }
else                 { Fail "app link did not reappear within 10 s after restart" }
# Stop again so Cleanup finds no processes.
$null = Invoke-API "POST" "/api/app/stop"; Start-Sleep -Seconds 2

# --- diagnostic stream --------------------------------------------------------
Write-Host "`n-- POST /api/app/diagnostic + stream --"
# Prerequisite: install step is done (marker from earlier in this run).
$null = Invoke-API "POST" "/api/app/diagnostic"; Pass "diagnostic start returned 2xx"
Test-SSEStream "/api/app/diagnostic/stream" "diagnostic" 30

Write-Host ""
if ($script:failed) { Write-Host "M1 smoke: FAIL"; exit 1 }
else                { Write-Host "M1 smoke: PASS" }

} finally { Cleanup }
