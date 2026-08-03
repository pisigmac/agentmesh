# OpenAPI specification

AgentMesh publishes a REST OpenAPI 3 document for the control plane gateway.

## Location

- **File**: [openapi.yml](openapi.yml)
- **Verify in CI**: `make verify-openapi`

## Covered paths

| Method | Path | Operation |
|--------|------|-----------|
| POST | `/v1/agents/register` | Register or refresh agent lease |
| GET | `/v1/agents/discover` | List ranked agents for a capability |
| POST | `/v1/route` | Select destination and fallback chain |
| GET | `/v1/circuit/{agent_id}` | Read circuit breaker state |

Health endpoints (`/health`, `/ready`) are intentionally excluded from the public contract; they remain unauthenticated operational probes.

## Security scheme

```yaml
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
    orgHeader:
      type: apiKey
      in: header
      name: X-Org-ID
```

Operations require `bearerAuth`. Clients should also send `X-Org-ID` or `X-Workspace-ID`.

## Code generation

You may generate clients with any OpenAPI 3 generator:

```bash
openapi-generator-cli generate -i docs/openapi.yml -g go -o gen/openapi
```

Field names follow protobuf JSON mapping (`org_id`, `agent_type`, `endpoint_url`).

## Versioning

The `/v1` prefix is stable for GA. Breaking changes will ship under `/v2` with a deprecation window documented in [20-CHANGELOG.md](20-CHANGELOG.md).

## Relationship to gRPC

Proto definitions in `api/proto/agentmesh/v1/agentmesh.proto` are authoritative for gRPC. OpenAPI mirrors the REST gateway in `internal/rest/gateway.go`; drift is caught by review when gateway handlers change.

## Examples

See bundled examples in `openapi.yml` for register, discover, route, and circuit responses.

## Agent types in schema

OpenAPI enumerates all nine GuardLoop-aligned `agent_type` values on registration bodies.

## Routing strategies in schema

`RoutePayload.routing_strategy` accepts `capability-match`, `least-latency`, `least-cost`, `round-robin`, and `score-based` (requires GuardLoop when used in production).
