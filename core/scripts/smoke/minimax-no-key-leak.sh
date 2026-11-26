#!/usr/bin/env bash
# MiniMax Cloud TTS (Go): no route returns the key, and no file but platforms/minimax/key.json holds it.
# Usage: minimax-no-key-leak.sh <front URL, e.g. http://127.0.0.1:3000> <install root>
# Saves a sentinel key (host: a closed port on this machine, so nothing leaves it), calls every
# minimax route and the pages that list generations, greps every answer and every file under the
# install root for the sentinel, then removes the key and restores the default host. Exit 1 on any hit.
# It does not call the real MiniMax service and needs no real key.
set -u
FRONT="${1:?front URL}"; ROOT="${2:?install root}"
KEY="mm-LEAKCHECK-$(date +%s)-$RANDOM"
J='Content-Type: application/json'
fail=0; out="$(mktemp)"; trap 'rm -f "$out"' EXIT
call() { # method path [body]; writes "method path -> [status] body" on one line
  local resp code
  if [ $# -ge 3 ]; then resp="$(curl -s -X "$1" -H "$J" -d "$3" -w '\n%{http_code}' "$FRONT$2")"
  else resp="$(curl -s -X "$1" -w '\n%{http_code}' "$FRONT$2")"; fi
  code="${resp##*$'\n'}"
  printf '%s %s -> [%s] %s\n' "$1" "$2" "$code" "$(printf '%s' "${resp%$'\n'*}" | tr '\n' ' ')" >>"$out"
}
S=/api/go-data/minimax
call PUT $S/settings "{\"base_url\":\"http://127.0.0.1:1\",\"key\":\"$KEY\"}"
call GET $S/settings
call POST $S/test '{}'
call POST $S/speech '{"text":"x"}'
call POST $S/speech "{\"text\":\"x\",\"key\":\"$KEY\"}"
call PUT $S/settings "{\"base_url\":\"ftp://x\",\"key\":\"$KEY\"}"
call PUT $S/settings "{\"base_url\":\"http://example.com\",\"key\":\"$KEY\"}"
call PUT $S/settings "{\"unknown\":1,\"key\":\"$KEY\"}"
call GET /api/go-data/outputs/outputs
call GET /api/go-data/host-status
call GET /minimax
# Cleanup: remove the key and go back to the default host.
call PUT $S/settings '{"key":""}'
call GET $S/settings
cat "$out"
if ! grep -q 'has_key":true' "$out"; then echo "FAIL: the key was not stored (is the app up at $FRONT, and are you on the machine it runs on?)"; fail=1; fi
if ! grep -q 'PUT /api/go-data/minimax/settings -> \[200\]' "$out"; then echo "FAIL: no settings change was accepted"; fail=1; fi
if grep -q "$KEY" "$out"; then echo "FAIL: a route returned the key"; fail=1; fi
# The key was removed at the end, so no file may hold it any more.
hits="$(grep -rl --exclude-dir=node_modules --exclude-dir=.git "$KEY" "$ROOT" 2>/dev/null || true)"
if [ -n "$hits" ]; then echo "FAIL: key left in: $hits"; fail=1; fi
[ $fail = 0 ] && echo "PASS: no answer and no file holds the key"
exit $fail
