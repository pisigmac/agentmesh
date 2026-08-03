"use client";

import { useEffect, useState } from "react";
import { getApiBase, getApiKey, getOrgId, saveSettings } from "@/lib/config";

export default function SettingsPage() {
  const [base, setBase] = useState("");
  const [org, setOrg] = useState("");
  const [key, setKey] = useState("");
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    setBase(getApiBase());
    setOrg(getOrgId());
    setKey(getApiKey());
  }, []);

  function onSave(e: React.FormEvent) {
    e.preventDefault();
    saveSettings(base.trim(), org.trim(), key.trim());
    setSaved(true);
    setTimeout(() => setSaved(false), 2500);
  }

  return (
    <>
      <h1 className="page-title">Settings</h1>
      <p className="page-desc">
        Connection to the control plane REST API. Uses Bearer token and <span className="mono">X-Org-ID</span>{" "}
        (KubeMind-compatible). Stored in browser localStorage only.
      </p>

      <form className="card" onSubmit={onSave}>
        <div className="field">
          <label htmlFor="base">Control plane URL</label>
          <input id="base" value={base} onChange={(e) => setBase(e.target.value)} placeholder="http://localhost:48080" />
        </div>
        <div className="field">
          <label htmlFor="org">Org ID</label>
          <input id="org" value={org} onChange={(e) => setOrg(e.target.value)} placeholder="default" />
        </div>
        <div className="field">
          <label htmlFor="key">API key</label>
          <input id="key" type="password" value={key} onChange={(e) => setKey(e.target.value)} autoComplete="off" />
        </div>
        <button type="submit" className="btn">
          Save
        </button>
        {saved && (
          <span style={{ marginLeft: "1rem", color: "var(--ok)", fontSize: "0.875rem", fontWeight: 600 }}>
            Saved
          </span>
        )}
      </form>

      <div className="card">
        <h2>Clerk SSO</h2>
        <p style={{ margin: 0, color: "var(--ink-muted)", fontSize: "0.875rem" }}>
          Production dashboards can set <span className="mono">AUTH_MODE=clerk</span> on the control plane.
          This UI uses org API keys for local development.
        </p>
      </div>
    </>
  );
}
