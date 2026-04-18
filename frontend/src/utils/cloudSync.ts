import type { TFunction } from "i18next";

import type { appconf, vo } from "../../wailsjs/go/models";

import { formatLocalDateTime } from "./time";

export function isCloudProviderConfigured(config?: appconf.AppConfig | null) {
  if (!config?.cloud_backup_enabled || !config.backup_user_id) {
    return false;
  }

  if (config.cloud_backup_provider === "onedrive") {
    return Boolean(config.onedrive_refresh_token);
  }

  return Boolean(config.s3_endpoint && config.s3_access_key);
}

export function getEffectiveCloudSyncStatus(
  syncStatus: vo.CloudSyncStatus | null,
  config?: appconf.AppConfig | null,
): vo.CloudSyncStatus {
  return (
    syncStatus ?? {
      enabled: config?.cloud_sync_enabled || false,
      configured: isCloudProviderConfigured(config),
      syncing: false,
      last_sync_time: config?.last_cloud_sync_time || "",
      last_sync_status: config?.last_cloud_sync_status || "idle",
      last_sync_error: config?.last_cloud_sync_error || "",
    }
  );
}

export function getCloudSyncStatusLabel(status: string, t: TFunction) {
  switch (status) {
    case "success":
      return t("settings.cloudBackup.syncStatusSuccess");
    case "failed":
      return t("settings.cloudBackup.syncStatusFailed");
    case "syncing":
      return t("settings.cloudBackup.syncStatusSyncing");
    default:
      return t("settings.cloudBackup.syncStatusIdle");
  }
}

export function getCloudSyncStatusClass(status: string) {
  switch (status) {
    case "success":
      return "bg-success-100 text-success-700 ring-1 ring-success-200 dark:bg-success-900/40 dark:text-success-300 dark:ring-success-800/60";
    case "failed":
      return "bg-error-100 text-error-700 ring-1 ring-error-200 dark:bg-error-900/40 dark:text-error-300 dark:ring-error-800/60";
    case "syncing":
      return "bg-warning-100 text-warning-700 ring-1 ring-warning-200 dark:bg-warning-900/40 dark:text-warning-300 dark:ring-warning-800/60";
    default:
      return "bg-brand-100 text-brand-700 ring-1 ring-brand-200 dark:bg-brand-700/70 dark:text-brand-200 dark:ring-brand-600/70";
  }
}

export function formatCloudSyncTime(
  value: string | undefined,
  timezone: string | undefined,
  fallback: string,
) {
  if (!value) {
    return fallback;
  }

  return formatLocalDateTime(value, timezone, {
    second: undefined,
  });
}

export function getSyncIntervalSeconds(config?: appconf.AppConfig | null) {
  if (!config?.cloud_sync_interval_sec || config.cloud_sync_interval_sec <= 0) {
    return 60;
  }

  return Math.max(15, config.cloud_sync_interval_sec);
}
