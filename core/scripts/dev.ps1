# Go dev loop: finds a Go that is new enough for go.mod, then runs air.
#   pwsh core/scripts/dev.ps1        (from anywhere)
# An old Go earlier on PATH (C:\Go) cannot build this module and rewrites
# go.mod/go.sum, so this puts a matching Go first and forbids go.mod edits.
$ErrorActionPreference = 'Stop'
$core = Split-Path -Parent $PSScriptRoot
$need = [version](((Select-String -Path (Join-Path $core 'go.mod') -Pattern '^go\s+(\S+)').Matches[0].Groups[1].Value) -replace '^(\d+\.\d+)$', '$1.0')

function Get-GoVersion([string]$exe) {
    try { $o = & $exe version 2>$null; if ($o -match 'go(\d+\.\d+(\.\d+)?)') { $v = $Matches[1]; if ($v -notmatch '\.\d+\.\d+$') { $v += '.0' }; return [version]$v } } catch {}
    return $null
}

$candidates = @()
if ($env:GOROOT) { $candidates += Join-Path $env:GOROOT 'bin\go.exe' }
$candidates += (Get-Command go -All -ErrorAction SilentlyContinue | ForEach-Object Source)
$candidates += Join-Path $env:ProgramFiles 'Go\bin\go.exe'
$candidates += Get-ChildItem (Join-Path $HOME 'sdk') -Directory -ErrorAction SilentlyContinue | Sort-Object Name -Descending | ForEach-Object { Join-Path $_.FullName 'bin\go.exe' }

$go = $candidates | Where-Object { $_ -and (Test-Path $_) } | Where-Object { $v = Get-GoVersion $_; $v -and $v -ge $need } | Select-Object -First 1
if (-not $go) { throw "No Go $need or newer found (looked at GOROOT, PATH, Program Files\Go, ~\sdk). Install it from https://go.dev/dl/" }

$goBin = Split-Path $go
# A GOROOT left over from an old install (a session started before the upgrade
# still has it) makes even a new Go read the old standard library.
$env:GOROOT = Split-Path $goBin
$env:PATH = "$goBin;$(Join-Path $HOME 'go\bin');$env:PATH"
$env:GOFLAGS = '-mod=readonly'
Write-Host "Using $(& $go version)"

if (-not (Get-Command air -ErrorAction SilentlyContinue)) {
    Write-Host 'Installing air...'
    & $go install github.com/air-verse/air@latest
}
Set-Location $core
air
