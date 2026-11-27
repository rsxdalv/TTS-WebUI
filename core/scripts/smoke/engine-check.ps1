#Requires -Version 5.1
# Tiered check of own-venv engines (P6). Windows only.
#
#   engine-check.ps1 -Root <install folder> -Packages tts_webui_extension.kokoro,... [-Tier 3] [-BatchSize 3]
#
# The launcher must be running on -Root (tts-webui.exe --root <Root> --no-browser --start).
# It makes the same calls the UI makes: the launcher's Extensions API installs, the
# engine page API opens the engine, the router serves its page.
#
# Tiers (each includes the ones before it):
#   1 smoke    installed from the catalog entry (or already there), the engine starts, the router
#              answers 200 on /<pkg>/. The harness imports the extension's main module to get there.
#   2 import   every module of the extension's direct requirements imports in its own venv.
#              Zero modules imported is a FAIL, not a pass.
#   3 pycheck  `uv pip check` is clean, the venv's Python is the one entry.json recorded, torch
#              imports, the venv path is under 200 characters.
#   5 inference  (-Tier 5) the same call the engine's React page makes (POST /api/gradio/<name> on the
#              app port) for the engines in $inferenceCalls below; the answer must carry an audio file and
#              that file must download with a real size. This downloads the weights (GBs for some) and can be
#              slow on CPU. Other engines: "not run (no call defined)". Tier 4 (load only) does not exist:
#              the engines load their model inside the generate call.
#              Set "validation": "full" in extensions.json only after this passes AND the clip plays in History.
#
# Installs run one at a time (the app allows one install at a time, "another install is running").
# The start and the checks of a batch run together, BatchSize engines at once.
# Exit code 1 if anything failed. A tier above -Tier is reported "not run".
param(
    [Parameter(Mandatory)] [string] $Root,
    [Parameter(Mandatory)] [string[]] $Packages,
    [ValidateRange(1, 5)] [int] $Tier = 3,
    [int] $BatchSize = 3,
    [string] $Launcher = "http://127.0.0.1:7700",
    [string] $App = "http://127.0.0.1:3000",
    [string] $Router = "http://127.0.0.1:7770",
    [int] $StartTimeoutSec = 300,
    [int] $InferenceTimeoutSec = 1800,
    # tier 5 on the GPU: the engines' own "use GPU" / device settings are switched on (float32 on Pascal)
    [switch] $Gpu
)
$ErrorActionPreference = "Stop"
$Root = (Resolve-Path $Root).Path
# powershell -File passes "a,b,c" as one string
$Packages = @($Packages | ForEach-Object { $_ -split "," } | Where-Object { $_ })

# The launcher gives its page a cookie on GET / and refuses API calls without it, like the UI does.
$script:session = $null
function Get-LauncherSession {
    if (-not $script:session) {
        Invoke-WebRequest -Uri "$Launcher/" -UseBasicParsing -SessionVariable s -TimeoutSec 15 | Out-Null
        $script:session = $s
    }
    $script:session
}

function Install-Engine($pkg) {
    $venv = Join-Path $Root ".venvs\$pkg"
    if ((Test-Path (Join-Path $venv "entry.json")) -and -not (Test-Path (Join-Path $venv "installing.json"))) { return "already installed" }
    $body = @{ package_name = $pkg } | ConvertTo-Json -Compress
    $r = Invoke-WebRequest -Uri "$Launcher/api/ext/op/install" -Method Post -ContentType "application/json" -Body $body -UseBasicParsing -TimeoutSec 3600 -WebSession (Get-LauncherSession) -Headers @{ Origin = $Launcher }
    # the stream is application/x-ndjson: Windows PowerShell hands the body over as bytes
    $text = if ($r.Content -is [byte[]]) { [Text.Encoding]::UTF8.GetString($r.Content) } else { [string]$r.Content }
    $lines = @($text -split "`n" | Where-Object { $_.Trim() })
    $last = $lines | Select-Object -Last 1 | ConvertFrom-Json
    if (-not $last.done -or -not $last.ok) { throw "install failed: $(($lines | Select-Object -Last 3) -join ' | ')" }
    "installed"
}

# Runs in a job: start the engine, wait for ready, fetch its page.
$smokeBlock = {
    param($pkg, $App, $Router, $timeout)
    $h = @{ Origin = $App }
    $a = Invoke-RestMethod -Uri "$App/api/go-data/engines/$pkg/open" -Method Post -ContentType "application/json" -Body "{}" -Headers $h
    $deadline = (Get-Date).AddSeconds($timeout)
    while ($a.state -ne "ready") {
        if ($a.state -in @("failed", "exited")) { throw "engine $($a.state): $($a.error)" }
        if ((Get-Date) -gt $deadline) { throw "not ready after $timeout s (state $($a.state))" }
        Start-Sleep -Milliseconds 500
        $a = Invoke-RestMethod -Uri "$App/api/go-data/engines/$pkg" -Headers $h
    }
    $page = Invoke-WebRequest -Uri "$Router/$pkg/" -UseBasicParsing -TimeoutSec 30
    if ($page.StatusCode -ne 200) { throw "page answered $($page.StatusCode)" }
    "ready, page 200"
}

$importPy = @'
import importlib, importlib.metadata as m, json, re, sys
dist = sys.argv[1].replace("_", "-").replace(".", "-")
owner = {}
for mod, ds in m.packages_distributions().items():
    for d in ds:
        owner.setdefault(re.sub(r"[-_.]+", "-", d).lower(), []).append(mod)
ok, bad = [], []
for req in (m.requires(dist) or []):
    if ";" in req and "extra" in req.split(";", 1)[1]:
        continue
    name = re.sub(r"[-_.]+", "-", re.split(r"[ <>=!~;\[(@]", req, 1)[0]).lower()
    junk = {"dist", "build", "tests", "test", "docs", "examples", "scripts", "src"}
    mods = [x for x in owner.get(name, []) if x not in junk and not x.startswith("_")]
    if not mods:
        # a dist with no top_level.txt: take the first path segment of its .py files
        try:
            seen = []
            for f in (m.distribution(name).files or []):
                top = str(f).replace("\\", "/").split("/")[0]
                top = top[:-3] if top.endswith(".py") else top
                if top.isidentifier() and top not in junk and not top.startswith("_") and top not in seen and str(f).endswith(".py"):
                    seen.append(top)
            mods = seen
        except Exception:
            pass
    mods.sort(key=lambda x: (x != name.replace("-", "_"), x))
    for mod in mods[:1]:
        try:
            importlib.import_module(mod); ok.append(mod)
        except BaseException as e:
            bad.append("%s: %s" % (mod, str(e)[:120]))
print(json.dumps({"ok": ok, "bad": bad}))
'@

function Check-Import($pkg) {
    $py = Join-Path $Root ".venvs\$pkg\Scripts\python.exe"
    $f = Join-Path ([IO.Path]::GetTempPath()) "engine-check-import.py"
    Set-Content -Path $f -Value $importPy -Encoding ASCII
    $c = Run-Native $py @($f, $pkg)
    $res = ($c.Out -split " (?=\{)" | Select-Object -Last 1) | ConvertFrom-Json
    if ($res.bad.Count) { throw "import failed: $($res.bad -join '; ')" }
    if (-not $res.ok.Count) { throw "no module of the direct requirements could be imported (nothing was checked)" }
    "$($res.ok.Count) modules imported"
}

function Run-Native($exe, $arguments) {
    # uv writes "Using Python ..." to stderr; that is not a failure, the exit code is
    $old = $ErrorActionPreference; $ErrorActionPreference = "Continue"
    try { $out = & $exe @arguments 2>&1 | ForEach-Object { "$_" }; return @{ Code = $LASTEXITCODE; Out = ($out -join " ") } }
    finally { $ErrorActionPreference = $old }
}

function Check-Pycheck($pkg) {
    $venv = Join-Path $Root ".venvs\$pkg"
    $py = Join-Path $venv "Scripts\python.exe"
    $uv = Join-Path $Root "platforms\uv\bin\uv.exe"
    $c = Run-Native $uv @("pip", "check", "--python", $py)
    if ($c.Code -ne 0) { throw "uv pip check: $($c.Out)" }
    $entry = Get-Content (Join-Path $venv "entry.json") -Raw | ConvertFrom-Json
    $ver = (Run-Native $py @("-c", "import sys;print('%d.%d.%d' % sys.version_info[:3])")).Out.Trim()
    if ($entry.python -and $ver -ne $entry.python -and -not $ver.StartsWith($entry.python)) { throw "python $ver, entry.json says $($entry.python)" }
    $c = Run-Native $py @("-c", "import torch;print(torch.__version__)")
    if ($c.Code -ne 0) { throw "torch does not import: $($c.Out)" }
    $t = $c.Out.Trim()
    if ($venv.Length -ge 200) { throw "venv path is $($venv.Length) characters" }
    "pip check clean, python $ver, torch $t, path $($venv.Length)"
}

# The Vite pages' own calls: package -> api name and the body the page sends with its default settings
# (initial*Params in frontend/src/tabs, use_random_seed removed, GPU off because the scratch platform is CPU).
$inferenceCalls = @{
    "tts_webui_extension.kokoro"     = @{ Name = "kokoro_generate"; Body = @{ seed = 0; text = "This is a short check of the engine."; voice = "af_heart"; speed = 1.0; model_name = "hexgrad/Kokoro-82M"; use_gpu = $false } }
    "tts_webui_extension.mms"        = @{ Name = "mms"; Body = @{ seed = 0; text = "This is a short check of the engine."; language = "eng"; speaking_rate = 1.0; noise_scale = 0.667; noise_scale_duration = 0.8 } }
    "tts_webui_extension.vall_e_x"   = @{ Name = "vall_e_x_generate"; Body = @{ seed = 0; text = "This is a short check of the engine."; prompt = ""; language = "Mix"; accent = "no-accent"; mode = "short" } }
    "tts_webui_extension.chatterbox" = @{ Name = "chatterbox_tts"; Body = @{ text = "This is a short check of the engine."; exaggeration = 0.5; cfg_weight = 0.5; temperature = 0.8; device = "cpu"; dtype = "float32"; model_name = "just_a_placeholder"; seed = -1; chunked = $false; halve_first_chunk = $false; desired_length = 200; max_length = 300; cpu_offload = $false; initial_forward_pass_backend = "eager"; generate_token_backend = "cudagraphs-manual"; max_new_tokens = 1000; max_cache_len = 1500; language_id = "en" } }
}

function Check-Inference($pkg) {
    $c = $inferenceCalls[$pkg]
    if (-not $c) { return "NOTRUN: no call defined for this engine" }
    $b = $c.Body.Clone()
    if ($Gpu) { if ($b.ContainsKey("use_gpu")) { $b.use_gpu = $true }; if ($b.ContainsKey("device")) { $b.device = "cuda" } }
    $body = $b | ConvertTo-Json -Compress
    $r = Invoke-WebRequest -Uri "$App/api/gradio/$($c.Name)" -Method Post -ContentType "application/json" -Body $body -UseBasicParsing -TimeoutSec $InferenceTimeoutSec -Headers @{ Origin = $App }
    $j = $r.Content | ConvertFrom-Json
    $url = $j.audio.url
    if (-not $url) { throw "no audio in the answer: $($r.Content.Substring(0, [Math]::Min(300, $r.Content.Length)))" }
    $f = Invoke-WebRequest -Uri "$App$url" -UseBasicParsing -TimeoutSec 120
    $n = $f.RawContentLength
    if ($f.StatusCode -ne 200 -or $n -lt 5000) { throw "audio file: status $($f.StatusCode), $n bytes" }
    "PASS: $n bytes of audio from $($c.Name)"
}

$results = @()
for ($i = 0; $i -lt $Packages.Count; $i += $BatchSize) {
    $batch = $Packages[$i..([Math]::Min($i + $BatchSize, $Packages.Count) - 1)]
    Write-Host "`n-- batch: $($batch -join ', ') --"
    $row = @{}
    foreach ($p in $batch) { $row[$p] = [ordered]@{ T1 = $null; T2 = "not run"; T3 = "not run"; T5 = "not run" } }
    # installs one at a time
    foreach ($p in $batch) {
        try { $inst = Install-Engine $p } catch { $row[$p].T1 = "FAIL: $($_.Exception.Message)"; continue }
        Write-Host "  ${p}: $inst"
    }
    # starts together
    $jobs = @{}
    foreach ($p in $batch) {
        if ($row[$p].T1) { continue }
        $jobs[$p] = Start-Job -ScriptBlock $smokeBlock -ArgumentList $p, $App, $Router, $StartTimeoutSec
    }
    foreach ($p in $jobs.Keys) {
        try { $row[$p].T1 = "PASS: " + (Receive-Job -Job $jobs[$p] -Wait -ErrorAction Stop) }
        catch { $row[$p].T1 = "FAIL: $($_.Exception.Message)" }
        Remove-Job -Job $jobs[$p] -Force
    }
    foreach ($p in $batch) {
        if ($row[$p].T1 -notlike "PASS*") { $row[$p].T2 = "not run (tier 1 failed)"; $row[$p].T3 = "not run (tier 1 failed)"; continue }
        if ($Tier -ge 2) {
            try { $row[$p].T2 = "PASS: " + (Check-Import $p) } catch { $row[$p].T2 = "FAIL: $($_.Exception.Message)" }
        }
        if ($Tier -ge 3 -and $row[$p].T2 -like "PASS*") {
            try { $row[$p].T3 = "PASS: " + (Check-Pycheck $p) } catch { $row[$p].T3 = "FAIL: $($_.Exception.Message)" }
        } elseif ($Tier -ge 3) { $row[$p].T3 = "not run (tier 2 failed)" }
        if ($Tier -ge 5) {
            try {
                $r5 = Check-Inference $p
                $row[$p].T5 = if ($r5 -like "NOTRUN*") { "not run ($($r5.Substring(7).Trim()))" } else { $r5 }
            } catch { $row[$p].T5 = "FAIL: $($_.Exception.Message)" }
        }
    }
    foreach ($p in $batch) { $results += [pscustomobject]@{ Package = $p; Smoke = $row[$p].T1; Import = $row[$p].T2; Pycheck = $row[$p].T3; Inference = $row[$p].T5 } }
}

Write-Host "`n== result (tier $Tier of 5) =="
foreach ($r in $results) {
    Write-Host $r.Package
    Write-Host "   1 smoke   : $($r.Smoke)"
    Write-Host "   2 import  : $($r.Import)"
    Write-Host "   3 pycheck : $($r.Pycheck)"
    if ($Tier -ge 5) { Write-Host "   5 inference: $($r.Inference)" }
}
$failed = @($results | Where-Object { "$($_.Smoke)$($_.Import)$($_.Pycheck)$($_.Inference)" -match "FAIL" })
if ($failed.Count) { Write-Host "`nFAIL: $($failed.Count) of $($results.Count) engines failed a tier"; exit 1 }
Write-Host "`nPASS: $($results.Count) engines through tier $Tier"
