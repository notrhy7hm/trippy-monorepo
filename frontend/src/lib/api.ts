const TOKEN_KEY = "trippy.token";

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string | null) {
  if (token) localStorage.setItem(TOKEN_KEY, token);
  else localStorage.removeItem(TOKEN_KEY);
}

export class ApiError extends Error {
  status: number;
  code?: string;
  constructor(message: string, status: number, code?: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

type Method = "GET" | "POST" | "PATCH" | "DELETE";

const API_BASE = apiBase(import.meta.env.VITE_API_BASE);

export async function api<T = unknown>(
  method: Method,
  path: string,
  body?: unknown,
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;

  const res = await fetch(`${API_BASE}${normalizePath(path)}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (res.status === 401) {
    setToken(null);
    window.dispatchEvent(new Event("trippy:unauthenticated"));
  }

  const text = await res.text();
  const parsed = text ? JSON.parse(text) : null;

  if (!res.ok) {
    const msg = (parsed && parsed.error) || res.statusText;
    throw new ApiError(msg, res.status, parsed?.code);
  }
  return parsed as T;
}

function apiBase(raw: string | undefined) {
  const base = raw?.trim().replace(/\/+$/, "");
  if (!base) return "/api/v1";
  return base.endsWith("/api/v1") ? base : `${base}/api/v1`;
}

function normalizePath(path: string) {
  return path.startsWith("/") ? path : `/${path}`;
}
