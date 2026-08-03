"use client";

import { useCallback, useEffect, useState } from "react";
import { getPolicy, putPolicy } from "@/lib/api";

export default function PolicyPage() {
  const [text, setText] = useState("");
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const doc = await getPolicy();
      setText(JSON.stringify(doc, null, 2));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load policy");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function save() {
    setMessage(null);
    setError(null);
    try {
      const doc = JSON.parse(text);
      await putPolicy(doc);
      setMessage("Policy saved to etcd.");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Invalid JSON or save failed");
    }
  }

  return (
    <>
      <h1 className="page-title">Org policy</h1>
      <p className="page-desc">
        Config-driven routing defaults, rate limits, and circuit overrides. Stored at{" "}
        <span className="mono">/agentmesh/orgs/&#123;org&#125;/policy</span>.
      </p>

      {error && <div className="alert alert-err">{error}</div>}
      {message && <div className="alert alert-ok">{message}</div>}

      <div className="card">
        {loading ? (
          <p className="empty">Loading…</p>
        ) : (
          <>
            <div className="field">
              <label htmlFor="json">Policy JSON</label>
              <textarea id="json" value={text} onChange={(e) => setText(e.target.value)} spellCheck={false} />
            </div>
            <div style={{ display: "flex", gap: "0.75rem" }}>
              <button type="button" className="btn" onClick={save}>
                Save to etcd
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
