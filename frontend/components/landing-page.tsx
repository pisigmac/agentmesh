import Link from "next/link";

const ADAPTERS = [
  "Cursor",
  "Claude Code",
  "GitHub Copilot",
  "OpenAI Codex",
  "Aider",
  "Continue",
  "Windsurf",
  "Devin",
  "Custom",
];

const FEATURES = [
  {
    title: "Registry & discovery",
    body: "Agents register in etcd with leases. Orchestrators discover by capability, cost ceiling, and routing strategy—no hard-coded vendor URLs.",
    tone: "brand",
  },
  {
    title: "Policy-driven routing",
    body: "Org policy in etcd sets default strategies, rate limits, and per-type circuit thresholds. Change behavior without redeploying agents.",
    tone: "route",
  },
  {
    title: "Circuit breakers",
    body: "Redis-backed breakers isolate failing destinations. Half-open probes recover automatically when traffic stabilizes.",
    tone: "ok",
  },
  {
    title: "Data plane proxy",
    body: "Sidecar gateway applies route decisions, retries, timeouts, and TraceLens spans on the hot path while the control plane stays advisory.",
    tone: "accent",
  },
];

const STEPS = [
  {
    n: "01",
    title: "Register agents",
    body: "POST /v1/agents/register with type, endpoint, and capabilities. Leases keep the catalog honest when nodes disappear.",
  },
  {
    n: "02",
    title: "Resolve routes",
    body: "POST /v1/route selects a destination by strategy—least latency, least cost, score-based, or capability match with fallbacks.",
  },
  {
    n: "03",
    title: "Operate with evidence",
    body: "Prometheus metrics, ClickHouse rollups, and TraceLens traces show what routed where and why a circuit opened.",
  },
];

export function LandingPage() {
  return (
    <div className="landing">
      <header className="landing-header">
        <Link href="/" className="landing-logo">
          <span className="landing-logo-mark" aria-hidden />
          AgentMesh
        </Link>
        <nav className="landing-nav" aria-label="Primary">
          <a href="#features">Features</a>
          <a href="#how-it-works">How it works</a>
          <a href="#adapters">Adapters</a>
        </nav>
        <Link href="/dashboard" className="btn landing-header-cta">
          Open console
        </Link>
      </header>

      <section className="landing-hero">
        <div className="landing-hero-inner">
          <p className="landing-eyebrow">Control plane for multi-agent development</p>
          <h1 className="landing-headline">
            Route coding agents like traffic—<em>with policy, not guesswork.</em>
          </h1>
          <p className="landing-lede">
            AgentMesh is a Go service mesh for Cursor, Claude Code, Copilot, and seven other adapter
            types. One registry, one routing API, circuit breakers, and org-scoped policy for teams
            running many agents at once.
          </p>
          <div className="landing-hero-actions">
            <Link href="/dashboard" className="btn">
              Launch operator console
            </Link>
            <a href="#how-it-works" className="btn btn-secondary">
              See the flow
            </a>
          </div>
          <dl className="landing-stats" aria-label="Platform highlights">
            <div>
              <dt>Adapter types</dt>
              <dd>9</dd>
            </div>
            <div>
              <dt>REST + gRPC</dt>
              <dd>/v1/*</dd>
            </div>
            <div>
              <dt>State</dt>
              <dd>etcd · Redis · CH</dd>
            </div>
          </dl>
        </div>
        <div className="landing-hero-panel" aria-hidden>
          <div className="landing-diagram">
            <div className="landing-diagram-row">
              <span className="landing-node landing-node--source">Orchestrator</span>
              <span className="landing-diagram-arrow">→</span>
              <span className="landing-node landing-node--cp">Control plane</span>
            </div>
            <div className="landing-diagram-row landing-diagram-row--sub">
              <span className="landing-diagram-label">discover · route · policy</span>
            </div>
            <div className="landing-diagram-row">
              <span className="landing-node landing-node--dp">Data plane</span>
              <span className="landing-diagram-arrow">→</span>
              <span className="landing-node landing-node--agent">Agent pool</span>
            </div>
          </div>
        </div>
      </section>

      <section id="features" className="landing-section">
        <div className="landing-section-head">
          <h2>Built for operators, not demos</h2>
          <p>Production primitives you expect from a mesh—applied to AI coding agents.</p>
        </div>
        <ul className="landing-features">
          {FEATURES.map((f) => (
            <li key={f.title} className={`landing-feature landing-feature--${f.tone}`}>
              <h3>{f.title}</h3>
              <p>{f.body}</p>
            </li>
          ))}
        </ul>
      </section>

      <section id="how-it-works" className="landing-section landing-section--muted">
        <div className="landing-section-head">
          <h2>How it works</h2>
          <p>Three calls between your orchestrator and a stable mesh.</p>
        </div>
        <ol className="landing-steps">
          {STEPS.map((s) => (
            <li key={s.n}>
              <span className="landing-step-n">{s.n}</span>
              <div>
                <h3>{s.title}</h3>
                <p>{s.body}</p>
              </div>
            </li>
          ))}
        </ol>
      </section>

      <section id="adapters" className="landing-section">
        <div className="landing-section-head">
          <h2>Nine adapter types</h2>
          <p>GuardLoop-aligned agent types register with the same schema and capability model.</p>
        </div>
        <ul className="landing-adapters">
          {ADAPTERS.map((name) => (
            <li key={name}>{name}</li>
          ))}
        </ul>
      </section>

      <section className="landing-cta-band">
        <div className="landing-cta-band-inner">
          <h2>Run the stack locally in minutes</h2>
          <p>
            <code className="mono">./scripts/start-all.sh</code> brings up etcd, Redis, ClickHouse,
            control plane, data plane, observability, and this UI.
          </p>
          <Link href="/dashboard" className="btn landing-cta-band-btn">
            Go to console
          </Link>
        </div>
      </section>

      <footer className="landing-footer">
        <span>AgentMesh</span>
        <span className="landing-footer-sep">·</span>
        <Link href="/dashboard">Console</Link>
        <span className="landing-footer-sep">·</span>
        <Link href="/settings">API settings</Link>
      </footer>
    </div>
  );
}
