# AgentMesh — Future Plan

Detailed product and engineering plan: market positioning, enhancement backlog, phases, and success criteria.

This document expands the strategic roadmap beyond the shorter notes in [`docs/10-FUTURE.md`](docs/10-FUTURE.md). Timelines are indicative, not commitments.

---

## 1. Current state (baseline)

AgentMesh today provides:

| Area | Capability |
|------|------------|
| Control plane | gRPC + REST: register, discover, route, circuit, org policy |
| Registry | etcd leases; persistent registrations for demos/catalog |
| Routing | Capability match, least-latency/cost, round-robin, score-based (optional GuardLoop) |
| Resilience | Redis circuit breakers, rate limits |
| Data plane | Advisory route + HTTP proxy path |
| Auth | Native JWT sessions + hashed org API keys (`amk_`) + RBAC (viewer/operator/admin) |
| Operator UI | Landing, login/register, dashboard, agents, routes, circuits, policy, settings |
| Inference layer | YAML model catalog (`deploy/models.yaml`), **Keymint**-backed provider keys + proxy, streaming chat, destination pin |
| Ops | Docker Compose, Helm sketch, scripts, manual QA + routing docs |

**Product thesis (unchanged):** a **service mesh for AI coding agents**—discovery, policy-driven routing, and failure isolation across Cursor, Claude Code, Copilot, and custom endpoints—not a replacement for IDE agents or agent frameworks.

---

## 2. Market positioning

### 2.1 Where AgentMesh sits

```
┌─────────────────────┐     ┌──────────────────────┐
│ Agent frameworks    │     │ LLM gateways         │
│ LangGraph, CrewAI   │     │ LiteLLM, Portkey     │
└─────────┬───────────┘     └──────────┬───────────┘
          │ compose logic              │ model IDs / keys
          ▼                            ▼
┌─────────────────────────────────────────────────┐
│              AgentMesh (this product)           │
│  registry · capability route · circuit · policy │
│  optional inference + operator console          │
└───────────────────────┬─────────────────────────┘
                        │ endpoint_url
                        ▼
┌─────────────────────────────────────────────────┐
│ IDE / agent runtimes (Cursor, Claude, custom…)  │
└─────────────────────────────────────────────────┘
```

| Segment | Examples | Relationship |
|---------|----------|--------------|
| LLM gateways | LiteLLM, Portkey, OpenRouter | Overlap on keys + chat; differentiate on **agent/capability/circuit**, not model list size |
| Agent frameworks | LangGraph, CrewAI, AutoGen | Complementary: they orchestrate; we **discover, route, break, policy** |
| Observability | Langfuse, Helicone, Phoenix | Complementary: we own route decisions; they own traces/evals |
| Service mesh | Istio, Linkerd | Same primitives applied to **agents**, not microservices |
| IDE agent platforms | Cursor, Copilot, Claude Code | Not competitors—**typed adapters** into the mesh |

### 2.2 Positioning statement

**Primary:** Service mesh for multi-vendor AI coding agents—org-scoped registry, capability routing, circuit breaking, config-driven policy, and an operator console.

**Secondary (convenience):** Optional model catalog + encrypted provider keys + chat UI so teams can try inference without leaving the console.

**Avoid:** Competing as “another ChatGPT UI” or racing gateways on model catalog alone.

### 2.3 Ideal customer profile (ICP)

- Engineering orgs running **multiple coding agents** (IDE + CLI + custom bots)
- Platform / DevEx teams who need **org policy**, cost/latency awareness, and **failure isolation**
- Companies already investing in **KubeMind-style** local + cloud LLM routing who want the same discipline for **agents**

### 2.4 Non-goals (near term)

- Hosting or fine-tuning models
- Replacing LangGraph/Crew as the orchestration runtime
- Consumer chatbot product
- Full Istio feature parity (mTLS mesh for every hop) before product–market fit

---

## 3. Enhancement backlog (detailed)

### 3.1 Must-have for real adoption

#### E1 — Destination pin and sticky routing — **done (Phase A)**

**Shipped:** `RoutePayload.destination_agent_id`, `prefer_agent_ids`, `force_pin`, `pin_failover`; Chat + Route debugger UI; `x-agentmesh-pin` header.

#### E2 — Streaming chat (SSE) — **done (Phase A)**

**Shipped:** `POST /v1/chat/stream` SSE (`meta` / `token` / `done` / `error`) via Keymint proxy; Chat UI incremental render + Cancel.

#### E3 — Secrets via Keymint — **done (Phase A)**

**Shipped:** Provider keys maintained in **Keymint** (vault + `/provider-keys` + virtual keys). AgentMesh secrets REST is a facade; chat uses Keymint `POST /v1/proxy/chat/completions`. Virtual-key bindings encrypted in etcd with `AGENTMESH_SECRETS_KEY`. Audit/rotation delegated to Keymint.

**Follow-ups:** surface Keymint audit in console; finer secrets-admin UX.

#### E4 — First-class health and lease keepalive — **done (Phase A)**

**Shipped:** `POST /v1/agents/health`; discover `include_unhealthy` + `healthy`/`persistent` on `AgentEndpoint`; Agents UI badges; keepalive/persistent docs.

#### E5 — Native auth (JWT + org API keys) — **implemented**

**Problem:** `MemoryKeyStore` + single dev key is not multi-tenant ready. Clerk/OIDC rejected in favor of first-party auth.

**Shipped:**

- etcd users, sessions (`jti`), org memberships, hashed org API keys (`amk_`)
- `POST /v1/auth/register|login`, `GET /v1/auth/me`, `POST /v1/auth/logout`
- Org API key CRUD; dual Bearer auth (JWT session or API key)
- RBAC: viewer / operator / admin
- Console login/register + Settings key management
- Keymint remains secrets/proxy only — not the IdP

**Acceptance:** Two orgs, isolated keys; UI login; machine keys work for `/v1/*`.

---

### 3.2 Differentiation

#### E6 — Intent → capability mapping

**Problem:** Apps must manually map “refactor this” → `refactor`.

**Plan:**

- Org policy section `intent_routes`: rules (keyword/regex) and optional lightweight classifier
- `POST /v1/route` accepts `intent` or free-text `query_hint` → resolved capability
- Chat: auto-suggest capability from question
- Align with KubeMind `prefer_targets` (code → deepseek_local, etc.) as **prefer_providers / prefer_agent_types**

**Acceptance:** Sample intents resolve to expected capabilities in tests; policy-driven overrides.

#### E7 — Cost / latency SLOs in policy

**Problem:** Cost fields exist; few enforceable SLOs.

**Plan:**

- Policy: `max_cost_usd`, `max_latency_ms`, `prefer_free` / `prefer_local`
- Router filters before rank; metrics for SLO violations
- UI policy editor fields + discover filter parity
- Optional: daily org budget with soft/hard reject

**Acceptance:** Route excludes over-budget agents; Prometheus counter for rejects.

#### E8 — Data plane failover that uses fallback_chain

**Problem:** Fallback chain is advisory; proxy may not walk it on failure.

**Plan:**

- On upstream 5xx/timeout, try next `fallback_chain` entry within `max_retries`
- Record circuit failures per destination
- Config: `DATAPLANE_FAILOVER=true`
- Integration test: primary down → secondary succeeds

**Acceptance:** Documented failover path; chaos test in `tests/chaos`.

#### E9 — Score-based routing production path

**Problem:** GuardLoop integration is optional and under-documented.

**Plan:**

- Default Noop score with deterministic fixture provider for demos
- Document GuardLoop env + timeout/cache behavior
- Dashboard: show score contribution when strategy is `score-based`
- Fallback to least-latency when score provider errors

**Acceptance:** `score-based` works with mock provider in CI; GuardLoop path in staging runbook.

#### E10 — Adapter SDKs (Go already; Node + Python)

**Problem:** Register/keepalive is DIY for most stacks.

**Plan:**

- Official Python + TypeScript SDKs from OpenAPI (register, discover, route, chat, keepalive loop)
- Example sidecars per adapter type (cursor, claude_code, custom)
- Versioned releases; semver aligned with control plane

**Acceptance:** `pip`/`npm` install examples in docs; CI smoke against compose stack.

---

### 3.3 Platform and GTM

#### E11 — Multi-tenant SaaS and Helm completeness

**Plan:**

- Separate Helm charts/templates for CP, DP, frontend; HPA/PDB/ServiceMonitor solid
- Workspace = org alias fully enforced on all `/v1/*`
- Stripe usage from ClickHouse `billing_events` (see `ops/01-PAYMENTS.md`)
- Staging env runbook (`ops/08-STAGING.md`) kept current

#### E12 — MCP and tool surface

**Plan:**

- MCP server exposing tools: `discover_agents`, `route`, `get_circuit`, `chat`
- So IDE agents can use the mesh as tools without raw REST
- Auth via mesh API key in MCP config

#### E13 — Eval and analytics hooks

**Plan:**

- Export route samples to Langfuse/OTLP with strategy, agent_type, cost
- Console: “who won last 24h” from ClickHouse
- Optional offline eval job comparing agents on fixed prompts

#### E14 — KubeMind catalog sync

**Plan:**

- `scripts/import-kubemind-gateway.sh` → merge/overwrite `deploy/models.yaml`
- Document field mapping (providers, models, skip embeddings)
- Optional watch mode for platform teams sharing one source of truth

---

### 3.4 Later / opportunistic

| ID | Idea | Notes |
|----|------|--------|
| E15 | Semantic cache | Port patterns from KubeMind Redis semantic cache |
| E16 | Prompt/version registry | Org-scoped prompt IDs injected on chat |
| E17 | Adapter plugin marketplace | Signed manifests for community adapters |
| E18 | mTLS to agents | Opt-in after auth story is solid |
| E19 | Weighted multi-objective routing | latency + cost + score weights in policy |
| E20 | Terraform module | Cloud bring-up for etcd/Redis/CH + Helm |

---

## 4. Phased delivery plan

### Phase A — Design-partner ready (≈ 4–6 weeks)

Focus: demos that do not fall over; clear “required agent” story.

| Order | Item | Outcome |
|------:|------|---------|
| 1 | E1 Destination pin | **Shipped** — reliable targeting |
| 2 | E3 Keymint secrets | **Shipped** — vault + proxy |
| 3 | E2 Streaming chat | **Shipped** — SSE console inference |
| 4 | E4 Health / keepalive UX | **Shipped** — honest agent liveness |
| 5 | E8 Dataplane failover (MVP) | Retries next in chain (remaining) |

**Exit criteria:** Design partner can pin an agent, stream chat with Keymint-backed keys, and see agents go stale when keepalive dies. (E8 still open.)

### Phase B — Platform hardening (≈ 6–8 weeks)

| Order | Item | Outcome |
|------:|------|---------|
| 1 | E5 Native auth (JWT + org API keys) | **Shipped** — multi-tenant auth |
| 2 | E3 follow-ups (Keymint audit UI) | Compliance-friendly ops |
| 3 | E7 Policy SLOs | Cost/latency guardrails |
| 4 | E9 Score-based path | Differentiated routing |
| 5 | E11 Helm + metering slice | Deployable + billable |

**Exit criteria:** Two orgs isolated; staging Helm install; usage events flowing.

### Phase C — Ecosystem (≈ 8–12 weeks)

| Order | Item | Outcome |
|------:|------|---------|
| 1 | E10 Python/TS SDKs | Framework integration |
| 2 | E12 MCP server | IDE tool use |
| 3 | E6 Intent → capability | Less app glue |
| 4 | E13 Eval hooks | Prove agent quality |
| 5 | E14 KubeMind import | Shared catalog with platform LLM gateway |

**Exit criteria:** External team integrates via SDK or MCP without reading Go internals.

### Phase D — Scale features (ongoing)

E15–E20 as demand warrants; prefer customer pull over speculative build.

---

## 5. Architecture evolution (target)

```
                    ┌──────────── UI / MCP / SDKs ────────────┐
                    │  Chat (SSE) · Agents · Policy · Secrets │
                    └──────────────────┬──────────────────────┘
                                       │ Bearer + Org
┌──────────────────────────────────────▼──────────────────────────────────────┐
│ Control plane                                                               │
│  Registry (etcd) · Policy · Secrets (AES-GCM) · Catalog (YAML)              │
│  Router (pin, intent, SLO filters, score) · Circuits/RL (Redis)             │
│  Chat/inference · Audit log                                                 │
└───────────────┬─────────────────────────────┬───────────────────────────────┘
                │ gRPC route                  │ OTLP / metrics
                ▼                             ▼
┌───────────────────────────┐    ┌────────────────────────────┐
│ Data plane                │    │ TraceLens / Prom / CH      │
│  Proxy + failover chain   │    └────────────────────────────┘
└─────────────┬─────────────┘
              ▼
     Agent endpoints / providers
```

---

## 6. Metrics of success

### Product

| Metric | Phase A target | Phase B target |
|--------|----------------|----------------|
| Time to first successful route (new env) | &lt; 15 min | &lt; 10 min |
| Console chat stream usable | Yes | Yes |
| Orgs with isolated auth | 1 (dev) | ≥ 2 |
| Design partners using pin or failover | ≥ 1 | ≥ 3 |

### Reliability

| Metric | Target |
|--------|--------|
| Control plane `/health` availability (staging) | 99.5% |
| Route p99 (exclude provider inference) | &lt; 50 ms in-region |
| False “empty agents” due to lease confusion | Near zero with keepalive docs |

### Business (when metering lands)

| Metric | Signal |
|--------|--------|
| Routes / org / day | Adoption |
| Distinct agent_types routed | Multi-vendor wedge |
| Paid inference vs mesh-only usage | Gateway attach rate |

---

## 7. Risks and mitigations

| Risk | Mitigation |
|------|------------|
| Seen as “LiteLLM clone” | Lead messaging with mesh + adapters; keep catalog secondary |
| Lease expiry frustrates users | Persistent demos + keepalive defaults + UI honesty (E4) |
| Secret leakage via UI/logs | Never return raw keys; audit; no provider keys in localStorage |
| Scope creep into full orchestration | Explicit non-goals; integrate frameworks via SDK/MCP |
| Provider API churn | YAML catalog + OpenAI-compat style; import from KubeMind |

---

## 8. Documentation deliverables (as features ship)

| Feature | Docs to update |
|---------|----------------|
| Pin / stream | `22-ROUTING-QUESTIONS.md`, `23-MODEL-INFERENCE.md`, OpenAPI |
| Auth / secrets | `14-ENV.md`, new security runbook under `ops/` |
| SDKs / MCP | New `docs/24-SDKS.md`, `docs/25-MCP.md` |
| Manual QA | `21-MANUAL-TESTING.md` checklist rows |
| Roadmap pulse | Keep `docs/10-FUTURE.md` as short quarterly view; this file as the detailed plan |

---

## 9. Immediate next actions

1. Phase A core (pin, Keymint secrets, stream, health UX) is implemented; keep `21` / `22` / `23` current.
2. Next Phase A remainder: dataplane failover MVP (E8).
3. Run design-partner demo script from updated `21-MANUAL-TESTING.md`.
4. Revisit positioning website copy: **mesh first**, inference second.

---

## 10. Summary

AgentMesh should own the **control plane for multi-agent coding traffic**: registry, capability (and later intent) routing, circuits, org policy, and operator visibility. Inference and the YAML catalog are accelerators, not the core moat.

Execute **Phase A** to become design-partner ready; **Phase B** for real tenancy and guardrails; **Phase C** for ecosystem lock-in via SDKs and MCP.
