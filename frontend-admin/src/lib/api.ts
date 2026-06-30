const API_PREFIX = process.env.NEXT_PUBLIC_API_PREFIX || "/api";

function getAdminToken() {
  if (typeof window !== "undefined") {
    return localStorage.getItem("qiyuan.adminToken") || "";
  }
  return "";
}

export async function api<T>(path: string, init?: { method?: string; body?: unknown }): Promise<T> {
  const headers: Record<string, string> = {};
  if (init?.body) {
    headers["Content-Type"] = "application/json";
  }
  const token = getAdminToken();
  if (token) {
    headers["X-Admin-Token"] = token;
  }
  const response = await fetch(`${API_PREFIX}${path}`, {
    method: init?.method || "GET",
    headers,
    body: init?.body ? JSON.stringify(init.body) : undefined
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `${response.status} ${response.statusText}`);
  }
  return (await response.json()) as T;
}

export function errorMessage(err: unknown) {
  if (err instanceof Error) {
    return err.message;
  }
  return "请求失败";
}
