/* API client for frontend-web.
 * /web/*  → Go API (29001) via next.config.ts rewrites
 */

export async function api<T>(
  path: string,
  init?: { method?: string; body?: unknown },
): Promise<T> {
  const headers: Record<string, string> = {};
  if (init?.body) {
    headers["Content-Type"] = "application/json";
  }
  const token =
    typeof window !== "undefined"
      ? localStorage.getItem("browser-agent.webToken") || ""
      : "";
  if (token) {
    headers["X-Web-Token"] = token;
  }
  const response = await fetch(path, {
    method: init?.method || "GET",
    headers,
    body: init?.body ? JSON.stringify(init.body) : undefined,
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `${response.status} ${response.statusText}`);
  }
  return (await response.json()) as T;
}

export function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  return "请求失败";
}

export function formatDateTime(value?: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}
