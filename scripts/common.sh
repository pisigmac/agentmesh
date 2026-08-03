#!/usr/bin/env bash
# Shared paths and defaults for AgentMesh utility scripts.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${REPO_ROOT}/deploy/docker-compose.yml"
COMPOSE=(docker compose -f "${COMPOSE_FILE}")

# Host ports (prefix 4) — override via deploy/ports.env or environment
if [[ -f "${REPO_ROOT}/deploy/ports.env" ]]; then
  # shellcheck disable=SC1091
  set -a && source "${REPO_ROOT}/deploy/ports.env" && set +a
fi

AM_PORT_ETCD="${AM_PORT_ETCD:-42379}"
AM_PORT_REDIS="${AM_PORT_REDIS:-46379}"
AM_PORT_CLICKHOUSE_HTTP="${AM_PORT_CLICKHOUSE_HTTP:-48123}"
AM_PORT_CLICKHOUSE_NATIVE="${AM_PORT_CLICKHOUSE_NATIVE:-49000}"
AM_PORT_CONTROL_PLANE_HTTP="${AM_PORT_CONTROL_PLANE_HTTP:-48080}"
AM_PORT_CONTROL_PLANE_GRPC="${AM_PORT_CONTROL_PLANE_GRPC:-49090}"
AM_PORT_DATAPLANE_HTTP="${AM_PORT_DATAPLANE_HTTP:-48081}"
AM_PORT_PROMETHEUS="${AM_PORT_PROMETHEUS:-49091}"
AM_PORT_GRAFANA="${AM_PORT_GRAFANA:-43001}"
AM_PORT_TRACELENS="${AM_PORT_TRACELENS:-48082}"
AM_PORT_FRONTEND="${AM_PORT_FRONTEND:-43300}"

export AGENTMESH_DEV_API_KEY="${AGENTMESH_DEV_API_KEY:-agentmesh-dev-key}"
export CONTROL_PLANE_URL="${CONTROL_PLANE_URL:-http://localhost:${AM_PORT_CONTROL_PLANE_HTTP}}"
export DATAPLANE_URL="${DATAPLANE_URL:-http://localhost:${AM_PORT_DATAPLANE_HTTP}}"
export GRAFANA_URL="${GRAFANA_URL:-http://localhost:${AM_PORT_GRAFANA}}"
export PROMETHEUS_URL="${PROMETHEUS_URL:-http://localhost:${AM_PORT_PROMETHEUS}}"
export FRONTEND_URL="${FRONTEND_URL:-http://localhost:${AM_PORT_FRONTEND}}"
export NEXT_PUBLIC_CONTROL_PLANE_URL="${NEXT_PUBLIC_CONTROL_PLANE_URL:-${CONTROL_PLANE_URL}}"

export COMPOSE_PROFILES="${COMPOSE_PROFILES:-}"

require_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    echo "error: docker not found" >&2
    exit 1
  fi
  if ! docker info >/dev/null 2>&1; then
    echo "error: docker daemon not running" >&2
    exit 1
  fi
}

print_endpoints() {
  cat <<EOF

AgentMesh stack endpoints (host ports use prefix 4):
  Control plane (REST):  ${CONTROL_PLANE_URL}
  Control plane (gRPC):  localhost:${AM_PORT_CONTROL_PLANE_GRPC}
  Data plane:            ${DATAPLANE_URL}
  Prometheus:            ${PROMETHEUS_URL}
  Grafana:               ${GRAFANA_URL}  (admin / admin)
  Dashboard (UI):        ${FRONTEND_URL}
  etcd:                  localhost:${AM_PORT_ETCD}
  Redis:                 localhost:${AM_PORT_REDIS}
  ClickHouse HTTP:       http://localhost:${AM_PORT_CLICKHOUSE_HTTP}

Dev API key: ${AGENTMESH_DEV_API_KEY}
  curl -H "Authorization: Bearer ${AGENTMESH_DEV_API_KEY}" -H "X-Org-ID: default" \\
    "${CONTROL_PLANE_URL}/v1/agents/discover?capability=code_gen"

EOF
}
