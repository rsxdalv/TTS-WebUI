#!/usr/bin/env bash
# Compute a deterministic hash of the core-app TypeScript sources and lockfile.
# CI recomputes this and fails if it differs from the committed bundle.hash,
# ensuring developers always rebuild after editing TypeScript sources.
#
# Line endings are normalised (CR removed) before hashing. .gitattributes makes
# git store these files with LF, but a working tree can hold CRLF copies (an
# editor or tool on Windows writes them), and hashing raw bytes then yields a
# hash that no Linux checkout of the same commit can reproduce. For LF files the
# result is identical to hashing the raw bytes.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SRC_DIR="core/ui/src/core-app"
LOCKFILE="core/ui/package-lock.json"
OUT="$REPO_ROOT/core/internal/webui/assets/bundle.hash"

mkdir -p "$(dirname "$OUT")"
# Lockfile first, then the sources sorted bytewise (for determinism). Each line
# is "<sha256 of LF-normalised content>  <path>", the format sha256sum prints.
(
  cd "$REPO_ROOT"
  {
    echo "$LOCKFILE"
    find "$SRC_DIR" -type f | LC_ALL=C sort
  } | while IFS= read -r f; do
    printf '%s  %s\n' "$(tr -d '\r' <"$f" | sha256sum | cut -d" " -f1)" "$f"
  done | sha256sum | cut -d" " -f1
) >"$OUT"
echo "Bundle hash written to $OUT"
