#!/usr/bin/env bash
# Start full AgentMesh stack (Docker Compose: infra + control plane + dataplane + observability).
set -euo pipefail
source "$(dirname "$0")/common.sh"

require_docker

BUILD="${BUILD:-1}"
if [[ "${BUILD}" == "1" ]]; then
  echo "Building and starting services..."
  "${COMPOSE[@]}" up -d --build
else
  echo "Starting services (no rebuild)..."
  "${COMPOSE[@]}" up -d
fi

echo "Waiting for control plane health..."
for i in $(seq 1 60); do
  if curl -sf "${CONTROL_PLANE_URL}/health" >/dev/null 2>&1; then
    echo "Control plane is up."
    break
  fi
  if [[ "$i" -eq 60 ]]; then
    echo "warning: control plane health check timed out (stack may still be starting)" >&2
  fi
  sleep 2
done

echo "Waiting for dashboard..."
for i in $(seq 1 30); do
  if curl -sf "${FRONTEND_URL}" >/dev/null 2>&1; then
    echo "Dashboard is up."
    break
  fi
  if [[ "$i" -eq 30 ]]; then
    echo "warning: dashboard health check timed out (frontend may still be starting)" >&2
  fi
  sleep 2
done

print_endpoints
