# Model keys and chat inference

How AgentMesh stores provider API keys and runs chat from the console.

## What is stored where

| Secret | Storage | Used by |
|--------|---------|---------|
| Mesh API key (`AGENTMESH_DEV_API_KEY`) | Env / Settings UI (localStorage) | Auth to control plane |
| Provider keys (OpenAI, Anthropic, custom) | **etcd**, encrypted AES-GCM with `AGENTMESH_SECRETS_KEY` | Control plane `POST /v1/chat` only |
| Model catalog | YAML (`deploy/models.yaml`, `MODELS_CATALOG_PATH`) | `GET /v1/models` |

Provider keys are **never** returned in full after save—only a short `key_hint`.

## APIs

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/v1/models` | Catalog (id, provider, capability, api_model) |
| `GET` | `/v1/secrets/providers` | Which providers are configured (hints only) |
| `PUT` | `/v1/secrets/providers/{provider}` | Save `{ api_key, base_url?, api_model? }` |
| `DELETE` | `/v1/secrets/providers/{provider}` | Remove key |
| `POST` | `/v1/chat` | Run inference |

### Chat body

```json
{
  "model": "gpt-4o-mini",
  "question": "Explain Go context cancellation",
  "system": "You are a helpful assistant.",
  "route_via_mesh": true,
  "source_agent_id": "console-chat"
}
```

Or pass `messages: [{role, content}, ...]`.

Flow:

1. Resolve model → provider + API model name + mesh `capability` (e.g. `model_openai`).
2. Load encrypted org secret for that provider.
3. If `route_via_mesh` (default true): call `RouteRequest` with that capability (advisory; missing agents do not block chat).
4. Call OpenAI-compatible `/chat/completions` or Anthropic `/v1/messages` with the server-held key.
5. Return `{ answer, model_id, provider, destination_agent_id?, trace_id?, … }`.

## Catalog YAML

Edit [`deploy/models.yaml`](../deploy/models.yaml) (mounted at `/etc/agentmesh/models.yaml` in Compose).

The default catalog mirrors **KubeMind** `services/router/config/gateway.yaml` providers/models:

| Provider | Examples |
|----------|----------|
| `ollama` | llama3.1, mistral, deepseek-r1 |
| `vllm` | Meta-Llama-3-8B-Instruct, DeepSeek-R1-Distill-Qwen-7B |
| `deepseek_local` | deepseek-r1, :7b, :14b |
| `groq` | llama-3.1-70b, mixtral-8x7b, llama-3.1-8b-instant |
| `google` | gemini-1.5-flash |
| `openrouter` | meta-llama/llama-3.1-70b |
| `openai` | gpt-4o-mini, gpt-4o |
| `anthropic` | claude-3-5-sonnet-20241022, claude-3-5-haiku-20241022 |
| `custom` | org-defined OpenAI-compatible gateway |

Embedding-only entries (e.g. `nomic-embed-text`) are omitted from chat. Local providers (`ollama`, `vllm`, `deepseek_local`) require `base_url` in **Models & keys**.

Restart the control plane after YAML edits. Path override: `MODELS_CATALOG_PATH`.

## Console

1. **Models & keys** (`/models`) — save keys for providers listed in YAML.
2. **Chat** (`/chat`) — pick a model from YAML, ask a question, see answer + route metadata.

Optional mesh destinations for `model_*` capabilities:

```bash
./scripts/seed-model-agents.sh
```

## Operations

- Set a strong `AGENTMESH_SECRETS_KEY` in production (do not reuse the mesh API key).
- Rotate provider keys via UI overwrite; rotate master key only with a planned re-encrypt (changing master key alone makes existing ciphertext unreadable).
- Custom provider: set `base_url` (…`/v1`) and `api_model` for OpenAI-compatible gateways.

## Related

- [22-ROUTING-QUESTIONS.md](22-ROUTING-QUESTIONS.md) — capability routing without inference
- [14-ENV.md](14-ENV.md) — `AGENTMESH_SECRETS_KEY`
