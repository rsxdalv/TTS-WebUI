#!/bin/bash

# Move to repository root
ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT_DIR"

export MICROMAMBA_EXE="$ROOT_DIR/installer_files/mamba/micromamba"

echo "Generating diagnostic..."

"$MICROMAMBA_EXE" run -p "$ROOT_DIR/installer_files/env" "$ROOT_DIR/installer_scripts/diagnostic_inner.sh"

echo "Diagnostic generated."
