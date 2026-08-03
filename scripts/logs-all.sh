#!/usr/bin/env bash
# Follow logs from all compose services (or pass service names as args).
set -euo pipefail
source "$(dirname "$0")/common.sh"

require_docker

if [[ $# -gt 0 ]]; then
  "${COMPOSE[@]}" logs -f "$@"
else
  "${COMPOSE[@]}" logs -f
fi
