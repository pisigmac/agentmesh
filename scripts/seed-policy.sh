#!/usr/bin/env bash
# Apply example org policy to running control plane.
set -euo pipefail
source "$(dirname "$0")/common.sh"

ORG="${ORG:-default}"
POLICY_FILE="${POLICY_FILE:-${REPO_ROOT}/deploy/examples/org-policy-default.json}"

if [[ ! -f "${POLICY_FILE}" ]]; then
  echo "error: policy file not found: ${POLICY_FILE}" >&2
  exit 1
fi

curl -sf -X PUT \
  -H "Authorization: Bearer ${AGENTMESH_DEV_API_KEY}" \
  -H "X-Org-ID: ${ORG}" \
  -H "Content-Type: application/json" \
  -d @"${POLICY_FILE}" \
  "${CONTROL_PLANE_URL}/v1/policy"

echo ""
echo "Policy applied for org=${ORG}"
