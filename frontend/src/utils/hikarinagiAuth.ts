import type { appconf, vo } from "../../src/bindings/models";

import {
  Disconnect,
  GetAuthStatus,
  GetProfile,
  StartAuth,
  SyncAllGameStatuses,
} from "../../bindings/lunabox/internal/service/hikarinagiservice";

export const HIKARINAGI_STATUS_SYNC_PROGRESS_EVENT
  = "hikarinagi:status-sync-progress";

export type HikarinagiAuthViewState
  = | "unauthorized"
    | "authorized"
    | "needs_reauth";

export type HikarinagiAuthStatus = {
  state: HikarinagiAuthViewState;
  identity: string;
  avatarUrl?: string;
  expiresAt?: string;
  lastError?: string;
};

function readString(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function getIdentity(username?: string, userID?: string): string {
  return readString(username) || readString(userID) || "Hikarinagi";
}

function deriveState(
  authorized: boolean,
  needsReauthorization: boolean,
): HikarinagiAuthViewState {
  if (needsReauthorization) {
    return "needs_reauth";
  }
  if (authorized) {
    return "authorized";
  }
  return "unauthorized";
}

function getStatusFromConfig(config: appconf.AppConfig): HikarinagiAuthStatus {
  const accessToken = readString(config.hikarinagi_access_token);
  const refreshToken = readString(config.hikarinagi_refresh_token);
  const username = readString(config.hikarinagi_authorized_username);
  const userID = readString(config.hikarinagi_authorized_user_id);
  const avatarUrl = readString(config.hikarinagi_authorized_avatar_url);
  const expiresAt = readString(config.hikarinagi_token_expires_at);
  const lastError = readString(config.hikarinagi_auth_error);

  return {
    state: deriveState(
      Boolean(accessToken || refreshToken),
      Boolean(lastError),
    ),
    identity: getIdentity(username, userID),
    avatarUrl: avatarUrl || undefined,
    expiresAt: expiresAt || undefined,
    lastError: lastError || undefined,
  };
}

function getStatusFromSnapshot(
  snapshot: vo.HikarinagiAuthStatus,
): HikarinagiAuthStatus {
  return {
    state: deriveState(
      Boolean(snapshot.authorized),
      Boolean(snapshot.needs_reauthorization),
    ),
    identity: getIdentity(snapshot.username, snapshot.user_id),
    avatarUrl: readString(snapshot.avatar_url) || undefined,
    expiresAt: readString(snapshot.access_token_expires_at) || undefined,
    lastError: readString(snapshot.last_error) || undefined,
  };
}

export function mergeHikarinagiAuthStatus(
  config: appconf.AppConfig,
  snapshot?: vo.HikarinagiAuthStatus | null,
): HikarinagiAuthStatus {
  if (snapshot) {
    return getStatusFromSnapshot(snapshot);
  }
  return getStatusFromConfig(config);
}

export function fetchHikarinagiAuthStatus(): Promise<vo.HikarinagiAuthStatus> {
  return GetAuthStatus();
}

export function fetchHikarinagiProfile(): Promise<vo.HikarinagiProfile> {
  return GetProfile();
}

export function startHikarinagiAuthorization(): Promise<vo.HikarinagiAuthStatus> {
  return StartAuth();
}

export function disconnectHikarinagiAuthorization(): Promise<vo.HikarinagiAuthStatus> {
  return Disconnect();
}

export function syncAllHikarinagiGameStatuses(): Promise<vo.RemoteStatusSyncProgress> {
  return SyncAllGameStatuses();
}
