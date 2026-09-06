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

  const parsed = await parseResponse(res);

  if (!res.ok) {
    const msg =
      parsed && typeof parsed === "object" && "error" in parsed
        ? String(parsed.error)
        : res.statusText || "Request failed";
    const code =
      parsed && typeof parsed === "object" && "code" in parsed
        ? String(parsed.code)
        : undefined;
    throw new ApiError(msg, res.status, code);
  }
  return parsed as T;
}

async function parseResponse(res: Response): Promise<unknown> {
  const text = await res.text();
  if (!text) return null;
  const contentType = res.headers.get("content-type") ?? "";
  if (!contentType.toLowerCase().includes("application/json")) {
    return text;
  }
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

function apiBase(raw: string | undefined) {
  const base = raw?.trim().replace(/\/+$/, "");
  if (!base) return "/api/v1";
  return base.endsWith("/api/v1") ? base : `${base}/api/v1`;
}

function normalizePath(path: string) {
  return path.startsWith("/") ? path : `/${path}`;
}
