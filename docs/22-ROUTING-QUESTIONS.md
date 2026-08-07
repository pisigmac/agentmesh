# Routing questions to agents

This guide explains how to send a user question (or any workload) to the **right agent** using AgentMesh.

AgentMesh does **not** run models or chat completion APIs. It:

1. Selects a registered agent from the registry (etcd)
2. Returns that agent’s `endpoint_url` (and headers / timeouts)
3. Optionally proxies HTTP through the data plane to that URL

Your app (or the data plane) then delivers the question payload to the agent process, which owns the model and prompt handling.

---

## Mental model

```
User question
    │
    ▼
Your orchestrator / app
    │
    │  POST /v1/route  { capability, strategy, … }
    ▼
Control plane  ──►  pick destination agent (rank + circuit + policy)
    │
    │  RouteResponse { destination_agent_id, endpoint_url, injected_headers, … }
    ▼
Agent HTTP endpoint  ──►  model inference / tool run
    │
    ▼
Answer back to your app / user
```

| Layer | Owns |
|--------|------|
| Your app | Question text, conversation state, auth to the agent |
| AgentMesh control plane | Discovery, ranking, circuit breaking, org policy |
| AgentMesh data plane (optional) | HTTP proxy to `endpoint_url` after route |
| Agent (Cursor, Claude Code, custom, …) | Model config, prompts, execution |

`POST /v1/route` accepts optional **`destination_agent_id`** (hard pin), **`prefer_agent_ids`** (soft preference), **`force_pin`**, and **`pin_failover`**. Without a pin, selection is by **capability**, **routing strategy**, optional **agent type filter**, and circuit state.

---

## Prerequisites

1. Stack is up (`./scripts/start-all.sh` or local CP/DP).
2. Agents are registered for your org (prefer `-persistent` for demos).
3. You have an API key and org headers.

Default local values (host Compose ports):

| Item | Value |
|------|--------|
| Control plane REST | `http://localhost:48080` |
| Data plane | `http://localhost:48081` |
| API key | `agentmesh-dev-key` |
| Org | `default` |

```bash
export CP=http://localhost:48080
export DP=http://localhost:48081
export KEY=agentmesh-dev-key
export ORG=default
export HDR=(-H "Authorization: Bearer ${KEY}" -H "X-Org-ID: ${ORG}" -H "Content-Type: application/json")
```

Seed a catalog if empty:

```bash
./scripts/seed-all-agents.sh
./scripts/seed-policy.sh
```

---

## Step 1 — Register agents with capabilities

Routing only considers **healthy** agents that advertise the requested **capability**.

### Adapter types and default capabilities

| `agent_type` | Default capabilities (from manifests) |
|--------------|----------------------------------------|
| `cursor` | `code_gen`, `refactor`, `browser_verify` |
| `claude_code` | `code_gen`, `refactor` |
| `github_copilot` | `code_gen`, `completion` |
| `openai_codex` | `code_gen` |
| `aider` | `code_gen`, `refactor` |
| `continue_dev` | `code_gen`, `completion` |
| `windsurf` | `code_gen`, `refactor` |
| `devin` | `code_gen`, `browser_verify`, `refactor` |
| `custom` | `custom` (add your own tags) |

See [04-AGENTS.md](04-AGENTS.md) and `adapters/register.go`.

### Example: register a Claude Code-style agent

```bash
./bin/agentmesh \
  -base "${CP}" \
  -key "${KEY}" \
  -org "${ORG}" \
  -type claude_code \
  -id claude-prod-1 \
  -endpoint http://127.0.0.1:9001 \
  -persistent
```

Or REST:

```bash
curl -s -X POST "${CP}/v1/agents/register" "${HDR[@]}" \
  -d '{
    "agent_id": "claude-prod-1",
    "agent_type": "claude_code",
    "endpoint_url": "http://127.0.0.1:9001",
    "capabilities": ["code_gen", "refactor", "support_qa"],
    "metadata": {
      "persistent": "true",
      "model": "claude-sonnet"
    }
  }'
```

Notes:

- `endpoint_url` must be reachable from the **data plane** (or from your app if you call it directly).
- `metadata.model` is informational only; AgentMesh does not load that model.
- `metadata.persistent=true` stores the registration without a short etcd lease (demo/catalog use). Live sidecars normally use leased registration + keepalive.

---

## Step 2 — Discover candidates (optional but recommended)

Before routing traffic, inspect who can serve a capability:

```bash
curl -s "${HDR[@]}" \
  "${CP}/v1/agents/discover?capability=code_gen&limit=50" | jq .
```

Example shape:

```json
{
  "agents": [
    {
      "agent_id": "claude-prod-1",
      "agent_type": "claude_code",
      "org_id": "default",
      "endpoint_url": "http://127.0.0.1:9001",
      "capabilities": ["code_gen", "refactor", "support_qa"],
      "rank_score": 42.5,
      "estimated_latency_ms": 100,
      "success_rate": 1
    }
  ]
}
```

Org `default` aggregates agents from all tenants when listing/discovering (see registry behavior). Other orgs see only their own agents.

**UI:** Console → **Agents** — org dropdown, agent dropdown, capability filter.

---

## Step 3 — Resolve a route (`POST /v1/route`)

This is the core control-plane call. It returns **where** to send the next request; it does **not** accept the question body as the routing key (use `capability` + filters).

### Request fields

| Field | Required | Description |
|-------|----------|-------------|
| `capability` | **Yes** | Capability tag the destination must have |
| `org_id` | No | Defaults from `X-Org-ID` |
| `source_agent_id` | No | Caller identity (rate limits, analytics, injected header) |
| `routing_strategy` | No | Overrides org policy default |
| `fallback_agent_types` | No | Restrict candidates to these `agent_type` values |
| `destination_agent_id` | No | Hard pin to this agent |
| `prefer_agent_ids` | No | Soft preference — move eligible IDs to front of ranked list |
| `force_pin` | No | Allow pin if unhealthy or missing capability |
| `pin_failover` | No | If pinned circuit is open, fall back to normal ranking |
| `headers` | No | Extra keys merged into `injected_headers` on the response |
| `payload_hint` | No | Opaque bytes (proto); not used by ranking today |

### Strategies

| Strategy | Behavior |
|----------|----------|
| `capability-match` | Filter by capability; rank by success / latency / cost heuristic |
| `least-latency` | Prefer lowest estimated latency |
| `least-cost` | Prefer lowest cost profile |
| `round-robin` | Rotate among matching candidates |
| `score-based` | Blend GuardLoop score (if enabled) with success / latency / cost |

If `routing_strategy` is omitted, org policy `routing.default_strategy` is used ([org policy example](../deploy/examples/org-policy-default.json)).

### Example: route a coding question

```bash
curl -s -X POST "${CP}/v1/route" "${HDR[@]}" \
  -d '{
    "capability": "code_gen",
    "source_agent_id": "web-chat",
    "routing_strategy": "least-latency",
    "fallback_agent_types": ["claude_code", "cursor"],
    "headers": {
      "x-user-session": "sess-123"
    }
  }' | jq .
```

### Example response

```json
{
  "destination_agent_id": "claude-prod-1",
  "destination_agent_type": "claude_code",
  "endpoint_url": "http://127.0.0.1:9001",
  "injected_headers": {
    "x-agentmesh-trace-id": "…",
    "x-agentmesh-source-agent": "web-chat",
    "x-agentmesh-dest-agent": "claude-prod-1",
    "x-agentmesh-routing-strategy": "least-latency",
    "x-user-session": "sess-123"
  },
  "fallback_chain": [
    {
      "agent_id": "claude-prod-1",
      "agent_type": "claude_code",
      "endpoint_url": "http://127.0.0.1:9001"
    }
  ],
  "timeout_ms": 30000,
  "max_retries": 2,
  "trace_context": "…"
}
```

**Interpretation**

- Call `endpoint_url` (or proxy via data plane) with your question JSON.
- Propagate `injected_headers` for tracing and support.
- Honor `timeout_ms` / `max_retries` in your client.
- `fallback_chain` lists ranked alternatives; the data plane walks it on 5xx/timeout when `DATAPLANE_FAILOVER=true` (clients may still implement their own failover).

### Selection algorithm (control plane)

1. Load org policy (strategy, timeouts, rate limits, circuit config).
2. List agents for the org (`default` = all orgs’ agents).
3. Filter: healthy + matching `capability` (+ max cost when used in discover).
4. Rank by `routing_strategy`.
5. Optionally keep only `fallback_agent_types`.
6. Walk ranked list; skip destinations whose **circuit breaker** is open.
7. Return the first allowed agent or error (`no route` / `circuit open for all candidates`).

**UI:** Console → **Route debugger** — same payload fields without writing curl.

---

## Step 4 — Deliver the question to the agent

Choose one of these patterns.

### Pattern A — App calls the agent directly (advisory)

1. `POST ${CP}/v1/route` → get `endpoint_url` + headers.
2. `POST ${endpoint_url}/…` with your chat/question body and `injected_headers`.

```bash
ROUTE=$(curl -s -X POST "${CP}/v1/route" "${HDR[@]}" \
  -d '{"capability":"code_gen","source_agent_id":"web-chat"}')

ENDPOINT=$(echo "$ROUTE" | jq -r '.endpoint_url')
TRACE=$(echo "$ROUTE" | jq -r '.injected_headers["x-agentmesh-trace-id"]')

# Shape depends on YOUR agent API — AgentMesh does not define chat schema
curl -s -X POST "${ENDPOINT}/v1/chat" \
  -H "Content-Type: application/json" \
  -H "x-agentmesh-trace-id: ${TRACE}" \
  -d '{
    "question": "Explain how to add retries to this Go HTTP client",
    "context": { "repo": "agentmesh", "lang": "go" }
  }'
```

Use this when agents are reachable from the client network and you want full control of the HTTP path.

### Pattern B — Data plane advisory then proxy

Data plane modes (see `DATAPLANE_MODE`):

| Mode | Behavior |
|------|----------|
| Advisory | Resolve route; return JSON `RouteResponse` (same as control plane) |
| Gateway / proxy | Resolve route; forward HTTP to `endpoint_url` |

Typical host URL: `http://localhost:48081`.

Proxy path shape (implementation):

- Client hits data plane with a route-related body and/or path under `/v1/proxy…`
- Data plane calls control plane `RouteRequest`, then proxies to `endpoint_url` (+ path suffix) with injected headers

Use Pattern B when you want a single ingress (retries, TraceLens spans, ClickHouse route metrics) in front of agent endpoints.

Consult [05-API.md](05-API.md) and dataplane logs if your deployment uses a custom gateway path.

### Pattern C — Discover, then pin in your app

If you must hit **one known** agent:

```bash
# 1) List
curl -s "${HDR[@]}" "${CP}/v1/agents/discover?capability=code_gen&limit=256" \
  | jq '.agents[] | select(.agent_id=="claude-prod-1")'

# 2) Call that endpoint_url yourself (skip /v1/route ranking)
```

Or combine: call `/v1/route` for policy/circuit metadata, but only proceed if `destination_agent_id` matches your expected ID; otherwise fail closed.

---

## Targeting a specific agent

| Goal | How |
|------|-----|
| Prefer a **type** (e.g. only Claude Code) | `"fallback_agent_types": ["claude_code"]` on `/v1/route` |
| Prefer **cheapest / fastest** among matches | `routing_strategy`: `least-cost` / `least-latency` |
| Force **one agent** | Register a **unique capability** (e.g. `billing_qa`) only on that agent; route with that capability |
| Force **one agent** (app-side) | Discover → select by `agent_id` → call `endpoint_url` |
| Soft preference via labels | Put hints in `metadata` (e.g. `model`, `tier`); ranking does not read them today—filter in your app after discover |

### Unique-capability pattern (recommended for “required agent”)

```bash
# Register support bot with exclusive capability
curl -s -X POST "${CP}/v1/agents/register" "${HDR[@]}" -d '{
  "agent_id": "support-bot-1",
  "agent_type": "custom",
  "endpoint_url": "http://127.0.0.1:9100",
  "capabilities": ["support_qa", "custom"],
  "metadata": {"persistent": "true"}
}'

# Every support question routes only to agents with support_qa
curl -s -X POST "${CP}/v1/route" "${HDR[@]}" -d '{
  "capability": "support_qa",
  "source_agent_id": "helpdesk-ui"
}'
```

---

## Org policy and defaults

Org policy (`GET`/`PUT /v1/policy` or `./scripts/seed-policy.sh`) controls:

- Default strategy when the client omits `routing_strategy`
- Default `timeout_ms` / `max_retries` returned on routes
- Rate limits on register/route
- Circuit thresholds (global, by agent type, by destination id)

**UI:** Console → **Org policy**.

Example default strategy:

```json
{
  "org_id": "default",
  "routing": {
    "default_strategy": "least-latency",
    "default_timeout_ms": 30000,
    "default_max_retries": 2
  }
}
```

---

## Circuit breakers

Per `(org, destination_agent_id)` in Redis:

- Failures open the circuit → that agent is skipped on later `/v1/route` calls.
- Routing walks the ranked list until a closed (or allowable) destination is found.
- Inspect: `GET /v1/circuit/{agent_id}` or Console → **Circuits**.

If all candidates are open, the API returns an unavailable / circuit error—your app should surface that and optionally retry later or widen `fallback_agent_types`.

---

## End-to-end checklist (coding question → Claude-type agent)

1. [ ] Control plane healthy: `curl -s ${CP}/health`
2. [ ] Agent registered with `claude_code` + `code_gen` (or seed script)
3. [ ] Discover shows the agent for `capability=code_gen`
4. [ ] `POST /v1/route` with `capability=code_gen` and `fallback_agent_types: ["claude_code"]`
5. [ ] Confirm `destination_agent_id` / `endpoint_url`
6. [ ] POST question JSON to that endpoint (or via data plane)
7. [ ] Keep `x-agentmesh-trace-id` for logs / TraceLens

---

## Orchestrator pseudocode

```text
function answer(userQuestion, intentCapability):
  route = POST /v1/route {
    capability: intentCapability,          // e.g. map "write code" → code_gen
    source_agent_id: "my-orchestrator",
    routing_strategy: "",                  // use org default
    fallback_agent_types: preferredTypes   // optional
  }

  if route failed:
    return error("no agent available")

  response = HTTP POST route.endpoint_url + "/v1/chat" {
    headers: route.injected_headers
    body: { question: userQuestion }
    timeout: route.timeout_ms
  }

  return response
```

Map natural-language intents to **capabilities** in your app (AgentMesh does not NLP-classify the question).

---

## Common failures

| Symptom | Cause | What to do |
|---------|--------|------------|
| `no route` / empty discover | No healthy agent with that capability | Register/seed agents; check capability spelling |
| Wrong agent type selected | Ranked among all matching capabilities | Set `fallback_agent_types` or unique capability |
| Agents disappear after ~30s | Ephemeral etcd lease without keepalive | Re-register with `metadata.persistent=true` or run `-keepalive` |
| 401 | Bad key / org | Settings / headers: Bearer + `X-Org-ID` |
| Circuit open for all | Destinations failing | `GET /v1/circuit/{id}`; fix agent health; wait half-open |
| CORS in browser | Old CP without CORS middleware | Rebuild/restart control plane |
| Proxy can’t reach agent | `endpoint_url` is `127.0.0.1` inside Docker | Use host-reachable URLs or Docker network DNS names |

---

## Related docs

| Doc | Topic |
|-----|--------|
| [04-AGENTS.md](04-AGENTS.md) | Agent types, registration, leases |
| [05-API.md](05-API.md) | REST reference |
| [12-ARCHITECTURE.md](12-ARCHITECTURE.md) | Control plane / data plane design |
| [21-MANUAL-TESTING.md](21-MANUAL-TESTING.md) | Manual QA including route debugger |
| [openapi.yml](openapi.yml) | Machine-readable `RoutePayload` / `RouteResponse` |
| [ops/09-EMPTY-STATES.md](../ops/09-EMPTY-STATES.md) | UI copy when no route matches |

---

## Summary

1. **Tag** agents with capabilities that match workloads (`code_gen`, `support_qa`, …).
2. **Route** with `POST /v1/route` using that capability (and optional type filter / strategy).
3. **Send** the question to the returned `endpoint_url` (directly or via data plane), with `injected_headers`.
4. **Pin** a required agent with `destination_agent_id` on `/v1/route` (or Chat pin control). Use `force_pin` / `pin_failover` as needed.
