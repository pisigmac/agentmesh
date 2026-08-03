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

export function getApiKey(): string {
  if (typeof window !== "undefined") {
    return localStorage.getItem("agentmesh_api_key") || "agentmesh-dev-key";
  }
  return "agentmesh-dev-key";
}

export function saveSettings(base: string, org: string, key: string) {
  localStorage.setItem("agentmesh_api_base", base);
  localStorage.setItem("agentmesh_org_id", org);
  localStorage.setItem("agentmesh_api_key", key);
}

export function setOrgId(org: string) {
  localStorage.setItem("agentmesh_org_id", org);
}

