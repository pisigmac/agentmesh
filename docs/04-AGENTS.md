# Agent types and adapters

AgentMesh supports **nine agent adapter types** matching the GuardLoop registry. Each type has a manifest with default capabilities and cost hints.

## Supported types

| `agent_type` | Typical use | Default capabilities |
|--------------|-------------|----------------------|
| `cursor` | Cursor IDE agent | `code_gen`, `refactor`, `browser_verify` |
| `claude_code` | Claude Code CLI / runner | `code_gen`, `refactor` |
| `github_copilot` | Copilot completions | `code_gen`, `completion` |
| `openai_codex` | Codex-style agents | `code_gen` |
| `aider` | Aider pair programming | `code_gen`, `refactor` |
| `continue_dev` | Continue.dev | `code_gen`, `completion` |
| `windsurf` | Windsurf | `code_gen`, `refactor` |
| `devin` | Autonomous dev agents | `code_gen`, `browser_verify`, `refactor` |
| `custom` | Bring-your-own endpoint | `custom` |

Constants live in `adapters/register.go` as `TypeCursor`, `TypeClaudeCode`, etc. `adapters.AllTypes` enumerates all values for validation UIs.

## Registration fields

Protobuf `AgentRegistration` fields:

- `org_id` — tenant (defaults from auth principal if omitted on REST)
- `agent_id` — unique within org
- `agent_type` — one of the table above
- `capabilities` — string tags for discovery
- `endpoint_url` — HTTP(S) target for data plane proxy
- `health` — optional initial health snapshot
- `cost_profile` — optional override of manifest defaults
- `metadata` — opaque key/value for operators

## Leases

Successful registration returns `lease_ttl_seconds`. Agents must refresh before TTL expiry or etcd will drop the record.

## Discovery

Query `GET /v1/agents/discover?capability=code_gen&routing_strategy=least-latency&max_cost=0.05`.

To turn a user question into a call on a selected agent, see **[22-ROUTING-QUESTIONS.md](22-ROUTING-QUESTIONS.md)**.

Streaming gRPC `DiscoverAgents` yields ranked `AgentEndpoint` messages with `rank_score`, `success_rate`, and `estimated_latency_ms`.

## Fallback chains

`RoutePayload.fallback_agent_types` restricts secondary candidates (e.g. `["github_copilot","aider"]`) when the primary type is circuit-open or over budget.

## Custom agents

Use `custom` when the endpoint speaks your protocol. Register explicit capabilities beyond `custom` so routing strategies can match workloads.

## SDK

`pkg/agentmesh/client.go` wraps register/discover/route for Go orchestrators. Pass org context via client configuration and auth headers.

## Frontend

The Next.js app lists routes and circuits per org; agent type filters map to the nine adapter constants.
