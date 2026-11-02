#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
VITE_SRC="$REPO_ROOT/launcher/vite-src"
ASSETS="$REPO_ROOT/core/internal/webui/assets"

echo "Building core entry ..."
cd "$VITE_SRC"
npm ci
npm run build:core

echo "Copying dist-core/ to $ASSETS ..."
rm -rf "$ASSETS"
mkdir -p "$ASSETS"
cp -r dist-core/. "$ASSETS/"

# core.html is the entry; rename to index.html for the Go server.
if [ -f "$ASSETS/core.html" ]; then
  mv "$ASSETS/core.html" "$ASSETS/index.html"
fi

echo "Done. Core entry built and copied to core/internal/webui/assets/"
