#!/usr/bin/env bash
# M1 smoke: Go launcher serves the real launcher HTML and handles step endpoints.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
PORT=18788

# 1. Build Go binary
echo "--- building tts-webui binary ---"
cd "$REPO_ROOT/core"
go build -o bin/tts-webui ./cmd/tts-webui
cd "$REPO_ROOT"

# 2. Scratch root with stub scripts
SCRATCH=$(mktemp -d)
mkdir -p "$SCRATCH/installer_scripts"
echo "exit 0" > "$SCRATCH/installer_scripts/init_mamba.sh"
chmod +x "$SCRATCH/installer_scripts/init_mamba.sh"
printf '@echo off\r\n' > "$SCRATCH/installer_scripts/init_mamba.bat"
echo "Scratch root: $SCRATCH"

# 3. Start server in background
"$REPO_ROOT/core/bin/tts-webui" launch --no-browser --root "$SCRATCH" --port $PORT &
SRV_PID=$!
sleep 2

cleanup() {
  kill "$SRV_PID" 2>/dev/null || true
  wait "$SRV_PID" 2>/dev/null || true
}
trap cleanup EXIT

# 4. GET / — must contain tts-webui-token meta tag (not the placeholder)
INDEX=$(curl -s "http://127.0.0.1:$PORT/")
if [[ "$INDEX" != *"tts-webui-token"* ]]; then
  echo "FAIL: GET / did not return launcher HTML (missing tts-webui-token meta tag)"
  echo "Response: ${INDEX:0:300}"
  exit 1
fi
echo "PASS: GET / returned launcher HTML"

# 5. Extract token
TOKEN=$(echo "$INDEX" | grep -o 'content="[^"]*"' | head -1 | sed 's/content="//;s/"//')
echo "Token: ${TOKEN:0:16}..."

# 6. GET /api/steps/status
STATUS=$(curl -s -H "Cookie: tts-token=$TOKEN" "http://127.0.0.1:$PORT/api/steps/status")
echo "Steps status: $STATUS"

# 7. POST /api/steps/preflight
curl -s -X POST -H "Cookie: tts-token=$TOKEN" "http://127.0.0.1:$PORT/api/steps/preflight" > /dev/null
echo "PASS: preflight ran"

# 8. POST /api/steps/gpu with label CPU
curl -s -X POST -H "Cookie: tts-token=$TOKEN" -H "Content-Type: application/json" \
  -d '{"label":"CPU"}' "http://127.0.0.1:$PORT/api/steps/gpu" > /dev/null
GPU=$(cat "$SCRATCH/installer_scripts/.gpu" 2>/dev/null | tr -d '\n\r' || true)
if [ "$GPU" != "CPU" ]; then
  echo "FAIL: .gpu = '$GPU', want 'CPU'"
  exit 1
fi
echo "PASS: .gpu = CPU"

# 9. SSE stream test (3-second window)
SSE_OUTPUT=$(curl -s --max-time 3 -H "Cookie: tts-token=$TOKEN" \
  "http://127.0.0.1:$PORT/api/steps/preflight/stream" 2>/dev/null || true)
echo "SSE output (first 200 chars): ${SSE_OUTPUT:0:200}"

# 10. app start + stop
curl -s -X POST -H "Cookie: tts-token=$TOKEN" "http://127.0.0.1:$PORT/api/app/start" > /dev/null
curl -s -X POST -H "Cookie: tts-token=$TOKEN" "http://127.0.0.1:$PORT/api/app/stop" > /dev/null
echo "PASS: app start/stop"

# 11. Orphan-process check: wait for any supervised children to exit, then verify
sleep 2
CHILDREN=$(pgrep -P "$SRV_PID" 2>/dev/null || true)
if [ -n "$CHILDREN" ]; then
    echo "FAIL: orphan processes after app/stop (pids: $CHILDREN)"
    exit 1
fi
echo "PASS: no orphan processes after app/stop"

echo "M1 smoke complete."
