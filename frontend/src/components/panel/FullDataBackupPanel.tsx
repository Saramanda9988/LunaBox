import type { vo } from "../../../wailsjs/go/models";
import { useEffect, useState } from "react";
import toast from "react-hot-toast";
import {
  CreateFullDataBackup,
  DeleteFullDataBackup,
  GetFullDataBackups,
  ScheduleFullDataRestore,
} from "../../../wailsjs/go/service/BackupService";
import { SafeQuit } from "../../../wailsjs/go/service/ConfigService";
import { useAppStore } from "../../store";
import { formatFileSize } from "../../utils/size";
import { formatLocalDateTime } from "../../utils/time";
import { ConfirmModal } from "../modal/ConfirmModal";

export function FullDataBackupPanel() {
  const { config } = useAppStore();
  const [fullBackups, setFullBackups] = useState<vo.DBBackupStatus | null>(null);
  const [isFullBackingUp, setIsFullBackingUp] = useState(false);
  const [restoringBackup, setRestoringBackup] = useState<string | null>(null);
  const [loadingFull, setLoadingFull] = useState(true);

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

  const loadFullBackups = async () => {
    setLoadingFull(true);
    try {
      const backups = await GetFullDataBackups();
      setFullBackups(backups);
    }
    catch (err) {
      console.error("Failed to load full backups:", err);
    }
    finally {
      setLoadingFull(false);
    }
  };

  const handleCreateFullBackup = async () => {
    if (isFullBackingUp)
      return;
    setIsFullBackingUp(true);
    try {
      await CreateFullDataBackup();
      await loadFullBackups();
      toast.success("全量数据备份成功");
    }
    catch (err: any) {
      toast.error(`全量备份失败: ${err}`);
    }
    finally {
      setIsFullBackingUp(false);
    }
  };

  const handleRestoreFullBackup = async (backupPath: string) => {
    setConfirmConfig({
      isOpen: true,
      title: "恢复全量数据",
      message: "确定要恢复全量数据吗？将覆盖应用配置、数据库和资源目录，程序会退出并在下次启动时恢复。",
      type: "danger",
      onConfirm: async () => {
        setRestoringBackup(backupPath);
        try {
          await ScheduleFullDataRestore(backupPath);
          toast.success("已安排全量恢复，程序即将退出...");
          setTimeout(() => SafeQuit(), 1500);
        }
        catch (err: any) {
          toast.error(`安排全量恢复失败: ${err}`);
          setRestoringBackup(null);
        }
      },
    });
  };

  const handleDeleteFullBackup = async (backupPath: string) => {
    setConfirmConfig({
      isOpen: true,
      title: "删除全量备份",
      message: "确定要删除此全量数据备份吗？此操作无法撤销。",
      type: "danger",
      onConfirm: async () => {
        try {
          await DeleteFullDataBackup(backupPath);
          await loadFullBackups();
          toast.success("全量备份已删除");
        }
        catch (err: any) {
          toast.error(`删除失败: ${err}`);
        }
      },
    });
  };

  useEffect(() => {
    loadFullBackups();
  }, []);

  const isDisabled = restoringBackup !== null || isFullBackingUp;

  return (
    <div className="space-y-6">
      {/* 全量数据备份操作区 */}
      <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-4">
          <div className="flex-1">
            <h3 className="text-lg font-semibold text-brand-900 dark:text-white">全量数据备份</h3>
            <p className="text-sm text-brand-500 dark:text-brand-400 mt-1">
              备份应用配置、数据库、封面与背景、图片缓存、日志及本地备份目录（暂不上传云端）
            </p>
            <p className="text-sm text-error-500 dark:text-error-400 mt-1">
              适合在进行重大更改前备份，或在不同设备间迁移数据使用。恢复后会覆盖现有数据，请谨慎操作。
            </p>
          </div>
          <button
            onClick={handleCreateFullBackup}
            disabled={isDisabled}
            className="glass-btn-neutral px-4 py-2 bg-neutral-600 text-white rounded-md hover:bg-neutral-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
          >
            {isFullBackingUp && <div className="i-mdi-loading animate-spin" />}
            {isFullBackingUp ? "打包中..." : "立即备份"}
          </button>
        </div>
        {fullBackups?.last_backup_time && (
          <p className="text-xs text-brand-500 dark:text-brand-400">
            上次备份:
            {" "}
            {formatLocalDateTime(fullBackups.last_backup_time, config?.time_zone)}
          </p>
        )}
      </div>

      {/* 全量数据备份列表 */}
      <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
        <h3 className="text-lg font-semibold text-brand-900 dark:text-white mb-4">全量数据包</h3>
        {loadingFull
          ? (
              <div className="flex justify-center py-8">
                <div className="i-mdi-loading animate-spin text-2xl text-brand-500" />
              </div>
            )
          : fullBackups?.backups && fullBackups.backups.length > 0
            ? (
                <div className="space-y-3">
                  {fullBackups.backups.map(backup => (
                    <div
                      key={backup.path}
                      className="data-glass:bg-white/1 data-glass:dark:bg-black/1 flex items-center justify-between p-4 bg-brand-50 dark:bg-brand-700 rounded-lg"
                    >
                      <div className="flex items-center gap-4">
                        <div className="i-mdi-package-variant text-2xl text-brand-500" />
                        <div>
                          <div className="font-medium text-brand-900 dark:text-white">
                            {formatLocalDateTime(backup.created_at, config?.time_zone)}
                          </div>
                          <div className="text-sm text-brand-500">
                            大小:
                            {formatFileSize(backup.size)}
                          </div>
                        </div>
                      </div>
                      <div className="flex gap-2">
                        <button
                          onClick={() => handleRestoreFullBackup(backup.path)}
                          disabled={isDisabled}
                          title="恢复全量数据"
                          className="p-2 text-warning-600 hover:bg-warning-100 dark:hover:bg-warning-900 rounded transition-colors disabled:opacity-50"
                        >
                          {restoringBackup === backup.path
                            ? (
                                <div className="i-mdi-loading text-xl animate-spin" />
                              )
                            : (
                                <div className="i-mdi-database-sync text-xl" />
                              )}
                        </button>
                        <button
                          onClick={() => handleDeleteFullBackup(backup.path)}
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
                <div className="text-center py-8 text-brand-500">暂无全量数据备份</div>
              )}
      </div>

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
