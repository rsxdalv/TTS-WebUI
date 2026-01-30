#!/bin/bash

# Get repository root from this script's path
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

export MICROMAMBA_EXE="$ROOT_DIR/installer_files/mamba/micromamba"

"$MICROMAMBA_EXE" run -p "$ROOT_DIR/installer_files/env" bash
