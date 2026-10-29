#Requires -Version 5.1
# M1 smoke: Go launcher serves the real launcher HTML and handles step endpoints.
$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path "$PSScriptRoot\..\..\..").Path
$port     = 18788
$bin      = Join-Path $repoRoot "core\bin\tts-webui.exe"

# 1. Build Go binary
Write-Host "--- building tts-webui binary ---"
Push-Location (Join-Path $repoRoot "core")
go build -o bin/tts-webui.exe ./cmd/tts-webui
Pop-Location

# 2. Scratch root with stub scripts
$scratch = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
$scripts = Join-Path $scratch "installer_scripts"
New-Item -ItemType Directory -Path $scripts | Out-Null
Set-Content -Path (Join-Path $scripts "init_mamba.bat") "@echo off`r`nexit /b 0"
Set-Content -Path (Join-Path $scripts "init_mamba.sh")  "exit 0"
Write-Host "Scratch root: $scratch"

# 3. Start server in background
$proc = Start-Process -FilePath $bin `
    -ArgumentList "launch","--no-browser","--root",$scratch,"--port",$port `
    -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 2

function Cleanup {
    if ($proc -and -not $proc.HasExited) { $proc.Kill() }
}

try {

# 4. GET / — must contain tts-webui-token meta tag
$indexResp = Invoke-WebRequest -Uri "http://127.0.0.1:$port/" -UseBasicParsing -TimeoutSec 5
$indexHtml = $indexResp.Content
if ($indexHtml -notmatch 'tts-webui-token') {
    Write-Host "FAIL: GET / did not return launcher HTML (missing tts-webui-token)"
    Write-Host "Response: $($indexHtml.Substring(0, [Math]::Min(300, $indexHtml.Length)))"
    exit 1
}
Write-Host "PASS: GET / returned launcher HTML"

# 5. Extract token from meta tag
if ($indexHtml -match 'content="([^"]+)"') {
    $token = $Matches[1]
} else {
    Write-Host "FAIL: could not extract token from meta tag"
    exit 1
}
Write-Host "Token: $($token.Substring(0,16))..."

# 6. GET /api/steps/status
$hdrs = @{ "X-TTS-Token" = $token }
$status = Invoke-RestMethod -Uri "http://127.0.0.1:$port/api/steps/status" -Headers $hdrs -TimeoutSec 5
Write-Host "Steps status: $($status | ConvertTo-Json -Compress)"

# 7. POST /api/steps/preflight
Invoke-RestMethod -Uri "http://127.0.0.1:$port/api/steps/preflight" -Method Post -Headers $hdrs -TimeoutSec 10 | Out-Null
Write-Host "PASS: preflight ran"

# 8. POST /api/steps/gpu
$gpuBody = '{"label":"CPU"}'
Invoke-RestMethod -Uri "http://127.0.0.1:$port/api/steps/gpu" -Method Post -Headers $hdrs `
    -ContentType "application/json" -Body $gpuBody -TimeoutSec 5 | Out-Null
$gpuFile = Join-Path $scripts ".gpu"
if (-not (Test-Path $gpuFile)) {
    Write-Host "FAIL: .gpu file not created"
    exit 1
}
$gpuVal = (Get-Content $gpuFile -Raw).Trim()
if ($gpuVal -ne "CPU") {
    Write-Host "FAIL: .gpu = '$gpuVal', want 'CPU'"
    exit 1
}
Write-Host "PASS: .gpu = CPU"

# 9. SSE stream test (3-second window, best-effort)
try {
    $sseResp = Invoke-WebRequest -Uri "http://127.0.0.1:$port/api/steps/preflight/stream" `
        -Headers $hdrs -TimeoutSec 3 -UseBasicParsing 2>$null
    $sseOut = $sseResp.Content.Substring(0, [Math]::Min(200, $sseResp.Content.Length))
} catch { $sseOut = "(timeout or error - expected for SSE)" }
Write-Host "SSE output: $sseOut"

# 10. app start + stop (expected to fail in scratch root without Python)
try {
    Invoke-RestMethod -Uri "http://127.0.0.1:$port/api/app/start" -Method Post -Headers $hdrs -TimeoutSec 5 | Out-Null
    Write-Host "PASS: app start (responded)"
} catch { Write-Host "INFO: app/start returned error (expected in scratch root): $($_.Exception.Message)" }
try {
    Invoke-RestMethod -Uri "http://127.0.0.1:$port/api/app/stop"  -Method Post -Headers $hdrs -TimeoutSec 5 | Out-Null
    Write-Host "PASS: app stop (responded)"
} catch { Write-Host "INFO: app/stop returned error (expected in scratch root): $($_.Exception.Message)" }

Write-Host "M1 smoke complete."

} finally { Cleanup }
