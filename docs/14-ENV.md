# Environment variables

Configuration is loaded from environment variables in `internal/config/config.go`. Process **listen** defaults remain `8080`/`9090`/`8081` inside containers.

**Local host access** (Docker Compose + `scripts/*`) uses **4-prefixed ports** — see [`deploy/ports.env`](../deploy/ports.env) and [15-DEPLOY.md](15-DEPLOY.md#ports). Scripts set `CONTROL_PLANE_URL`, `ETCD_ENDPOINTS`, etc. when using `start-local.sh`.

## Global

| Variable | Default | Description |
|----------|---------|-------------|
| `ENV` | `development` | Environment name |

## Control plane

| Variable | Default | Description |
|----------|---------|-------------|
| `CONTROL_PLANE_GRPC_PORT` | `9090` | gRPC listen port |
| `CONTROL_PLANE_HTTP_PORT` | `8080` | REST gateway port |

## Data plane

| Variable | Default | Description |
|----------|---------|-------------|
| `DATAPLANE_MODE` | `gateway` | Operating mode |
| `DATAPLANE_HTTP_PORT` | `8081` | HTTP listen port |
| `CONTROL_PLANE_ADDR` | `localhost:49090` | gRPC upstream (host; use `control-plane:9090` inside compose) |
| `DATAPLANE_PROXY_TIMEOUT_MS` | `30000` | Upstream proxy timeout |
| `DATAPLANE_MAX_RETRIES` | `2` | Retry count |
| `DATAPLANE_ROUTE_CACHE_TTL_MS` | `500` | Route decision cache TTL |

## etcd

| Variable | Default | Description |
|----------|---------|-------------|
| `ETCD_ENDPOINTS` | `localhost:42379` | Comma-separated endpoints (host) |
| `ETCD_PREFIX` | `/agentmesh` | Key prefix |

## Redis

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_ADDR` | `localhost:46379` | Redis address (host) |
| `REDIS_PASSWORD` | `` | Password |
| `REDIS_DB` | `0` | DB index |

## ClickHouse

| Variable | Default | Description |
|----------|---------|-------------|
| `CLICKHOUSE_ADDR` | `localhost:49000` | Native protocol addr (host) |
| `CLICKHOUSE_DATABASE` | `agentmesh` | Database name |
| `CLICKHOUSE_USER` | `default` | Username |
| `CLICKHOUSE_PASSWORD` | `` | Password |

## Auth

| Variable | Default | Description |
|----------|---------|-------------|
| `AUTH_MODE` | `api_key` | `api_key` or `clerk` |

Development uses in-memory key `agentmesh-dev-key` for org `default` when wired in main.

## Model provider secrets (Keymint)

| Variable | Default | Description |
|----------|---------|-------------|
| `KEYMINT_BASE_URL` | `` | Keymint API base (required for Models/Chat) |
| `KEYMINT_API_KEY` | `` | Bearer token for Keymint protected APIs |
| `KEYMINT_TEAM_ID` | `` | Team UUID that owns AgentMesh virtual keys |
| `AGENTMESH_SECRETS_KEY` | falls back to `AGENTMESH_DEV_API_KEY` | AES-GCM key for encrypting Keymint virtual-key **bindings** in etcd (not provider plaintext) |
| `MODELS_CATALOG_PATH` | `deploy/models.yaml` (or `/etc/agentmesh/models.yaml` in containers) | Providers + models YAML catalog |

`PUT /v1/secrets/providers/{id}` creates a Keymint provider-key + virtual key; chat uses Keymint `POST /v1/proxy/chat/completions`. Catalog: [`deploy/models.yaml`](../deploy/models.yaml). See [23-MODEL-INFERENCE.md](23-MODEL-INFERENCE.md).

## GuardLoop (optional)

| Variable | Default | Description |
|----------|---------|-------------|
| `GUARDLOOP_ENABLED` | `false` | Enable score provider |
| `GUARDLOOP_BASE_URL` | `` | API base |
| `GUARDLOOP_API_KEY` | `` | API key |
| `GUARDLOOP_TIMEOUT_MS` | `5000` | Request timeout |
| `GUARDLOOP_CACHE_TTL_SEC` | `60` | Score cache TTL |

## TraceLens

| Variable | Default | Description |
|----------|---------|-------------|
| `TRACELENS_ENABLED` | `true` | Enable OTLP export |
| `TRACELENS_OTLP_ENDPOINT` | `http://localhost:48082/v1/traces` | OTLP HTTP endpoint (TraceLens host port) |
| `TRACELENS_JWT` | `` | Bearer JWT for TraceLens |

## Circuit breaker

| Variable | Default | Description |
|----------|---------|-------------|
| `CIRCUIT_FAILURE_THRESHOLD` | `5` | Failures to open |
| `CIRCUIT_WINDOW_SECONDS` | `60` | Sliding window |
| `CIRCUIT_HALF_OPEN_SECONDS` | `30` | Time until half-open probe |

## Feature flags

Any `FF_<NAME>` env var read by `internal/featureflags` (values `1`, `true` enable).

## Helm

Chart values in `deploy/helm/agentmesh/values.yaml` map to the same settings via container `env` blocks—override per environment with `-f` values files.

## Secrets handling

Never commit `.env` files with production keys. Use Kubernetes Secrets for `TRACELENS_JWT`, `GUARDLOOP_API_KEY`, Redis password, and ClickHouse credentials.
