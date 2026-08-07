"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import {
  ChatResult,
  ModelInfo,
  chatCompletionStream,
  listModels,
  listProviderSecrets,
} from "@/lib/api";

type Turn = { role: "user" | "assistant"; content: string };

export default function ChatPage() {
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [model, setModel] = useState("gpt-4o-mini");
  const [question, setQuestion] = useState("");
  const [system, setSystem] = useState("You are a helpful engineering assistant.");
  const [routeViaMesh, setRouteViaMesh] = useState(true);
  const [destination, setDestination] = useState("");
  const [prefer, setPrefer] = useState("");
  const [forcePin, setForcePin] = useState(false);
  const [pinFailover, setPinFailover] = useState(false);
  const [turns, setTurns] = useState<Turn[]>([]);
  const [meta, setMeta] = useState<Partial<ChatResult> | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [readyProviders, setReadyProviders] = useState<Set<string>>(new Set());
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    listModels()
      .then((m) => {
        setModels(m);
        if (m.length && !m.some((x) => x.id === model)) setModel(m[0].id);
      })
      .catch(() => setModels([]));
    listProviderSecrets()
      .then((s) => setReadyProviders(new Set(s.filter((x) => x.configured).map((x) => x.provider))))
      .catch(() => setReadyProviders(new Set()));
  }, []);

  const selected = models.find((m) => m.id === model);
  const providerReady = selected ? readyProviders.has(selected.provider) : false;

  function onCancel() {
    abortRef.current?.abort();
    abortRef.current = null;
    setLoading(false);
  }

  async function onSend(e: React.FormEvent) {
    e.preventDefault();
    if (!question.trim()) return;
    setLoading(true);
    setError(null);
    const userText = question.trim();
    setQuestion("");
    setTurns((t) => [...t, { role: "user", content: userText }, { role: "assistant", content: "" }]);
    const ac = new AbortController();
    abortRef.current = ac;
    let assistant = "";
    try {
      const preferIds = prefer
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean);
      await chatCompletionStream(
        {
          model,
          question: userText,
          system,
          route_via_mesh: routeViaMesh,
          source_agent_id: "console-chat",
          destination_agent_id: destination.trim() || undefined,
          prefer_agent_ids: preferIds.length ? preferIds : undefined,
          force_pin: forcePin || undefined,
          pin_failover: pinFailover || undefined,
        },
        {
          signal: ac.signal,
          onMeta: (m) => {
            setMeta({
              model_id: String(m.model_id ?? model),
              provider: String(m.provider ?? selected?.provider ?? ""),
              api_model: selected?.api_model ?? "",
              destination_agent_id: m.destination_agent_id ? String(m.destination_agent_id) : undefined,
              trace_id: m.trace_id ? String(m.trace_id) : undefined,
              routed_via_mesh: Boolean(m.routed_via_mesh),
            });
          },
          onToken: (text) => {
            assistant += text;
            const snapshot = assistant;
            setTurns((t) => {
              const next = [...t];
              next[next.length - 1] = { role: "assistant", content: snapshot };
              return next;
            });
          },
          onError: (message) => {
            setError(message);
          },
          onDone: () => {
            /* stream finished */
          },
        },
      );
      if (!assistant) {
        setError("Empty stream response");
        setTurns((t) => t.slice(0, -2));
        setQuestion(userText);
      }
    } catch (err) {
      if ((err as Error).name === "AbortError") {
        setError("Cancelled");
      } else {
        setError(err instanceof Error ? err.message : "Chat failed");
        setTurns((t) => t.slice(0, -2));
        setQuestion(userText);
      }
    } finally {
      abortRef.current = null;
      setLoading(false);
    }
  }

  return (
    <>
      <h1 className="page-title">Chat</h1>
      <p className="page-desc">
        Streaming chat via Keymint proxy. Mesh routing attaches destination metadata when a matching
        agent exists; optional pin forces a destination agent.
      </p>

      {!providerReady && selected && (
        <div className="alert alert-err">
          No Keymint key for provider <span className="mono">{selected.provider}</span>. Save one under{" "}
          <Link href="/models">Models & keys</Link>.
        </div>
      )}
      {error && <div className="alert alert-err">{error}</div>}

      <div className="card">
        <div style={{ display: "flex", gap: "1rem", flexWrap: "wrap", alignItems: "flex-end" }}>
          <div className="field" style={{ margin: 0, minWidth: "240px" }}>
            <label htmlFor="model">Model</label>
            <select id="model" value={model} onChange={(e) => setModel(e.target.value)}>
              {models.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.label} · {m.provider}
                </option>
              ))}
            </select>
          </div>
          <label style={{ display: "flex", gap: "0.5rem", alignItems: "center", fontSize: "0.875rem" }}>
            <input
              type="checkbox"
              checked={routeViaMesh}
              onChange={(e) => setRouteViaMesh(e.target.checked)}
            />
            Route via mesh ({selected?.capability ?? "—"})
          </label>
        </div>
        <div style={{ display: "flex", gap: "1rem", flexWrap: "wrap", marginTop: "0.75rem", alignItems: "flex-end" }}>
          <div className="field" style={{ margin: 0, minWidth: "220px" }}>
            <label htmlFor="pin">Pin agent ID</label>
            <input
              id="pin"
              value={destination}
              onChange={(e) => setDestination(e.target.value)}
              placeholder="optional hard pin"
              disabled={!routeViaMesh}
            />
          </div>
          <div className="field" style={{ margin: 0, minWidth: "240px" }}>
            <label htmlFor="prefer">Prefer agent IDs</label>
            <input
              id="prefer"
              value={prefer}
              onChange={(e) => setPrefer(e.target.value)}
              placeholder="comma-separated soft preference"
              disabled={!routeViaMesh}
            />
          </div>
          <label style={{ display: "flex", gap: "0.5rem", alignItems: "center", fontSize: "0.875rem" }}>
            <input
              type="checkbox"
              checked={forcePin}
              onChange={(e) => setForcePin(e.target.checked)}
              disabled={!routeViaMesh}
            />
            Force pin
          </label>
          <label style={{ display: "flex", gap: "0.5rem", alignItems: "center", fontSize: "0.875rem" }}>
            <input
              type="checkbox"
              checked={pinFailover}
              onChange={(e) => setPinFailover(e.target.checked)}
              disabled={!routeViaMesh}
            />
            Pin failover
          </label>
        </div>
        {selected && (
          <p style={{ margin: "0.75rem 0 0", fontSize: "0.8125rem", color: "var(--ink-muted)" }}>
            {selected.description} · API model <span className="mono">{selected.api_model}</span>
          </p>
        )}
      </div>

      <div className="card">
        <div className="field">
          <label htmlFor="sys">System prompt</label>
          <input id="sys" value={system} onChange={(e) => setSystem(e.target.value)} />
        </div>

        <div className="chat-thread">
          {turns.length === 0 && (
            <p className="empty" style={{ padding: "1rem 0" }}>
              Ask anything. Tokens stream from Keymint; provider keys stay in the vault.
            </p>
          )}
          {turns.map((t, i) => (
            <div key={i} className={`chat-bubble chat-bubble--${t.role}`}>
              <div className="chat-bubble-role">{t.role}</div>
              <div className="chat-bubble-body">{t.content || (loading && i === turns.length - 1 ? "…" : "")}</div>
            </div>
          ))}
        </div>

        <form onSubmit={onSend} style={{ marginTop: "1rem" }}>
          <div className="field">
            <label htmlFor="q">Question</label>
            <textarea
              id="q"
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
              rows={3}
              placeholder="e.g. How should I structure retries for an HTTP client in Go?"
              disabled={loading || !providerReady}
            />
          </div>
          <div style={{ display: "flex", gap: "0.75rem" }}>
            <button type="submit" className="btn" disabled={loading || !providerReady}>
              {loading ? "Streaming…" : "Send"}
            </button>
            {loading && (
              <button type="button" className="btn btn-secondary" onClick={onCancel}>
                Cancel
              </button>
            )}
          </div>
        </form>
      </div>

      {meta && (
        <div className="card">
          <h2>Last route / inference</h2>
          <table className="data">
            <tbody>
              <tr>
                <th>Model</th>
                <td className="mono">
                  {meta.model_id} → {meta.api_model}
                </td>
              </tr>
              <tr>
                <th>Provider</th>
                <td>{meta.provider}</td>
              </tr>
              <tr>
                <th>Mesh</th>
                <td>{meta.routed_via_mesh ? "yes" : "direct via Keymint"}</td>
              </tr>
              {meta.destination_agent_id && (
                <tr>
                  <th>Destination</th>
                  <td className="mono">{meta.destination_agent_id}</td>
                </tr>
              )}
              {meta.trace_id && (
                <tr>
                  <th>Trace</th>
                  <td className="mono">{meta.trace_id}</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
