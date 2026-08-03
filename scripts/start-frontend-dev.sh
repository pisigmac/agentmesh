#!/usr/bin/env bash
# Run Next.js dashboard on the host (hot reload). Control plane must be reachable at CONTROL_PLANE_URL.
set -euo pipefail
source "$(dirname "$0")/common.sh"

FRONTEND_DIR="${REPO_ROOT}/frontend"
if [[ ! -d "${FRONTEND_DIR}/node_modules" ]]; then
  echo "Installing frontend dependencies..."
  npm ci --prefix "${FRONTEND_DIR}"
fi

export NEXT_PUBLIC_CONTROL_PLANE_URL="${NEXT_PUBLIC_CONTROL_PLANE_URL}"
echo "Dashboard dev server → http://localhost:${AM_PORT_FRONTEND}"
echo "API base (browser):   ${NEXT_PUBLIC_CONTROL_PLANE_URL}"
exec npm run dev --prefix "${FRONTEND_DIR}"
