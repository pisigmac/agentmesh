"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { isLoggedIn } from "@/lib/config";

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const path = usePathname();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (!isLoggedIn()) {
      router.replace(`/login?next=${encodeURIComponent(path || "/dashboard")}`);
      return;
    }
    setReady(true);
  }, [path, router]);

  if (!ready) {
    return <div className="empty">Checking session…</div>;
  }
  return <>{children}</>;
}
