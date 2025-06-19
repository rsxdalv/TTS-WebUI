$logFile = Join-Path -Path $PSScriptRoot -ChildPath "output.log"
Start-Transcript -Path $logFile

# Check if running from System32 directory
if ($PWD.Path -eq "C:\Windows\System32") {
    Write-Host "ERROR: Do not run this script as administrator!"
    Write-Host "The script is running from C:\Windows\System32 which will cause the installation to fail."
    exit 1
}


$title    = 'Confirmation'
$question = 'Are you sure you want to proceed?'
$choices  = '&Yes', '&No'

# check if there are spaces in the path
if ($PSScriptRoot -match " ") {
    Write-Host "Warning: The installation directory's path contains a space character. Conda will fail to install. Please change the directory."
    Write-Host "For example, C:\AI\tts-webui\"
    # ask user if they still want to continue
    $decision = $Host.UI.PromptForChoice($title, $question, $choices, 1)
    if ($decision -eq 1) {
        exit 1
    }
}


# check if long paths are enabled
$longPathsEnabled = (Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem' -Name 'LongPathsEnabled').LongPathsEnabled
if ($longPathsEnabled -ne 1) {
    Write-Host "Warning: Long paths are not enabled, please enable them and restart the installer."
    Write-Host "The installer is likely to fail without long paths enabled, that is why this is required."
    Write-Host "You can enable long paths by running the enable_long_paths.reg file in the installer_scripts folder."
    Write-Host "For more information, please visit:"
    Write-Host "https://learn.microsoft.com/en-us/windows/win32/fileio/maximum-file-path-limitation?tabs=registry#enable-long-paths-in-windows-10-version-1607-and-later"
    # ask user if they still want to continue
    $decision = $Host.UI.PromptForChoice($title, $question, $choices, 1)
    if ($decision -eq 1) {
        exit 1
    }
}

# check if conda is already installed
if (Get-Command "conda" -ErrorAction SilentlyContinue) {
    # conda might interfere, warn the user and ask if they want to continue
    Write-Host "Warning: Conda is already installed, this might interfere with the installation."
    # $decision = $Host.UI.PromptForChoice($title, $question, $choices, 1)
    # if ($decision -eq 1) {
    #     exit 1
    # }
    # Check if Base environment is active, if so, deactivate it or WARN the user
    $condaEnv = conda info --envs | Select-String "base"
    if ($condaEnv -match "\*") {
        Write-Host "Warning: Base environment is active, please deactivate it before running the installer."
        Write-Host "You can deactivate the base environment by running 'conda deactivate' in a command prompt."
        # ask user if they still want to continue
        $decision = $Host.UI.PromptForChoice($title, $question, $choices, 1)
        if ($decision -eq 1) {
            exit 1
        }
    }

    # conda info --envs
    # conda deactivate
}

& "$PSScriptRoot\init_mamba.bat"
if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to init mamba, exiting..."
    exit 1
}

$env:Path += ";$PSScriptRoot\..\installer_files\env\Library\bin\"

if (!(Get-Command "vswhere" -ErrorAction SilentlyContinue)) {
    Write-Host "Critical Warning: vswhere is not installed, automatic validation of Visual Studio Build Tools installation will not work."
    Write-Host "For more information, please visit:"
    Write-Host "https://github.com/microsoft/vswhere"
    Write-Host "The app will try to launch but might fail."
} else {
    Write-Host "vswhere is installed, checking for Visual Studio Build Tools installation..."
    $vswhereOutput = vswhere -products * -format json | ConvertFrom-Json
    if ($vswhereOutput.length -eq 0) {
        Write-Host "Warning: Visual Studio compiler is not installed."
        if (!(Get-Command "winget" -ErrorAction SilentlyContinue)) {
            Write-Host "Warning: winget is not installed, automatic installation of Visual Studio Build Tools will not work."
            Write-Host "Please install Visual Studio Build Tools manually and restart the installer."
            Write-Host "(Note: The full Visual Studio is NOT required, only the Build Tools)"
            Write-Host "For more information, please visit:"
            Write-Host "https://learn.microsoft.com/en-us/cpp/build/vscpp-step-0-installation?view=msvc-170"
            exit 1
        } else {
            Write-Host "Attempting to install Visual Studio Build Tools using winget..."
            Write-Host "This will open a new window, please follow the instructions."
            # quiet install does not seem user friendly
            # winget install Microsoft.VisualStudio.2022.BuildTools --silent --override "--wait --quiet --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended"
            winget install Microsoft.VisualStudio.2022.BuildTools --silent --override "--wait --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended"
        }
    } else {
        Write-Host "Visual Studio Build Tools is installed, continuing..."
    }
}

& "$PSScriptRoot\init_app.bat"
if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to init the app, exiting..."
    exit 1
}

& "$PSScriptRoot\start_app.bat"
if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to start the app, exiting..."
    exit 1
}


Stop-Transcript
