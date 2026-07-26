// ConCoord API istemcisi — access token bellekte, refresh token localStorage'da.
// 401 alındığında bir kez sessizce refresh denenir; başarısızsa oturum düşer.

export type User = {
  id: string;
  email: string;
  username: string;
  full_name: string;
  phone?: string;
  is_active: boolean;
  last_login_at?: string;
};

export type ApiError = { code: string; message: string; details?: unknown; request_id?: string };

const REFRESH_KEY = "ipks.refresh";
const BASE = import.meta.env.VITE_API_URL ?? "";
let accessToken: string | null = null;
let onSessionLost: (() => void) | null = null;

export function setOnSessionLost(fn: () => void) {
  onSessionLost = fn;
}
export function setRefreshToken(t: string | null) {
  if (t) localStorage.setItem(REFRESH_KEY, t);
  else localStorage.removeItem(REFRESH_KEY);
}
export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_KEY);
}
export function setAccessToken(t: string | null) {
  accessToken = t;
}
export function hasSession(): boolean {
  return !!getRefreshToken();
}

class RequestError extends Error {
  status: number;
  api?: ApiError;
  constructor(status: number, api?: ApiError) {
    super(api?.message || `HTTP ${status}`);
    this.status = status;
    this.api = api;
  }
}
export { RequestError };

async function refresh(): Promise<boolean> {
  const rt = getRefreshToken();
  if (!rt) return false;
  const res = await fetch(`${BASE}/api/v1/auth/refresh`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: rt }),
  });
  if (!res.ok) return false;
  const data = await res.json();
  accessToken = data.tokens.access_token;
  setRefreshToken(data.tokens.refresh_token);
  return true;
}

type Opts = { method?: string; body?: unknown; projectId?: string | null; retry?: boolean };

export async function api<T = unknown>(path: string, opts: Opts = {}): Promise<T> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (accessToken) headers["Authorization"] = `Bearer ${accessToken}`;
  if (opts.projectId) headers["X-Project-Id"] = opts.projectId;

  const res = await fetch(`${BASE}/api/v1${path}`, {
    method: opts.method || "GET",
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  });

  if (res.status === 401 && !opts.retry) {
    if (await refresh()) {
      return api<T>(path, { ...opts, retry: true });
    }
    setAccessToken(null);
    setRefreshToken(null);
    onSessionLost?.();
    throw new RequestError(401);
  }

  if (!res.ok) {
    let apiErr: ApiError | undefined;
    try {
      apiErr = (await res.json()).error;
    } catch {
      /* gövde yok */
    }
    throw new RequestError(res.status, apiErr);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export async function login(identifier: string, password: string) {
  const res = await fetch(`${BASE}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ identifier, password }),
  });
  if (!res.ok) {
    let err: ApiError | undefined;
    try {
      err = (await res.json()).error;
    } catch {
      /* */
    }
    throw new RequestError(res.status, err);
  }
  const data = await res.json();
  accessToken = data.tokens.access_token;
  setRefreshToken(data.tokens.refresh_token);
  return data.user as User;
}

export async function logout() {
  const rt = getRefreshToken();
  try {
    await fetch(`${BASE}/api/v1/auth/logout`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: rt }),
    });
  } catch {
    /* yoksay */
  }
  setAccessToken(null);
  setRefreshToken(null);
}

export type MeResponse = { user: User; permissions: string[]; project_id: string | null };

export async function fetchMe(projectId?: string | null): Promise<MeResponse> {
  return api<MeResponse>(`/auth/me${projectId ? `?project_id=${projectId}` : ""}`);
}

export async function apiUpload<T = unknown>(
  path: string,
  form: FormData,
  retry = false
): Promise<T> {
  const headers: Record<string, string> = {};
  if (accessToken) headers["Authorization"] = `Bearer ${accessToken}`;
  const res = await fetch(`${BASE}/api/v1${path}`, { method: "POST", headers, body: form });

  if (res.status === 401 && !retry) {
    if (await refresh()) return apiUpload<T>(path, form, true);
    setAccessToken(null);
    setRefreshToken(null);
    onSessionLost?.();
    throw new RequestError(401);
  }
  if (!res.ok) {
    let apiErr: ApiError | undefined;
    try {
      apiErr = (await res.json()).error;
    } catch {
      /* */
    }
    throw new RequestError(res.status, apiErr);
  }
  return (await res.json()) as T;
}

export async function apiDownload(path: string, fallbackName: string, retry = false): Promise<void> {
  const headers: Record<string, string> = {};
  if (accessToken) headers["Authorization"] = `Bearer ${accessToken}`;
  const res = await fetch(`${BASE}/api/v1${path}`, { headers });

  if (res.status === 401 && !retry) {
    if (await refresh()) return apiDownload(path, fallbackName, true);
    setAccessToken(null);
    setRefreshToken(null);
    onSessionLost?.();
    throw new RequestError(401);
  }
  if (!res.ok) throw new RequestError(res.status);

  const blob = await res.blob();
  let name = fallbackName;
  const cd = res.headers.get("Content-Disposition");
  const match = cd && /filename="?([^"]+)"?/.exec(cd);
  if (match) name = match[1];

  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
