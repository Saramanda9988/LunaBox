import { useState } from "react";
import toast from "react-hot-toast";
import {
  CreateFullDataBackup,
  ScheduleFullDataRestore,
  SelectBackupRestorePath,
  SelectBackupSavePath,
} from "../../../wailsjs/go/service/BackupService";
import { SafeQuit } from "../../../wailsjs/go/service/ConfigService";
import { useAppStore } from "../../store";
import { formatLocalDateTime } from "../../utils/time";
import { ConfirmModal } from "../modal/ConfirmModal";

export function FullDataBackupPanel() {
  const { config } = useAppStore();
  const [isFullBackingUp, setIsFullBackingUp] = useState(false);
  const [isRestoring, setIsRestoring] = useState(false);

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

  const handleCreateFullBackup = async () => {
    if (isFullBackingUp)
      return;

    try {
      // 调用后端打开保存文件对话框
      const savePath = await SelectBackupSavePath();

      if (!savePath) {
        return; // 用户取消
      }

      setIsFullBackingUp(true);
      await CreateFullDataBackup(savePath);
      toast.success(`全量数据备份成功: ${savePath}`);
    }
    catch (err: any) {
      toast.error(`全量备份失败: ${err}`);
    }
    finally {
      setIsFullBackingUp(false);
    }
  };

  const handleRestoreFullBackup = async () => {
    if (isRestoring)
      return;

    try {
      // 调用后端打开文件选择对话框
      const backupPath = await SelectBackupRestorePath();

      if (!backupPath) {
        return; // 用户取消
      }

      setConfirmConfig({
        isOpen: true,
        title: "恢复全量数据",
        message: `确定要恢复全量数据吗？\n\n备份文件: ${backupPath}\n\n将覆盖应用配置、数据库和资源目录，程序会退出并在下次启动时恢复。`,
        type: "danger",
        onConfirm: async () => {
          setIsRestoring(true);
          try {
            await ScheduleFullDataRestore(backupPath);
            toast.success("已安排全量恢复，程序即将退出...");
            setTimeout(() => SafeQuit(), 1500);
          }
          catch (err: any) {
            toast.error(`安排全量恢复失败: ${err}`);
            setIsRestoring(false);
          }
        },
      });
    }
    catch (err: any) {
      toast.error(`选择文件失败: ${err}`);
    }
  };

  const isDisabled = isRestoring || isFullBackingUp;

  return (
    <div className="space-y-6">
      {/* 全量数据备份说明和操作区 */}
      <div className="rounded-lg">
        <div className="mb-6">
          <div className="space-y-2 text-sm">
            <p className="text-brand-600 dark:text-brand-300">
              备份应用配置、数据库、封面与背景、图片缓存、日志及本地备份目录
            </p>
            <p className="text-error-600 dark:text-error-400">
              <span className="font-medium">注意：</span>
              适合在进行重大更改前备份，或在不同设备间迁移数据使用。恢复后会覆盖现有数据，请谨慎操作。
            </p>
          </div>
        </div>

        <div className="flex flex-col sm:flex-row gap-3">
          <button
            onClick={handleCreateFullBackup}
            disabled={isDisabled}
            className="glass-btn-neutral px-6 py-3 bg-brand-600 text-white rounded-md hover:bg-brand-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
          >
            {isFullBackingUp && <div className="i-mdi-loading animate-spin" />}
            <div className="i-mdi-export text-xl" />
            {isFullBackingUp ? "正在打包..." : "导出备份"}
          </button>

          <button
            onClick={handleRestoreFullBackup}
            disabled={isDisabled}
            className="glass-btn-neutral px-6 py-3 bg-warning-600 text-white rounded-md hover:bg-warning-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
          >
            {isRestoring && <div className="i-mdi-loading animate-spin" />}
            <div className="i-mdi-import text-xl" />
            {isRestoring ? "正在准备..." : "导入恢复"}
          </button>
        </div>

        {config?.last_full_backup_time && (
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-4">
            上次备份时间：
            {formatLocalDateTime(config.last_full_backup_time, config?.time_zone)}
          </p>
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
