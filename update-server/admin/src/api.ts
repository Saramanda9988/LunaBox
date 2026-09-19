import type { DashboardData, ReleaseDetailData, ReleaseFilters } from "./types";

const TOKEN_KEY = "lunabox-admin-token";

export class UnauthorizedError extends Error {}

export function readToken(): string {
  return sessionStorage.getItem(TOKEN_KEY) ?? "";
}

export function saveToken(token: string): void {
  sessionStorage.setItem(TOKEN_KEY, token);
}

export function clearToken(): void {
  sessionStorage.removeItem(TOKEN_KEY);
}

export async function getDashboard(token: string, signal?: AbortSignal): Promise<DashboardData> {
  return request<DashboardData>("/v1/admin/dashboard", token, signal);
}

export async function getReleaseDetails(
  token: string,
  version: string,
  filters: ReleaseFilters,
  signal?: AbortSignal,
): Promise<ReleaseDetailData> {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(filters)) {
    if (value !== undefined && value !== "")
      search.set(key, String(value));
  }
  const suffix = search.size ? `?${search.toString()}` : "";
  return request<ReleaseDetailData>(`/v1/admin/releases/${encodeURIComponent(version)}${suffix}`, token, signal);
}

async function request<T>(path: string, token: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, {
    headers: { authorization: `Bearer ${token}` },
    cache: "no-store",
    signal,
  });
  if (response.status === 401)
    throw new UnauthorizedError("管理令牌验证失败");
  if (!response.ok)
    throw new Error(`服务返回 ${response.status}`);
  return response.json() as Promise<T>;
}
