@echo off

title TTS WebUI

powershell.exe -ExecutionPolicy Bypass -command "& '%~dp0\installer_scripts\root.ps1'"

pause
