"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const NAV = [
  { href: "/dashboard", label: "Overview" },
  { href: "/agents", label: "Agents" },
  { href: "/models", label: "Models & keys" },
  { href: "/chat", label: "Chat" },
  { href: "/routes", label: "Route debugger" },
  { href: "/circuits", label: "Circuits" },
  { href: "/policy", label: "Org policy" },
  { href: "/settings", label: "Settings" },
];

function navActive(path: string, href: string) {
  if (href === "/dashboard") return path === "/dashboard";
  return path === href || path.startsWith(`${href}/`);
}

export function Shell({ children }: { children: React.ReactNode }) {
  const path = usePathname();

  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <Link href="/" className="sidebar-brand-link">
            <h1>AgentMesh</h1>
            <p>Control plane</p>
          </Link>
        </div>
        <nav>
          {NAV.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className={`nav-link ${navActive(path, item.href) ? "active" : ""}`}
            >
              {item.label}
            </Link>
          ))}
        </nav>
        <div className="sidebar-foot">
          <Link href="/settings" className="sidebar-foot-link">
            Account / keys
          </Link>
          <Link href="/" className="sidebar-foot-link">
            ← Site home
          </Link>
        </div>
      </aside>
      <main className="main">{children}</main>
    </div>
  );
}
