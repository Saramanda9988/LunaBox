import type { vo } from "../../../wailsjs/go/models";
import { useEffect, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  CreateAndUploadDBBackup,
  DeleteDBBackup,
  GetCloudDBBackups,
  GetDBBackups,
  ScheduleDBRestore,
  ScheduleDBRestoreFromCloud,
  UploadDBBackupToCloud,
} from "../../../wailsjs/go/service/BackupService";
import { SafeQuit } from "../../../wailsjs/go/service/ConfigService";
import { useAppStore } from "../../store";
import { formatFileSize } from "../../utils/size";
import { formatLocalDateTime } from "../../utils/time";
import { ConfirmModal } from "../modal/ConfirmModal";

export function DBBackupPanel() {
  const { t } = useTranslation();
  const { config } = useAppStore();
  const [dbBackups, setDbBackups] = useState<vo.DBBackupStatus | null>(null);
  const [cloudDBBackups, setCloudDBBackups] = useState<vo.CloudBackupItem[]>([]);
  const [isBackingUp, setIsBackingUp] = useState(false);
  const [restoringBackup, setRestoringBackup] = useState<string | null>(null);
  const [uploadingBackup, setUploadingBackup] = useState<string | null>(null);
  const [loadingLocal, setLoadingLocal] = useState(true);
  const [loadingCloud, setLoadingCloud] = useState(false);

  const [confirmConfig, setConfirmConfig] = useState<{
    isOpen: boolean;
    title: string;
    message: string;
    type: "danger" | "info";
    onConfirm: () => void;
  }>({
    isOpen: false,
    title: "",
    message: "",
    type: "info",
    onConfirm: () => { },
  });

  const cloudProvider = config?.cloud_backup_provider;

  const cloudEnabled = (() => {
    if (!config?.cloud_backup_enabled) {
      return false;
    }
    if (cloudProvider === "onedrive") {
      return !!config?.onedrive_refresh_token;
    }
    if (cloudProvider === "s3") {
      return !!config?.backup_user_id;
    }
    return false;
  })();

  const loadDBBackups = async () => {
    setLoadingLocal(true);
    try {
      const backups = await GetDBBackups();
      setDbBackups(backups);
    }
    catch (err) {
      console.error("Failed to load DB backups:", err);
    }
    finally {
      setLoadingLocal(false);
    }
  };

  const loadCloudDBBackups = async () => {
    setLoadingCloud(true);
    try {
      const backups = await GetCloudDBBackups();
      setCloudDBBackups(backups || []);
    }
    catch (err) {
      console.error("Failed to load cloud DB backups:", err);
      setCloudDBBackups([]);
    }
    finally {
      setLoadingCloud(false);
    }
  };

  const handleCreateBackup = async () => {
    if (isBackingUp)
      return;
    setIsBackingUp(true);
    try {
      await CreateAndUploadDBBackup();
      await loadDBBackups();
      if (cloudEnabled)
        await loadCloudDBBackups();
      toast.success(cloudEnabled && config?.auto_upload_db_to_cloud
        ? t("settings.dbBackup.toast.backupSuccessCloud")
        : t("settings.dbBackup.toast.backupSuccess"));
    }
    catch (err: any) {
      if (err.toString().includes("本地备份成功") || err.toString().includes("local backup")) {
        await loadDBBackups();
        toast.success(t("settings.dbBackup.toast.localBackupSuccess"));
        toast.error(err.toString());
      }
      else {
        toast.error(t("settings.dbBackup.toast.backupFailed", { error: err }));
      }
    }
    finally {
      setIsBackingUp(false);
    }
  };

  const handleRestoreDB = async (backupPath: string) => {
    setConfirmConfig({
      isOpen: true,
      title: t("settings.dbBackup.modal.restoreTitle"),
      message: t("settings.dbBackup.modal.restoreMsg"),
      type: "info",
      onConfirm: async () => {
        setRestoringBackup(backupPath);
        try {
          await ScheduleDBRestore(backupPath);
          toast.success(t("settings.dbBackup.toast.restoreScheduled"));
          setTimeout(() => SafeQuit(), 1500);
        }
        catch (err: any) {
          toast.error(t("settings.dbBackup.toast.restoreScheduleFailed", { error: err }));
          setRestoringBackup(null);
        }
      },
    });
  };

  const handleDeleteDBBackup = async (backupPath: string) => {
    setConfirmConfig({
      isOpen: true,
      title: t("settings.dbBackup.modal.deleteTitle"),
      message: t("settings.dbBackup.modal.deleteMsg"),
      type: "danger",
      onConfirm: async () => {
        try {
          await DeleteDBBackup(backupPath);
          await loadDBBackups();
          toast.success(t("settings.dbBackup.toast.deleteSuccess"));
        }
        catch (err: any) {
          toast.error(t("settings.dbBackup.toast.deleteFailed", { error: err }));
        }
      },
    });
  };

  const handleUploadDBBackup = async (backupPath: string) => {
    setUploadingBackup(backupPath);
    try {
      await UploadDBBackupToCloud(backupPath);
      await loadCloudDBBackups();
      toast.success(t("settings.dbBackup.toast.uploadSuccess"));
    }
    catch (err: any) {
      toast.error(t("settings.dbBackup.toast.uploadFailed", { error: err }));
    }
    finally {
      setUploadingBackup(null);
    }
  };

  const handleRestoreFromCloud = async (cloudKey: string) => {
    setConfirmConfig({
      isOpen: true,
      title: t("settings.dbBackup.modal.restoreCloudTitle"),
      message: t("settings.dbBackup.modal.restoreCloudMsg"),
      type: "info",
      onConfirm: async () => {
        setRestoringBackup(cloudKey);
        try {
          await ScheduleDBRestoreFromCloud(cloudKey);
          toast.success(t("settings.dbBackup.toast.restoreScheduled"));
          setTimeout(() => SafeQuit(), 1500);
        }
        catch (err: any) {
          toast.error(t("settings.dbBackup.toast.restoreScheduleFailed", { error: err }));
          setRestoringBackup(null);
        }
      },
    });
  };

  useEffect(() => {
    loadDBBackups();
  }, []);

  useEffect(() => {
    if (cloudEnabled) {
      loadCloudDBBackups();
    }
    else {
      setCloudDBBackups([]);
    }
  }, [cloudEnabled, cloudProvider]);

  const isDisabled = restoringBackup !== null || uploadingBackup !== null || isBackingUp;

  return (
    <div className="space-y-6">
      {/* Backup Action Area */}
      <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h3 className="text-lg font-semibold text-brand-900 dark:text-white">{t("settings.dbBackup.sectionTitle")}</h3>
            <p className="text-sm text-brand-500 dark:text-brand-400 mt-1">
              {t("settings.dbBackup.sectionHint")}
            </p>
          </div>
          <button
            type="button"
            onClick={handleCreateBackup}
            disabled={isDisabled}
            className="glass-btn-neutral px-4 py-2 bg-neutral-600 text-white rounded-md hover:bg-neutral-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
          >
            {isBackingUp && <div className="i-mdi-loading animate-spin" />}
            {isBackingUp ? t("settings.dbBackup.backingUp") : t("settings.dbBackup.backupBtn")}
          </button>
        </div>
        {dbBackups?.last_backup_time && (
          <p className="text-xs text-brand-500 dark:text-brand-400">
            {t("settings.dbBackup.lastBackup")}
            {" "}
            {formatLocalDateTime(dbBackups.last_backup_time, config?.time_zone)}
          </p>
        )}
      </div>

      {/* Local Backup List */}
      <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
        <div className="flex items-center gap-2 mb-4">
          <h3 className="text-lg font-semibold text-brand-900 dark:text-white">{t("settings.dbBackup.localBackups")}</h3>
          {config?.auto_backup_db && (
            <span className="px-2 py-0.5 text-xs font-medium bg-success-100 text-success-700 dark:bg-success-900/30 dark:text-success-400 rounded-full flex items-center gap-1">
              <div className="i-mdi-shield-check text-sm" />
              {t("settings.dbBackup.autoBackupEnabled")}
            </span>
          )}
        </div>
        {loadingLocal
          ? (
              <div className="flex justify-center py-8">
                <div className="i-mdi-loading animate-spin text-2xl text-brand-500" />
              </div>
            )
          : dbBackups?.backups && dbBackups.backups.length > 0
            ? (
                <div className="space-y-3">
                  {dbBackups.backups.map(backup => (
                    <div
                      key={backup.path}
                      className="data-glass:bg-white/1 data-glass:dark:bg-black/1 flex items-center justify-between p-4 bg-brand-50 dark:bg-brand-700 rounded-lg"
                    >
                      <div className="flex items-center gap-4">
                        <div className="i-mdi-database text-2xl text-brand-500" />
                        <div>
                          <div className="font-medium text-brand-900 dark:text-white">
                            {formatLocalDateTime(backup.created_at, config?.time_zone)}
                          </div>
                          <div className="text-sm text-brand-500">
                            {t("settings.dbBackup.sizeLabel")}
                            {formatFileSize(backup.size)}
                          </div>
                        </div>
                      </div>
                      <div className="flex gap-2">
                        {cloudEnabled && (
                          <button
                            type="button"
                            onClick={() => handleUploadDBBackup(backup.path)}
                            disabled={isDisabled}
                            title={t("settings.dbBackup.uploadToCloud")}
                            className="p-2 text-neutral-600 hover:bg-neutral-100 dark:hover:bg-neutral-900 rounded transition-colors disabled:opacity-50"
                          >
                            {uploadingBackup === backup.path
                              ? (
                                  <div className="i-mdi-loading text-xl animate-spin" />
                                )
                              : (
                                  <div className="i-mdi-cloud-upload text-xl" />
                                )}
                          </button>
                        )}
                        <button
                          type="button"
                          onClick={() => handleRestoreDB(backup.path)}
                          disabled={isDisabled}
                          title={t("settings.dbBackup.restoreBackup")}
                          className="p-2 text-success-600 hover:bg-success-100 dark:hover:bg-success-900 rounded transition-colors disabled:opacity-50"
                        >
                          {restoringBackup === backup.path
                            ? (
                                <div className="i-mdi-loading text-xl animate-spin" />
                              )
                            : (
                                <div className="i-mdi-backup-restore text-xl" />
                              )}
                        </button>
                        <button
                          type="button"
                          onClick={() => handleDeleteDBBackup(backup.path)}
                          disabled={isDisabled}
                          title={t("settings.dbBackup.deleteBackup")}
                          className="p-2 text-error-600 hover:bg-error-100 dark:hover:bg-error-900 rounded transition-colors disabled:opacity-50"
                        >
                          <div className="i-mdi-delete text-xl" />
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )
            : (
                <div className="text-center py-8 text-brand-500">{t("settings.dbBackup.noLocalBackups")}</div>
              )}
      </div>

      {/* Cloud Backup List */}
      {cloudEnabled && (
        <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold text-brand-900 dark:text-white flex items-center gap-2">
              <div className="i-mdi-cloud text-xl text-neutral-500" />
              {t("settings.dbBackup.cloudBackups")}
            </h3>
            <button
              type="button"
              onClick={loadCloudDBBackups}
              disabled={loadingCloud || isDisabled}
              title={t("settings.dbBackup.refreshCloudList")}
              className="p-2 text-brand-600 hover:bg-brand-100 dark:hover:bg-brand-700 rounded transition-colors disabled:opacity-50"
            >
              <div className={`i-mdi-refresh text-xl ${loadingCloud ? "animate-spin" : ""}`} />
            </button>
          </div>
          {loadingCloud
            ? (
                <div className="flex justify-center py-8">
                  <div className="i-mdi-loading animate-spin text-2xl text-brand-500" />
                </div>
              )
            : cloudDBBackups.length > 0
              ? (
                  <div className="space-y-3">
                    {cloudDBBackups.map(backup => (
                      <div
                        key={backup.key}
                        className="data-glass:bg-white/1 data-glass:dark:bg-black/1 flex items-center justify-between p-4 bg-neutral-50 dark:bg-neutral-900/30 rounded-lg"
                      >
                        <div className="flex items-center gap-4">
                          <div className="i-mdi-cloud-check text-2xl text-neutral-500" />
                          <div>
                            <div className="font-medium text-brand-900 dark:text-white">
                              {backup.name || formatLocalDateTime(backup.created_at, config?.time_zone)}
                            </div>
                            <div className="text-sm text-brand-500">
                              {formatLocalDateTime(backup.created_at, config?.time_zone)}
                            </div>
                          </div>
                        </div>
                        <div className="flex gap-2">
                          <button
                            type="button"
                            onClick={() => handleRestoreFromCloud(backup.key)}
                            disabled={isDisabled}
                            title={t("settings.dbBackup.restoreFromCloud")}
                            className="p-2 text-success-600 hover:bg-success-100 dark:hover:bg-success-900 rounded transition-colors disabled:opacity-50"
                          >
                            {restoringBackup === backup.key
                              ? (
                                  <div className="i-mdi-loading text-xl animate-spin" />
                                )
                              : (
                                  <div className="i-mdi-cloud-download text-xl" />
                                )}
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                )
              : (
                  <div className="text-center py-8 text-brand-500">{t("settings.dbBackup.noCloudBackups")}</div>
                )}
        </div>
      )}

      {/* Cloud Backup Not Configured Hint */}
      {!cloudEnabled && (
        <div className="bg-brand-50 dark:bg-brand-800 p-4 rounded-lg border border-brand-200 dark:border-brand-700">
          <div className="flex items-center gap-3">
            <div className="i-mdi-cloud-off-outline text-2xl text-brand-400" />
            <div>
              <div className="font-medium text-brand-700 dark:text-brand-300">{t("settings.dbBackup.cloudNotEnabled")}</div>
              <div className="text-sm text-brand-500">{t("settings.dbBackup.cloudNotEnabledHint")}</div>
            </div>
          </div>
        </div>
      )}

      <ConfirmModal
        isOpen={confirmConfig.isOpen}
        title={confirmConfig.title}
        message={confirmConfig.message}
        type={confirmConfig.type}
        onClose={() => setConfirmConfig({ ...confirmConfig, isOpen: false })}
        onConfirm={confirmConfig.onConfirm}
      />
    </div>
  );
}
