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
find "$SRC_DIR" -type f | sort | xargs sha256sum "$LOCKFILE" | sha256sum > "$OUT"
echo "Bundle hash written to $OUT"
