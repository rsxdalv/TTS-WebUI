#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
VITE_SRC="$REPO_ROOT/launcher/vite-src"
ASSETS="$REPO_ROOT/core/internal/webui/assets"

echo "Building launcher with VITE_PLATFORM=core ..."
cd "$VITE_SRC"
npm ci
VITE_PLATFORM=core npm run build

echo "Copying dist/ to $ASSETS ..."
rm -rf "$ASSETS"
mkdir -p "$ASSETS"
cp -r dist/. "$ASSETS/"

echo "Done. Launcher built and copied to core/internal/webui/assets/"
