#!/bin/bash

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Change to the script's directory
cd "$SCRIPT_DIR"

export MICROMAMBA_EXE="./installer_files/mamba/micromamba"

$MICROMAMBA_EXE run -p ./installer_files/env bash
