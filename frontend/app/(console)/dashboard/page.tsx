"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { discoverAgents, fetchHealth } from "@/lib/api";
import { getApiBase } from "@/lib/config";

export default function DashboardPage() {
  const [health, setHealth] = useState<"loading" | "ok" | "down">("loading");
  const [agentCount, setAgentCount] = useState<number | null>(null);

  useEffect(() => {
    fetchHealth()
      .then((h) => setHealth(h.status === "ok" ? "ok" : "down"))
      .catch(() => setHealth("down"));

    discoverAgents()
      .then((a) => setAgentCount(a.length))
      .catch(() => setAgentCount(0));
  }, []);

  return (
    <>
      <h1 className="page-title">Overview</h1>
      <p className="page-desc">
        Registry health, discovery, and routing for inter-agent traffic. Data plane and traces run
        separately (TraceLens, Prometheus).
      </p>

      <div className="grid-2">
        <div className="stat">
          <div className="stat-label">Control plane</div>
          <div className="stat-value" style={{ fontSize: "1.25rem" }}>
            {health === "loading" && "…"}
            {health === "ok" && (
              <span className="badge badge-ok" style={{ fontSize: "0.875rem" }}>
                Healthy
              </span>
            )}
            {health === "down" && (
              <span className="badge badge-err" style={{ fontSize: "0.875rem" }}>
                Unreachable
              </span>
            )}
          </div>
          <p className="mono" style={{ margin: "0.75rem 0 0", color: "var(--ink-muted)" }}>
            {getApiBase()}
          </p>
        </div>
        <div className="stat">
          <div className="stat-label">Registered agents</div>
          <div className="stat-value">{agentCount ?? "—"}</div>
        </div>
        <div className="stat">
          <div className="stat-label">Default strategy</div>
          <div className="stat-value" style={{ fontSize: "1rem", fontWeight: 500 }}>
            From org policy
          </div>
          <p style={{ margin: "0.5rem 0 0", fontSize: "0.8125rem", color: "var(--ink-muted)" }}>
            <Link href="/policy">View policy →</Link>
          </p>
        </div>
      </div>

      <div className="card">
        <h2>Quick actions</h2>
        <p style={{ margin: "0 0 1rem", color: "var(--ink-muted)", fontSize: "0.875rem" }}>
          Register agents with the CLI, then discover by capability or debug a route.
        </p>
        <div style={{ display: "flex", gap: "0.75rem", flexWrap: "wrap" }}>
          <Link href="/agents" className="btn">
            Browse agents
          </Link>
          <Link href="/routes" className="btn btn-secondary">
            Debug route
          </Link>
        </div>
      </div>
    </>
  );
}
