# REST API

The control plane exposes JSON REST on `CONTROL_PLANE_HTTP_PORT` ( **8080** inside containers; **48080** on host via Compose). All `/v1/*` routes require authentication.

## Authentication

| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes | `Bearer <api_key>` |
| `X-Org-ID` | Recommended | Tenant identifier (1–64 chars, alphanumeric, `-`, `_`) |
| `X-Workspace-ID` | Optional | Alias for org when `X-Org-ID` omitted |

Invalid or missing credentials return **401** with:

```json
{"error":{"code":"UNAUTHORIZED","message":"..."}}
```

## Health (unauthenticated)

- `GET /health` — liveness
- `GET /ready` — readiness

## POST /v1/agents/register

Creates or updates an agent lease in etcd.

**Body**: `AgentRegistration` (JSON field names match proto json tags: `org_id`, `agent_id`, `agent_type`, etc.)

**Response**: **201** `RegistrationAck` with `agent_id`, `lease_ttl_seconds`, `message`.

## GET /v1/agents/discover

Query parameters:

- `capability` — filter by capability tag
- `routing_strategy` — e.g. `least-latency`, `score-based`
- `max_cost` — float USD ceiling

**Response**: **200** `{ "agents": [ AgentEndpoint, ... ] }`

## POST /v1/route

For a full walkthrough (capabilities → route → send the question to the agent), see **[22-ROUTING-QUESTIONS.md](22-ROUTING-QUESTIONS.md)**.

**Body**: `RoutePayload`

| Field | Description |
|-------|-------------|
| `capability` | Required capability to satisfy |
| `routing_strategy` | Strategy name (`pin` when hard-pinned) |
| `source_agent_id` | Calling agent (for analytics) |
| `fallback_agent_types` | Ordered type fallbacks |
| `destination_agent_id` | Optional hard pin to this agent |
| `prefer_agent_ids` | Soft preference (eligible agents moved to front) |
| `force_pin` | Allow pin if unhealthy / missing capability |
| `pin_failover` | If pinned circuit is open, fall back to normal ranking |
| `headers` | Hints forwarded in routing decision |

**Response**: **200** `RouteResponse` including `destination_agent_id`, `endpoint_url`, `injected_headers` (includes `x-agentmesh-pin=true` when hard-pinned), `fallback_chain`, `timeout_ms`, `max_retries`, `trace_context`.

## GET /v1/agents/discover

Query params: `capability`, `routing_strategy`, `max_cost`, `limit`, `org_id`, `include_unhealthy=1` (catalog UI; includes unhealthy agents with `healthy` / `persistent` flags).

## POST /v1/agents/health

**Body**: `{ "agent_id", "healthy", "message?" }` — updates registry health for the authenticated org.

## POST /v1/chat and POST /v1/chat/stream

Inference via Keymint proxy. See [23-MODEL-INFERENCE.md](23-MODEL-INFERENCE.md). Stream uses SSE (`meta` / `token` / `done` / `error`).

Chat body pin fields (passed to `RouteRequest` when `route_via_mesh` is true): `destination_agent_id`, `prefer_agent_ids`, `force_pin`, `pin_failover`, plus `source_agent_id`. OpenAPI schema: `ChatRequest`.

## GET /v1/circuit/{agent_id}

Returns circuit breaker state for destination agent in the authenticated org.

**Response**: **200** `CircuitState` — `state` (`closed` | `open` | `half-open`), `failure_count`, timestamps, threshold, window.

## gRPC equivalent

Clients may call `agentmesh.v1.AgentMesh` on port **9090** (internal) or **49090** (host) for `RegisterAgent`, `DiscoverAgents` (stream), `RouteRequest`, `ReportHealth`, `GetCircuitState`.

## Data plane

The data plane ( **8081** internal, **48081** host) proxies executed requests to selected endpoints and emits TraceLens spans. Configure via `DATAPLANE_MODE=gateway` and `CONTROL_PLANE_ADDR`.

## Error shape

Application errors use:

```json
{"error":{"code":"REGISTER_FAILED","message":"..."}}
```

See [19-ERRORS.md](19-ERRORS.md) for codes and remediation.

## OpenAPI

Machine-readable spec: [openapi.yml](openapi.yml) and [06-OPENAPI.md](06-OPENAPI.md).
