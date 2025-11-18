@echo off

cd %~dp0
cd ..
call .\installer_scripts\activate.bat


echo Starting command prompt for user to run commands in

call conda --version
echo Node
call node --version
call python --version

cmd /k "%*"

