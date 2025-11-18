@echo off

cd %~dp0..\..

echo Starting diagnostic... > .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt

@REM echo current time
echo %date% %time% >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt

echo CD: %cd% >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt
echo where git (before calling activate.bat): >> .\installer_scripts\diagnostic.txt 2>&1
call where git >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt
echo where python (before calling activate.bat): >> .\installer_scripts\diagnostic.txt 2>&1
call where python >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt
echo where conda (before calling activate.bat): >> .\installer_scripts\diagnostic.txt 2>&1
call where conda >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt
echo where node (before calling activate.bat): >> .\installer_scripts\diagnostic.txt 2>&1
call where node >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt

call .\installer_scripts\activate.bat

echo: >> .\installer_scripts\diagnostic.txt
echo where git (after calling activate.bat): >> .\installer_scripts\diagnostic.txt 2>&1
call where git >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt
echo where python (after calling activate.bat): >> .\installer_scripts\diagnostic.txt 2>&1
call where python >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt
echo where conda (after calling activate.bat): >> .\installer_scripts\diagnostic.txt 2>&1
call where conda >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt
echo where node (after calling activate.bat): >> .\installer_scripts\diagnostic.txt 2>&1
call where node >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt


@REM versions
echo python --version: >> .\installer_scripts\diagnostic.txt 2>&1
call python --version >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt
echo node --version: >> .\installer_scripts\diagnostic.txt 2>&1
call node --version >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt
echo conda --version: >> .\installer_scripts\diagnostic.txt 2>&1
call conda --version >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt

@REM git status
echo git status: >> .\installer_scripts\diagnostic.txt 2>&1
call git status >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt

@REM git log HEAD~1..HEAD
echo git log HEAD~1..HEAD: >> .\installer_scripts\diagnostic.txt 2>&1
call git log HEAD~1..HEAD >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt

@REM package managers
echo pip list: >> .\installer_scripts\diagnostic.txt 2>&1
call pip list >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt
echo conda list: >> .\installer_scripts\diagnostic.txt 2>&1
call conda list >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt
echo npm list -g: >> .\installer_scripts\diagnostic.txt 2>&1
call npm list -g >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt

@REM core packages
echo pip show python-dotenv gradio torch torchaudio torchvision: >> .\installer_scripts\diagnostic.txt 2>&1
call pip show python-dotenv gradio torch torchaudio torchvision >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt

@REM .gpu
echo .gpu: >> .\installer_scripts\diagnostic.txt 2>&1
type .\installer_scripts\.gpu >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt

@REM .major_version
echo .major_version: >> .\installer_scripts\diagnostic.txt 2>&1
type .\installer_scripts\.major_version >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt

@REM .python_version
echo .python_version: >> .\installer_scripts\diagnostic.txt 2>&1
type .\installer_scripts\.python_version >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt

@REM npm list ./react-ui/package.json
echo npm list --prefix ./react-ui/ >> .\installer_scripts\diagnostic.txt 2>&1
call npm list --prefix ./react-ui/ >> .\installer_scripts\diagnostic.txt 2>&1
echo: >> .\installer_scripts\diagnostic.txt

