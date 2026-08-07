"use client";

import { useState } from "react";
import { RouteResponse, routeRequest } from "@/lib/api";

const STRATEGIES = [
  "capability-match",
  "least-latency",
  "least-cost",
  "round-robin",
  "score-based",
];

export default function RoutesPage() {
  const [source, setSource] = useState("cursor-1");
  const [capability, setCapability] = useState("code_gen");
  const [strategy, setStrategy] = useState("");
  const [destination, setDestination] = useState("");
  const [prefer, setPrefer] = useState("");
  const [forcePin, setForcePin] = useState(false);
  const [pinFailover, setPinFailover] = useState(false);
  const [result, setResult] = useState<RouteResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const preferIds = prefer
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean);
      const res = await routeRequest({
        source_agent_id: source,
        capability,
        routing_strategy: strategy || undefined,
        destination_agent_id: destination.trim() || undefined,
        prefer_agent_ids: preferIds.length ? preferIds : undefined,
        force_pin: forcePin || undefined,
        pin_failover: pinFailover || undefined,
      });
      setResult(res);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Route failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      <h1 className="page-title">Route debugger</h1>
      <p className="page-desc">
        Calls <span className="mono">POST /v1/route</span> on the control plane. Empty strategy uses
        org policy defaults. Set a destination agent ID to hard-pin.
      </p>

      <form className="card" onSubmit={onSubmit}>
        <div className="field">
          <label htmlFor="src">Source agent ID</label>
          <input id="src" value={source} onChange={(e) => setSource(e.target.value)} required />
        </div>
        <div className="field">
          <label htmlFor="cap">Capability</label>
          <input id="cap" value={capability} onChange={(e) => setCapability(e.target.value)} required />
        </div>
        <div className="field">
          <label htmlFor="strat">Routing strategy</label>
          <select id="strat" value={strategy} onChange={(e) => setStrategy(e.target.value)}>
            <option value="">(org default)</option>
            {STRATEGIES.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <label htmlFor="dest">Destination agent ID (pin)</label>
          <input
            id="dest"
            value={destination}
            onChange={(e) => setDestination(e.target.value)}
            placeholder="optional hard pin"
          />
        </div>
        <div className="field">
          <label htmlFor="prefer">Prefer agent IDs (comma-separated)</label>
          <input
            id="prefer"
            value={prefer}
            onChange={(e) => setPrefer(e.target.value)}
            placeholder="optional soft preference"
          />
        </div>
        <label style={{ display: "flex", gap: "0.5rem", alignItems: "center", fontSize: "0.875rem", marginBottom: "0.75rem" }}>
          <input type="checkbox" checked={forcePin} onChange={(e) => setForcePin(e.target.checked)} />
          Force pin (ignore unhealthy / capability)
        </label>
        <label style={{ display: "flex", gap: "0.5rem", alignItems: "center", fontSize: "0.875rem", marginBottom: "1rem" }}>
          <input type="checkbox" checked={pinFailover} onChange={(e) => setPinFailover(e.target.checked)} />
          Pin failover (if circuit open, rank normally)
        </label>
        <button type="submit" className="btn" disabled={loading}>
          {loading ? "Routing…" : "Resolve route"}
        </button>
      </form>

      {error && <div className="alert alert-err">{error}</div>}

      {result && (
        <div className="card">
          <h2>Route response</h2>
          <p style={{ margin: "0 0 0.75rem" }}>
            Destination{" "}
            <span className="badge badge-neutral">{result.destination_agent_type}</span>{" "}
            <span className="mono">{result.destination_agent_id}</span>
            {result.injected_headers?.["x-agentmesh-pin"] === "true" && (
              <>
                {" "}
                <span className="badge badge-ok">pinned</span>
              </>
            )}
          </p>
          <p className="mono" style={{ wordBreak: "break-all", color: "var(--ink-muted)" }}>
            {result.endpoint_url}
          </p>
          <p style={{ fontSize: "0.8125rem", color: "var(--ink-muted)" }}>
            Trace: {result.trace_context} · Timeout {result.timeout_ms}ms · Retries {result.max_retries}
          </p>
          <pre className="mono" style={{ margin: "1rem 0 0", overflow: "auto", fontSize: "0.75rem" }}>
            {JSON.stringify(result, null, 2)}
          </pre>
        </div>
      )}
    </>
  );
}
