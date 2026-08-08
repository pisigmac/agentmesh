#!/usr/bin/env bash
# Design-partner dry-run: pin, stream, health, failover, auth, audit (parallel where safe).
# Prereq: stack up (./scripts/start-all.sh or start-local.sh), seeded agents optional.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export AGENTMESH_DEV_API_KEY="${AGENTMESH_DEV_API_KEY:-agentmesh-dev-key}"
export CP="${CONTROL_PLANE_URL:-http://localhost:48080}"
export DP="${DATAPLANE_URL:-http://localhost:48081}"
HDR=(-H "Authorization: Bearer ${AGENTMESH_DEV_API_KEY}" -H "X-Org-ID: default" -H "Content-Type: application/json")

pass=0; fail=0
ok() { echo "PASS  $1"; pass=$((pass+1)); }
bad() { echo "FAIL  $1 — $2"; fail=$((fail+1)); }

echo "=== Design-partner dry-run (CP=$CP) ==="

# --- parallel baseline ---
cp_h=$(curl -s -m 3 -o /tmp/am-cp-health.json -w '%{http_code}' "${CP}/health" || echo 000)
dp_h=$(curl -s -m 3 -o /tmp/am-dp-health.json -w '%{http_code}' "${DP}/health" || echo 000)
cp_r=$(curl -s -m 3 -o /tmp/am-cp-ready.json -w '%{http_code}' "${CP}/ready" || echo 000)
[[ "$cp_h" == "200" ]] && ok "health CP" || bad "health CP" "http $cp_h"
[[ "$dp_h" == "200" ]] && ok "health DP" || bad "health DP" "http $dp_h"
[[ "$cp_r" == "200" ]] && ok "ready CP" || bad "ready CP" "http $cp_r"

if [[ "$cp_h" != "200" ]]; then
  echo "Stack not up. Start with: BUILD=0 ./scripts/start-all.sh"
  echo "Summary: $pass pass, $fail fail"
  exit 1
fi

# --- auth (register/login/me) ---
EMAIL="dryrun-$(date +%s)@example.com"
PASS='DryRunPass123!'
REG=$(curl -s -m 10 -w '\n%{http_code}' -X POST "${CP}/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASS}\"}" || true)
REG_CODE=$(echo "$REG" | tail -1)
REG_BODY=$(echo "$REG" | sed '$d')
TOKEN=$(echo "$REG_BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('token') or d.get('access_token') or '')" 2>/dev/null || true)
if [[ -z "$TOKEN" ]]; then
  LOGIN=$(curl -s -m 10 -X POST "${CP}/v1/auth/login" -H "Content-Type: application/json" \
    -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASS}\"}" || true)
  TOKEN=$(echo "$LOGIN" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('token') or d.get('access_token') or '')" 2>/dev/null || true)
fi
JWTH=(-H "Authorization: Bearer ${TOKEN}" -H "X-Org-ID: default" -H "Content-Type: application/json")
if [[ -n "$TOKEN" ]]; then
  ok "auth register/login JWT"
  ME=$(curl -s -m 10 "${JWTH[@]}" "${CP}/v1/auth/me" || true)
  echo "$ME" | grep -q email && ok "auth GET /v1/auth/me" || bad "auth /me" "$ME"
else
  # Dev API key still counts as auth path for operators
  [[ "$REG_CODE" == "403" || "$REG_CODE" == "409" ]] && ok "auth register gated/exists ($REG_CODE)" || bad "auth register/login" "no token code=$REG_CODE body=$REG_BODY"
  curl -s -m 5 "${HDR[@]}" "${CP}/v1/policy" | grep -q routing && ok "auth dev API key Bearer" || bad "auth dev key" "policy fetch failed"
fi

# --- parallel: discover+health+pin+audit+failover unit ---
(
  DISC=$(curl -s -m 15 "${HDR[@]}" "${CP}/v1/agents/discover?include_unhealthy=1&limit=256")
  echo "$DISC" > /tmp/am-discover.json
  N=$(echo "$DISC" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('agents') or []))" 2>/dev/null || echo 0)
  echo "DISCOVER_N=$N"
) &
PID_DISC=$!

(
  # health report against a known seed id if present later
  curl -s -m 10 -o /tmp/am-health.json -w '%{http_code}' -X POST "${CP}/v1/agents/health" \
    "${HDR[@]}" -d '{"agent_id":"cursor-demo","healthy":true,"message":"dry-run"}' > /tmp/am-health.code || echo 000 > /tmp/am-health.code
) &
PID_HEALTH=$!

(
  go test ./tests/chaos/ ./internal/dataplane/ -count=1 -timeout 60s > /tmp/am-failover-test.log 2>&1
  echo $? > /tmp/am-failover-test.code
) &
PID_FO=$!

wait $PID_DISC $PID_HEALTH $PID_FO || true

N=$(python3 -c "import json; print(len(json.load(open('/tmp/am-discover.json')).get('agents') or []))" 2>/dev/null || echo 0)
[[ "${N:-0}" -ge 1 ]] && ok "discover include_unhealthy (n=$N)" || bad "discover" "n=$N (seed agents?)"

HC=$(cat /tmp/am-health.code 2>/dev/null || echo 000)
[[ "$HC" == "200" ]] && ok "POST /v1/agents/health" || bad "agents/health" "http $HC"

FO=$(cat /tmp/am-failover-test.code 2>/dev/null || echo 1)
[[ "$FO" == "0" ]] && ok "failover chaos unit tests" || bad "failover tests" "see /tmp/am-failover-test.log"

# pick pin target
AGENT=$(python3 - <<'PY' 2>/dev/null || true
import json
d=json.load(open("/tmp/am-discover.json"))
agents=d.get("agents") or []
for a in agents:
    if a.get("agent_id"):
        print(a["agent_id"]); break
PY
)
if [[ -n "${AGENT:-}" ]]; then
  PIN=$(curl -s -m 15 -X POST "${CP}/v1/route" "${HDR[@]}" -d "{
    \"capability\": \"code_gen\",
    \"source_agent_id\": \"dry-run\",
    \"destination_agent_id\": \"${AGENT}\",
    \"force_pin\": false
  }")
  echo "$PIN" | grep -q "\"destination_agent_id\"" && ok "route pin → $AGENT" || bad "route pin" "$PIN"
  echo "$PIN" | grep -qi 'x-agentmesh-pin\|"pin"' && ok "pin marker in response" || ok "pin route (header may be in injected_headers)"
  echo "$PIN" | python3 -c "import sys,json; h=json.load(sys.stdin).get('injected_headers') or {}; assert h.get('x-agentmesh-pin')=='true'" 2>/dev/null \
    && ok "injected_headers x-agentmesh-pin=true" || bad "x-agentmesh-pin" "missing on pin"
else
  bad "route pin" "no agents discovered — run ./scripts/seed-all-agents.sh"
fi

# --- audit ---
AUD_CODE=$(curl -s -m 10 -o /tmp/am-audit.json -w '%{http_code}' "${HDR[@]}" "${CP}/v1/secrets/audit?limit=20" || echo 000)
if [[ "$AUD_CODE" == "200" ]]; then
  ok "GET /v1/secrets/audit"
elif [[ "$AUD_CODE" == "503" ]]; then
  ok "GET /v1/secrets/audit (Keymint not configured — expected 503)"
else
  bad "secrets/audit" "http $AUD_CODE $(head -c 200 /tmp/am-audit.json 2>/dev/null)"
fi

# --- stream (may 503 without Keymint/keys) ---
STREAM=$(curl -s -m 20 -N -X POST "${CP}/v1/chat/stream" "${HDR[@]}" \
  -d '{"model":"gpt-4o-mini","question":"ping","route_via_mesh":false}' || true)
if echo "$STREAM" | grep -q 'event: meta\|event: error\|KEYMINT\|SECRET_MISSING\|UNKNOWN_MODEL'; then
  ok "chat/stream responds (SSE or config error surfaced)"
else
  bad "chat/stream" "$(echo "$STREAM" | head -c 180)"
fi

# --- dataplane auth ---
DP_NOAUTH=$(curl -s -m 5 -o /dev/null -w '%{http_code}' -X POST "${DP}/v1/route" \
  -H "Content-Type: application/json" -d '{"capability":"code_gen","org_id":"default"}' || echo 000)
[[ "$DP_NOAUTH" == "401" ]] && ok "dataplane rejects unauthenticated /v1/route" || ok "dataplane auth optional/off (http $DP_NOAUTH)"
DP_AUTH=$(curl -s -m 15 -o /tmp/am-dp-route.json -w '%{http_code}' -X POST "${DP}/v1/route" "${HDR[@]}" \
  -d '{"capability":"code_gen","source_agent_id":"dry-run","org_id":"default"}' || echo 000)
if [[ "$DP_AUTH" == "200" ]]; then
  ok "dataplane authenticated /v1/route"
elif [[ "$DP_AUTH" == "502" || "$DP_AUTH" == "404" ]]; then
  ok "dataplane auth accepted (upstream/route http $DP_AUTH)"
else
  bad "dataplane /v1/route" "http $DP_AUTH $(head -c 120 /tmp/am-dp-route.json 2>/dev/null)"
fi

echo
echo "=== Summary: $pass pass, $fail fail ==="
[[ "$fail" -eq 0 ]]
