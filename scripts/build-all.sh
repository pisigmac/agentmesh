#!/usr/bin/env bash
# Build Go binaries and Docker images (without starting containers).
set -euo pipefail
source "$(dirname "$0")/common.sh"

echo "Building Go binaries..."
make -C "${REPO_ROOT}" build

require_docker
echo "Building Docker images..."
"${COMPOSE[@]}" build control-plane dataplane frontend

echo "Build complete. Run scripts/start-all.sh to start."
