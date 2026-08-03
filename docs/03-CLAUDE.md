# Claude Code integration

AgentMesh treats **Claude Code** as first-class adapter type `claude_code` with GuardLoop-aligned capabilities and default cost assumptions.

## Adapter manifest

From `adapters/register.go`:

- **Type**: `claude_code`
- **Default capabilities**: `code_gen`, `refactor`
- **Default cost profile**: ~$0.025 per request, ~$0.018 per 1k tokens (override at registration)

## Registration example

```bash
curl -s -X POST "$CONTROL_PLANE/v1/agents/register" \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Org-ID: acme" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "claude-west-1",
    "agent_type": "claude_code",
    "endpoint_url": "https://claude-runner.internal:8443/v1/invoke",
    "capabilities": ["code_gen", "refactor"],
    "cost_profile": {"cost_per_request_usd": 0.022, "cost_per_1k_tokens_usd": 0.016},
    "metadata": {"region": "us-west", "model": "claude-sonnet"}
  }'
```

Use the Go helper:

```go
import "github.com/agentmesh/agentmesh/adapters"

err := adapters.Register(ctx, client, adapters.TypeClaudeCode, "claude-west-1", endpointURL, nil)
```

## Routing recommendations

- Use `least-cost` for batch refactors where latency SLAs are relaxed.
- Use `least-latency` for interactive IDE sessions.
- Enable GuardLoop (`GUARDLOOP_ENABLED=true`) and `routing_strategy: score-based` when policy scores must influence selection.

## Headers injected on route

The control plane returns `injected_headers` in `RouteResponse` for downstream proxies. Propagate TraceLens trace context from `trace_context` when forwarding to Claude runners.

## Health reporting

Agents should call `ReportHealth` over gRPC or rely on lease renewal via periodic re-registration. Unhealthy agents are excluded from discovery.

## Multi-tenant isolation

Always set `X-Org-ID` (or `X-Workspace-ID` as alias). etcd keys are namespaced under `/agentmesh/{org_id}/...`.

## Failure modes

- Circuit open on destination: route API returns fallback chain; retry another agent from `fallback_chain`.
- Rate limited: HTTP 429 from gateway when org/agent exceeds Redis limit (see ops rate limits).

## Testing

Run `go test ./adapters/...` for manifest coverage. Load tests in `tests/load/smoke.sh` include Claude-type registration smoke checks when `AGENT_TYPE=claude_code`.
