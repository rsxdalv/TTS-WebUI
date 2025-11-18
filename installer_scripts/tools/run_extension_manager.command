#!/bin/bash
echo "Starting Extension Management UI..."

# Move to repo root
ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT_DIR"

echo "Activating conda environment..."
source "./installer_scripts/activate.sh"

echo "Running Extension server..."
python ./installer_scripts/tools/extension_manager.py
