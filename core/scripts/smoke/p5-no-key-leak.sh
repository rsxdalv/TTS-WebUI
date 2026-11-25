#!/usr/bin/env bash
# P5: no route returns an endpoint's key, and no file but platforms/openai/keys.json holds it.
# Usage: p5-no-key-leak.sh <front URL, e.g. http://127.0.0.1:3000> <install root>
# Stores an endpoint "leakcheck" with a sentinel key (pointing at a closed port), calls every
# openai route and the pages that list generations, greps every answer and every file under
# the install root for the sentinel, then deletes the endpoint. Exit 1 on any hit.
set -u
FRONT="${1:?front URL}"; ROOT="${2:?install root}"
KEY="sk-LEAKCHECK-$(date +%s)-$RANDOM"
J='Content-Type: application/json'
fail=0; out="$(mktemp)"; trap 'rm -f "$out"' EXIT
call() { # method path [body]
  printf '%s %s -> ' "$1" "$2" >>"$out"
  if [ $# -ge 3 ]; then curl -s -X "$1" -H "$J" -d "$3" -w ' [%{http_code}]\n' "$FRONT$2" >>"$out"
  else curl -s -X "$1" -w ' [%{http_code}]\n' "$FRONT$2" >>"$out"; fi
}
call PUT /api/go-data/openai/endpoints/leakcheck "{\"url\":\"http://127.0.0.1:1/v1\",\"model\":\"m\",\"voice\":\"v\",\"key\":\"$KEY\"}"
call GET /api/go-data/openai/endpoints
call POST /api/go-data/openai/endpoints/leakcheck/test '{}'
call POST /api/go-data/openai/speech '{"endpoint":"leakcheck","input":"x"}'
call PUT /api/go-data/openai/endpoints/leakcheck "{\"url\":\"ftp://x\",\"key\":\"$KEY\"}"
call PUT /api/go-data/openai/endpoints/leakcheck "{\"url\":\"http://example.com/v1\",\"key\":\"$KEY\"}"
call GET /api/go-data/outputs/outputs
call GET /api/go-data/host-status
call GET /openai
call DELETE /api/go-data/openai/endpoints/leakcheck
cat "$out"
if grep -q "$KEY" "$out"; then echo "FAIL: a route returned the key"; fail=1; fi
# While the endpoint existed the key was in keys.json; it is deleted now, so no file may hold it.
hits="$(grep -rl --exclude-dir=node_modules --exclude-dir=.git "$KEY" "$ROOT" 2>/dev/null || true)"
if [ -n "$hits" ]; then echo "FAIL: key left in: $hits"; fail=1; fi
[ $fail = 0 ] && echo "PASS: no answer and no file holds the key"
exit $fail
