import { useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
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
  const { t } = useTranslation();
  const { config } = useAppStore();
  const [isFullBackingUp, setIsFullBackingUp] = useState(false);
  const [isRestoring, setIsRestoring] = useState(false);

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

  const handleCreateFullBackup = async () => {
    if (isFullBackingUp)
      return;

    try {
      const savePath = await SelectBackupSavePath();

      if (!savePath) {
        return;
      }

      setIsFullBackingUp(true);
      await CreateFullDataBackup(savePath);
      toast.success(t("settings.fullDataBackup.toast.exportSuccess", { path: savePath }));
    }
    catch (err: any) {
      toast.error(t("settings.fullDataBackup.toast.exportFailed", { error: err }));
    }
    finally {
      setIsFullBackingUp(false);
    }
  };

  const handleRestoreFullBackup = async () => {
    if (isRestoring)
      return;

    try {
      const backupPath = await SelectBackupRestorePath();

      if (!backupPath) {
        return;
      }

      setConfirmConfig({
        isOpen: true,
        title: t("settings.fullDataBackup.modal.restoreTitle"),
        message: t("settings.fullDataBackup.modal.restoreMsg", { path: backupPath }),
        type: "danger",
        onConfirm: async () => {
          setIsRestoring(true);
          try {
            await ScheduleFullDataRestore(backupPath);
            toast.success(t("settings.fullDataBackup.toast.restoreScheduled"));
            setTimeout(() => SafeQuit(), 1500);
          }
          catch (err: any) {
            toast.error(t("settings.fullDataBackup.toast.restoreScheduleFailed", { error: err }));
            setIsRestoring(false);
          }
        },
      });
    }
    catch (err: any) {
      toast.error(t("settings.fullDataBackup.toast.selectFileFailed", { error: err }));
    }
  };

  const isDisabled = isRestoring || isFullBackingUp;

  return (
    <div className="space-y-6">
      <div className="rounded-lg">
        <div className="mb-6">
          <div className="space-y-2 text-sm">
            <p className="text-brand-600 dark:text-brand-300">
              {t("settings.fullDataBackup.hint")}
            </p>
            <p className="text-error-600 dark:text-error-400">
              <span className="font-medium">{t("settings.fullDataBackup.warningNote")}</span>
              {t("settings.fullDataBackup.warningHint")}
            </p>
          </div>
        </div>

        <div className="flex flex-col sm:flex-row gap-3">
          <button
            type="button"
            onClick={handleCreateFullBackup}
            disabled={isDisabled}
            className="glass-btn-neutral px-6 py-3 bg-brand-600 text-white rounded-md hover:bg-brand-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
          >
            {isFullBackingUp && <div className="i-mdi-loading animate-spin" />}
            <div className="i-mdi-export text-xl" />
            {isFullBackingUp ? t("settings.fullDataBackup.exporting") : t("settings.fullDataBackup.exportBtn")}
          </button>

          <button
            type="button"
            onClick={handleRestoreFullBackup}
            disabled={isDisabled}
            className="glass-btn-neutral px-6 py-3 bg-warning-600 text-white rounded-md hover:bg-warning-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
          >
            {isRestoring && <div className="i-mdi-loading animate-spin" />}
            <div className="i-mdi-import text-xl" />
            {isRestoring ? t("settings.fullDataBackup.importing") : t("settings.fullDataBackup.importBtn")}
          </button>
        </div>

        {config?.last_full_backup_time && (
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-4">
            {t("settings.fullDataBackup.lastBackup")}
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
