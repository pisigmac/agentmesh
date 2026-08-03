# Rate limits

Rate limiting uses Redis counters in `internal/ratelimit/limiter.go`.

## Key format

```
{prefix}:rl:{org_id}:{agent_id}
```

Default prefix aligns with Redis circuit keys under agentmesh configuration.

## Algorithm

1. `INCR` key
2. On first increment, `EXPIRE` with window duration
3. Allow if count `<= limit`

## Default limits (production baseline)

| Scope | Limit | Window | Notes |
|-------|-------|--------|-------|
| Starter org | 10 req | 1s | Trial tier |
| Growth org | 100 req | 1s | Per org aggregate |
| Per agent | 50 req | 1s | Prevents single agent starvation |
| Register | 5 req | 60s | Anti-abuse on `/v1/agents/register` |

Tune via configuration in deployment values—not hard-coded in OSS defaults; operators set limits in control plane wiring.

## HTTP response

Clients should expect **429 Too Many Requests** with `Retry-After` when limited (gateway middleware).

## Headers (recommended)

| Header | Purpose |
|--------|---------|
| `X-RateLimit-Limit` | Configured limit |
| `X-RateLimit-Remaining` | Remaining in window |
| `X-RateLimit-Reset` | Unix reset time |

## Bypass

Internal break-glass keys tagged in KeyStore with `kid=breakglass` may skip limits—audit all usage.

## Redis failure

If Redis is unavailable, prefer **fail closed** (deny) for mutating routes; read-only discover may fail open per incident policy.

## Monitoring

Alert on sustained 429 rate per org—see `ops/monitoring/prometheus-rules.yml`.

## Testing

Integration test: burst N+1 requests, assert 429 on final request.

## Coordination with circuit breaker

Rate limits protect the mesh; circuits protect unhealthy agents. Both use Redis—size cluster for combined QPS.
