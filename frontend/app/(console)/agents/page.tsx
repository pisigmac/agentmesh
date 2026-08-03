"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { AgentEndpoint, discoverAgents, listOrgs } from "@/lib/api";
import { getOrgId, setOrgId } from "@/lib/config";

function agentLabel(a: AgentEndpoint) {
  const id = a.agent_id ?? "unknown";
  const type = a.agent_type ? ` (${a.agent_type})` : "";
  const org = a.org_id && a.org_id !== "default" ? ` · ${a.org_id}` : "";
  return `${id}${type}${org}`;
}

export default function AgentsPage() {
  const [orgs, setOrgs] = useState<string[]>(["default"]);
  const [selectedOrg, setSelectedOrg] = useState(getOrgId);
  const [capability, setCapability] = useState("");
  const [maxCost, setMaxCost] = useState("");
  const [selectedAgentId, setSelectedAgentId] = useState("");
  const [allAgents, setAllAgents] = useState<AgentEndpoint[]>([]);
  const [agents, setAgents] = useState<AgentEndpoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    listOrgs()
      .then(setOrgs)
      .catch(() => setOrgs(["default"]));
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const mc = maxCost ? parseFloat(maxCost) : undefined;
      const org = selectedOrg || "default";
      const catalog = await discoverAgents(undefined, undefined, { orgId: org, limit: 256 });
      setAllAgents(catalog);
      const filtered = await discoverAgents(capability || undefined, mc, { orgId: org, limit: 256 });
      setAgents(filtered);
      if (selectedAgentId && !filtered.some((a) => a.agent_id === selectedAgentId)) {
        setSelectedAgentId("");
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
      setAgents([]);
      setAllAgents([]);
    } finally {
      setLoading(false);
    }
  }, [capability, maxCost, selectedOrg]);

  useEffect(() => {
    load();
  }, [load]);

  const onOrgChange = (org: string) => {
    setSelectedOrg(org);
    setOrgId(org);
    setSelectedAgentId("");
  };

  const tableRows = useMemo(() => {
    if (!selectedAgentId) return agents;
    return agents.filter((a) => a.agent_id === selectedAgentId);
  }, [agents, selectedAgentId]);

  const showOrgColumn = selectedOrg === "default";

  return (
    <>
      <h1 className="page-title">Agents</h1>
      <p className="page-desc">
        Discovery scoped by org. The <span className="mono">default</span> org aggregates agents from every
        tenant in the mesh.
      </p>

      <div className="card">
        <div style={{ display: "flex", gap: "1rem", flexWrap: "wrap", alignItems: "flex-end" }}>
          <div className="field" style={{ margin: 0 }}>
            <label htmlFor="org">Org</label>
            <select id="org" value={selectedOrg} onChange={(e) => onOrgChange(e.target.value)}>
              {orgs.map((o) => (
                <option key={o} value={o}>
                  {o}
                  {o === "default" ? " (all agents)" : ""}
                </option>
              ))}
            </select>
          </div>
          <div className="field" style={{ margin: 0, minWidth: "220px" }}>
            <label htmlFor="agent">Agent</label>
            <select
              id="agent"
              value={selectedAgentId}
              onChange={(e) => setSelectedAgentId(e.target.value)}
              disabled={loading || allAgents.length === 0}
            >
              <option value="">All agents ({allAgents.length})</option>
              {allAgents.map((a) => (
                <option key={`${a.org_id ?? selectedOrg}:${a.agent_id}`} value={a.agent_id ?? ""}>
                  {agentLabel(a)}
                </option>
              ))}
            </select>
          </div>
          <div className="field" style={{ margin: 0 }}>
            <label htmlFor="cap">Capability filter</label>
            <input
              id="cap"
              value={capability}
              onChange={(e) => setCapability(e.target.value)}
              placeholder="optional, e.g. code_gen"
            />
          </div>
          <div className="field" style={{ margin: 0 }}>
            <label htmlFor="cost">Max cost (USD)</label>
            <input id="cost" value={maxCost} onChange={(e) => setMaxCost(e.target.value)} placeholder="optional" />
          </div>
          <button type="button" className="btn" onClick={load}>
            Refresh
          </button>
        </div>
      </div>

      {error && <div className="alert alert-err">{error}</div>}

      <div className="card" style={{ padding: 0, overflow: "hidden" }}>
        {loading ? (
          <div className="empty">Loading…</div>
        ) : tableRows.length === 0 ? (
          <div className="empty">
            <strong>No agents match this filter</strong>
            Seed the catalog for the default org:
            <br />
            <code className="mono" style={{ display: "inline-block", marginTop: "1rem" }}>
              ./scripts/seed-all-agents.sh
            </code>
          </div>
        ) : (
          <table className="data">
            <thead>
              <tr>
                {showOrgColumn && <th>Org</th>}
                <th>Agent ID</th>
                <th>Type</th>
                <th>Capabilities</th>
                <th>Rank</th>
                <th>Latency</th>
              </tr>
            </thead>
            <tbody>
              {tableRows.map((a, i) => (
                <tr key={`${a.org_id ?? ""}:${a.agent_id ?? i}`}>
                  {showOrgColumn && <td className="mono">{a.org_id ?? "—"}</td>}
                  <td className="mono">{a.agent_id}</td>
                  <td>
                    <span className="badge badge-neutral">{a.agent_type}</span>
                  </td>
                  <td>{(a.capabilities ?? []).join(", ") || "—"}</td>
                  <td>{a.rank_score != null ? a.rank_score.toFixed(1) : "—"}</td>
                  <td>{a.estimated_latency_ms != null ? `${Math.round(a.estimated_latency_ms)} ms` : "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}
