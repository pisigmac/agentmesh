"use client";

import { FormEvent, Suspense, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { login } from "@/lib/api";

function LoginForm() {
  const router = useRouter();
  const params = useSearchParams();
  const next = params.get("next") || "/dashboard";
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await login(email, password);
      router.replace(next);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <form className="card auth-card" onSubmit={onSubmit}>
      <h1 className="page-title">Sign in</h1>
      <p className="page-desc">AgentMesh console — email/password JWT, or register a new account.</p>
      {error && <div className="alert alert-err">{error}</div>}
      <div className="field">
        <label htmlFor="email">Email</label>
        <input id="email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
      </div>
      <div className="field">
        <label htmlFor="password">Password</label>
        <input
          id="password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          minLength={8}
        />
      </div>
      <button type="submit" className="btn" disabled={loading}>
        {loading ? "Signing in…" : "Sign in"}
      </button>
      <p style={{ marginTop: "1rem", fontSize: "0.875rem" }}>
        No account? <Link href="/register">Register</Link>
      </p>
    </form>
  );
}

export default function LoginPage() {
  return (
    <div className="auth-page">
      <Suspense fallback={<div className="card auth-card">Loading…</div>}>
        <LoginForm />
      </Suspense>
    </div>
  );
}
