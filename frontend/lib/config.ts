export function getApiBase(): string {
  if (typeof window !== "undefined") {
    return localStorage.getItem("agentmesh_api_base") || process.env.NEXT_PUBLIC_CONTROL_PLANE_URL || "http://localhost:48080";
  }
  return process.env.NEXT_PUBLIC_CONTROL_PLANE_URL || "http://localhost:48080";
}

export function getOrgId(): string {
  if (typeof window !== "undefined") {
    return localStorage.getItem("agentmesh_org_id") || "default";
  }
  return "default";
}

/** Bearer credential: JWT after login, or org API key (amk_… / legacy dev key). */
export function getApiKey(): string {
  if (typeof window !== "undefined") {
    return (
      localStorage.getItem("agentmesh_token") ||
      localStorage.getItem("agentmesh_api_key") ||
      "agentmesh-dev-key"
    );
  }
  return "agentmesh-dev-key";
}

export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("agentmesh_token");
}

export function setToken(token: string) {
  localStorage.setItem("agentmesh_token", token);
}

export function clearToken() {
  localStorage.removeItem("agentmesh_token");
}

export function saveSettings(base: string, org: string, key: string) {
  localStorage.setItem("agentmesh_api_base", base);
  localStorage.setItem("agentmesh_org_id", org);
  if (key) {
    localStorage.setItem("agentmesh_api_key", key);
  }
}

export function setOrgId(org: string) {
  localStorage.setItem("agentmesh_org_id", org);
}

export function isLoggedIn(): boolean {
  return Boolean(getToken() || (typeof window !== "undefined" && localStorage.getItem("agentmesh_api_key")));
}
