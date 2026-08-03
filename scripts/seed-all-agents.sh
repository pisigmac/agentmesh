#!/usr/bin/env bash
# Register one agent per adapter type under org default (requires control plane up).
set -euo pipefail
source "$(dirname "$0")/common.sh"

make -C "${REPO_ROOT}" build 2>/dev/null || true

ORG="${ORG:-default}"
BASE_PORT="${BASE_PORT:-9990}"

types=(
  "cursor"
  "claude_code"
  "github_copilot"
  "openai_codex"
  "aider"
  "continue_dev"
  "windsurf"
  "devin"
  "custom"
)

port="${BASE_PORT}"
for t in "${types[@]}"; do
  id="${t//_/-}-demo"
  endpoint="http://127.0.0.1:${port}"
  echo "Registering ${ORG}/${id} (${t}) -> ${endpoint}"
  "${REPO_ROOT}/bin/agentmesh" \
    -base "${CONTROL_PLANE_URL}" \
    -key "${AGENTMESH_DEV_API_KEY}" \
    -org "${ORG}" \
    -type "${t}" \
    -id "${id}" \
    -endpoint "${endpoint}" \
    -persistent
  port=$((port + 1))
done

echo "Done. Discover with org=${ORG} (default org includes all orgs in the mesh)."
