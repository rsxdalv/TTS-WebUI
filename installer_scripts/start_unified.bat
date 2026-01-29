@echo off

call %~dp0activate.bat

echo Starting TTS WebUI Unified Server...
call node %~dp0js\processManager.js %*

@REM start command prompt for user to run commands in case of failure
echo ""
echo ""
echo App exitted or crashed.
echo Starting command prompt for user to run commands in case of failure...
echo ""
cmd /k "%*"

pause
