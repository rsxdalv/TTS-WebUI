#!/bin/bash

# Move to repository root
ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT_DIR"

echo "Starting diagnostic..." > ./installer_scripts/diagnostic.txt
echo "" >> ./installer_scripts/diagnostic.txt

# Log current time
echo "$(date)" >> ./installer_scripts/diagnostic.txt
echo "" >> ./installer_scripts/diagnostic.txt

echo "CD: $(pwd)" >> ./installer_scripts/diagnostic.txt
echo "" >> ./installer_scripts/diagnostic.txt
echo "which git (before calling activate.sh):" >> ./installer_scripts/diagnostic.txt
which git >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt
echo "which python (before calling activate.sh):" >> ./installer_scripts/diagnostic.txt
which python >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt
echo "which conda (before calling activate.sh):" >> ./installer_scripts/diagnostic.txt
which conda >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt
echo "which node (before calling activate.sh):" >> ./installer_scripts/diagnostic.txt
which node >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt

source "$ROOT_DIR/installer_scripts/activate.sh"

echo "" >> ./installer_scripts/diagnostic.txt
echo "which git (after calling activate.sh):" >> ./installer_scripts/diagnostic.txt
which git >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt
echo "which python (after calling activate.sh):" >> ./installer_scripts/diagnostic.txt
which python >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt
echo "which conda (after calling activate.sh):" >> ./installer_scripts/diagnostic.txt
which conda >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt
echo "which node (after calling activate.sh):" >> ./installer_scripts/diagnostic.txt
which node >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt

# Log versions
echo "python --version:" >> ./installer_scripts/diagnostic.txt
python --version >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt
echo "node --version:" >> ./installer_scripts/diagnostic.txt
node --version >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt
echo "conda --version:" >> ./installer_scripts/diagnostic.txt
conda --version >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt

# Git status and log
echo "git status:" >> ./installer_scripts/diagnostic.txt
git status >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt
echo "git log HEAD~1..HEAD:" >> ./installer_scripts/diagnostic.txt
git log HEAD~1..HEAD >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt

# Package managers
echo "pip list:" >> ./installer_scripts/diagnostic.txt
pip list >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt
echo "conda list:" >> ./installer_scripts/diagnostic.txt
conda list >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt
echo "npm list -g:" >> ./installer_scripts/diagnostic.txt
npm list -g >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt

# Core packages
echo "pip show python-dotenv gradio torch torchaudio torchvision:" >> ./installer_scripts/diagnostic.txt
pip show python-dotenv gradio torch torchaudio torchvision >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt

# Configuration files
echo ".gpu:" >> ./installer_scripts/diagnostic.txt
cat ./installer_scripts/.gpu >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt

echo ".major_version:" >> ./installer_scripts/diagnostic.txt
cat ./installer_scripts/.major_version >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt

echo ".python_version:" >> ./installer_scripts/diagnostic.txt
cat ./installer_scripts/.python_version >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt

# npm list in react-ui
echo "npm list --prefix ./react-ui/" >> ./installer_scripts/diagnostic.txt
npm list --prefix ./react-ui/ >> ./installer_scripts/diagnostic.txt 2>&1
echo "" >> ./installer_scripts/diagnostic.txt
