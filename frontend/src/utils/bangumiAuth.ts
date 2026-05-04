import type { appconf, vo } from "../../wailsjs/go/models";

import {
  Disconnect,
  GetAuthStatus,
  GetProfile,
  StartAuth,
} from "../../wailsjs/go/service/BangumiService";

export type BangumiAuthViewState
  = | "unauthorized"
    | "authorized"
    | "needs_reauth";

export type BangumiAuthStatus = {
  state: BangumiAuthViewState;
  identity: string;
  expiresAt?: string;
  lastError?: string;
};

function readString(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function getIdentity(username?: string, userID?: string): string {
  return readString(username) || readString(userID) || "Bangumi";
}

function deriveState(
  authorized: boolean,
  needsReauthorization: boolean,
): BangumiAuthViewState {
  if (needsReauthorization) {
    return "needs_reauth";
  }
  if (authorized) {
    return "authorized";
  }
  return "unauthorized";
}

function getStatusFromConfig(config: appconf.AppConfig): BangumiAuthStatus {
  const accessToken = readString(config.access_token);
  const refreshToken = readString(config.bangumi_refresh_token);
  const username = readString(config.bangumi_authorized_username);
  const userID = readString(config.bangumi_authorized_user_id);
  const expiresAt = readString(config.bangumi_token_expires_at);
  const lastError = readString(config.bangumi_auth_error);

  return {
    state: deriveState(
      Boolean(accessToken || refreshToken),
      Boolean(lastError),
    ),
    identity: getIdentity(username, userID),
    expiresAt: expiresAt || undefined,
    lastError: lastError || undefined,
  };
}

function getStatusFromSnapshot(
  snapshot: vo.BangumiAuthStatus,
): BangumiAuthStatus {
  return {
    state: deriveState(
      Boolean(snapshot.authorized),
      Boolean(snapshot.needs_reauthorization),
    ),
    identity: getIdentity(snapshot.username, snapshot.user_id),
    expiresAt: readString(snapshot.access_token_expires_at) || undefined,
    lastError: readString(snapshot.last_error) || undefined,
  };
}

export function mergeBangumiAuthStatus(
  config: appconf.AppConfig,
  snapshot?: vo.BangumiAuthStatus | null,
): BangumiAuthStatus {
  if (snapshot) {
    return getStatusFromSnapshot(snapshot);
  }
  return getStatusFromConfig(config);
}

export function fetchBangumiAuthStatus(): Promise<vo.BangumiAuthStatus> {
  return GetAuthStatus();
}

export function fetchBangumiProfile(): Promise<vo.BangumiProfile> {
  return GetProfile();
}

export function startBangumiAuthorization(): Promise<vo.BangumiAuthStatus> {
  return StartAuth();
}

export function disconnectBangumiAuthorization(): Promise<vo.BangumiAuthStatus> {
  return Disconnect();
}
