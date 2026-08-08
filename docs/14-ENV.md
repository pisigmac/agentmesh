# Environment variables

Configuration is loaded from environment variables in `internal/config/config.go`. Process **listen** defaults remain `8080`/`9090`/`8081` inside containers.

**Local host access** (Docker Compose + `scripts/*`) uses **4-prefixed ports** — see [`deploy/ports.env`](../deploy/ports.env) and [15-DEPLOY.md](15-DEPLOY.md#ports). Scripts set `CONTROL_PLANE_URL`, `ETCD_ENDPOINTS`, etc. when using `start-local.sh`.

## Global

| Variable | Default | Description |
|----------|---------|-------------|
| `ENV` | `development` | Environment name. When `production`, forces `AGGREGATE_DEFAULT_ORG=false` (E21) |
| `AGGREGATE_DEFAULT_ORG` | `true` (non-prod) | When true, `ListByOrg("default")` unions all tenants. **Forced off** if `ENV=production` |

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
| `DATAPLANE_MAX_RETRIES` | `2` | Max retries after first attempt (`attempts = max_retries + 1`) |
| `DATAPLANE_FAILOVER` | `true` | On upstream 5xx/timeout, walk `fallback_chain` (records circuit failures per destination) |
| `DATAPLANE_ROUTE_CACHE_TTL_MS` | `500` | Route decision cache TTL |
| `DATAPLANE_AUTH_ENABLED` | `true` | Require Bearer JWT/API key on `/v1/route` and `/v1/proxy/*` (`/health` + `/metrics` stay open). Needs etcd + `AUTH_JWT_SECRET` / org keys |

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

## Auth (native JWT + org API keys)

| Variable | Default | Description |
|----------|---------|-------------|
| `AUTH_JWT_SECRET` | falls back to secrets key / dev string | HS256 secret (min 32 chars) |
| `AUTH_ALLOW_REGISTER` | `true` | Allow `POST /v1/auth/register` |
| `AUTH_TOKEN_TTL_HOURS` | `24` | JWT / session lifetime |
| `AGENTMESH_BOOTSTRAP_EMAIL` | `` | Optional seed admin email when no users exist |
| `AGENTMESH_BOOTSTRAP_PASSWORD` | `` | Optional seed admin password |
| `AGENTMESH_DEV_API_KEY` | `agentmesh-dev-key` | Seeded hashed API key for org `default` (scripts/CLI) |

Console uses Bearer JWT after login. Machines use Bearer org API keys (`amk_…`). Keymint is not the IdP.

## Model provider secrets (Keymint)

| Variable | Default | Description |
|----------|---------|-------------|
| `KEYMINT_BASE_URL` | `` | Keymint API base (required for Models/Chat) |
| `KEYMINT_API_KEY` | `` | Bearer token for Keymint protected APIs |
| `KEYMINT_TEAM_ID` | `` | Team UUID that owns AgentMesh virtual keys |
| `AGENTMESH_SECRETS_KEY` | falls back to `AGENTMESH_DEV_API_KEY` | AES-GCM key for encrypting Keymint virtual-key **bindings** in etcd (not provider plaintext) |
| `MODELS_CATALOG_PATH` | `deploy/models.yaml` (or `/etc/agentmesh/models.yaml` in containers) | Providers + models YAML catalog |

`PUT /v1/secrets/providers/{id}` creates a Keymint provider-key + virtual key; chat uses Keymint `POST /v1/proxy/chat/completions`. Catalog: [`deploy/models.yaml`](../deploy/models.yaml). See [23-MODEL-INFERENCE.md](23-MODEL-INFERENCE.md).

## Score-based routing

| Variable | Default | Description |
|----------|---------|-------------|
| `SCORE_PROVIDER` | `auto` | `auto` \| `noop` \| `fixture` \| `guardloop`. `auto` → GuardLoop when enabled, else deterministic **fixture** (demos/CI) |

When `routing_strategy=score-based`, the control plane blends provider score with success/latency/cost. If the provider returns an error, ranking **falls back to least-latency**. Contributions appear on `POST /v1/route` as `score` and headers `x-agentmesh-score-*`.

## GuardLoop (optional)

| Variable | Default | Description |
|----------|---------|-------------|
| `GUARDLOOP_ENABLED` | `false` | Prefer GuardLoop when `SCORE_PROVIDER=auto` |
| `GUARDLOOP_BASE_URL` | `` | API base (`GET {base}/scores?limit=100`) |
| `GUARDLOOP_API_KEY` | `` | Bearer token |
| `GUARDLOOP_TIMEOUT_MS` | `5000` | HTTP client timeout (failed fetch → least-latency fallback) |
| `GUARDLOOP_CACHE_TTL_SEC` | `60` | In-process score cache TTL per agent type |

Staging: set `GUARDLOOP_ENABLED=true`, `SCORE_PROVIDER=guardloop` (or `auto`), and sandbox URL/key. See [ops/08-STAGING.md](../ops/08-STAGING.md).

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

## Stripe metering + webhooks (optional)

| Variable | Default | Description |
|----------|---------|-------------|
| `STRIPE_ENABLED` | `false` | Post meter events after successful routes |
| `STRIPE_SECRET_KEY` | `` | `sk_test_…` / `sk_live_…` |
| `STRIPE_METER_EVENT_NAME` | `agentmesh_routes` | Stripe Billing Meter event name |
| `STRIPE_ORG_CUSTOMERS` | `` | `org_id:cus_xxx,org2:cus_yyy` map |
| `STRIPE_WEBHOOK_SECRET` | `` | `whsec_…` for `POST /v1/billing/stripe/webhook` |

Every successful control-plane route inserts ClickHouse `billing_events` (idempotent `stripe_event_id=route:{trace_id}`) and, when enabled + customer mapped, calls Stripe `POST /v1/billing/meter_events`. Webhooks set org billing status (`ok` / `past_due` / `blocked`) used for soft warn / hard reject on route. See [ops/01-PAYMENTS.md](../ops/01-PAYMENTS.md).

## Feature flags

Any `FF_<NAME>` env var read by `internal/featureflags` (values `1`, `true` enable).

## Helm

Chart: [`deploy/helm/agentmesh`](../deploy/helm/agentmesh). Single chart MVP (CP/DP/frontend) — multi-chart split remains a follow-up. Staging: `-f values.yaml -f values-staging.yaml`. Map Stripe via `stripe.secretKeyExistingSecret` (`secret_key`) and `stripe.webhookSecretExistingSecret` (`webhook_secret`).

## Secrets handling

Never commit `.env` files with production keys. Use Kubernetes Secrets for `TRACELENS_JWT`, `GUARDLOOP_API_KEY`, `STRIPE_SECRET_KEY`, Redis password, and ClickHouse credentials.
