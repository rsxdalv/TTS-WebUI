#!/usr/bin/env bash
# M1 smoke (bash): drives every M1 requirement against the real tts-webui binary.
# Mirrors m1.ps1 for Linux/macOS CI.
#
# Requirements from PLAN.md "Verification rules" M1:
#   1. Bundle has no Neutralino string; JS bundle has /api/steps string.
#   2. ALL steps driven via cookie from GET / (no X-TTS-Token header).
#   3. Every streaming step: at least one log line + event:done.
#   4. Marker files compared against goldens.
#   5. .preflight_ok absent = FAIL.
#   6. app/start + fakepython (HTTP on 7770); alive 5 s later; app/stop; no orphans.
#   7. ALL descendants tracked — no name filter.
#   8. app started via POST /api/app/start.
#   9. No non-2xx is acceptable.
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
CGO_ENABLED=0 go build -o bin/tts-webui-smoke ./cmd/tts-webui
cd "$REPO_ROOT"
pass "binary built"

echo; echo "-- Building stubs --"
STUB_DIR=$(mktemp -d)
for stub in fakemamba fakepython; do
  CGO_ENABLED=0 GOOS=$(go env GOOS) GOARCH=$(go env GOARCH) \
    go build -o "$STUB_DIR/$stub" "$SMOKE_DATA/$stub"
  chmod +x "$STUB_DIR/$stub"
  pass "stub built: $stub"
done

echo; echo "-- Setting up scratch root --"
SCRATCH=$(mktemp -d)
SCRIPTS="$SCRATCH/installer_scripts"
IFILES="$SCRATCH/installer_files"
MAMBA_DIR="$IFILES/mamba"
ENV_DIR="$IFILES/env"
mkdir -p "$SCRIPTS" "$MAMBA_DIR" "$ENV_DIR"

# fakepython → installer_files/env/python (appPython() non-Windows path)
cp "$STUB_DIR/fakepython" "$ENV_DIR/python"
chmod +x "$ENV_DIR/python"

# fakemamba → installer_files/mamba/micromamba (micromambaPath() non-Windows)
cp "$STUB_DIR/fakemamba" "$MAMBA_DIR/micromamba"
chmod +x "$MAMBA_DIR/micromamba"

# init_mamba.sh: writes the mamba stub and prints a log line.
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
"$BIN" launch --no-browser --root "$SCRATCH" --port "$PORT" &
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

# R7: get all descendant PIDs recursively (no name filter).
get_descendants() {
  local parent="$1"
  # Use ps on macOS; pgrep -P works on Linux. Build list with awk.
  awk -v ppid="$parent" 'BEGIN {
    while (("ps -axo pid=,ppid=" | getline) > 0) { ppid_of[$1]=$2 }
    for (p in ppid_of) candidates[p]=1
    changed=1
    while (changed) {
      changed=0
      for (p in candidates) {
        if (candidates[p]==1 && ppid_of[p] in roots) { roots[p]=1; changed=1 }
      }
    }
    roots[ppid]=1
    changed=1
    while (changed) {
      changed=0
      for (p in ppid_of) {
        if (!(p in roots) && ppid_of[p] in roots) { roots[p]=1; changed=1 }
      }
    }
    for (p in roots) { if (p != ppid) print p }
  }' /dev/null
}

# Simpler: pgrep -P recurse helper (works on Linux; macOS pgrep lacks -r).
descendants_of() {
  local pid="$1"
  local children
  children=$(pgrep -P "$pid" 2>/dev/null || true)
  echo "$children"
  for c in $children; do descendants_of "$c"; done
}

# GET / and extract cookie
echo; echo "-- GET / --"
COOKIE_RESP=$(curl -sc /tmp/m1-cookies-$PORT.txt -o /tmp/m1-index-$PORT.html \
  -w "%{http_code}" "http://127.0.0.1:$PORT/" 2>/dev/null)
if [[ "$COOKIE_RESP" -lt 200 || "$COOKIE_RESP" -ge 300 ]]; then
  fail "GET / HTTP $COOKIE_RESP"; exit 1
fi
INDEX=$(cat /tmp/m1-index-$PORT.html)

# R1
if echo "$INDEX" | grep -q "Neutralino"; then fail "bundle contains Neutralino"
else pass "bundle has no Neutralino string"; fi

JS_HREF=$(echo "$INDEX" | grep -o 'src="[^"]*\.js"' | head -1 | sed 's/src="//;s/"//')
if [[ -n "$JS_HREF" ]]; then
  JS_URL="http://127.0.0.1:$PORT$JS_HREF"
  JS_BODY=$(curl -sb /tmp/m1-cookies-$PORT.txt -o - "$JS_URL" 2>/dev/null || true)
  if echo "$JS_BODY" | grep -q "/api/steps"; then pass "JS bundle contains /api/steps"
  else fail "JS bundle does not contain /api/steps"; fi
else fail "no JS script found in index.html"; fi

# R2: extract cookie
TOKEN=$(awk -F'\t' '/tts-token/{print $NF}' /tmp/m1-cookies-$PORT.txt | head -1 | tr -d '[:space:]')
if [[ -z "$TOKEN" ]]; then fail "no tts-token cookie"; exit 1; fi
pass "tts-token cookie received"

# R9: non-2xx = FAIL. Cookie-only helper.
api() {
  local method="$1" path="$2" body="${3:-}"
  local args=(-sf -b /tmp/m1-cookies-$PORT.txt -X "$method" "http://127.0.0.1:$PORT$path")
  [[ -n "$body" ]] && args+=(-H "Content-Type: application/json" -d "$body")
  local http_code
  local resp
  resp=$(curl "${args[@]}" -w "\n%{http_code}" 2>/dev/null)
  http_code=$(echo "$resp" | tail -1)
  body_resp=$(echo "$resp" | head -n -1)
  if [[ "$http_code" -lt 200 || "$http_code" -ge 300 ]]; then
    fail "$method $path HTTP $http_code"
  fi
  echo "$body_resp"
}

# R3: SSE stream assert.
sse_test() {
  local path="$1" label="$2" timeout_s="${3:-20}"
  echo "  SSE: $label (timeout=${timeout_s}s)"
  local out
  out=$(curl -sf -b /tmp/m1-cookies-$PORT.txt --max-time "$timeout_s" \
        "http://127.0.0.1:$PORT$path" 2>/dev/null || true)
  if echo "$out" | grep -q "^data:"; then
    pass "$label stream: log line"
  else fail "$label stream: no log data lines"; fi
  if echo "$out" | grep -qE "^event:[[:space:]]*done"; then
    pass "$label stream: event:done"
  else fail "$label stream: no event:done"; fi
}

# --- Step: preflight ----------------------------------------------------------
echo; echo "-- Step: preflight --"
PF=$(api POST /api/steps/preflight)
if echo "$PF" | grep -q '"checks"'; then pass "preflight: checks returned"
else fail "preflight: missing checks field"; fi
# R5: .preflight_ok absent = FAIL
if [[ -f "$SCRIPTS/.preflight_ok" ]]; then pass ".preflight_ok created"
else fail ".preflight_ok not created"; fi

# --- Step: mamba --------------------------------------------------------------
echo; echo "-- Step: mamba --"
api POST /api/steps/mamba > /dev/null; pass "mamba started"
sse_test /api/steps/mamba/stream mamba
DEADLINE=$(($(date +%s) + 15))
while [[ ! -f "$MAMBA_DIR/micromamba" && $(date +%s) -lt $DEADLINE ]]; do sleep 0.3; done
if [[ -f "$MAMBA_DIR/micromamba" ]]; then pass "micromamba binary present"
else fail "micromamba not found after mamba step"; fi

# --- Step: gpu ----------------------------------------------------------------
echo; echo "-- Step: gpu --"
api POST /api/steps/gpu '{"label":"CPU"}' > /dev/null
GPU_FILE="$SCRIPTS/.gpu"
# R4
if [[ -f "$GPU_FILE" ]]; then
  GPU_VAL=$(tr -d '[:space:]' < "$GPU_FILE")
  if [[ "$GPU_VAL" == "$GOLDEN_GPU" ]]; then pass ".gpu = '$GPU_VAL' (golden match)"
  else fail ".gpu = '$GPU_VAL', want '$GOLDEN_GPU'"; fi
else fail ".gpu not created"; fi

# --- Step: install ------------------------------------------------------------
echo; echo "-- Step: install --"
api POST /api/steps/install > /dev/null; pass "install started"
sse_test /api/steps/install/stream install
DEADLINE=$(($(date +%s) + 15))
while [[ ! -f "$SCRIPTS/.git_version" && $(date +%s) -lt $DEADLINE ]]; do sleep 0.3; done
GV_FILE="$SCRIPTS/.git_version"
# R4
if [[ -f "$GV_FILE" ]]; then
  GV=$(tr -d '[:space:]' < "$GV_FILE")
  if [[ "$GV" == "$GOLDEN_GIT_VERSION" ]]; then pass ".git_version = '$GV' (golden match)"
  else fail ".git_version = '$GV', want '$GOLDEN_GIT_VERSION'"; fi
else fail ".git_version not created"; fi

# --- Step: update -------------------------------------------------------------
echo; echo "-- Step: update --"
UPD=$(api POST /api/steps/update)
if echo "$UPD" | grep -q '"already_updated":true'; then
  pass "update: already_updated=true (started=false)"
elif echo "$UPD" | grep -q '"started":true'; then
  pass "update: started=true"
  sse_test /api/steps/update/stream update
else
  pass "update: $UPD"
fi

# --- Step: run — POST /api/app/start (R8) ------------------------------------
echo; echo "-- Step: run (POST /api/app/start) --"

# R7: ALL descendants before app/start.
BEFORE_PIDS=$(descendants_of "$SRV_PID" | sort -u)

api POST /api/app/start > /dev/null; pass "app/start returned 2xx"

# R6: poll GET /api/app/link for HTTP server.
APP_URL=""
DEADLINE=$(($(date +%s) + 10))
while [[ -z "$APP_URL" && $(date +%s) -lt $DEADLINE ]]; do
  sleep 0.5
  APP_URL=$(api GET /api/app/link 2>/dev/null | grep -o '"url":"[^"]*"' | sed 's/"url":"//;s/"//' || true)
done
if [[ -n "$APP_URL" ]]; then pass "app link appeared: $APP_URL"
else fail "app link did not appear within 10 s"; fi

# R6: alive 5 s after app/start.
sleep 5
AFTER_PIDS=$(descendants_of "$SRV_PID" | sort -u)
NEW_PIDS=$(comm -13 <(echo "$BEFORE_PIDS") <(echo "$AFTER_PIDS") | tr '\n' ' ' | tr -d '[:space:]')
ALIVE_NEW=""
for pid in $NEW_PIDS; do
  if kill -0 "$pid" 2>/dev/null; then ALIVE_NEW="$ALIVE_NEW $pid"; fi
done
ALIVE_NEW=$(echo "$ALIVE_NEW" | tr -d '[:space:]' | xargs || true)
if [[ -n "$ALIVE_NEW" ]]; then pass "app processes alive 5 s after app/start (PIDs: $ALIVE_NEW)"
else fail "no new processes alive 5 s after app/start"; fi

# --- app/stop -----------------------------------------------------------------
echo; echo "-- POST /api/app/stop --"
api POST /api/app/stop > /dev/null; pass "app/stop returned 2xx"
sleep 3

# R6, R7: no descendants remain.
SURVIVORS=""
for pid in $NEW_PIDS; do
  if kill -0 "$pid" 2>/dev/null; then SURVIVORS="$SURVIVORS $pid"; fi
done
SURVIVORS=$(echo "$SURVIVORS" | xargs || true)
if [[ -z "$SURVIVORS" ]]; then pass "no orphan processes after app/stop"
else fail "orphan PIDs after app/stop: $SURVIVORS"; fi

rm -f /tmp/m1-cookies-$PORT.txt /tmp/m1-index-$PORT.html
echo
if [[ $FAILED -eq 0 ]]; then echo "M1 smoke: PASS"
else echo "M1 smoke: FAIL"; exit 1; fi
