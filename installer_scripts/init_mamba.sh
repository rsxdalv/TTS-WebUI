#!/bin/bash

# License

# Copyright (c) 2022 cmdr2 and contributors

# Section I

# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:

# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.

# The person obtaining a copy of the Software meets the Use-based restrictions
# as referenced in Section II paragraph 1.

# The person obtaining a copy of the Software accepts that the Model or
# Derivatives of the Model (as defined in the "CreativeML Open RAIL-M" license
# accompanying this License) are subject to Section II paragraph 1.

# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.

# Section II

# 1. Use-based restrictions. The restrictions set forth in Attachment A are 
# considered Use-based restrictions. Therefore the person obtaining a copy of the
# Software cannot use the Software for the specified restricted uses. The person
# obtaining a copy of the Software may use the Software only for lawful purposes.

# 2. Except as set forth herein, the authors or copyright holders claim no rights
# in the results of operating the Software. The person obtaining a copy of the
# Software is accountable for the results of operating the Software and its
# subsequent uses.

# 3. If any provision of this License is held to be invalid, illegal or 
# unenforceable, the remaining provisions shall be unaffected thereby and 
# remain valid as if such provision had not been set forth herein. 

# END OF TERMS AND CONDITIONS 





# Attachment A

# Use Restrictions

# The person obtaining a copy of the Software agrees not to use the Software: 
# - In any way that violates any applicable national, federal, state, local 
# or international law or regulation; 
# - For the purpose of exploiting, harming or attempting to exploit or harm 
# minors in any way; 
# - To generate or disseminate verifiably false information and/or content 
# with the purpose of harming others; 
# - To generate or disseminate personal identifiable information that can 
# be used to harm an individual; 
# - To defame, disparage or otherwise harass others; 
# - For fully automated decision making that adversely impacts an 
# individual’s legal rights or otherwise creates or modifies a binding, 
# enforceable obligation; 
# - For any use intended to or which has the effect of discriminating 
# against or harming individuals or groups based on online or offline 
# social behavior or known or predicted personal or personality 
# characteristics; 
# - To exploit any of the vulnerabilities of a specific group of persons 
# based on their age, social, physical or mental characteristics, in order 
# to materially distort the behavior of a person pertaining to that group 
# in a manner that causes or is likely to cause that person or another 
# person physical or psychological harm; 
# - For any use intended to or which has the effect of discriminating 
# against individuals or groups based on legally protected characteristics 
# or categories; 
# - To provide medical advice and medical results interpretation; 
# - To generate or disseminate information for the purpose to be used for 
# administration of justice, law enforcement, immigration or asylum 
# processes, such as predicting an individual will commit fraud/crime 
# commitment (e.g. by text profiling, drawing causal relationships between 
# assertions made in documents, indiscriminate and arbitrarily-targeted 
# use). 


# This script will install git and conda (if not found on the PATH variable)
#  using micromamba (an 8mb static-linked single-file binary, conda replacement).
# For users who already have git and conda, this step will be skipped.

# This enables a user to install this project without manually installing conda and git.

fail() {
    echo
    echo "EEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEE"
    echo
    if [ "$1" != "" ]; then
        echo ERROR: $1
    else
        echo An error occurred.
    fi
    cat <<EOF

Error downloading TTS WebUI. Sorry about that, please try to:
 1. Run this installer again.
 2. If those steps don't help, please copy *all* the error messages in this window, and ask the community at https://discord.gg/V8BKTVRtJ9
 3. If that doesn't solve the problem, please file an issue at https://github.com/rsxdalv/tts-webui/issues

Thanks!


EOF
    read -p "Press any key to continue"
    exit 1

}

set -o pipefail

OS_NAME=$(uname -s)
case "${OS_NAME}" in
    Linux*)     OS_NAME="linux";;
    Darwin*)    OS_NAME="osx";;
    *)          echo "Unknown OS: $OS_NAME! This script runs only on Linux or Mac" && exit
esac

OS_ARCH=$(uname -m)
case "${OS_ARCH}" in
    x86_64*)    OS_ARCH="64";;
    arm64*)     OS_ARCH="arm64";;
    aarch64*)     OS_ARCH="arm64";;
    *)          echo "Unknown system architecture: $OS_ARCH! This script runs only on x86_64 or arm64" && exit
esac

if ! which curl; then fail "'curl' not found. Please install curl."; fi
if ! which tar; then fail "'tar' not found. Please install tar."; fi
if ! which bzip2; then fail "'bzip2' not found. Please install bzip2."; fi

if pwd | grep ' '; then fail "The installation directory's path contains a space character. Conda will fail to install. Please change the directory."; fi

# https://mamba.readthedocs.io/en/latest/installation.html
if [ "$OS_NAME" == "linux" ] && [ "$OS_ARCH" == "arm64" ]; then OS_ARCH="aarch64"; fi

# config
export MAMBA_ROOT_PREFIX="$(pwd)/installer_files/mamba"
INSTALL_ENV_DIR="$(pwd)/installer_files/env"
LEGACY_INSTALL_ENV_DIR="$(pwd)/installer"
MICROMAMBA_DOWNLOAD_URL="https://micro.mamba.pm/api/micromamba/${OS_NAME}-${OS_ARCH}/latest"
umamba_exists="F"

# figure out whether git and conda needs to be installed
if [ -e "$INSTALL_ENV_DIR" ]; then export PATH="$INSTALL_ENV_DIR/bin:$PATH"; fi

PACKAGES_TO_INSTALL="python=3.10.11 conda-forge::nodejs=22.9.0 conda ninja conda-forge::postgresql=16.4 conda-forge::ffmpeg=4.4.2[build=lgpl*]"

if ! hash "git" &>/dev/null; then PACKAGES_TO_INSTALL="$PACKAGES_TO_INSTALL git"; fi

if "$MAMBA_ROOT_PREFIX/micromamba" --version &>/dev/null; then umamba_exists="T"; fi

# (if necessary) install git and conda into a contained environment
if [ "$PACKAGES_TO_INSTALL" != "" ]; then
    # download micromamba
    if [ "$umamba_exists" == "F" ]; then
        echo "Downloading micromamba from $MICROMAMBA_DOWNLOAD_URL to $MAMBA_ROOT_PREFIX/micromamba"

        mkdir -p "$MAMBA_ROOT_PREFIX"
        curl -L "$MICROMAMBA_DOWNLOAD_URL" | tar -xvj -O bin/micromamba > "$MAMBA_ROOT_PREFIX/micromamba"

        if [ "$?" != "0" ]; then
            echo
            echo "EE micromamba download failed"
            echo "EE If the lines above contain 'bzip2: Cannot exec', your system doesn't have bzip2 installed"
            echo "EE If there are network errors, please check your internet setup"
            fail "micromamba download failed"
        fi

        chmod u+x "$MAMBA_ROOT_PREFIX/micromamba"

        # test the mamba binary
        echo "Micromamba version:"
        "$MAMBA_ROOT_PREFIX/micromamba" --version
    fi

    # create the installer env
    echo "Checking if the Micromamba/Conda environment is already installed"
    if [ ! -e "$INSTALL_ENV_DIR" ]; then
        echo "Creating the Micromamba/Conda environment in $INSTALL_ENV_DIR"
        "$MAMBA_ROOT_PREFIX/micromamba" create -y --prefix "$INSTALL_ENV_DIR" || fail "unable to create the install environment"
        echo "Packages to install:$PACKAGES_TO_INSTALL"

        "$MAMBA_ROOT_PREFIX/micromamba" install -y --prefix "$INSTALL_ENV_DIR" -c conda-forge $PACKAGES_TO_INSTALL
        if [ "$?" != "0" ]; then
            fail "Installation of the packages '$PACKAGES_TO_INSTALL' failed."
        fi
    else
        echo "Micromamba/Conda environment already installed in $INSTALL_ENV_DIR, skipping installation"
    fi
   
    if [ ! -e "$INSTALL_ENV_DIR" ]; then
        fail "There was a problem while installing $PACKAGES_TO_INSTALL using micromamba. Cannot continue."
    fi

fi
