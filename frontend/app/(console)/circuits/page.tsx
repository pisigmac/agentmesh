"use client";

import { useState } from "react";
import { CircuitState, getCircuitState } from "@/lib/api";

export default function CircuitsPage() {
  const [agentId, setAgentId] = useState("");
  const [state, setState] = useState<CircuitState | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function lookup(e: React.FormEvent) {
    e.preventDefault();
    if (!agentId.trim()) return;
    setLoading(true);
    setError(null);
    setState(null);
    try {
      setState(await getCircuitState(agentId.trim()));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Lookup failed");
    } finally {
      setLoading(false);
    }
  }

  function badgeFor(s?: string) {
    if (s === "open") return "badge-err";
    if (s === "half-open") return "badge-warn";
    return "badge-ok";
  }

  return (
    <>
      <h1 className="page-title">Circuit breakers</h1>
      <p className="page-desc">
        Per-destination breaker state in Redis. Thresholds come from org policy (by agent type or
        destination ID).
      </p>

      <form className="card" onSubmit={lookup}>
        <div className="field">
          <label htmlFor="dest">Destination agent ID</label>
          <input
            id="dest"
            value={agentId}
            onChange={(e) => setAgentId(e.target.value)}
            placeholder="cursor-1"
            required
          />
        </div>
        <button type="submit" className="btn" disabled={loading}>
          {loading ? "Loading…" : "Get state"}
        </button>
      </form>

      {error && <div className="alert alert-err">{error}</div>}

      {state && (
        <div className="card">
          <h2>State</h2>
          <p style={{ margin: "0 0 1rem" }}>
            <span className={`badge ${badgeFor(state.state)}`}>{state.state ?? "closed"}</span>
          </p>
          <table className="data">
            <tbody>
              <tr>
                <th>Failures (window)</th>
                <td>
                  {state.failure_count} / {state.threshold}
                </td>
              </tr>
              <tr>
                <th>Window</th>
                <td>{state.window_seconds}s</td>
              </tr>
              {state.opened_at_unix ? (
                <tr>
                  <th>Opened at</th>
                  <td>{new Date(state.opened_at_unix * 1000).toLocaleString()}</td>
                </tr>
              ) : null}
            </tbody>
          </table>
          {state.state === "closed" && (state.failure_count ?? 0) === 0 && (
            <p style={{ marginTop: "1rem", color: "var(--ink-muted)", fontSize: "0.875rem" }}>
              No recent failures recorded for this destination.
            </p>
          )}
        </div>
      )}

      {!state && !error && (
        <div className="card">
          <p className="empty" style={{ padding: "1rem 0" }}>
            Enter a destination agent ID to inspect breaker state.
          </p>
        </div>
      )}
    </>
  );
}
