#!/bin/bash

# Get the directory where the script is located
SCRIPT_DIR="$(dirname "$0")"

# Source the activate script to set up the environment
source "$SCRIPT_DIR/activate.sh"

echo "Starting TTS WebUI Unified Server..."

# Run the unified process manager
node "$SCRIPT_DIR/js/processManager.js" "$@"

# If the app crashes, provide a shell for debugging
echo ""
echo ""
echo "App exited or crashed."
echo "Starting shell for user to run commands in case of failure..."
echo ""
exec $SHELL
