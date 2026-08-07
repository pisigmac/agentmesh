import { getApiBase, getApiKey, getOrgId } from "./config";

export type ApiError = { code: string; message: string };

export type RequestOptions = {
  orgId?: string;
};

async function request<T>(path: string, init?: RequestInit, opts?: RequestOptions): Promise<T> {
  const org = opts?.orgId ?? getOrgId();
  const res = await fetch(`${getApiBase()}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${getApiKey()}`,
      "X-Org-ID": org,
      ...init?.headers,
    },
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    const err = body?.error as ApiError | undefined;
    throw new Error(err?.message || `HTTP ${res.status}`);
  }
  return body as T;
}

export async function fetchHealth(): Promise<{ status: string }> {
  const res = await fetch(`${getApiBase()}/health`);
  return res.json();
}

export type AgentEndpoint = {
  agent_id?: string;
  agent_type?: string;
  org_id?: string;
  endpoint_url?: string;
  capabilities?: string[];
  rank_score?: number;
  estimated_latency_ms?: number;
  success_rate?: number;
  healthy?: boolean;
  persistent?: boolean;
};

export async function listOrgs(): Promise<string[]> {
  const data = await request<{ orgs: string[] }>("/v1/orgs");
  const orgs = data.orgs ?? [];
  if (!orgs.includes("default")) {
    return ["default", ...orgs].sort();
  }
  return orgs;
}

export async function discoverAgents(
  capability?: string,
  maxCost?: number,
  opts?: { orgId?: string; limit?: number; includeUnhealthy?: boolean },
) {
  const q = new URLSearchParams();
  if (capability) q.set("capability", capability);
  if (maxCost != null && maxCost > 0) q.set("max_cost", String(maxCost));
  if (opts?.limit != null && opts.limit > 0) q.set("limit", String(opts.limit));
  if (opts?.includeUnhealthy) q.set("include_unhealthy", "1");
  const data = await request<{ agents: AgentEndpoint[] }>(`/v1/agents/discover?${q}`, undefined, {
    orgId: opts?.orgId,
  });
  return data.agents ?? [];
}

export async function reportAgentHealth(
  body: { agent_id: string; healthy: boolean; message?: string },
  opts?: RequestOptions,
) {
  return request<{ accepted: boolean }>("/v1/agents/health", {
    method: "POST",
    body: JSON.stringify(body),
  }, opts);
}

export type RoutePayload = {
  source_agent_id?: string;
  capability?: string;
  routing_strategy?: string;
  fallback_agent_types?: string[];
  destination_agent_id?: string;
  prefer_agent_ids?: string[];
  force_pin?: boolean;
  pin_failover?: boolean;
};

export type RouteResponse = {
  destination_agent_id?: string;
  destination_agent_type?: string;
  endpoint_url?: string;
  injected_headers?: Record<string, string>;
  timeout_ms?: number;
  max_retries?: number;
  trace_context?: string;
};

export async function routeRequest(payload: RoutePayload, opts?: RequestOptions) {
  const org = opts?.orgId ?? getOrgId();
  return request<RouteResponse>(
    "/v1/route",
    {
      method: "POST",
      body: JSON.stringify({ org_id: org, ...payload }),
    },
    opts,
  );
}

export type CircuitState = {
  state?: string;
  failure_count?: number;
  threshold?: number;
  window_seconds?: number;
  opened_at_unix?: number;
};

export async function getCircuitState(agentId: string, opts?: RequestOptions) {
  return request<CircuitState>(`/v1/circuit/${encodeURIComponent(agentId)}`, undefined, opts);
}

export async function getPolicy(opts?: RequestOptions): Promise<unknown> {
  return request("/v1/policy", undefined, opts);
}

export async function putPolicy(doc: unknown, opts?: RequestOptions) {
  return request("/v1/policy", { method: "PUT", body: JSON.stringify(doc) }, opts);
}

export type ModelInfo = {
  id: string;
  label: string;
  provider: string;
  api_model: string;
  capability: string;
  description?: string;
  default_base_url?: string;
};

export type ProviderInfo = {
  id: string;
  label: string;
  api_style: string;
  default_base_url?: string;
  requires_base_url?: boolean;
  key_hint?: string;
};

export type ProviderSecretView = {
  provider: string;
  label?: string;
  configured: boolean;
  key_hint?: string;
  base_url?: string;
  api_model?: string;
  updated_at?: string;
  backend?: string;
};

export type KeymintStatus = {
  configured: boolean;
  base_url?: string;
  backend?: string;
};

export async function listModels(opts?: RequestOptions) {
  const data = await request<{ models: ModelInfo[]; providers?: ProviderInfo[] }>("/v1/models", undefined, opts);
  return data.models ?? [];
}

export async function listProviders(opts?: RequestOptions) {
  const data = await request<{ providers: ProviderInfo[] }>("/v1/models/providers", undefined, opts);
  return data.providers ?? [];
}

export async function listProviderSecrets(opts?: RequestOptions) {
  const data = await request<{ secrets: ProviderSecretView[] }>("/v1/secrets/providers", undefined, opts);
  return data.secrets ?? [];
}

export async function getKeymintStatus(opts?: RequestOptions) {
  return request<KeymintStatus>("/v1/secrets/keymint", undefined, opts);
}

export async function putProviderSecret(
  provider: string,
  body: { api_key?: string; base_url?: string; api_model?: string },
  opts?: RequestOptions,
) {
  return request<{ status: string }>(
    `/v1/secrets/providers/${encodeURIComponent(provider)}`,
    { method: "PUT", body: JSON.stringify(body) },
    opts,
  );
}

export async function deleteProviderSecret(provider: string, opts?: RequestOptions) {
  return request<{ status: string }>(
    `/v1/secrets/providers/${encodeURIComponent(provider)}`,
    { method: "DELETE" },
    opts,
  );
}

export type ChatResult = {
  answer: string;
  model_id: string;
  provider: string;
  api_model: string;
  destination_agent_id?: string;
  destination_agent_type?: string;
  endpoint_url?: string;
  trace_id?: string;
  routed_via_mesh?: boolean;
  usage?: Record<string, unknown>;
};

export type ChatBody = {
  model: string;
  question?: string;
  system?: string;
  messages?: { role: string; content: string }[];
  route_via_mesh?: boolean;
  source_agent_id?: string;
  destination_agent_id?: string;
  prefer_agent_ids?: string[];
  force_pin?: boolean;
  pin_failover?: boolean;
};

export async function chatCompletion(body: ChatBody, opts?: RequestOptions) {
  return request<ChatResult>("/v1/chat", { method: "POST", body: JSON.stringify(body) }, opts);
}

export type StreamHandlers = {
  onMeta?: (meta: Record<string, unknown>) => void;
  onToken?: (text: string) => void;
  onDone?: (done: Record<string, unknown>) => void;
  onError?: (message: string) => void;
  signal?: AbortSignal;
};

export async function chatCompletionStream(body: ChatBody, handlers: StreamHandlers, opts?: RequestOptions) {
  const org = opts?.orgId ?? getOrgId();
  const res = await fetch(`${getApiBase()}/v1/chat/stream`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${getApiKey()}`,
      "X-Org-ID": org,
    },
    body: JSON.stringify(body),
    signal: handlers.signal,
  });
  if (!res.ok) {
    const bodyJson = await res.json().catch(() => ({}));
    const err = bodyJson?.error as ApiError | undefined;
    throw new Error(err?.message || `HTTP ${res.status}`);
  }
  if (!res.body) {
    throw new Error("No response body for stream");
  }
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let event = "message";
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const parts = buffer.split("\n");
    buffer = parts.pop() ?? "";
    for (const line of parts) {
      if (line.startsWith("event:")) {
        event = line.slice(6).trim();
        continue;
      }
      if (!line.startsWith("data:")) continue;
      const data = line.slice(5).trim();
      let parsed: Record<string, unknown> = {};
      try {
        parsed = JSON.parse(data);
      } catch {
        continue;
      }
      if (event === "meta") handlers.onMeta?.(parsed);
      else if (event === "token") handlers.onToken?.(String(parsed.text ?? ""));
      else if (event === "done") handlers.onDone?.(parsed);
      else if (event === "error") handlers.onError?.(String(parsed.message ?? "stream error"));
      event = "message";
    }
  }
}
