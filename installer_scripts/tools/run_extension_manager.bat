@echo off

cd %~dp0
cd ..\..
call .\installer_scripts\activate.bat

call conda --version
echo Node
call node --version
call python --version

echo Starting Extension Management UI...
python .\installer_scripts\tools\extension_manager.py

pause
