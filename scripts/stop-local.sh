#!/usr/bin/env bash
# Stop locally run control-plane / dataplane (from start-local.sh).
set -euo pipefail
source "$(dirname "$0")/common.sh"

PID_DIR="${REPO_ROOT}/.run"

stop_pid() {
  local name="$1" file="${PID_DIR}/${name}.pid"
  if [[ -f "$file" ]]; then
    local pid
    pid="$(cat "$file")"
    if kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      echo "Stopped ${name} (pid ${pid})"
    fi
    rm -f "$file"
  fi
}

stop_pid control-plane
stop_pid dataplane
echo "Done."
