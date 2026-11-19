#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
VITE_SRC="$REPO_ROOT/core/ui"
ASSETS="$REPO_ROOT/core/internal/webui/assets"

echo "Building core entry ..."
cd "$VITE_SRC"
npm ci
npm run build

echo "Copying dist-core/ to $ASSETS ..."
rm -rf "$ASSETS"
mkdir -p "$ASSETS"
cp -r dist-core/. "$ASSETS/"

# core.html is the entry; rename to index.html for the Go server.
if [ -f "$ASSETS/core.html" ]; then
  mv "$ASSETS/core.html" "$ASSETS/index.html"
fi

echo "Done. Core entry built and copied to core/internal/webui/assets/"

echo "Computing bundle source hash ..."
bash "$REPO_ROOT/core/scripts/bundle-hash.sh"
