# Error codes and remediation

REST errors return JSON `{ "error": { "code", "message" } }` unless noted.

## Authentication (401)

| Code | Cause | Fix |
|------|-------|-----|
| `UNAUTHORIZED` | Missing/invalid Bearer token | Send `Authorization: Bearer <key>` |
| (message) `missing authorization` | Empty Authorization header | Add Bearer token |
| (message) `invalid api key` | Key not in KeyStore for org | Rotate key, verify org mapping |
| (message) `invalid org id` | Malformed `X-Org-ID` | Use 1–64 alnum/`_`/`-` |

Org resolution: prefer `X-Org-ID`; fallback `X-Workspace-ID`; default `default` in dev.

## Client errors (400)

| Code | Cause | Fix |
|------|-------|-----|
| `INVALID_JSON` | Body parse failure | Validate JSON against OpenAPI schemas |

## Server errors (500)

| Code | Cause | Fix |
|------|-------|-----|
| `INTERNAL` | gRPC dial failure to control plane | Check `CONTROL_PLANE_ADDR`, network |
| `REGISTER_FAILED` | etcd lease/write error | Verify etcd health, quotas |
| `DISCOVER_FAILED` | Stream error | Check registry data, org ID |
| `ROUTE_FAILED` | No candidates or internal ranking error | Register agents, relax filters |
| `CIRCUIT_FAILED` | Redis read error | Check Redis connectivity |

## Circuit breaker behavior

When destination circuit is **open**, routing may exclude agent or return fallbacks. Clients see reduced `fallback_chain` or errors from data plane proxy.

| State | Meaning |
|-------|---------|
| `closed` | Normal traffic |
| `open` | Failures exceeded threshold in window |
| `half-open` | Probe allowed after `CIRCUIT_HALF_OPEN_SECONDS` |

Query: `GET /v1/circuit/{agent_id}`.

## Rate limiting

When Redis limiter denies request, gateway may return **429** (implementation-dependent). Back off exponentially; see [ops/02-RATE-LIMITS.md](../ops/02-RATE-LIMITS.md).

## GuardLoop

Timeout or 5xx from GuardLoop during `score-based` routing: engine falls back to latency/cost ranking—no distinct error code to client.

## TraceLens export failures

OTLP export errors are logged server-side; requests may still succeed. Verify `TRACELENS_JWT` and endpoint URL.

## etcd partition

Registration returns `REGISTER_FAILED`; discovery may return empty sets. Restore quorum before forcing writes.

## Debugging checklist

1. `GET /health` and `/ready` on control plane
2. Confirm headers: Bearer + org
3. Inspect circuit state for destination
4. ClickHouse `routes` for recent failures
5. TraceLens trace for span `route`

## gRPC status codes

Direct gRPC clients receive standard `codes.Unavailable`, `codes.InvalidArgument`, etc., mapped from internal errors—prefer REST doc codes when using gateway.
