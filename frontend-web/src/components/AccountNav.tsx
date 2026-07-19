"use client";

import Link from "next/link";
import { LogIn, LogOut } from "lucide-react";
import { useEffect, useState } from "react";

type AccountSession = {
  user: { nickname: string; email: string };
};

export function AccountNav() {
  const [session, setSession] = useState<AccountSession | null | undefined>(undefined);
  const [pending, setPending] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    fetch("/api/v1/auth/me", { credentials: "include", signal: controller.signal })
      .then(async (response) => (response.ok ? ((await response.json()) as AccountSession) : null))
      .then(setSession)
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") return;
        setSession(null);
      });
    return () => controller.abort();
  }, []);

  async function logout() {
    setPending(true);
    try {
      await fetch("/api/v1/auth/logout", { method: "POST", credentials: "include" });
    } finally {
      window.location.assign("/login");
    }
  }

  if (session === undefined) {
    return <span className="account-nav-placeholder" aria-hidden="true" />;
  }
  if (!session) {
    return (
      <Link href="/login" className="nav-link">
        <LogIn size={16} />
        <span>登录</span>
      </Link>
    );
  }
  return (
    <div className="account-nav" title={session.user.email}>
      <span className="account-name">{session.user.nickname}</span>
      <button type="button" className="nav-link account-logout" onClick={logout} disabled={pending}>
        <LogOut size={16} />
        <span>{pending ? "退出中…" : "退出"}</span>
      </button>
    </div>
  );
}
