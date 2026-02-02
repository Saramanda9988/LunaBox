import type { models, vo } from "../../../wailsjs/go/models";
import { useCallback, useEffect, useState } from "react";
import toast from "react-hot-toast";
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
    onConfirm: () => {},
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
      toast.error("请先设置存档路径");
      return;
    }
    setIsBackingUp(true);
    try {
      await CreateBackup(gameId);
      await loadBackups();
      toast.success("备份创建成功");
    }
    catch (err: any) {
      toast.error(`备份失败: ${err}`);
    }
    finally {
      setIsBackingUp(false);
    }
  };

  const handleRestoreBackup = async (backupPath: string, createdAt: any) => {
    const time = formatLocalDateTime(createdAt);
    setConfirmConfig({
      isOpen: true,
      title: "恢复存档",
      message: `确定要恢复到 ${time} 的备份吗？当前存档将被覆盖。`,
      type: "info",
      onConfirm: async () => {
        try {
          await RestoreBackup(backupPath);
          toast.success("存档已恢复");
        }
        catch (err: any) {
          toast.error(`恢复失败: ${err}`);
        }
      },
    });
  };

  const handleDeleteBackup = async (backupPath: string) => {
    setConfirmConfig({
      isOpen: true,
      title: "删除备份",
      message: "确定要删除此本地备份吗？此操作无法撤销。",
      type: "danger",
      onConfirm: async () => {
        try {
          await DeleteBackup(backupPath);
          await loadBackups();
          toast.success("备份已删除");
        }
        catch (err: any) {
          toast.error(`删除失败: ${err}`);
        }
      },
    });
  };

  const handleOpenBackupFolder = async () => {
    try {
      await OpenBackupFolder(gameId);
    }
    catch (err: any) {
      toast.error(`打开文件夹失败: ${err}`);
    }
  };

  const handleUploadToCloud = async (backupPath: string) => {
    setIsUploading(true);
    try {
      await UploadGameBackupToCloud(gameId, backupPath);
      await loadCloudBackups();
      toast.success("已上传到云端");
    }
    catch (err: any) {
      toast.error(`上传失败: ${err}`);
    }
    finally {
      setIsUploading(false);
    }
  };

  const handleRestoreFromCloud = async (cloudKey: string, name: string) => {
    setConfirmConfig({
      isOpen: true,
      title: "从云端恢复",
      message: `确定要从云端恢复 ${name} 吗？当前存档将被覆盖。`,
      type: "info",
      onConfirm: async () => {
        try {
          await RestoreFromCloud(cloudKey, gameId);
          toast.success("存档已从云端恢复");
        }
        catch (err: any) {
          toast.error(`恢复失败: ${err}`);
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
            <h3 className="text-lg font-semibold text-brand-900 dark:text-white">存档备份</h3>
            <p className="text-sm text-brand-500 dark:text-brand-400 mt-1">
              {savePath ? `存档路径: ${savePath}` : "请先在编辑页面设置存档路径（文件或文件夹）"}
            </p>
          </div>
          <div className="flex gap-2">
            <button
              onClick={handleOpenBackupFolder}
              className="glass-btn-none px-4 py-2 text-brand-600 dark:text-brand-400 hover:bg-brand-100 dark:hover:bg-brand-700 rounded-md transition-colors"
            >
              打开备份文件夹
            </button>
            <button
              onClick={handleCreateBackup}
              disabled={isBackingUp || !savePath}
              className="glass-btn-neutral px-4 py-2 bg-neutral-600 text-white rounded-md hover:bg-neutral-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
            >
              {isBackingUp && <div className="i-mdi-loading animate-spin" />}
              {isBackingUp ? "备份中..." : "立即备份"}
            </button>
          </div>
        </div>
      </div>

      {/* 本地备份历史列表 */}
      <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
        <div className="flex items-center gap-2 mb-4">
          <h3 className="text-lg font-semibold text-brand-900 dark:text-white">本地备份</h3>
          {config?.auto_backup_game_save && (
            <span className="px-2 py-0.5 text-xs font-medium bg-success-100 text-success-700 dark:bg-success-900/30 dark:text-success-400 rounded-full flex items-center gap-1">
              <div className="i-mdi-shield-check text-sm" />
              自动备份已启用
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
                <div className="text-center py-8 text-brand-500">暂无本地备份记录</div>
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
                            {formatLocalDateTime(backup.created_at)}
                          </div>
                          <div className="text-sm text-brand-500">
                            大小:
                            {formatFileSize(backup.size)}
                          </div>
                        </div>
                      </div>
                      <div className="flex gap-2">
                        {cloudEnabled && (
                          <button
                            onClick={() => handleUploadToCloud(backup.path)}
                            disabled={isUploading}
                            title="上传到云端"
                            className="p-2 text-neutral-600 hover:bg-neutral-100 dark:hover:bg-neutral-900 rounded transition-colors disabled:opacity-50"
                          >
                            <div className={`i-mdi-cloud-upload text-xl ${isUploading ? "animate-pulse" : ""}`} />
                          </button>
                        )}
                        <button
                          onClick={() => handleRestoreBackup(backup.path, backup.created_at)}
                          title="恢复备份"
                          className="p-2 text-success-600 hover:bg-success-100 dark:hover:bg-success-900 rounded transition-colors"
                        >
                          <div className="i-mdi-backup-restore text-xl" />
                        </button>
                        <button
                          onClick={() => handleDeleteBackup(backup.path)}
                          title="删除备份"
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
              云端备份
            </h3>
            <button
              onClick={loadCloudBackups}
              disabled={loadingCloud || !cloudEnabled}
              title="刷新云端备份列表"
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
                  <div className="text-center py-8 text-brand-500">暂无云端备份记录</div>
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
                              {backup.name || formatLocalDateTime(backup.created_at)}
                            </div>
                            <div className="text-sm text-brand-500">
                              {formatLocalDateTime(backup.created_at)}
                            </div>
                          </div>
                        </div>
                        <div className="flex gap-2">
                          <button
                            onClick={() => handleRestoreFromCloud(backup.key, backup.name)}
                            title="从云端恢复"
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
              <div className="font-medium text-brand-700 dark:text-brand-300">云备份未启用</div>
              <div className="text-sm text-brand-500">前往设置页面配置云备份，将存档同步到云端</div>
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
