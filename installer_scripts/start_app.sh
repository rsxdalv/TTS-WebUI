#!/bin/bash

# Get the directory where the script is located
SCRIPT_DIR="$(dirname "$0")"

LAUNCHER="$SCRIPT_DIR/../tts-webui"
if [ ! -x "$LAUNCHER" ]; then
    echo "The launcher was not found: $LAUNCHER"
    echo "Download tts-webui for your system from the releases page into the TTS WebUI folder, then start again."
    exit 1
fi

# Run the launcher, which starts the app
"$LAUNCHER" launch --start
# node "$SCRIPT_DIR/js/server.js"

# If the app crashes, provide a shell for debugging
echo ""
echo ""
echo "App exited or crashed."
echo "Starting shell for user to run commands in case of failure..."
echo ""
exec $SHELL
