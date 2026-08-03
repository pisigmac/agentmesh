#!/usr/bin/env bash
# Show compose service status and HTTP health checks.
set -euo pipefail
source "$(dirname "$0")/common.sh"

require_docker

echo "=== Docker Compose ==="
"${COMPOSE[@]}" ps

check() {
  local name="$1" url="$2"
  if curl -sf "$url" >/dev/null 2>&1; then
    echo "OK  $name  $url"
  else
    echo "FAIL $name  $url"
  fi
}

echo ""
echo "=== Health checks ==="
check "control-plane" "${CONTROL_PLANE_URL}/health"
check "control-plane ready" "${CONTROL_PLANE_URL}/ready"
check "dataplane" "${DATAPLANE_URL}/health"
check "prometheus" "${PROMETHEUS_URL}/-/ready"
check "clickhouse" "http://localhost:${AM_PORT_CLICKHOUSE_HTTP}/ping"
check "frontend" "${FRONTEND_URL}"
