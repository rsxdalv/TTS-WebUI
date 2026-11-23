@echo off

set "LAUNCHER=%~dp0..\tts-webui.exe"
if not exist "%LAUNCHER%" (
    echo The launcher was not found: %LAUNCHER%
    echo Download the TTS WebUI zip again (it contains tts-webui.exe) or build it: cd core ^&^& go build -o ..\tts-webui.exe .\cmd\tts-webui
    exit /b 1
)

"%LAUNCHER%" launch --start
@REM call node %~dp0js\server.js

@REM start command prompt for user to run commands in case of failure
echo ""
echo ""
echo App exitted or crashed.
echo Starting command prompt for user to run commands in case of failure...
echo ""
cmd /k "%*"

pause
