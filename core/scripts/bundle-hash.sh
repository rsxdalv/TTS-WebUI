#!/usr/bin/env bash
# Compute a deterministic hash of the core-app TypeScript sources and lockfile.
# CI recomputes this and fails if it differs from the committed bundle.hash,
# ensuring developers always rebuild after editing TypeScript sources.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SRC_DIR="$REPO_ROOT/launcher/vite-src/src/core-app"
LOCKFILE="$REPO_ROOT/launcher/vite-src/package-lock.json"
OUT="$REPO_ROOT/core/internal/webui/assets/bundle.hash"

mkdir -p "$(dirname "$OUT")"
# Hash all source files (sorted for determinism) plus the lockfile.
(cd "$REPO_ROOT" && find launcher/vite-src/src/core-app -type f | LC_ALL=C sort | xargs sha256sum launcher/vite-src/package-lock.json | sed "s/ \*/  /" | sha256sum | cut -d" " -f1) > "$OUT"
echo "Bundle hash written to $OUT"
