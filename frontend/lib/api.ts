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
  opts?: { orgId?: string; limit?: number },
) {
  const q = new URLSearchParams();
  if (capability) q.set("capability", capability);
  if (maxCost != null && maxCost > 0) q.set("max_cost", String(maxCost));
  if (opts?.limit != null && opts.limit > 0) q.set("limit", String(opts.limit));
  const data = await request<{ agents: AgentEndpoint[] }>(`/v1/agents/discover?${q}`, undefined, {
    orgId: opts?.orgId,
  });
  return data.agents ?? [];
}

export type RoutePayload = {
  source_agent_id?: string;
  capability?: string;
  routing_strategy?: string;
  fallback_agent_types?: string[];
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
