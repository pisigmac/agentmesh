#!/usr/bin/env bash
# Stop AgentMesh Docker Compose stack (containers only; volumes preserved).
set -euo pipefail
source "$(dirname "$0")/common.sh"

require_docker

REMOVE_VOLUMES="${REMOVE_VOLUMES:-0}"

if [[ "${REMOVE_VOLUMES}" == "1" ]]; then
  echo "Stopping stack and removing volumes..."
  "${COMPOSE[@]}" down -v
else
  echo "Stopping stack..."
  "${COMPOSE[@]}" down
fi

echo "Done."
