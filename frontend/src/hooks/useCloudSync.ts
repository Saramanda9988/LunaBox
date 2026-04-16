import { useCallback, useEffect, useMemo, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";

import type { appconf, vo } from "../../wailsjs/go/models";

import {
  GetCloudSyncStatus,
  SyncNow,
} from "../../wailsjs/go/service/CloudSyncService";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  getEffectiveCloudSyncStatus,
  isCloudProviderConfigured,
} from "../utils/cloudSync";

type UseCloudSyncOptions = {
  config?: appconf.AppConfig | null;
};

export function useCloudSync({ config }: UseCloudSyncOptions) {
  const { t } = useTranslation();
  const [syncStatus, setSyncStatus] = useState<vo.CloudSyncStatus | null>(null);
  const [syncingNow, setSyncingNow] = useState(false);

  useEffect(() => {
    const unsubscribe = EventsOn(
      "cloud-sync:status-changed",
      (status: vo.CloudSyncStatus) => {
        setSyncStatus(status);
      },
    );

    return unsubscribe;
  }, []);

  const refreshSyncStatus = useCallback(async () => {
    if (!config?.cloud_backup_enabled || !config?.cloud_sync_enabled) {
      setSyncStatus(null);
      return null;
    }

    try {
      const status = await GetCloudSyncStatus();
      setSyncStatus(status);
      return status;
    }
    catch (err) {
      console.error("Failed to refresh cloud sync status:", err);
      return null;
    }
  }, [config?.cloud_backup_enabled, config?.cloud_sync_enabled]);

  useEffect(() => {
    if (!config) {
      setSyncStatus(null);
      return;
    }

    void refreshSyncStatus();
  }, [
    config?.backup_user_id,
    config?.cloud_backup_enabled,
    config?.cloud_backup_provider,
    config?.cloud_sync_enabled,
    config?.last_cloud_sync_error,
    config?.last_cloud_sync_status,
    config?.last_cloud_sync_time,
    config?.onedrive_refresh_token,
    config?.s3_access_key,
    config?.s3_endpoint,
    config?.time_zone,
    refreshSyncStatus,
  ]);

  const effectiveSyncStatus = useMemo(
    () => getEffectiveCloudSyncStatus(syncStatus, config),
    [config, syncStatus],
  );
  const syncConfigured
    = effectiveSyncStatus.configured || isCloudProviderConfigured(config);
  const syncBusy = syncingNow || effectiveSyncStatus.syncing;
  const canSyncNow = Boolean(
    config?.cloud_backup_enabled
    && config?.cloud_sync_enabled
    && syncConfigured
    && !syncBusy,
  );

  const handleSyncNow = useCallback(async () => {
    if (
      !config?.cloud_backup_enabled
      || !config?.cloud_sync_enabled
      || !syncConfigured
      || syncBusy
    ) {
      return null;
    }

    setSyncingNow(true);
    const loading = toast.loading(t("settings.cloudBackup.syncingNow"));

    try {
      const status = await SyncNow();
      setSyncStatus(status);
      toast.dismiss(loading);
      toast.success(t("settings.cloudBackup.toast.syncSuccess"));
      return status;
    }
    catch (err: any) {
      toast.dismiss(loading);
      toast.error(
        t("settings.cloudBackup.toast.syncFailed", {
          error: err?.message || err,
        }),
      );
      return null;
    }
    finally {
      setSyncingNow(false);
      void refreshSyncStatus();
    }
  }, [
    config?.cloud_backup_enabled,
    config?.cloud_sync_enabled,
    refreshSyncStatus,
    syncBusy,
    syncConfigured,
    t,
  ]);

  return {
    effectiveSyncStatus,
    syncBusy,
    syncConfigured,
    canSyncNow,
    refreshSyncStatus,
    handleSyncNow,
  };
}
