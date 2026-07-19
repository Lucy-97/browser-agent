"use client";

import Link from "next/link";
import { ArrowLeft, CheckCircle2, KeyRound, LoaderCircle, MonitorUp, ShieldCheck } from "lucide-react";
import { FormEvent, useEffect, useState } from "react";
import { useRouter } from "next/navigation";

type Session = {
  user: { nickname: string; email: string };
  membership: { tenant: { tenant_name: string }; role: string };
};

function errorText(payload: unknown, fallback: string): string {
  if (payload && typeof payload === "object" && "error" in payload) {
    const detail = (payload as { error?: { message?: string } }).error;
    if (detail?.message) return detail.message;
  }
  return fallback;
}

export default function PairWorkerPage() {
  const router = useRouter();
  const [session, setSession] = useState<Session | null>(null);
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(true);
  const [pending, setPending] = useState(false);
  const [paired, setPaired] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    fetch("/api/v1/auth/me", { credentials: "include" })
      .then(async (response) => {
        if (response.status === 401) {
          router.replace("/login");
          return null;
        }
        if (!response.ok) throw new Error("无法读取当前登录空间。");
        return (await response.json()) as Session;
      })
      .then((value) => value && setSession(value))
      .catch((requestError) => setError(requestError instanceof Error ? requestError.message : "无法读取当前登录空间。"))
      .finally(() => setLoading(false));
  }, [router]);

  async function approve(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalized = code.trim().toLowerCase().replace(/[^a-z0-9]/g, "");
    if (!normalized) return;
    setPending(true);
    setError("");
    try {
      const response = await fetch(`/web/worker/pairings/${encodeURIComponent(normalized)}/approve`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: "{}",
      });
      const payload = await response.json().catch(() => null);
      if (!response.ok) throw new Error(errorText(payload, "配对码无效、已过期或已被使用。"));
      setPaired(true);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "设备配对失败，请重试。");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="pair-page">
      <div className="pair-header">
        <Link href="/" className="pair-back"><ArrowLeft size={16} /> 返回工作台</Link>
        <span>02 — 连接执行节点</span>
      </div>

      <div className="pair-grid">
        <section className="pair-intro">
          <span className="auth-eyebrow">Local Worker / Device Grant</span>
          <h1>让这台电脑成为你的执行节点。</h1>
          <p>在 Worker 中启动配对后，把屏幕上的一次性代码填到这里。批准只对当前租户空间生效。</p>
          <div className="pair-safety">
            <ShieldCheck size={20} />
            <div><strong>本地优先</strong><span>浏览器资料与第三方登录态不会随配对上传。</span></div>
          </div>
        </section>

        <section className="pair-card">
          {loading ? (
            <div className="pair-loading"><LoaderCircle className="spin" size={24} /> 正在确认登录空间…</div>
          ) : paired ? (
            <div className="pair-success">
              <CheckCircle2 size={42} />
              <span>授权完成</span>
              <h2>Worker 已加入当前空间</h2>
              <p>设备获取凭据后会自动上线，你可以回到工作台创建第一条任务。</p>
              <Link href="/" className="auth-submit">进入工作台</Link>
            </div>
          ) : (
            <>
              <div className="pair-context">
                <MonitorUp size={20} />
                <div>
                  <span>当前空间</span>
                  <strong>{session?.membership.tenant.tenant_name || "尚未加载"}</strong>
                  {session && <small>{session.user.nickname} · {session.membership.role}</small>}
                </div>
              </div>
              <form className="pair-form" onSubmit={approve}>
                <label htmlFor="pairing-code">一次性配对码</label>
                <div className="pair-code-wrap">
                  <KeyRound size={20} />
                  <input
                    id="pairing-code"
                    value={code}
                    onChange={(event) => setCode(event.target.value.toUpperCase())}
                    required
                    maxLength={16}
                    autoComplete="one-time-code"
                    spellCheck={false}
                    placeholder="例如 8F2A91"
                    autoFocus
                  />
                </div>
                {error && <div className="auth-error" role="alert">{error}</div>}
                <button className="auth-submit" type="submit" disabled={pending || !session}>
                  {pending ? "正在授权…" : "批准并连接 Worker"}
                </button>
              </form>
              <p className="pair-help">配对码 10 分钟内有效且只能批准一次。无法识别时，请在 Worker 中重新开始配对。</p>
            </>
          )}
        </section>
      </div>
    </div>
  );
}
