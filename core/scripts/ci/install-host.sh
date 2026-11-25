#!/usr/bin/env bash
# Installs the Gradio host (entry gradio-host) through the launcher, the way the
# Extensions page does: uv, the pinned Python and the app's requirements into
# .venvs/gradio-host, then checks that the host's Python imports tts_webui.
#
#   install-host.sh <launcher binary> <install root> [port]
#
# It needs no Python, git or Node on PATH. Runs on Linux, macOS and Windows (Git Bash).
set -euo pipefail

BIN="$1"
ROOT="$2"
PORT="${3:-17701}"
BASE="http://127.0.0.1:${PORT}"

case "$(uname -s)" in
  Darwin*) GPU="Apple M Series Chip" ;;
  *) GPU="CPU" ;;
esac

"$BIN" launch --root "$ROOT" --port "$PORT" --no-browser >"$ROOT/install-host.launcher.log" 2>&1 &
LAUNCHER=$!
trap 'kill "$LAUNCHER" 2>/dev/null || true' EXIT

for _ in $(seq 1 60); do
  curl -fsS "$BASE/api/health" >/dev/null 2>&1 && break
  sleep 1
done
curl -fsS "$BASE/api/health" >/dev/null || { echo "launcher did not come up"; cat "$ROOT/install-host.launcher.log"; exit 1; }

TOKEN="$(curl -fsS "$BASE/" | sed -n 's/.*name="tts-webui-token" content="\([^"]*\)".*/\1/p' | head -n1)"
[ -n "$TOKEN" ] || { echo "no token in the launcher page"; exit 1; }

echo "== installing gradio-host (GPU choice: $GPU)"
OUT="$ROOT/install-host.out"
curl -fsS -N -X POST "$BASE/api/ext/op/install" \
  -H "X-TTS-Token: $TOKEN" -H "Content-Type: application/json" \
  -d "{\"package_name\":\"gradio-host\",\"gpu\":\"$GPU\"}" | tee "$OUT"

# The stream ends with {"done":true,"ok":true|false}.
if ! tail -n 1 "$OUT" | grep -q '"ok":true'; then
  echo "the install of gradio-host failed"
  exit 1
fi

if [ -x "$ROOT/.venvs/gradio-host/bin/python" ]; then
  PY="$ROOT/.venvs/gradio-host/bin/python"
else
  PY="$ROOT/.venvs/gradio-host/Scripts/python.exe"
fi
cd "$ROOT"
"$PY" -c "import tts_webui, gradio, torch; print('tts_webui', 'gradio', gradio.__version__, 'torch', torch.__version__)"
echo "== gradio-host is installed and imports"
