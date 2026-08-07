"use client";

import { FormEvent, useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  AuthUser,
  KeymintStatus,
  OrgAPIKey,
  OrgMembership,
  createOrgAPIKey,
  fetchMe,
  getKeymintStatus,
  listOrgAPIKeys,
  logout,
  revokeOrgAPIKey,
} from "@/lib/api";
import { getApiBase, getApiKey, getOrgId, saveSettings, setOrgId } from "@/lib/config";
import { useRouter } from "next/navigation";

export default function SettingsPage() {
  const router = useRouter();
  const [base, setBase] = useState("");
  const [org, setOrg] = useState("");
  const [machineKey, setMachineKey] = useState("");
  const [saved, setSaved] = useState(false);
  const [keymint, setKeymint] = useState<KeymintStatus | null>(null);
  const [user, setUser] = useState<AuthUser | null>(null);
  const [memberships, setMemberships] = useState<OrgMembership[]>([]);
  const [role, setRole] = useState<string>("");
  const [keys, setKeys] = useState<OrgAPIKey[]>([]);
  const [newKeyName, setNewKeyName] = useState("ci");
  const [createdPlain, setCreatedPlain] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const loadKeys = useCallback(async (orgId: string) => {
    try {
      setKeys(await listOrgAPIKeys(orgId));
    } catch {
      setKeys([]);
    }
  }, []);

  useEffect(() => {
    setBase(getApiBase());
    setOrg(getOrgId());
    setMachineKey("");
    getKeymintStatus()
      .then(setKeymint)
      .catch(() => setKeymint({ configured: false }));
    fetchMe()
      .then((m) => {
        setUser(m.user);
        setMemberships(m.orgs ?? []);
        setRole(m.role ?? "");
        if (m.current_org) setOrg(m.current_org);
      })
      .catch(() => setUser(null));
    loadKeys(getOrgId());
  }, [loadKeys]);

  function onSave(e: FormEvent) {
    e.preventDefault();
    saveSettings(base.trim(), org.trim(), machineKey.trim());
    setSaved(true);
    setTimeout(() => setSaved(false), 2500);
  }

  async function onLogout() {
    await logout();
    router.replace("/login");
  }

  async function onCreateKey(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setCreatedPlain(null);
    try {
      const res = await createOrgAPIKey(org || getOrgId(), newKeyName);
      setCreatedPlain(res.api_key);
      await loadKeys(org || getOrgId());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Create key failed");
    }
  }

  async function onRevoke(keyId: string) {
    setError(null);
    try {
      await revokeOrgAPIKey(org || getOrgId(), keyId);
      await loadKeys(org || getOrgId());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Revoke failed");
    }
  }

  return (
    <>
      <h1 className="page-title">Settings</h1>
      <p className="page-desc">
        Native AgentMesh auth: JWT after login, or org API keys (<span className="mono">amk_…</span>) for
        machines. Control plane: <span className="mono">{getApiKey().startsWith("amk_") || getApiKey().includes(".") ? "credential set" : "using stored credential"}</span>
      </p>

      {error && <div className="alert alert-err">{error}</div>}

      <div className="card">
        <h2>Session</h2>
        {user ? (
          <p style={{ margin: "0 0 0.75rem", fontSize: "0.875rem" }}>
            Signed in as <span className="mono">{user.email}</span>
            {role ? (
              <>
                {" "}
                · role <span className="badge badge-neutral">{role}</span>
              </>
            ) : null}
          </p>
        ) : (
          <p style={{ margin: "0 0 0.75rem", fontSize: "0.875rem", color: "var(--ink-muted)" }}>
            Using API key auth (no JWT session). <Link href="/login">Sign in</Link>
          </p>
        )}
        {memberships.length > 0 && (
          <div className="field">
            <label htmlFor="mem-org">Org</label>
            <select
              id="mem-org"
              value={org}
              onChange={(e) => {
                setOrg(e.target.value);
                setOrgId(e.target.value);
                loadKeys(e.target.value);
              }}
            >
              {memberships.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.name || m.id} ({m.role})
                </option>
              ))}
            </select>
          </div>
        )}
        <button type="button" className="btn btn-secondary" onClick={onLogout}>
          Sign out
        </button>
      </div>

      <form className="card" onSubmit={onSave}>
        <h2>Connection</h2>
        <div className="field">
          <label htmlFor="base">Control plane URL</label>
          <input id="base" value={base} onChange={(e) => setBase(e.target.value)} placeholder="http://localhost:48080" />
        </div>
        <div className="field">
          <label htmlFor="org">Org ID (X-Org-ID)</label>
          <input id="org" value={org} onChange={(e) => setOrg(e.target.value)} placeholder="default" />
        </div>
        <div className="field">
          <label htmlFor="key">Machine API key override (optional)</label>
          <input
            id="key"
            type="password"
            value={machineKey}
            onChange={(e) => setMachineKey(e.target.value)}
            autoComplete="off"
            placeholder="amk_… or leave blank to keep JWT"
          />
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
        <h2>Org API keys</h2>
        <p style={{ fontSize: "0.8125rem", color: "var(--ink-muted)" }}>
          For CLI/scripts. Plaintext shown once on create. Requires operator+ role.
        </p>
        <form onSubmit={onCreateKey} style={{ display: "flex", gap: "0.75rem", flexWrap: "wrap", marginBottom: "1rem" }}>
          <input value={newKeyName} onChange={(e) => setNewKeyName(e.target.value)} placeholder="name" />
          <button type="submit" className="btn">
            Create key
          </button>
        </form>
        {createdPlain && (
          <div className="alert alert-ok">
            Copy now: <span className="mono">{createdPlain}</span>
          </div>
        )}
        <table className="data">
          <thead>
            <tr>
              <th>Name</th>
              <th>ID</th>
              <th>Role</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {keys.map((k) => (
              <tr key={k.key_id}>
                <td>{k.name}</td>
                <td className="mono">{k.key_id}</td>
                <td>{k.role}</td>
                <td>
                  {!k.revoked_at && (
                    <button type="button" className="btn btn-secondary" onClick={() => onRevoke(k.key_id)}>
                      Revoke
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="card">
        <h2>Keymint</h2>
        {keymint == null ? (
          <p style={{ margin: 0, color: "var(--ink-muted)", fontSize: "0.875rem" }}>Checking…</p>
        ) : keymint.configured ? (
          <p style={{ margin: 0, fontSize: "0.875rem" }}>
            <span className="badge badge-ok">Connected</span>{" "}
            <span className="mono">{keymint.base_url}</span>
          </p>
        ) : (
          <p style={{ margin: 0, color: "var(--ink-muted)", fontSize: "0.875rem" }}>
            <span className="badge badge-warn">Not configured</span> Set{" "}
            <span className="mono">KEYMINT_BASE_URL</span>, <span className="mono">KEYMINT_API_KEY</span>,{" "}
            <span className="mono">KEYMINT_TEAM_ID</span> on the control plane for Models & Chat.
          </p>
        )}
      </div>
    </>
  );
}
