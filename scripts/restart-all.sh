#!/usr/bin/env bash
# Restart AgentMesh stack (stop then start).
set -euo pipefail
DIR="$(dirname "$0")"
BUILD="${BUILD:-0}" REMOVE_VOLUMES=0 "${DIR}/stop-all.sh"
BUILD="${BUILD:-1}" "${DIR}/start-all.sh"
