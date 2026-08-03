#!/usr/bin/env bash
# Load smoke test — run against dataplane with mocked upstream.
set -euo pipefail
TARGET="${TARGET:-http://localhost:48081/health}"
echo "Smoke GET $TARGET"
curl -sf "$TARGET" >/dev/null
echo OK
