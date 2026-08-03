"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ChatResult, ModelInfo, chatCompletion, listModels, listProviderSecrets } from "@/lib/api";

type Turn = { role: "user" | "assistant"; content: string };

export default function ChatPage() {
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [model, setModel] = useState("gpt-4o-mini");
  const [question, setQuestion] = useState("");
  const [system, setSystem] = useState("You are a helpful engineering assistant.");
  const [routeViaMesh, setRouteViaMesh] = useState(true);
  const [turns, setTurns] = useState<Turn[]>([]);
  const [meta, setMeta] = useState<ChatResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [readyProviders, setReadyProviders] = useState<Set<string>>(new Set());

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

  async function onSend(e: React.FormEvent) {
    e.preventDefault();
    if (!question.trim()) return;
    setLoading(true);
    setError(null);
    const userText = question.trim();
    setQuestion("");
    setTurns((t) => [...t, { role: "user", content: userText }]);
    try {
      const res = await chatCompletion({
        model,
        question: userText,
        system,
        route_via_mesh: routeViaMesh,
        source_agent_id: "console-chat",
      });
      setMeta(res);
      setTurns((t) => [...t, { role: "assistant", content: res.answer }]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Chat failed");
      setTurns((t) => t.slice(0, -1));
      setQuestion(userText);
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      <h1 className="page-title">Chat</h1>
      <p className="page-desc">
        Choose a catalog model, ask a question, and the control plane calls the provider with your
        org’s encrypted key. Mesh routing attaches destination metadata when a matching agent exists.
      </p>

      {!providerReady && selected && (
        <div className="alert alert-err">
          No API key for provider <span className="mono">{selected.provider}</span>. Save one under{" "}
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
              Ask anything. Answers come from the selected provider; keys stay on the server.
            </p>
          )}
          {turns.map((t, i) => (
            <div key={i} className={`chat-bubble chat-bubble--${t.role}`}>
              <div className="chat-bubble-role">{t.role}</div>
              <div className="chat-bubble-body">{t.content}</div>
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
          <button type="submit" className="btn" disabled={loading || !providerReady}>
            {loading ? "Running…" : "Send"}
          </button>
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
                <td>{meta.routed_via_mesh ? "yes" : "direct provider"}</td>
              </tr>
              {meta.destination_agent_id && (
                <tr>
                  <th>Destination</th>
                  <td className="mono">
                    {meta.destination_agent_id} ({meta.destination_agent_type})
                  </td>
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
