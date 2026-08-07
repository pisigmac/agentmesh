"use client";

import { useCallback, useEffect, useState } from "react";
import { getPolicy, putPolicy } from "@/lib/api";

type RoutingForm = {
  default_strategy: string;
  max_cost_usd: string;
  max_latency_ms: string;
  prefer_free: boolean;
  prefer_local: boolean;
  default_timeout_ms: string;
  default_max_retries: string;
};

const emptyRouting: RoutingForm = {
  default_strategy: "capability-match",
  max_cost_usd: "",
  max_latency_ms: "",
  prefer_free: false,
  prefer_local: false,
  default_timeout_ms: "30000",
  default_max_retries: "2",
};

function routingFromDoc(doc: Record<string, unknown>): RoutingForm {
  const r = (doc.routing ?? {}) as Record<string, unknown>;
  const maxCost =
    typeof r.max_cost_usd === "number"
      ? r.max_cost_usd
      : typeof r.default_max_cost === "number"
        ? r.default_max_cost
        : "";
  return {
    default_strategy: String(r.default_strategy ?? "capability-match"),
    max_cost_usd: maxCost === "" ? "" : String(maxCost),
    max_latency_ms: r.max_latency_ms != null ? String(r.max_latency_ms) : "",
    prefer_free: Boolean(r.prefer_free),
    prefer_local: Boolean(r.prefer_local),
    default_timeout_ms: r.default_timeout_ms != null ? String(r.default_timeout_ms) : "30000",
    default_max_retries: r.default_max_retries != null ? String(r.default_max_retries) : "2",
  };
}

function applyRouting(doc: Record<string, unknown>, form: RoutingForm): Record<string, unknown> {
  const routing: Record<string, unknown> = {
    ...((doc.routing as Record<string, unknown>) ?? {}),
    default_strategy: form.default_strategy,
    prefer_free: form.prefer_free,
    prefer_local: form.prefer_local,
  };
  const maxCost = parseFloat(form.max_cost_usd);
  if (form.max_cost_usd.trim() !== "" && !Number.isNaN(maxCost) && maxCost > 0) {
    routing.max_cost_usd = maxCost;
    routing.default_max_cost = maxCost;
  } else {
    delete routing.max_cost_usd;
    delete routing.default_max_cost;
  }
  const maxLat = parseFloat(form.max_latency_ms);
  if (form.max_latency_ms.trim() !== "" && !Number.isNaN(maxLat) && maxLat > 0) {
    routing.max_latency_ms = maxLat;
  } else {
    delete routing.max_latency_ms;
  }
  const timeout = parseInt(form.default_timeout_ms, 10);
  if (!Number.isNaN(timeout) && timeout > 0) routing.default_timeout_ms = timeout;
  const retries = parseInt(form.default_max_retries, 10);
  if (!Number.isNaN(retries) && retries >= 0) routing.default_max_retries = retries;
  return { ...doc, routing };
}

export default function PolicyPage() {
  const [doc, setDoc] = useState<Record<string, unknown> | null>(null);
  const [routing, setRouting] = useState<RoutingForm>(emptyRouting);
  const [text, setText] = useState("");
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const raw = (await getPolicy()) as Record<string, unknown>;
      setDoc(raw);
      setRouting(routingFromDoc(raw));
      setText(JSON.stringify(raw, null, 2));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load policy");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function saveFromForm() {
    setMessage(null);
    setError(null);
    try {
      const base = doc ?? {};
      const next = applyRouting(base, routing);
      await putPolicy(next);
      setMessage("Policy SLOs saved to etcd.");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Save failed");
    }
  }

  async function saveJSON() {
    setMessage(null);
    setError(null);
    try {
      const parsed = JSON.parse(text) as Record<string, unknown>;
      await putPolicy(parsed);
      setMessage("Policy JSON saved to etcd.");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Invalid JSON or save failed");
    }
  }

  return (
    <>
      <h1 className="page-title">Org policy</h1>
      <p className="page-desc">
        Routing SLOs (cost/latency), preferences, rate limits, and circuit overrides. Stored at{" "}
        <span className="mono">/agentmesh/orgs/&#123;org&#125;/policy</span>.
      </p>

      {error && <div className="alert alert-err">{error}</div>}
      {message && <div className="alert alert-ok">{message}</div>}

      <div className="card">
        <h2>Routing SLOs</h2>
        <p style={{ margin: "0 0 1rem", color: "var(--ink-muted)", fontSize: "0.875rem" }}>
          Hard filters drop candidates over budget before rank. Prefer free/local soft-ranks matching agents
          first (<span className="mono">cost=0</span> / <span className="mono">localhost</span> or{" "}
          <span className="mono">metadata.local=true</span>).
        </p>
        {loading ? (
          <p className="empty">Loading…</p>
        ) : (
          <>
            <div style={{ display: "grid", gap: "0.75rem", gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))" }}>
              <div className="field" style={{ margin: 0 }}>
                <label htmlFor="strategy">Default strategy</label>
                <select
                  id="strategy"
                  value={routing.default_strategy}
                  onChange={(e) => setRouting({ ...routing, default_strategy: e.target.value })}
                >
                  <option value="capability-match">capability-match</option>
                  <option value="least-latency">least-latency</option>
                  <option value="least-cost">least-cost</option>
                  <option value="round-robin">round-robin</option>
                  <option value="score-based">score-based</option>
                </select>
              </div>
              <div className="field" style={{ margin: 0 }}>
                <label htmlFor="maxcost">Max cost USD</label>
                <input
                  id="maxcost"
                  type="number"
                  step="0.001"
                  min="0"
                  placeholder="e.g. 0.02"
                  value={routing.max_cost_usd}
                  onChange={(e) => setRouting({ ...routing, max_cost_usd: e.target.value })}
                />
              </div>
              <div className="field" style={{ margin: 0 }}>
                <label htmlFor="maxlat">Max latency ms</label>
                <input
                  id="maxlat"
                  type="number"
                  step="1"
                  min="0"
                  placeholder="e.g. 500"
                  value={routing.max_latency_ms}
                  onChange={(e) => setRouting({ ...routing, max_latency_ms: e.target.value })}
                />
              </div>
              <div className="field" style={{ margin: 0 }}>
                <label htmlFor="timeout">Default timeout ms</label>
                <input
                  id="timeout"
                  type="number"
                  value={routing.default_timeout_ms}
                  onChange={(e) => setRouting({ ...routing, default_timeout_ms: e.target.value })}
                />
              </div>
              <div className="field" style={{ margin: 0 }}>
                <label htmlFor="retries">Default max retries</label>
                <input
                  id="retries"
                  type="number"
                  value={routing.default_max_retries}
                  onChange={(e) => setRouting({ ...routing, default_max_retries: e.target.value })}
                />
              </div>
            </div>
            <div style={{ display: "flex", gap: "1.25rem", marginTop: "0.75rem", flexWrap: "wrap" }}>
              <label style={{ display: "flex", alignItems: "center", gap: "0.4rem", fontSize: "0.875rem" }}>
                <input
                  type="checkbox"
                  checked={routing.prefer_free}
                  onChange={(e) => setRouting({ ...routing, prefer_free: e.target.checked })}
                />
                Prefer free
              </label>
              <label style={{ display: "flex", alignItems: "center", gap: "0.4rem", fontSize: "0.875rem" }}>
                <input
                  type="checkbox"
                  checked={routing.prefer_local}
                  onChange={(e) => setRouting({ ...routing, prefer_local: e.target.checked })}
                />
                Prefer local
              </label>
            </div>
            <div style={{ display: "flex", gap: "0.75rem", marginTop: "1rem" }}>
              <button type="button" className="btn" onClick={saveFromForm}>
                Save SLOs
              </button>
              <button type="button" className="btn btn-secondary" onClick={load}>
                Reload
              </button>
            </div>
          </>
        )}
      </div>

      <div className="card">
        <h2>Full policy JSON</h2>
        {loading ? (
          <p className="empty">Loading…</p>
        ) : (
          <>
            <div className="field">
              <label htmlFor="json">Policy JSON</label>
              <textarea id="json" value={text} onChange={(e) => setText(e.target.value)} spellCheck={false} />
            </div>
            <div style={{ display: "flex", gap: "0.75rem" }}>
              <button type="button" className="btn" onClick={saveJSON}>
                Save JSON
              </button>
              <button type="button" className="btn btn-secondary" onClick={load}>
                Reload
              </button>
            </div>
          </>
        )}
      </div>
    </>
  );
}
