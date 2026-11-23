#!/bin/bash

# Get the directory where the script is located
SCRIPT_DIR="$(dirname "$0")"

LAUNCHER=""
case "$(uname -s)" in Darwin) OS=macos ;; *) OS=linux ;; esac
case "$(uname -m)" in arm64|aarch64) ARCH=arm64 ;; *) ARCH=amd64 ;; esac
for candidate in "$SCRIPT_DIR/../tts-webui" "$SCRIPT_DIR/../tts-webui-$OS-$ARCH"; do
    if [ -x "$candidate" ]; then LAUNCHER="$candidate"; break; fi
done
if [ -z "$LAUNCHER" ]; then
    echo "The launcher was not found in $SCRIPT_DIR/.. (tts-webui or tts-webui-$OS-$ARCH)."
    echo "Download the TTS WebUI zip again (it contains the launchers) or build it: cd core && go build -o ../tts-webui ./cmd/tts-webui"
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
