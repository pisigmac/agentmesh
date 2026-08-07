"use client";

import { useCallback, useEffect, useState } from "react";
import {
  ProviderInfo,
  ProviderSecretView,
  deleteProviderSecret,
  listProviderSecrets,
  listProviders,
  putProviderSecret,
} from "@/lib/api";

export default function ModelsPage() {
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [secrets, setSecrets] = useState<ProviderSecretView[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const [provider, setProvider] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [baseURL, setBaseURL] = useState("");
  const [apiModel, setApiModel] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [prov, secs] = await Promise.all([listProviders(), listProviderSecrets()]);
      setProviders(prov);
      setSecrets(secs);
      if (!provider && prov.length) setProvider(prov[0].id);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, [provider]);

  useEffect(() => {
    load();
  }, [load]);

  const selected = providers.find((p) => p.id === provider);

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    setMessage(null);
    setError(null);
    try {
      await putProviderSecret(provider, {
        api_key: apiKey || undefined,
        base_url: baseURL || undefined,
        api_model: apiModel || undefined,
      });
      setApiKey("");
      setMessage(`Saved ${provider} credentials in Keymint.`);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    }
  }

  async function onDelete(p: string) {
    setError(null);
    setMessage(null);
    try {
      await deleteProviderSecret(p);
      setMessage(`Removed ${p} key.`);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Delete failed");
    }
  }

  return (
    <>
      <h1 className="page-title">Models & keys</h1>
      <p className="page-desc">
        Providers and models are loaded from <span className="mono">deploy/models.yaml</span>. Provider
        API keys are stored in Keymint (vault + virtual keys); AgentMesh never keeps plaintext provider
        keys in etcd.
      </p>

      {error && <div className="alert alert-err">{error}</div>}
      {message && <div className="alert alert-ok">{message}</div>}

      <div className="card">
        <h2>Configured providers</h2>
        {loading ? (
          <p className="empty">Loading…</p>
        ) : (
          <table className="data">
            <thead>
              <tr>
                <th>Provider</th>
                <th>Status</th>
                <th>Hint</th>
                <th>Base URL</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {secrets.map((s) => (
                <tr key={s.provider}>
                  <td>
                    <span className="mono">{s.provider}</span>
                    {s.label ? ` · ${s.label}` : ""}
                  </td>
                  <td>
                    {s.configured ? (
                      <span className="badge badge-ok">Configured in Keymint</span>
                    ) : (
                      <span className="badge badge-warn">Missing</span>
                    )}
                  </td>
                  <td className="mono">{s.key_hint || "—"}</td>
                  <td className="mono">{s.base_url || "—"}</td>
                  <td>
                    {s.configured && (
                      <button type="button" className="btn btn-secondary" onClick={() => onDelete(s.provider)}>
                        Remove
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <form className="card" onSubmit={onSave}>
        <h2>Save provider key</h2>
        <div className="field">
          <label htmlFor="prov">Provider</label>
          <select id="prov" value={provider} onChange={(e) => setProvider(e.target.value)} required>
            {providers.map((p) => (
              <option key={p.id} value={p.id}>
                {p.label || p.id}
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <label htmlFor="key">API key</label>
          <input
            id="key"
            type="password"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder={selected?.key_hint || "API key"}
            autoComplete="off"
          />
          <p style={{ margin: "0.35rem 0 0", fontSize: "0.75rem", color: "var(--ink-muted)" }}>
            Leave blank to keep the existing key when updating base URL / model only.
          </p>
        </div>
        <div className="field">
          <label htmlFor="base">Base URL {selected?.requires_base_url ? "(required)" : "(optional)"}</label>
          <input
            id="base"
            value={baseURL}
            onChange={(e) => setBaseURL(e.target.value)}
            placeholder={selected?.default_base_url || "https://…"}
          />
        </div>
        {selected?.requires_base_url && (
          <div className="field">
            <label htmlFor="am">API model name</label>
            <input
              id="am"
              value={apiModel}
              onChange={(e) => setApiModel(e.target.value)}
              placeholder="e.g. llama-3.1-70b"
            />
          </div>
        )}
        <button type="submit" className="btn">
          Save to Keymint
        </button>
      </form>

      <div className="card">
        <h2>Catalog</h2>
        <p style={{ margin: 0, color: "var(--ink-muted)", fontSize: "0.875rem" }}>
          Edit <span className="mono">deploy/models.yaml</span>, remount/restart control plane. Chat
          picks models from <span className="mono">GET /v1/models</span>. Optional mesh agents:{" "}
          <span className="mono">./scripts/seed-model-agents.sh</span>.
        </p>
      </div>
    </>
  );
}
