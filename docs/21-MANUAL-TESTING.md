# Manual testing guide

Step-by-step manual QA for AgentMesh on a developer machine. Use this after code changes, before releases, or when onboarding.

**Time:** ~45–60 minutes for the full pass (API + UI + observability). A **smoke pass** (sections 1–5 only) takes ~15 minutes.

---

## Prerequisites

| Requirement | Check |
|-------------|--------|
| Docker Engine running | `docker info` succeeds |
| Go 1.22+ (local binaries path) | `go version` |
| Node 22+ (optional UI dev) | `node -v` |
| Repo cloned | You are at the repository root |
| Host ports free | See [Port map](#port-map) — default prefix **4** (e.g. `48080`) |

Clone path example: `/path/to/agentmesh`. All commands below assume **repo root**.

---

## Port map

From [`deploy/ports.env`](../deploy/ports.env) (override via env or that file):

| Service | Host URL / port |
|---------|-----------------|
| Control plane REST | `http://localhost:48080` |
| Control plane gRPC | `localhost:49090` |
| Data plane HTTP | `http://localhost:48081` |
| Dashboard (Docker) | `http://localhost:43000` |
| Dashboard (npm dev) | `http://localhost:43000` (or your `-p` port) |
| Grafana | `http://localhost:43001` (admin / admin) |
| Prometheus | `http://localhost:49091` |
| etcd (client) | `localhost:42379` |
| Redis | `localhost:46379` |
| ClickHouse HTTP | `http://localhost:48123` |

**Default dev credentials**

- API key: `agentmesh-dev-key` (env: `AGENTMESH_DEV_API_KEY`)
- Org: `default`
- Headers on every `/v1/*` call: `Authorization: Bearer agentmesh-dev-key`, `X-Org-ID: default`

Export once per shell session:

```bash
export AGENTMESH_DEV_API_KEY=agentmesh-dev-key
export CONTROL_PLANE_URL=http://localhost:48080
export CP="${CONTROL_PLANE_URL}"
export HDR=(-H "Authorization: Bearer ${AGENTMESH_DEV_API_KEY}" -H "X-Org-ID: default")
```

---

## 1. Start the stack

### Option A — Full Docker (recommended for manual QA)

```bash
./scripts/start-all.sh
```

**Expected**

- Script prints “Control plane is up” and “Dashboard is up” (or warnings if slow; retry status).
- `./scripts/status-all.sh` shows `OK` for control-plane, dataplane, prometheus, clickhouse, frontend.

**If build is slow**

```bash
BUILD=0 ./scripts/start-all.sh
```

### Option B — Infra in Docker, CP/DP from local `bin/`

For faster Go iteration:

```bash
./scripts/start-local.sh
```

**Expected:** Processes in `.run/control-plane.pid` and `.run/dataplane.pid`; logs under `.run/*.log`.

**Stop:** `./scripts/stop-local.sh` (does not stop Docker infra).

---

## 2. Baseline health (API)

Run in order; record pass/fail.

| Step | Command | Expected |
|------|---------|----------|
| 2.1 | `curl -s "${CP}/health"` | JSON: `{"status":"ok"}` |
| 2.2 | `curl -s "${CP}/ready"` | JSON: `{"status":"ready"}` |
| 2.3 | `curl -s http://localhost:48081/health` | HTTP 200, healthy body |
| 2.4 | `curl -s http://localhost:49091/-/ready` | Prometheus ready |
| 2.5 | `curl -s http://localhost:48123/ping` | `Ok.` |

**Failure hints**

- `connection refused` on `48080` → control plane not up; `./scripts/logs-all.sh control-plane`.
- Frontend OK but API fails → check `CONTROL_PLANE_URL` in Settings / `deploy/ports.env`.

---

## 3. Seed org policy

```bash
./scripts/seed-policy.sh
```

**Expected:** `Policy applied for org=default` (no curl error).

**Verify**

```bash
curl -s "${HDR[@]}" "${CP}/v1/policy" | jq .
```

**Expected fields (example):** `routing.default_strategy`, `rate_limit`, `circuit` blocks matching [`deploy/examples/org-policy-default.json`](../deploy/examples/org-policy-default.json).

---

## 4. Seed agents (persistent catalog)

```bash
make build
./scripts/seed-all-agents.sh
```

**Expected:** Nine lines `Registering default/...` and `registered ... persistent=true`.

**Why persistent:** Ephemeral registrations use a short etcd lease (~30s) and **disappear** when nothing renews them. Seeding with `-persistent` keeps agents visible after you leave the Agents UI or wait.

**Verify discover (all nine types)**

```bash
curl -s "${HDR[@]}" "${CP}/v1/agents/discover?limit=256" | jq '.agents | length'
```

**Expected:** `9` (or more if you registered extras).

**Verify org list**

```bash
curl -s "${HDR[@]}" "${CP}/v1/orgs" | jq .
```

**Expected:** `orgs` array includes `"default"`.

**Single agent (optional)**

```bash
./scripts/register-sample-agent.sh
```

Uses `-persistent` by default.

---

## 5. REST API walkthrough

Use the same headers as section 3. Replace `jq` with manual inspection if needed.

### 5.1 Discover with capability filter

```bash
curl -s "${HDR[@]}" \
  "${CP}/v1/agents/discover?capability=code_gen&limit=50" | jq '.agents[0]'
```

**Expected:** At least one agent; fields include `agent_id`, `agent_type`, `capabilities`, `rank_score`, `org_id`.

### 5.2 Discover as `default` org (global catalog)

Org `default` aggregates agents from **all** tenants in etcd.

```bash
curl -s "${HDR[@]}" "${CP}/v1/agents/discover?limit=256" | jq '[.agents[].agent_type] | unique'
```

**Expected:** Multiple adapter types (e.g. `cursor`, `claude_code`, …).

### 5.3 Route request

```bash
curl -s -X POST "${CP}/v1/route" \
  "${HDR[@]}" \
  -H "Content-Type: application/json" \
  -d '{
    "capability": "code_gen",
    "source_agent_id": "cursor-demo",
    "routing_strategy": "least-latency"
  }' | jq .
```

**Expected:** `destination_agent_id`, `destination_agent_type`, `endpoint_url`, `timeout_ms`, `trace_context` (non-empty).

**Negative case:** Unknown capability with strict filters may return 500 / `ROUTE_FAILED` — document as acceptable if no matching healthy agent.

### 5.4 Circuit state

Pick an agent id from discover (e.g. `cursor-demo`):

```bash
curl -s "${HDR[@]}" "${CP}/v1/circuit/cursor-demo" | jq .
```

**Expected:** `state` is `closed` for a fresh seed; `failure_count` likely `0`.

### 5.5 Policy update (optional)

```bash
curl -s -X PUT "${HDR[@]}" \
  -H "Content-Type: application/json" \
  "${CP}/v1/policy" \
  -d @deploy/examples/org-policy-default.json
```

**Expected:** `{"status":"updated"}` or 200 with success body.

### 5.6 CORS (browser only)

1. Open dashboard at `http://localhost:43000` (or dev port).
2. Open DevTools → Network.
3. Visit **Overview** or **Agents**.

**Expected:** Requests to `http://localhost:48080/health` and `/v1/agents/discover` succeed (no CORS errors). Localhost origins are allowed by default; production uses `AGENTMESH_CORS_ORIGINS`.

---

## 6. Dashboard UI walkthrough

Open **`http://localhost:43000`** (Docker) or run `./scripts/start-frontend-dev.sh` and use the printed URL.

**Settings first (if API calls fail)**

1. Go to **Settings**.
2. Control plane URL: `http://localhost:48080`
3. Org ID: `default`
4. API key: `agentmesh-dev-key`
5. **Save** → “Saved” confirmation.

### 6.1 Landing page (`/`)

| Step | Action | Expected |
|------|--------|----------|
| 1 | Load `/` | Hero, features, “Open console” |
| 2 | Click **Launch operator console** | Navigates to `/dashboard` |

### 6.2 Overview (`/dashboard`)

| Step | Action | Expected |
|------|--------|----------|
| 1 | Open **Overview** | Control plane badge **Healthy** |
| 2 | Check registered count | **9** after seed (or your count) |
| 3 | Click **Browse agents** | Goes to `/agents` |

### 6.3 Agents (`/agents`) — regression for “tab switch”

| Step | Action | Expected |
|------|--------|----------|
| 1 | Org **default (all agents)** | Agent dropdown lists all seeded agents |
| 2 | Note count **All agents (N)** | N ≥ 9 |
| 3 | Navigate to **Route debugger**, then back to **Agents** | Same count; table still populated |
| 4 | Switch browser tab away 2+ minutes, return, **Refresh** | Agents still present (requires `-persistent` seed + current control plane) |
| 5 | Org dropdown | Includes `default`; list from `/v1/orgs` |
| 6 | Pick one agent in dropdown | Table filters to that row |
| 7 | Capability `code_gen` + **Refresh** | Subset with `code_gen` |
| 8 | Clear capability, **Refresh** | Full list again |

### 6.4 Route debugger (`/routes`)

| Step | Action | Expected |
|------|--------|----------|
| 1 | Source: `cursor-demo`, capability: `code_gen` | — |
| 2 | Strategy: `(org default)` or `least-latency` | — |
| 3 | **Resolve route** | Destination card + JSON block |
| 4 | Invalid capability | Error alert (route failed) |

### 6.5 Circuits (`/circuits`)

| Step | Action | Expected |
|------|--------|----------|
| 1 | Destination ID: `cursor-demo` | — |
| 2 | **Get state** | Badge **closed**, thresholds from policy |

### 6.6 Org policy (`/policy`)

| Step | Action | Expected |
|------|--------|----------|
| 1 | Page load | JSON policy document |
| 2 | Edit `default_strategy` to `least-cost`, **Save** | Success alert |
| 3 | **Reload** | Change persisted |
| 4 | Restore from example file via API or revert manually | — |

### 6.7 Site home link

From console sidebar, **← Site home** → landing page. **Open console** returns to dashboard.

---

## 7. Data plane (smoke)

```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:48081/health
```

**Expected:** `200`.

Full proxy behavior depends on destination endpoints (seed uses `127.0.0.1:999x` — not real agents). Manual proxy test is optional; confirm dataplane process logs show no crash loops:

```bash
./scripts/logs-all.sh dataplane
```

---

## 8. Observability (optional)

| Step | URL | Expected |
|------|-----|----------|
| Grafana login | `http://localhost:43001` | admin / admin |
| Prometheus targets | `http://localhost:49091/targets` | `control-plane` / `dataplane` targets UP if scraped (chart-dependent) |
| CP metrics | `curl -s http://localhost:48080/metrics \| head` | Prometheus text format |

ClickHouse analytics may be empty until traffic is ingested — empty charts are OK for first run.

---

## 9. Automated checks (sanity before manual pass)

```bash
make test
make verify-openapi
cd frontend && npm run build
```

**Expected:** All pass; frontend build completes.

Optional load smoke:

```bash
./tests/load/smoke.sh
```

---

## 10. Teardown

```bash
./scripts/stop-all.sh
```

**Remove volumes (ClickHouse data):**

```bash
REMOVE_VOLUMES=1 ./scripts/stop-all.sh
```

**Local CP/DP only:**

```bash
./scripts/stop-local.sh
```

---

## 11. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Agents visible once, empty after ~30s or tab change | Old ephemeral registrations | Rebuild CP, `./scripts/seed-all-agents.sh` (uses `-persistent`) |
| CORS errors in browser | Old control plane image | Rebuild/restart control plane; hard-refresh UI |
| 401 on `/v1/*` | Wrong key or org | Settings: `agentmesh-dev-key`, org `default`; dev key works for other org headers |
| Discover returns 0 | No seed or wrong org | Run seed; use org `default` for global catalog |
| Frontend can’t reach API | Wrong URL / port | `48080` not `8080` on host; see `deploy/ports.env` |
| `status-all` FAIL frontend | Container still starting | Wait 30s, rerun `./scripts/status-all.sh` |

**Logs**

```bash
./scripts/status-all.sh
./scripts/logs-all.sh control-plane
./scripts/logs-all.sh frontend
```

---

## 12. Manual test checklist (printable)

Copy and mark pass/fail:

```
[ ] Stack start (start-all or start-local)
[ ] health / ready / dataplane / prometheus / clickhouse ping
[ ] seed-policy.sh
[ ] seed-all-agents.sh (9 agents, persistent)
[ ] GET /v1/orgs
[ ] GET /v1/agents/discover?limit=256
[ ] POST /v1/route
[ ] GET /v1/circuit/{agent_id}
[ ] GET/PUT /v1/policy
[ ] UI Settings saved
[ ] UI Agents: org + agent dropdowns, tab navigation persistence
[ ] UI Route debugger
[ ] UI Circuits
[ ] UI Policy editor
[ ] Landing ↔ console navigation
[ ] make test && frontend npm run build
[ ] stop-all (or stop-local)
```

---

## Related docs

- [01-README.md](01-README.md) — product overview
- [05-API.md](05-API.md) — REST reference
- [11-TESTS.md](11-TESTS.md) — automated tests and chaos
- [ops/09-EMPTY-STATES.md](../ops/09-EMPTY-STATES.md) — expected empty UI copy
- [scripts/README.md](../scripts/README.md) — utility scripts
