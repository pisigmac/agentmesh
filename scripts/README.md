# AgentMesh utility scripts

Run from repo root or any directory (scripts resolve paths automatically).

| Script | Purpose |
|--------|---------|
| [`start-all.sh`](start-all.sh) | Docker Compose up (build + health wait) |
| [`stop-all.sh`](stop-all.sh) | Compose down (`REMOVE_VOLUMES=1` to drop volumes) |
| [`restart-all.sh`](restart-all.sh) | Stop then start |
| [`status-all.sh`](status-all.sh) | `docker compose ps` + HTTP health checks |
| [`logs-all.sh`](logs-all.sh) | `docker compose logs -f` (optional service names) |
| [`build-all.sh`](build-all.sh) | `make build` + compose image build |
| [`start-local.sh`](start-local.sh) | Infra in Docker; CP/DP from `bin/` |
| [`stop-local.sh`](stop-local.sh) | Stop local CP/DP processes |
| [`seed-policy.sh`](seed-policy.sh) | PUT example org policy to control plane |
| [`seed-all-agents.sh`](seed-all-agents.sh) | Register all nine adapter types under `default` org |
| [`seed-model-agents.sh`](seed-model-agents.sh) | Placeholder agents for `model_*` chat routing metadata |
| [`start-frontend-dev.sh`](start-frontend-dev.sh) | Next.js dev server on host (`43000`, no Docker) |

## Examples

```bash
./scripts/start-all.sh
./scripts/seed-policy.sh
./scripts/register-sample-agent.sh
./scripts/status-all.sh

# TraceLens profile (placeholder service in compose)
COMPOSE_PROFILES=tracelens ./scripts/start-all.sh

# Tear down including ClickHouse data
REMOVE_VOLUMES=1 ./scripts/stop-all.sh

# Local Go binaries (faster iteration)
./scripts/start-local.sh
./scripts/stop-local.sh
```

Environment overrides: see [`common.sh`](common.sh) and [`deploy/ports.env`](../deploy/ports.env) (`AGENTMESH_DEV_API_KEY`, `CONTROL_PLANE_URL`, `KEYMINT_*`, etc.).

**Agent leases:** non-persistent agents should use CLI `-keepalive` so they do not vanish after TTL; seed/catalog agents use `-persistent` (see `seed-all-agents.sh`).

**Manual QA:** full step-by-step guide in [`docs/21-MANUAL-TESTING.md`](../docs/21-MANUAL-TESTING.md).

**Default host ports (prefix 4):** CP HTTP `48080`, gRPC `49090`, dataplane `48081`, Prometheus `49091`, Grafana `43001`, dashboard `43000` (included in `start-all.sh` via Docker).

The dashboard image is built with `NEXT_PUBLIC_CONTROL_PLANE_URL` (default `http://localhost:48080`) so the browser calls the published control-plane port on the host. Override via `deploy/ports.env` or env before `docker compose build`.
