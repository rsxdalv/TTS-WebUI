@REM adapted from https://github.com/easydiffusion/easydiffusion
@REM License

@REM Copyright (c) 2022 cmdr2 and contributors

@REM Section I

@REM Permission is hereby granted, free of charge, to any person obtaining a copy
@REM of this software and associated documentation files (the "Software"), to deal
@REM in the Software without restriction, including without limitation the rights
@REM to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
@REM copies of the Software, and to permit persons to whom the Software is
@REM furnished to do so, subject to the following conditions:

@REM The above copyright notice and this permission notice shall be included in all
@REM copies or substantial portions of the Software.

@REM The person obtaining a copy of the Software meets the Use-based restrictions
@REM as referenced in Section II paragraph 1.

@REM The person obtaining a copy of the Software accepts that the Model or
@REM Derivatives of the Model (as defined in the "CreativeML Open RAIL-M" license
@REM accompanying this License) are subject to Section II paragraph 1.

@REM THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
@REM IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
@REM FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
@REM AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
@REM LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
@REM OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
@REM SOFTWARE.

@REM Section II

@REM 1. Use-based restrictions. The restrictions set forth in Attachment A are 
@REM considered Use-based restrictions. Therefore the person obtaining a copy of the
@REM Software cannot use the Software for the specified restricted uses. The person
@REM obtaining a copy of the Software may use the Software only for lawful purposes.

@REM 2. Except as set forth herein, the authors or copyright holders claim no rights
@REM in the results of operating the Software. The person obtaining a copy of the
@REM Software is accountable for the results of operating the Software and its
@REM subsequent uses.

@REM 3. If any provision of this License is held to be invalid, illegal or 
@REM unenforceable, the remaining provisions shall be unaffected thereby and 
@REM remain valid as if such provision had not been set forth herein. 

@REM END OF TERMS AND CONDITIONS 





@REM Attachment A

@REM Use Restrictions

@REM The person obtaining a copy of the Software agrees not to use the Software: 
@REM - In any way that violates any applicable national, federal, state, local 
@REM or international law or regulation; 
@REM - For the purpose of exploiting, harming or attempting to exploit or harm 
@REM minors in any way; 
@REM - To generate or disseminate verifiably false information and/or content 
@REM with the purpose of harming others; 
@REM - To generate or disseminate personal identifiable information that can 
@REM be used to harm an individual; 
@REM - To defame, disparage or otherwise harass others; 
@REM - For fully automated decision making that adversely impacts an 
@REM individual’s legal rights or otherwise creates or modifies a binding, 
@REM enforceable obligation; 
@REM - For any use intended to or which has the effect of discriminating 
@REM against or harming individuals or groups based on online or offline 
@REM social behavior or known or predicted personal or personality 
@REM characteristics; 
@REM - To exploit any of the vulnerabilities of a specific group of persons 
@REM based on their age, social, physical or mental characteristics, in order 
@REM to materially distort the behavior of a person pertaining to that group 
@REM in a manner that causes or is likely to cause that person or another 
@REM person physical or psychological harm; 
@REM - For any use intended to or which has the effect of discriminating 
@REM against individuals or groups based on legally protected characteristics 
@REM or categories; 
@REM - To provide medical advice and medical results interpretation; 
@REM - To generate or disseminate information for the purpose to be used for 
@REM administration of justice, law enforcement, immigration or asylum 
@REM processes, such as predicting an individual will commit fraud/crime 
@REM commitment (e.g. by text profiling, drawing causal relationships between 
@REM assertions made in documents, indiscriminate and arbitrarily-targeted 
@REM use). 

@echo off
setlocal enabledelayedexpansion

@rem This script will install git and conda (if not found on the PATH variable)
@rem  using micromamba (an 8mb static-linked single-file binary, conda replacement).
@rem For users who already have git and conda, this step will be skipped.

@rem This enables a user to install this project without manually installing conda and git.

@rem config
set MAMBA_ROOT_PREFIX=%cd%\installer_files\mamba
set INSTALL_ENV_DIR=%cd%\installer_files\env
set MICROMAMBA_DOWNLOAD_URL=https://github.com/easydiffusion/easydiffusion/releases/download/v1.1/micromamba.exe
set umamba_exists=F

set PYTHONHOME=

set OLD_APPDATA=%APPDATA%
set OLD_USERPROFILE=%USERPROFILE%
set APPDATA=%cd%\installer_files\appdata
set USERPROFILE=%cd%\installer_files\profile

@rem figure out whether git and conda needs to be installed
if exist "%INSTALL_ENV_DIR%" set PATH=%INSTALL_ENV_DIR%;%INSTALL_ENV_DIR%\Library\bin;%INSTALL_ENV_DIR%\Scripts;%INSTALL_ENV_DIR%\Library\usr\bin;%PATH%

set PACKAGES_TO_INSTALL=git python=3.10.11 conda-forge::nodejs=22.9.0 conda ninja conda-forge::postgresql=16.4 conda-forge::vswhere conda-forge::ffmpeg=4.4.2[build=lgpl*]

@REM if not exist "%MAMBA_ROOT_PREFIX%\micromamba.exe" set umamba_exists=F
if not exist "temp" mkdir temp
call "%MAMBA_ROOT_PREFIX%\micromamba.exe" --version >%cd%\temp\tmp1 2>%cd%\temp\tmp2
if "!ERRORLEVEL!" EQU "0" set umamba_exists=T

@rem download micromamba
if "%umamba_exists%" == "F" (
    echo "Downloading micromamba from %MICROMAMBA_DOWNLOAD_URL% to %MAMBA_ROOT_PREFIX%\micromamba.exe"

    mkdir "%MAMBA_ROOT_PREFIX%"
    call curl -Lk "%MICROMAMBA_DOWNLOAD_URL%" > "%MAMBA_ROOT_PREFIX%\micromamba.exe"

    if "!ERRORLEVEL!" NEQ "0" (
        echo "There was a problem downloading micromamba. Cannot continue."
        pause
        exit /b
    )

    mkdir "%APPDATA%"
    mkdir "%USERPROFILE%"

    @rem test the mamba binary
    echo Micromamba version:
    call "%MAMBA_ROOT_PREFIX%\micromamba.exe" --version
)

@rem create the installer env
echo "Checking if the Micromamba/Conda environment is already installed."
if not exist "%INSTALL_ENV_DIR%" (
    echo "Creating the Micromamba/Conda environment in %INSTALL_ENV_DIR%"
    call "%MAMBA_ROOT_PREFIX%\micromamba.exe" create -y --prefix "%INSTALL_ENV_DIR%"
    echo "Packages to install:%PACKAGES_TO_INSTALL%"
    call "%MAMBA_ROOT_PREFIX%\micromamba.exe" install -y --prefix "%INSTALL_ENV_DIR%" -c conda-forge %PACKAGES_TO_INSTALL%
) else (
    echo "Micromamba/Conda environment already installed in %INSTALL_ENV_DIR%, skipping installation"
)

if not exist "%INSTALL_ENV_DIR%" (
    echo "There was a problem while installing%PACKAGES_TO_INSTALL% using micromamba. Cannot continue."
    pause
    exit /b
)

@rem revert to the old APPDATA. only needed it for bypassing a bug in micromamba (with special characters)
set APPDATA=%OLD_APPDATA%
set USERPROFILE=%OLD_USERPROFILE%
