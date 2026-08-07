# Model keys and chat inference

How AgentMesh stores provider API keys (via **Keymint**) and runs chat from the console.

## What is stored where

| Secret | Storage | Used by |
|--------|---------|---------|
| Mesh API key (`AGENTMESH_DEV_API_KEY`) | Env / Settings UI (localStorage) | Auth to control plane |
| Provider keys (OpenAI, Anthropic, …) | **Keymint** AES vault + Postgres (`/provider-keys`) | Keymint proxy only |
| Org virtual key binding | etcd (`orgs/<org>/keymint/…`), encrypted with `AGENTMESH_SECRETS_KEY` | Control plane → Keymint proxy |
| Model catalog | YAML (`deploy/models.yaml`, `MODELS_CATALOG_PATH`) | `GET /v1/models` |

Provider plaintext is **never** stored in AgentMesh etcd and **never** returned after save—only a short `key_hint`. Audit/rotation live in Keymint.

## Keymint env

| Variable | Required | Description |
|----------|----------|-------------|
| `KEYMINT_BASE_URL` | yes (for Models/Chat) | e.g. `http://localhost:9090` |
| `KEYMINT_API_KEY` | yes | Bearer for Keymint protected APIs |
| `KEYMINT_TEAM_ID` | yes | Team UUID that owns AgentMesh virtual keys |
| `AGENTMESH_SECRETS_KEY` | recommended | Encrypts virtual-key bindings in etcd |

See Keymint `docs/ENV.md` in the Keymint repo for vault/Postgres setup.

## APIs

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/v1/models` | Catalog (id, provider, capability, api_model) |
| `GET` | `/v1/secrets/keymint` | Whether Keymint is configured |
| `GET` | `/v1/secrets/providers` | Which providers are configured (hints only) |
| `PUT` | `/v1/secrets/providers/{provider}` | Save key → Keymint provider-key + virtual key |
| `DELETE` | `/v1/secrets/providers/{provider}` | Remove Keymint key + binding |
| `POST` | `/v1/chat` | Non-streaming inference via Keymint proxy |
| `POST` | `/v1/chat/stream` | SSE streaming (`meta` → `token` → `done`) |

### Chat body

```json
{
  "model": "gpt-4o-mini",
  "question": "Explain Go context cancellation",
  "system": "You are a helpful assistant.",
  "route_via_mesh": true,
  "source_agent_id": "console-chat",
  "destination_agent_id": "optional-pin",
  "force_pin": false,
  "pin_failover": false
}
```

Or pass `messages: [{role, content}, ...]`.

Flow:

1. Resolve model → provider + API model name + mesh `capability`.
2. Load org→Keymint virtual-key binding for that provider.
3. If `route_via_mesh` (default true): call `RouteRequest` (supports pin fields; missing agents do not block chat).
4. Call Keymint `POST /v1/proxy/chat/completions` with the virtual key (OpenAI-compatible; `stream: true` for SSE).
5. Return answer + route metadata, or SSE events for `/v1/chat/stream`.

### Stream events

| Event | Payload |
|-------|---------|
| `meta` | `{ trace_id, model_id, provider, destination_agent_id?, routed_via_mesh }` |
| `token` | `{ text }` |
| `done` | `{ usage? }` |
| `error` | `{ message }` |

## Catalog YAML

Edit [`deploy/models.yaml`](../deploy/models.yaml) (mounted at `/etc/agentmesh/models.yaml` in Compose).

Restart the control plane after YAML edits. Path override: `MODELS_CATALOG_PATH`.

## Console

1. **Models & keys** (`/models`) — save keys into Keymint for providers listed in YAML.
2. **Chat** (`/chat`) — streaming UI with Cancel; optional pin destination.
3. **Settings** — Keymint connectivity status.

Optional mesh destinations for `model_*` capabilities:

```bash
./scripts/seed-model-agents.sh
```

## Migration note

Older etcd AES provider secrets (`internal/secrets`) are no longer used for chat. Re-enter keys under Models after upgrading (they are created in Keymint).

## Related

- [22-ROUTING-QUESTIONS.md](22-ROUTING-QUESTIONS.md) — capability routing and pin
- [14-ENV.md](14-ENV.md) — Keymint + `AGENTMESH_SECRETS_KEY`
