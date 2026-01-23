import { useEffect, useState } from "react";
import toast from "react-hot-toast";
import {
  CreateAndUploadDBBackup,
  DeleteDBBackup,
  GetCloudDBBackups,
  GetDBBackups,
  ScheduleDBRestore,
  ScheduleDBRestoreFromCloud,
  UploadDBBackupToCloud,
} from "../../../bindings/lunabox/internal/service/BackupService";
import { SafeQuit } from "../../../bindings/lunabox/internal/service/ConfigService";
import { useAppStore } from "../../store";
import { formatFileSize } from "../../utils/size";
import { formatLocalDateTime } from "../../utils/time";
import { ConfirmModal } from "../modal/ConfirmModal";
import {CloudBackupItem, DBBackupStatus} from "../../../bindings/lunabox/internal/vo";

export function DBBackupPanel() {
  const { config } = useAppStore();
  const [dbBackups, setDbBackups] = useState<DBBackupStatus | null>(null);
  const [cloudDBBackups, setCloudDBBackups] = useState<CloudBackupItem[]>([]);
  const [isBackingUp, setIsBackingUp] = useState(false);
  const [restoringBackup, setRestoringBackup] = useState<string | null>(null);
  const [uploadingBackup, setUploadingBackup] = useState<string | null>(null);
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

  const cloudProvider = config?.cloud_backup_provider;

  // 检查云备份是否真正可用
  const cloudEnabled = (() => {
    if (!config?.cloud_backup_enabled) {
      return false;
    }
    // 如果是OneDrive，需要检查是否已授权
    if (cloudProvider === "onedrive") {
      return !!config?.onedrive_refresh_token;
    }
    // S3或其他provider需要backup_user_id
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
      toast.success(cloudEnabled && config?.auto_upload_db_to_cloud ? "数据库备份成功并已上传云端" : "数据库备份成功");
    }
    catch (err: any) {
      if (err.toString().includes("本地备份成功")) {
        await loadDBBackups();
        toast.success("本地备份成功");
        toast.error(err.toString());
      }
      else {
        toast.error(`备份失败: ${err}`);
      }
    }
    finally {
      setIsBackingUp(false);
    }
  };

  const handleRestoreDB = async (backupPath: string) => {
    setConfirmConfig({
      isOpen: true,
      title: "恢复数据库",
      message: "确定要恢复到此备份吗？程序将退出并在下次启动时完成恢复。",
      type: "info",
      onConfirm: async () => {
        setRestoringBackup(backupPath);
        try {
          await ScheduleDBRestore(backupPath);
          toast.success("已安排恢复，程序即将退出...");
          setTimeout(() => SafeQuit(), 1500);
        }
        catch (err: any) {
          toast.error(`安排恢复失败: ${err}`);
          setRestoringBackup(null);
        }
      },
    });
  };

  const handleDeleteDBBackup = async (backupPath: string) => {
    setConfirmConfig({
      isOpen: true,
      title: "删除备份",
      message: "确定要删除此本地备份吗？此操作无法撤销。",
      type: "danger",
      onConfirm: async () => {
        try {
          await DeleteDBBackup(backupPath);
          await loadDBBackups();
          toast.success("备份已删除");
        }
        catch (err: any) {
          toast.error(`删除失败: ${err}`);
        }
      },
    });
  };

  const handleUploadDBBackup = async (backupPath: string) => {
    setUploadingBackup(backupPath);
    try {
      await UploadDBBackupToCloud(backupPath);
      await loadCloudDBBackups();
      toast.success("已上传到云端");
    }
    catch (err: any) {
      toast.error(`上传失败: ${err}`);
    }
    finally {
      setUploadingBackup(null);
    }
  };

  const handleRestoreFromCloud = async (cloudKey: string) => {
    setConfirmConfig({
      isOpen: true,
      title: "从云端恢复",
      message: "确定要从云端恢复此备份吗？程序将退出并在下次启动时完成恢复。",
      type: "info",
      onConfirm: async () => {
        setRestoringBackup(cloudKey);
        try {
          await ScheduleDBRestoreFromCloud(cloudKey);
          toast.success("已安排恢复，程序即将退出...");
          setTimeout(() => SafeQuit(), 1500);
        }
        catch (err: any) {
          toast.error(`安排恢复失败: ${err}`);
          setRestoringBackup(null);
        }
      },
    });
  };

  useEffect(() => {
    loadDBBackups();
  }, []);

  // 云存储提供商变化时自动刷新云备份列表
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
      {/* 备份操作区 */}
      <div className="bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h3 className="text-lg font-semibold text-brand-900 dark:text-white">数据库备份</h3>
            <p className="text-sm text-brand-500 dark:text-brand-400 mt-1">
              备份游戏库元数据、分类、游玩记录等应用数据
            </p>
          </div>
          <button
            onClick={handleCreateBackup}
            disabled={isDisabled}
            className="px-4 py-2 bg-neutral-600 text-white rounded-md hover:bg-neutral-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
          >
            {isBackingUp && <div className="i-mdi-loading animate-spin" />}
            {isBackingUp ? "备份中..." : "立即备份"}
          </button>
        </div>
        {dbBackups?.last_backup_time && (
          <p className="text-xs text-brand-500 dark:text-brand-400">
            上次备份:
            {" "}
            {formatLocalDateTime(dbBackups.last_backup_time)}
          </p>
        )}
      </div>

      {/* 本地备份列表 */}
      <div className="bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
        <div className="flex items-center gap-2 mb-4">
          <h3 className="text-lg font-semibold text-brand-900 dark:text-white">本地备份</h3>
          {config?.auto_backup_db && (
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
          : dbBackups?.backups && dbBackups.backups.length > 0
            ? (
                <div className="space-y-3">
                  {dbBackups.backups.map(backup => (
                    <div
                      key={backup.path}
                      className="flex items-center justify-between p-4 bg-brand-50 dark:bg-brand-700 rounded-lg"
                    >
                      <div className="flex items-center gap-4">
                        <div className="i-mdi-database text-2xl text-brand-500" />
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
                            onClick={() => handleUploadDBBackup(backup.path)}
                            disabled={isDisabled}
                            title="上传到云端"
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
                          onClick={() => handleRestoreDB(backup.path)}
                          disabled={isDisabled}
                          title="恢复此备份"
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
                          onClick={() => handleDeleteDBBackup(backup.path)}
                          disabled={isDisabled}
                          title="删除备份"
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
                <div className="text-center py-8 text-brand-500">暂无本地备份记录</div>
              )}
      </div>

      {/* 云端备份列表 */}
      {cloudEnabled && (
        <div className="bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold text-brand-900 dark:text-white flex items-center gap-2">
              <div className="i-mdi-cloud text-xl text-neutral-500" />
              云端备份
            </h3>
            <button
              onClick={loadCloudDBBackups}
              disabled={loadingCloud || isDisabled}
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
            : cloudDBBackups.length > 0
              ? (
                  <div className="space-y-3">
                    {cloudDBBackups.map(backup => (
                      <div
                        key={backup.key}
                        className="flex items-center justify-between p-4 bg-neutral-50 dark:bg-neutral-900/30 rounded-lg"
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
                            onClick={() => handleRestoreFromCloud(backup.key)}
                            disabled={isDisabled}
                            title="从云端恢复"
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
                  <div className="text-center py-8 text-brand-500">暂无云端备份记录</div>
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
              <div className="text-sm text-brand-500">在上方配置云备份后，可将数据库同步到云端</div>
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
