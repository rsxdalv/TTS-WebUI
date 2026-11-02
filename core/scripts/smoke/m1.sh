#!/usr/bin/env bash
# M1 smoke (bash): drives every M1 requirement against the real tts-webui binary.
# See m1.ps1 for the authoritative implementation; this mirrors it for Linux/macOS.
set -euo pipefail

FAILED=0
fail() { echo "FAIL: $*"; FAILED=1; }
pass() { echo "PASS: $*"; }

REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
SMOKE_DATA="$(dirname "$0")/testdata"
PORT=18788
BIN="$REPO_ROOT/core/bin/tts-webui-smoke"

echo; echo "-- Building tts-webui --"
cd "$REPO_ROOT/core"
go build -o bin/tts-webui-smoke ./cmd/tts-webui
cd "$REPO_ROOT"
pass "binary built"

echo; echo "-- Building stubs --"
STUB_DIR=$(mktemp -d)
for stub in fakemamba fakepython; do
  GOOS=$(go env GOOS) GOARCH=$(go env GOARCH) CGO_ENABLED=0 \
    go build -o "$STUB_DIR/$stub" "$SMOKE_DATA/$stub/main.go"
  pass "stub built: $stub"
done

echo; echo "-- Setting up scratch root --"
SCRATCH=$(mktemp -d)
SCRIPTS="$SCRATCH/installer_scripts"
IFILES="$SCRATCH/installer_files"
MAMBA_DIR="$IFILES/mamba"
ENV_DIR="$IFILES/env"
mkdir -p "$SCRIPTS" "$MAMBA_DIR" "$ENV_DIR"

cp "$STUB_DIR/fakepython" "$ENV_DIR/python"
chmod +x "$ENV_DIR/python"

# init_mamba.sh: creates the micromamba stub
cat > "$SCRIPTS/init_mamba.sh" <<MAMBA
#!/usr/bin/env bash
set -euo pipefail
echo "mamba: initializing micromamba"
mkdir -p installer_files/mamba
cp "$STUB_DIR/fakemamba" installer_files/mamba/micromamba
chmod +x installer_files/mamba/micromamba
echo "mamba: micromamba installed successfully"
MAMBA
chmod +x "$SCRIPTS/init_mamba.sh"
echo "# M1 smoke placeholder" > "$SCRATCH/server.py"
echo "Scratch root: $SCRATCH"

GOLDEN_GPU="CPU"
GOLDEN_GIT_VERSION="smoke-test-version"

echo; echo "-- Starting tts-webui --"
"$BIN" launch --no-browser --root "$SCRATCH" --port $PORT &
SRV_PID=$!
sleep 3

cleanup() {
  kill "$SRV_PID" 2>/dev/null || true
  wait "$SRV_PID" 2>/dev/null || true
  rm -rf "$STUB_DIR" "$SCRATCH" "$BIN"
}
trap cleanup EXIT

if ! kill -0 "$SRV_PID" 2>/dev/null; then
  fail "tts-webui exited immediately"; exit 1
fi
pass "tts-webui running (PID $SRV_PID)"

echo; echo "-- GET / --"
INDEX=$(curl -sf "http://127.0.0.1:$PORT/")
if [[ "$INDEX" == *"Neutralino"* ]]; then fail "bundle contains Neutralino"
else pass "bundle has no Neutralino string"; fi

# Extract cookie from Set-Cookie header
COOKIE_HDR=$(curl -sI "http://127.0.0.1:$PORT/" | grep -i "set-cookie" | grep "tts-token" | head -1 || true)
if [[ -z "$COOKIE_HDR" ]]; then fail "no tts-token Set-Cookie header"; exit 1; fi
TOKEN=$(echo "$COOKIE_HDR" | sed 's/.*tts-token=\([^;]*\).*/\1/' | tr -d '[:space:]')
pass "tts-token cookie received"

# Check JS bundle
JS_HREF=$(echo "$INDEX" | grep -o 'src="[^"]*\.js"' | head -1 | sed 's/src="//;s/"//')
if [[ -n "$JS_HREF" ]]; then
  JS_URL="http://127.0.0.1:$PORT$JS_HREF"
  JS_BODY=$(curl -sf --cookie "tts-token=$TOKEN" "$JS_URL")
  if [[ "$JS_BODY" == *"/api/steps"* ]]; then pass "JS bundle contains /api/steps"
  else fail "JS bundle does not contain /api/steps"; fi
else fail "no JS script in index.html"; fi

api() {
  local method="$1" path="$2" body="${3:-}"
  local args=(-sf --cookie "tts-token=$TOKEN" -X "$method" "http://127.0.0.1:$PORT$path")
  if [[ -n "$body" ]]; then args+=(-H "Content-Type: application/json" -d "$body"); fi
  local resp
  resp=$(curl "${args[@]}")
  echo "$resp"
}

sse_test() {
  local path="$1" label="$2" timeout_s="${3:-12}"
  echo "  SSE: $label"
  local out
  out=$(curl -sf --cookie "tts-token=$TOKEN" --max-time "$timeout_s" \
        "http://127.0.0.1:$PORT$path" 2>/dev/null || true)
  if echo "$out" | grep -q "^data:" ; then
    pass "$label stream: log line received"
  else
    fail "$label stream: no log data lines"
  fi
  if echo "$out" | grep -q "^event: done"; then
    pass "$label stream: event:done received"
  else
    fail "$label stream: no event:done received"
  fi
}

echo; echo "-- Step: status --"
STATUS=$(api GET /api/steps/status); pass "status: $STATUS"

echo; echo "-- Step: preflight --"
PF=$(api POST /api/steps/preflight)
if echo "$PF" | grep -q '"checks"'; then pass "preflight: checks returned"
else fail "preflight: missing checks field"; fi
if [[ -f "$SCRIPTS/.preflight_ok" ]]; then pass ".preflight_ok created"
else echo "NOTE: .preflight_ok absent (some env checks may have failed)"; fi

echo; echo "-- Step: mamba --"
api POST /api/steps/mamba > /dev/null; pass "mamba started"
sse_test /api/steps/mamba/stream mamba
DEADLINE=$(($(date +%s) + 12))
while [[ ! -f "$MAMBA_DIR/micromamba" && $(date +%s) -lt $DEADLINE ]]; do sleep 0.3; done
if [[ -f "$MAMBA_DIR/micromamba" ]]; then pass "micromamba binary created"
else fail "micromamba not created by mamba step"; fi

echo; echo "-- Step: gpu --"
api POST /api/steps/gpu '{"label":"CPU"}' > /dev/null
GPU_FILE="$SCRIPTS/.gpu"
if [[ -f "$GPU_FILE" ]]; then
  GPU_VAL=$(tr -d '[:space:]' < "$GPU_FILE")
  if [[ "$GPU_VAL" == "$GOLDEN_GPU" ]]; then pass ".gpu = '$GPU_VAL' (golden match)"
  else fail ".gpu = '$GPU_VAL', want '$GOLDEN_GPU'"; fi
else fail ".gpu not created"; fi

echo; echo "-- Step: install --"
api POST /api/steps/install > /dev/null; pass "install started"
sse_test /api/steps/install/stream install
DEADLINE=$(($(date +%s) + 12))
while [[ ! -f "$SCRIPTS/.git_version" && $(date +%s) -lt $DEADLINE ]]; do sleep 0.3; done
GV_FILE="$SCRIPTS/.git_version"
if [[ -f "$GV_FILE" ]]; then
  GV=$(tr -d '[:space:]' < "$GV_FILE")
  if [[ "$GV" == "$GOLDEN_GIT_VERSION" ]]; then pass ".git_version = '$GV' (golden match)"
  else fail ".git_version = '$GV', want '$GOLDEN_GIT_VERSION'"; fi
else fail ".git_version not created by install step"; fi

echo; echo "-- Step: update --"
UPDATE=$(api POST /api/steps/update); pass "update: $UPDATE"

echo; echo "-- Step: run --"
api POST /api/steps/run > /dev/null; pass "run started"
sse_test /api/steps/run/stream run

echo; echo "-- App lifecycle --"
BEFORE_PIDS=$(pgrep -P "$SRV_PID" -f fakepython || true)
api POST /api/app/start > /dev/null; pass "app/start returned 200"
sleep 5
AFTER_PIDS=$(pgrep -P "$SRV_PID" -f fakepython || true)
NEW_PIDS=$(comm -13 <(echo "$BEFORE_PIDS" | sort) <(echo "$AFTER_PIDS" | sort) || true)
if [[ -n "$NEW_PIDS" ]]; then pass "fake python alive 5 s after app/start (PID $NEW_PIDS)"
else fail "fake python not alive 5 s after app/start"; fi

api POST /api/app/stop > /dev/null; pass "app/stop returned 200"
sleep 2
SURVIVORS=""
for pid in $NEW_PIDS; do
  if kill -0 "$pid" 2>/dev/null; then SURVIVORS="$SURVIVORS $pid"; fi
done
if [[ -z "$SURVIVORS" ]]; then pass "no orphan processes after app/stop"
else fail "orphan processes after app/stop: $SURVIVORS"; fi

echo
if [[ $FAILED -eq 0 ]]; then echo "M1 smoke: PASS"
else echo "M1 smoke: FAIL"; exit 1; fi
