"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { isLoggedIn } from "@/lib/config";

const ADAPTERS = [
  { name: "Cursor", logo: "/logos/cursor.svg" },
  { name: "Claude Code", logo: "/logos/claude.svg" },
  { name: "GitHub Copilot", logo: "/logos/github.svg" },
  { name: "OpenAI Codex", logo: "/logos/openai.svg" },
  { name: "Aider", logo: "/logos/aider.svg" },
  { name: "Continue", logo: "/logos/continue.svg" },
  { name: "Windsurf", logo: "/logos/windsurf.svg" },
  { name: "Devin", logo: "/logos/devin.svg" },
  { name: "Custom tools", logo: "/logos/custom.svg" },
];

const FEATURES = [
  {
    title: "Know which agents are available",
    body: "Teams add their coding agents once. Everyone else can see what’s online and what each one is good at—without chasing links around Slack.",
  },
  {
    title: "Pick the right agent for the job",
    body: "Ask for something like “write code” or “review a PR,” and AgentMesh chooses an agent that fits. You can set simple preferences for cost, speed, or a favorite tool.",
  },
  {
    title: "Skip agents that are down",
    body: "If an agent keeps failing, the next good option takes over so one broken setup doesn’t stall the whole team.",
  },
  {
    title: "See what happened",
    body: "The console shows who handled the job, how long it took, and when something was skipped—so you don’t have to guess.",
  },
];

const STEPS = [
  {
    n: "01",
    title: "Connect your agents",
    body: "Point each coding tool at AgentMesh. They stay on the list while they’re working.",
  },
  {
    n: "02",
    title: "Send the work",
    body: "Your app or the console asks AgentMesh which agent should handle a task. You get a clear answer back.",
  },
  {
    n: "03",
    title: "Watch and adjust",
    body: "Use the console to see what’s online, change team preferences, and fix issues when something drops out.",
  },
];

function AtlasBg() {
  return (
    <svg className="landing-atlas" viewBox="0 0 1440 900" preserveAspectRatio="xMidYMid slice" aria-hidden>
      <defs>
        <linearGradient id="atlas-line" x1="0" y1="0" x2="1" y2="0.4">
          <stop offset="0%" stopColor="#0b4f6c" stopOpacity="0.08" />
          <stop offset="50%" stopColor="#0b4f6c" stopOpacity="0.45" />
          <stop offset="100%" stopColor="#0b4f6c" stopOpacity="0.1" />
        </linearGradient>
        <pattern id="atlas-grid" width="48" height="48" patternUnits="userSpaceOnUse">
          <path d="M48 0 H0 V48" fill="none" stroke="#0b4f6c" strokeOpacity="0.07" strokeWidth="1" />
        </pattern>
      </defs>
      <rect width="1440" height="900" fill="url(#atlas-grid)" />
      <g className="landing-atlas-routes" stroke="url(#atlas-line)" strokeWidth="1.5" fill="none">
        <path d="M120 220 C320 160, 420 320, 640 260 S980 140, 1320 240" />
        <path d="M80 480 C280 420, 460 560, 720 500 S1040 400, 1360 520" />
        <path d="M160 720 C380 640, 560 780, 820 700 S1100 620, 1300 760" />
        <path d="M520 80 C540 220, 500 380, 560 540 S620 780, 580 880" />
        <path d="M960 40 C940 200, 1000 360, 940 520 S900 760, 980 860" />
      </g>
      <g className="landing-atlas-nodes" fill="#0b4f6c">
        <circle cx="640" cy="260" r="6" />
        <circle cx="720" cy="500" r="5" />
        <circle cx="820" cy="700" r="6" />
        <circle cx="960" cy="200" r="4" />
        <circle cx="1120" cy="460" r="5" />
        <circle cx="420" cy="360" r="4" />
      </g>
    </svg>
  );
}

export function LandingPage() {
  const [consoleHref, setConsoleHref] = useState("/login?next=%2Fdashboard");

  useEffect(() => {
    if (isLoggedIn()) {
      setConsoleHref("/dashboard");
    }
  }, []);

  return (
    <div className="landing">
      <header className="landing-header">
        <Link href="/" className="landing-logo">
          AgentMesh
        </Link>
        <nav className="landing-nav" aria-label="Primary">
          <a href="#features">What you get</a>
          <a href="#how-it-works">How it works</a>
          <a href="#adapters">Tools</a>
        </nav>
        <Link href={consoleHref} className="btn landing-header-cta">
          Open console
        </Link>
      </header>

      <section className="landing-hero">
        <AtlasBg />
        <div className="landing-hero-inner">
          <h1 className="landing-brand">AgentMesh</h1>
          <p className="landing-headline">One place to find and choose coding agents for your team.</p>
          <p className="landing-lede">
            Connect tools like Cursor, Claude Code, and Copilot. AgentMesh keeps them in one list and
            helps you send each job to a good fit.
          </p>
          <div className="landing-hero-actions">
            <Link href={consoleHref} className="btn">
              Open console
            </Link>
            <a href="#how-it-works" className="btn btn-secondary">
              See how it works
            </a>
          </div>
        </div>
      </section>

      <section id="features" className="landing-section landing-section--split">
        <div className="landing-section-head landing-section-head--sticky">
          <h2>Less hunting. Clearer choices.</h2>
          <p>Built for teams that already use more than one coding agent.</p>
        </div>
        <ol className="landing-features">
          {FEATURES.map((f, i) => (
            <li key={f.title} className="landing-feature">
              <span className="landing-feature-n" aria-hidden>
                {String(i + 1).padStart(2, "0")}
              </span>
              <div>
                <h3>{f.title}</h3>
                <p>{f.body}</p>
              </div>
            </li>
          ))}
        </ol>
      </section>

      <section id="how-it-works" className="landing-section landing-section--band">
        <div className="landing-section-head">
          <h2>How it works</h2>
          <p>Three simple steps from connect to day-to-day use.</p>
        </div>
        <ol className="landing-steps">
          {STEPS.map((s) => (
            <li key={s.n}>
              <span className="landing-step-n">{s.n}</span>
              <h3>{s.title}</h3>
              <p>{s.body}</p>
            </li>
          ))}
        </ol>
      </section>

      <section id="adapters" className="landing-section landing-section--ribbon">
        <div className="landing-section-head">
          <h2>Works with the tools you already use</h2>
          <p>Same idea for each one: connect it, then choose it when the work fits.</p>
        </div>
        <div className="landing-ribbon" aria-label="Supported tools">
          <div className="landing-ribbon-track">
            {[...ADAPTERS, ...ADAPTERS].map((tool, i) => (
              <div className="landing-ribbon-item" key={`${tool.name}-${i}`} aria-hidden={i >= ADAPTERS.length}>
                <img src={tool.logo} alt="" width={36} height={36} className="landing-ribbon-logo" />
                <span>{tool.name}</span>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section className="landing-cta-band">
        <div className="landing-cta-band-inner">
          <div>
            <h2>Try it on your machine</h2>
            <p>Start the local setup, then open the console to connect agents and try a first task.</p>
          </div>
          <Link href={consoleHref} className="btn landing-cta-band-btn">
            Open console
          </Link>
        </div>
      </section>

      <footer className="landing-footer">
        <span className="landing-footer-brand">AgentMesh</span>
        <div className="landing-footer-links">
          <Link href={consoleHref}>Console</Link>
          <Link href="/login">Log in</Link>
          <Link href="/settings">Settings</Link>
        </div>
      </footer>
    </div>
  );
}
