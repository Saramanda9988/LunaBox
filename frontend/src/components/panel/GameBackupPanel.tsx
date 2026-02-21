import type { models, vo } from "../../../wailsjs/go/models";
import { useCallback, useEffect, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  CreateBackup,
  DeleteBackup,
  GetCloudBackupStatus,
  GetCloudGameBackups,
  GetGameBackups,
  OpenBackupFolder,
  RestoreBackup,
  RestoreFromCloud,
  UploadGameBackupToCloud,
} from "../../../wailsjs/go/service/BackupService";
import { useAppStore } from "../../store";
import { formatFileSize } from "../../utils/size";
import { formatLocalDateTime } from "../../utils/time";
import { ConfirmModal } from "../modal/ConfirmModal";

interface GameBackupPanelProps {
  gameId: string;
  savePath?: string;
}

export function GameBackupPanel({ gameId, savePath }: GameBackupPanelProps) {
  const { t } = useTranslation();
  const { config } = useAppStore();
  const [backups, setBackups] = useState<models.GameBackup[]>([]);
  const [cloudBackups, setCloudBackups] = useState<vo.CloudBackupItem[]>([]);
  const [cloudStatus, setCloudStatus] = useState<vo.CloudBackupStatus | null>(null);
  const [isBackingUp, setIsBackingUp] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [loadingLocal, setLoadingLocal] = useState(true);
  const [loadingCloud, setLoadingCloud] = useState(false);

  // 确认弹窗状态
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

  const loadBackups = useCallback(async () => {
    setLoadingLocal(true);
    try {
      const data = await GetGameBackups(gameId);
      setBackups(data || []);
    }
    catch (err) {
      console.error("Failed to load backups:", err);
    }
    finally {
      setLoadingLocal(false);
    }
  }, [gameId]);

  const loadCloudStatus = useCallback(async () => {
    try {
      const status = await GetCloudBackupStatus();
      setCloudStatus(status);
    }
    catch (err) {
      console.error("Failed to load cloud status:", err);
    }
  }, []);

  const loadCloudBackups = useCallback(async () => {
    setLoadingCloud(true);
    try {
      const data = await GetCloudGameBackups(gameId);
      setCloudBackups(data || []);
    }
    catch (err) {
      console.error("Failed to load cloud backups:", err);
    }
    finally {
      setLoadingCloud(false);
    }
  }, [gameId]);

  useEffect(() => {
    loadBackups();
    loadCloudStatus();
  }, [loadBackups, loadCloudStatus]);

  useEffect(() => {
    if (cloudStatus?.configured && cloudStatus?.enabled) {
      loadCloudBackups();
    }
  }, [cloudStatus, loadCloudBackups]);

  const handleCreateBackup = async () => {
    if (!savePath) {
      toast.error(t("gameBackup.toast.savePathRequired"));
      return;
    }
    setIsBackingUp(true);
    try {
      await CreateBackup(gameId);
      await loadBackups();
      toast.success(t("gameBackup.toast.backupCreated"));
    }
    catch (err: any) {
      toast.error(t("gameBackup.toast.backupFailed", { error: err }));
    }
    finally {
      setIsBackingUp(false);
    }
  };

  const handleRestoreBackup = async (backupPath: string, createdAt: any) => {
    const time = formatLocalDateTime(createdAt, config?.time_zone);
    setConfirmConfig({
      isOpen: true,
      title: t("gameBackup.modal.restoreTitle"),
      message: t("gameBackup.modal.restoreMsg", { time }),
      type: "info",
      onConfirm: async () => {
        try {
          await RestoreBackup(backupPath);
          toast.success(t("gameBackup.toast.restored"));
        }
        catch (err: any) {
          toast.error(t("gameBackup.toast.restoreFailed", { error: err }));
        }
      },
    });
  };

  const handleDeleteBackup = async (backupPath: string) => {
    setConfirmConfig({
      isOpen: true,
      title: t("gameBackup.modal.deleteTitle"),
      message: t("gameBackup.modal.deleteMsg"),
      type: "danger",
      onConfirm: async () => {
        try {
          await DeleteBackup(backupPath);
          await loadBackups();
          toast.success(t("gameBackup.toast.deleted"));
        }
        catch (err: any) {
          toast.error(t("gameBackup.toast.deleteFailed", { error: err }));
        }
      },
    });
  };

  const handleOpenBackupFolder = async () => {
    try {
      await OpenBackupFolder(gameId);
    }
    catch (err: any) {
      toast.error(t("gameBackup.toast.openFolderFailed", { error: err }));
    }
  };

  const handleUploadToCloud = async (backupPath: string) => {
    setIsUploading(true);
    try {
      await UploadGameBackupToCloud(gameId, backupPath);
      await loadCloudBackups();
      toast.success(t("gameBackup.toast.uploaded"));
    }
    catch (err: any) {
      toast.error(t("gameBackup.toast.uploadFailed", { error: err }));
    }
    finally {
      setIsUploading(false);
    }
  };

  const handleRestoreFromCloud = async (cloudKey: string, name: string) => {
    setConfirmConfig({
      isOpen: true,
      title: t("gameBackup.modal.restoreCloudTitle"),
      message: t("gameBackup.modal.restoreCloudMsg", { name }),
      type: "info",
      onConfirm: async () => {
        try {
          await RestoreFromCloud(cloudKey, gameId);
          toast.success(t("gameBackup.toast.restoredFromCloud"));
        }
        catch (err: any) {
          toast.error(t("gameBackup.toast.restoreFailed", { error: err }));
        }
      },
    });
  };

  const cloudEnabled = cloudStatus?.configured && cloudStatus?.enabled;

  return (
    <div className="space-y-6">
      {/* 备份操作区 */}
      <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h3 className="text-lg font-semibold text-brand-900 dark:text-white">{t("gameBackup.title")}</h3>
            <p className="text-sm text-brand-500 dark:text-brand-400 mt-1">
              {savePath ? t("gameBackup.pathLabel", { path: savePath }) : t("gameBackup.pathPlaceholder")}
            </p>
          </div>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={handleOpenBackupFolder}
              className="glass-btn-none px-4 py-2 text-brand-600 dark:text-brand-400 hover:bg-brand-100 dark:hover:bg-brand-700 rounded-md transition-colors"
            >
              {t("gameBackup.openFolder")}
            </button>
            <button
              type="button"
              onClick={handleCreateBackup}
              disabled={isBackingUp || !savePath}
              className="glass-btn-neutral px-4 py-2 bg-neutral-600 text-white rounded-md hover:bg-neutral-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
            >
              {isBackingUp && <div className="i-mdi-loading animate-spin" />}
              {isBackingUp ? t("gameBackup.backingUp") : t("gameBackup.backupNow")}
            </button>
          </div>
        </div>
      </div>

      {/* 本地备份历史列表 */}
      <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
        <div className="flex items-center gap-2 mb-4">
          <h3 className="text-lg font-semibold text-brand-900 dark:text-white">{t("gameBackup.localBackups")}</h3>
          {config?.auto_backup_game_save && (
            <span className="px-2 py-0.5 text-xs font-medium bg-success-100 text-success-700 dark:bg-success-900/30 dark:text-success-400 rounded-full flex items-center gap-1">
              <div className="i-mdi-shield-check text-sm" />
              {t("gameBackup.autoBackupEnabled")}
            </span>
          )}
        </div>
        {loadingLocal
          ? (
              <div className="flex justify-center py-8">
                <div className="i-mdi-loading animate-spin text-2xl text-brand-500" />
              </div>
            )
          : backups.length === 0
            ? (
                <div className="text-center py-8 text-brand-500">{t("gameBackup.noLocalBackups")}</div>
              )
            : (
                <div className="space-y-3">
                  {backups.map(backup => (
                    <div
                      key={backup.path}
                      className="data-glass:bg-white/1 data-glass:dark:bg-black/1 flex items-center justify-between p-4 bg-brand-50 dark:bg-brand-700 rounded-lg"
                    >
                      <div className="flex items-center gap-4">
                        <div className="i-mdi-archive text-2xl text-brand-500" />
                        <div>
                          <div className="font-medium text-brand-900 dark:text-white">
                            {formatLocalDateTime(backup.created_at, config?.time_zone)}
                          </div>
                          <div className="text-sm text-brand-500">
                            {t("gameBackup.size")}
                            {formatFileSize(backup.size)}
                          </div>
                        </div>
                      </div>
                      <div className="flex gap-2">
                        {cloudEnabled && (
                          <button
                            onClick={() => handleUploadToCloud(backup.path)}
                            disabled={isUploading}
                            title={t("gameBackup.uploadToCloud")}
                            className="p-2 text-neutral-600 hover:bg-neutral-100 dark:hover:bg-neutral-900 rounded transition-colors disabled:opacity-50"
                          >
                            <div className={`i-mdi-cloud-upload text-xl ${isUploading ? "animate-pulse" : ""}`} />
                          </button>
                        )}
                        <button
                          onClick={() => handleRestoreBackup(backup.path, backup.created_at)}
                          title={t("gameBackup.restore")}
                          className="p-2 text-success-600 hover:bg-success-100 dark:hover:bg-success-900 rounded transition-colors"
                        >
                          <div className="i-mdi-backup-restore text-xl" />
                        </button>
                        <button
                          onClick={() => handleDeleteBackup(backup.path)}
                          title={t("common.delete")}
                          className="p-2 text-error-600 hover:bg-error-100 dark:hover:bg-error-900 rounded transition-colors"
                        >
                          <div className="i-mdi-delete text-xl" />
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
      </div>

      {/* 云端备份列表 */}
      {cloudEnabled && (
        <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold text-brand-900 dark:text-white flex items-center gap-2">
              <div className="i-mdi-cloud text-xl text-neutral-500" />
              {t("gameBackup.cloudBackups")}
            </h3>
            <button
              type="button"
              onClick={loadCloudBackups}
              disabled={loadingCloud || !cloudEnabled}
              title={t("gameBackup.refreshCloudList")}
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
            : cloudBackups.length === 0
              ? (
                  <div className="text-center py-8 text-brand-500">{t("gameBackup.noCloudBackups")}</div>
                )
              : (
                  <div className="space-y-3">
                    {cloudBackups.map(backup => (
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
                            onClick={() => handleRestoreFromCloud(backup.key, backup.name)}
                            title={t("gameBackup.restoreFromCloud")}
                            className="p-2 text-success-600 hover:bg-success-100 dark:hover:bg-success-900 rounded transition-colors"
                          >
                            <div className="i-mdi-cloud-download text-xl" />
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
        </div>
      )}

      {/* 云备份未配置提示 */}
      {!cloudEnabled && (
        <div className="bg-brand-50 dark:bg-brand-800 p-4 rounded-lg border border-brand-200 dark:border-brand-700">
          <div className="flex items-center gap-3">
            <div className="i-mdi-cloud-off-outline text-2xl text-brand-400" />
            <div>
              <div className="font-medium text-brand-700 dark:text-brand-300">{t("gameBackup.cloudNotEnabled")}</div>
              <div className="text-sm text-brand-500">{t("gameBackup.cloudConfigHint")}</div>
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
