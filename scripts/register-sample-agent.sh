#!/usr/bin/env bash
# Register a sample cursor agent (requires control plane up).
set -euo pipefail
source "$(dirname "$0")/common.sh"

make -C "${REPO_ROOT}" build 2>/dev/null || true

"${REPO_ROOT}/bin/agentmesh" \
  -base "${CONTROL_PLANE_URL}" \
  -key "${AGENTMESH_DEV_API_KEY}" \
  -org "${ORG:-default}" \
  -type "${AGENT_TYPE:-cursor}" \
  -id "${AGENT_ID:-cursor-1}" \
  -endpoint "${AGENT_ENDPOINT:-http://127.0.0.1:9999}" \
  -persistent
