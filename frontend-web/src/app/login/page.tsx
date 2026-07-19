"use client";

import { ArrowRight, Check, KeyRound, ShieldCheck, Workflow } from "lucide-react";
import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";

type AuthMode = "login" | "register";

type AuthResponse = {
  user: { nickname: string; email: string };
  tenant: { tenant_name: string };
};

function readError(payload: unknown, fallback: string): string {
  if (payload && typeof payload === "object" && "error" in payload) {
    const detail = (payload as { error?: { message?: string } }).error;
    if (detail?.message) return detail.message;
  }
  return fallback;
}

export default function LoginPage() {
  const router = useRouter();
  const [mode, setMode] = useState<AuthMode>("login");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError("");
    const form = new FormData(event.currentTarget);
    const body = Object.fromEntries(form.entries());
    try {
      const response = await fetch(`/api/v1/auth/${mode}`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const payload = (await response.json().catch(() => null)) as AuthResponse | null;
      if (!response.ok) {
        throw new Error(readError(payload, "身份验证失败，请检查输入后重试。"));
      }
      router.push("/worker/pair");
      router.refresh();
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "身份验证失败，请稍后重试。");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="auth-page">
      <section className="trust-panel" aria-labelledby="trust-title">
        <div>
          <span className="auth-eyebrow">Browser Agent / Trust Link</span>
          <h2 id="trust-title">把自动化留在你的电脑，把控制权交给可信空间。</h2>
          <p>
            云端只调度任务和保存经你授权的结果。网站登录态、Cookie 与浏览器资料继续留在本地 Worker。
          </p>
        </div>
        <ol className="trust-rail">
          <li className="is-active">
            <span><ShieldCheck size={18} /></span>
            <div><strong>验证账号</strong><small>确认操作者身份</small></div>
          </li>
          <li>
            <span><Workflow size={18} /></span>
            <div><strong>进入租户空间</strong><small>任务与数据独立隔离</small></div>
          </li>
          <li>
            <span><KeyRound size={18} /></span>
            <div><strong>连接本地 Worker</strong><small>用一次性配对码授权</small></div>
          </li>
        </ol>
        <div className="trust-note"><Check size={15} /> 生产环境使用短期会话与 HttpOnly Cookie</div>
      </section>

      <section className="auth-card" aria-labelledby="auth-title">
        <div className="auth-card-heading">
          <span className="auth-index">01 — 身份入口</span>
          <h1 id="auth-title">{mode === "login" ? "登录控制台" : "创建内测空间"}</h1>
          <p>{mode === "login" ? "继续管理你的任务与本地执行节点。" : "注册后将自动创建一个隔离的租户空间。"}</p>
        </div>

        <div className="auth-tabs" role="tablist" aria-label="身份方式">
          <button type="button" className={mode === "login" ? "is-active" : ""} onClick={() => { setMode("login"); setError(""); }}>
            登录
          </button>
          <button type="button" className={mode === "register" ? "is-active" : ""} onClick={() => { setMode("register"); setError(""); }}>
            创建空间
          </button>
        </div>

        <form className="auth-form" onSubmit={submit}>
          {mode === "register" && (
            <div className="auth-field-row">
              <label>
                <span>你的称呼</span>
                <input name="nickname" required maxLength={64} autoComplete="name" placeholder="例如：Lucy" />
              </label>
              <label>
                <span>空间名称</span>
                <input name="tenant_name" required maxLength={255} placeholder="例如：版权运营组" />
              </label>
            </div>
          )}
          <label>
            <span>邮箱</span>
            <input name="email" required type="email" autoComplete="email" placeholder="name@company.com" />
          </label>
          <label>
            <span>密码</span>
            <input
              name="password"
              required
              type="password"
              minLength={12}
              maxLength={72}
              autoComplete={mode === "login" ? "current-password" : "new-password"}
              placeholder={mode === "login" ? "输入密码" : "至少 12 个字符"}
            />
          </label>
          {error && <div className="auth-error" role="alert">{error}</div>}
          <button className="auth-submit" type="submit" disabled={pending}>
            <span>{pending ? "正在验证…" : mode === "login" ? "登录并继续" : "创建空间并继续"}</span>
            {!pending && <ArrowRight size={18} />}
          </button>
        </form>

        {mode === "register" && <p className="auth-footnote">封闭内测期间，管理员可能关闭公开注册；已有账号可直接登录。</p>}
      </section>
    </div>
  );
}
