#!/usr/bin/env bash
# Load/API smoke — health + authenticated route/discover against a running stack.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

CP="${CONTROL_PLANE_URL:-http://localhost:48080}"
DP="${DATAPLANE_URL:-http://localhost:48081}"
KEY="${AGENTMESH_DEV_API_KEY:-agentmesh-dev-key}"
HDR=(-H "Authorization: Bearer ${KEY}" -H "X-Org-ID: default" -H "Content-Type: application/json")

echo "Smoke CP health $CP/health"
curl -sf -m 5 "$CP/health" >/dev/null
echo "Smoke CP ready $CP/ready"
curl -sf -m 5 "$CP/ready" >/dev/null
echo "Smoke DP health $DP/health"
curl -sf -m 5 "$DP/health" >/dev/null

echo "Smoke discover"
curl -sf -m 15 "${HDR[@]}" "$CP/v1/agents/discover?limit=10" | grep -q agents

echo "Smoke route"
curl -sf -m 15 -X POST "$CP/v1/route" "${HDR[@]}" \
  -d '{"capability":"code_gen","source_agent_id":"load-smoke","routing_strategy":"least-latency"}' \
  | grep -q destination_agent_id

echo "Smoke dataplane advisory (auth)"
curl -sf -m 15 -X POST "$DP/v1/route" "${HDR[@]}" \
  -d '{"capability":"code_gen","source_agent_id":"load-smoke","org_id":"default"}' \
  | grep -q destination_agent_id

echo OK
