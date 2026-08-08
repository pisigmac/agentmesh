#!/usr/bin/env bash
# Start infra only (etcd, redis, clickhouse, prom, grafana) and run CP/DP from local bin/.
set -euo pipefail
source "$(dirname "$0")/common.sh"

require_docker

echo "Starting infrastructure services..."
"${COMPOSE[@]}" up -d etcd redis clickhouse prometheus grafana

make -C "${REPO_ROOT}" build

export ETCD_ENDPOINTS="${ETCD_ENDPOINTS:-localhost:${AM_PORT_ETCD}}"
export REDIS_ADDR="${REDIS_ADDR:-localhost:${AM_PORT_REDIS}}"
export CLICKHOUSE_ADDR="${CLICKHOUSE_ADDR:-localhost:${AM_PORT_CLICKHOUSE_NATIVE}}"
export CLICKHOUSE_DATABASE="${CLICKHOUSE_DATABASE:-agentmesh}"
export AGENTMESH_DEV_API_KEY="${AGENTMESH_DEV_API_KEY}"
export AUTH_JWT_SECRET="${AUTH_JWT_SECRET:-agentmesh-dev-jwt-secret-change-me!!}"
export AGGREGATE_DEFAULT_ORG="${AGGREGATE_DEFAULT_ORG:-true}"
export DATAPLANE_AUTH_ENABLED="${DATAPLANE_AUTH_ENABLED:-true}"
export CONTROL_PLANE_GRPC_PORT="${CONTROL_PLANE_GRPC_PORT:-${AM_PORT_CONTROL_PLANE_GRPC}}"
export CONTROL_PLANE_HTTP_PORT="${CONTROL_PLANE_HTTP_PORT:-${AM_PORT_CONTROL_PLANE_HTTP}}"
export DATAPLANE_HTTP_PORT="${DATAPLANE_HTTP_PORT:-${AM_PORT_DATAPLANE_HTTP}}"
export CONTROL_PLANE_ADDR="${CONTROL_PLANE_ADDR:-localhost:${AM_PORT_CONTROL_PLANE_GRPC}}"
export TRACELENS_ENABLED="${TRACELENS_ENABLED:-false}"

PID_DIR="${REPO_ROOT}/.run"
mkdir -p "${PID_DIR}"

if [[ -f "${PID_DIR}/control-plane.pid" ]] && kill -0 "$(cat "${PID_DIR}/control-plane.pid")" 2>/dev/null; then
  echo "control-plane already running (pid $(cat "${PID_DIR}/control-plane.pid"))"
else
  echo "Starting control-plane..."
  nohup "${REPO_ROOT}/bin/control-plane" >"${PID_DIR}/control-plane.log" 2>&1 &
  echo $! >"${PID_DIR}/control-plane.pid"
fi

sleep 1

if [[ -f "${PID_DIR}/dataplane.pid" ]] && kill -0 "$(cat "${PID_DIR}/dataplane.pid")" 2>/dev/null; then
  echo "dataplane already running (pid $(cat "${PID_DIR}/dataplane.pid"))"
else
  echo "Starting dataplane..."
  nohup "${REPO_ROOT}/bin/dataplane" >"${PID_DIR}/dataplane.log" 2>&1 &
  echo $! >"${PID_DIR}/dataplane.pid"
fi

print_endpoints
echo "Logs: ${PID_DIR}/control-plane.log ${PID_DIR}/dataplane.log"
echo "Stop local processes: scripts/stop-local.sh"
