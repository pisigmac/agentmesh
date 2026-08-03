import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: {
    default: "AgentMesh",
    template: "%s · AgentMesh",
  },
  description: "Service mesh for AI coding agents — discovery, routing, circuit breaking, and org policy.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
