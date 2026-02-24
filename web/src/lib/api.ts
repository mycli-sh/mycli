const API_URL = import.meta.env.VITE_API_URL || "http://localhost:8080";

let accessToken: string | null = null;
let refreshToken: string | null = null;

export function setTokens(access: string | null, refresh: string | null) {
  accessToken = access;
  refreshToken = refresh;
  if (access) localStorage.setItem("access_token", access);
  else localStorage.removeItem("access_token");
  if (refresh) localStorage.setItem("refresh_token", refresh);
  else localStorage.removeItem("refresh_token");
}

export function loadTokens() {
  accessToken = localStorage.getItem("access_token");
  refreshToken = localStorage.getItem("refresh_token");
}

export function getAccessToken() {
  return accessToken;
}

export function clearTokens() {
  setTokens(null, null);
  localStorage.removeItem("session_id");
}

async function refreshAccessToken(): Promise<boolean> {
  if (!refreshToken) return false;
  try {
    const res = await fetch(`${API_URL}/v1/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    if (!res.ok) {
      clearTokens();
      return false;
    }
    const data = await res.json();
    accessToken = data.access_token;
    if (accessToken) localStorage.setItem("access_token", accessToken);
    return true;
  } catch {
    clearTokens();
    return false;
  }
}

export async function api<T = unknown>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const headers = new Headers(options.headers);
  if (!headers.has("Content-Type") && options.body) {
    headers.set("Content-Type", "application/json");
  }
  if (accessToken) {
    headers.set("Authorization", `Bearer ${accessToken}`);
  }

  let res = await fetch(`${API_URL}${path}`, { ...options, headers });

  // Auto-refresh on 401
  if (res.status === 401 && refreshToken) {
    const refreshed = await refreshAccessToken();
    if (refreshed) {
      if (accessToken) headers.set("Authorization", `Bearer ${accessToken}`);
      res = await fetch(`${API_URL}${path}`, { ...options, headers });
    }
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new ApiError(res.status, body?.error?.code || "UNKNOWN", body?.error?.message || res.statusText);
  }

  return res.json();
}

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

// Typed API methods

export interface Library {
  id: string;
  slug: string;
  name: string;
  description: string;
  owner_id: string | null;
  git_url: string | null;
  is_public: boolean;
  install_count: number;
  latest_version?: string;
  created_at: string;
  updated_at: string;
}

export interface LibraryCommand {
  command_id: string;
  slug: string;
  name: string;
  description: string;
  updated_at: string;
}

export interface LibraryDetail {
  library: Library;
  owner: string;
  commands: LibraryCommand[];
  installed: boolean;
}

export interface LibraryRelease {
  id: string;
  library_id: string;
  version: string;
  tag: string;
  commit_hash: string;
  command_count: number;
  released_by: string;
  released_at: string;
}

export interface SpecJson {
  schemaVersion: number;
  kind: string;
  metadata: {
    name: string;
    slug: string;
    description?: string;
    tags?: string[];
    aliases?: string[];
  };
  defaults?: {
    shell?: string;
    timeout?: string;
    env?: Record<string, string>;
  };
  dependencies?: string[];
  args?: {
    positional?: {
      name: string;
      description?: string;
      required?: boolean;
      default?: string;
    }[];
    flags?: {
      name: string;
      short?: string;
      description?: string;
      type?: "string" | "bool" | "int";
      default?: unknown;
      required?: boolean;
    }[];
  };
  steps: {
    name: string;
    run: string;
    env?: Record<string, string>;
    timeout?: string;
    continueOnError?: boolean;
    shell?: string;
  }[];
  policy?: {
    requireConfirmation?: boolean;
    allowedExecutables?: string[];
  };
}

export interface CommandVersion {
  id: string;
  command_id: string;
  version: number;
  spec_json: SpecJson;
  spec_hash: string;
  message: string;
  created_by: string;
  created_at: string;
}

export interface CommandDetail {
  command: {
    id: string;
    slug: string;
    name: string;
    description: string;
    tags: string[];
    library_id: string;
    created_at: string;
    updated_at: string;
  };
  latest_version?: CommandVersion;
}

export interface LibrarySearchResult {
  libraries: (Library & { owner: string })[];
  total: number;
}

export interface Session {
  id: string;
  user_agent: string;
  ip_address: string;
  device_id: string;
  device_name: string;
  last_used_at: string;
  expires_at: string;
  created_at: string;
}

export interface User {
  id: string;
  email: string;
  username?: string;
  needs_username: boolean;
}

export interface SyncSummary {
  user_commands_count: number;
  installed_libraries: { slug: string; name: string; command_count: number }[];
  total_commands: number;
}

export const libraryApi = {
  search: (q: string, limit = 20, offset = 0) =>
    api<LibrarySearchResult>(`/v1/libraries?q=${encodeURIComponent(q)}&limit=${limit}&offset=${offset}`),
  getDetail: (owner: string, slug: string) =>
    api<LibraryDetail>(`/v1/libraries/${encodeURIComponent(owner)}/${encodeURIComponent(slug)}`),
  listReleases: (owner: string, slug: string) =>
    api<{ releases: LibraryRelease[] }>(`/v1/libraries/${encodeURIComponent(owner)}/${encodeURIComponent(slug)}/releases`),
  getCommand: (owner: string, slug: string, commandSlug: string) =>
    api<CommandDetail>(`/v1/libraries/${encodeURIComponent(owner)}/${encodeURIComponent(slug)}/commands/${encodeURIComponent(commandSlug)}`),
  listCommandVersions: (owner: string, slug: string, commandSlug: string) =>
    api<{ versions: CommandVersion[] }>(`/v1/libraries/${encodeURIComponent(owner)}/${encodeURIComponent(slug)}/commands/${encodeURIComponent(commandSlug)}/versions`),
};

export const authApi = {
  webLogin: (email: string) =>
    api<{ sent: boolean }>("/v1/auth/web/login", {
      method: "POST",
      body: JSON.stringify({ email }),
    }),
  webVerify: (token: string) =>
    api<{ access_token: string; refresh_token: string; session_id: string; expires_in: number }>(
      "/v1/auth/web/verify",
      { method: "POST", body: JSON.stringify({ token }) }
    ),
  getMe: () => api<User>("/v1/me"),
};

export const sessionApi = {
  list: () => api<{ sessions: Session[] }>("/v1/sessions"),
  revoke: (id: string) =>
    api<{ revoked: boolean }>(`/v1/sessions/${id}`, { method: "DELETE" }),
  revokeAll: (currentSessionId: string) =>
    api<{ revoked_count: number }>(`/v1/sessions?current_session_id=${encodeURIComponent(currentSessionId)}`, {
      method: "DELETE",
    }),
};

export const meApi = {
  syncSummary: () => api<SyncSummary>("/v1/me/sync-summary"),
  setUsername: (username: string) =>
    api<{ username: string }>("/v1/me/username", {
      method: "PATCH",
      body: JSON.stringify({ username }),
    }),
  checkAvailable: (username: string) =>
    api<{ available: boolean; reason?: string }>(
      `/v1/usernames/${encodeURIComponent(username)}/available`
    ),
};
