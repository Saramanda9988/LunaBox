import { useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  ExportLogsZip,
  OpenDataDirectory,
} from "../../../wailsjs/go/service/ConfigService";
import { BetterButton } from "../ui/better/BetterButton";

export function AppDataSettingsPanel() {
  const { t } = useTranslation();
  const [isExportingLogs, setIsExportingLogs] = useState(false);
  const [isOpeningDataDir, setIsOpeningDataDir] = useState(false);

  const handleExportLogs = async () => {
    if (isExportingLogs) {
      return;
    }

    setIsExportingLogs(true);
    try {
      const savePath = await ExportLogsZip();
      if (!savePath) {
        return;
      }
      toast.success(
        t("settings.appData.toast.exportLogsSuccess", { path: savePath }),
      );
    }
    catch (err: any) {
      toast.error(t("settings.appData.toast.exportLogsFailed", { error: err }));
    }
    finally {
      setIsExportingLogs(false);
    }
  };

  const handleOpenDataDirectory = async () => {
    if (isOpeningDataDir) {
      return;
    }

    setIsOpeningDataDir(true);
    try {
      await OpenDataDirectory();
    }
    catch (err: any) {
      toast.error(
        t("settings.appData.toast.openDataDirFailed", { error: err }),
      );
    }
    finally {
      setIsOpeningDataDir(false);
    }
  };

  return (
    <div className="space-y-5">
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.appData.logsTitle")}
        </label>
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.appData.logsHint")}
        </p>
        <BetterButton
          type="button"
          variant="primary"
          icon="i-mdi-folder-zip-outline"
          isLoading={isExportingLogs}
          onClick={handleExportLogs}
        >
          {isExportingLogs
            ? t("settings.appData.exportingLogs")
            : t("settings.appData.exportLogs")}
        </BetterButton>
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.appData.dataDirTitle")}
        </label>
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.appData.dataDirHint")}
        </p>
        <BetterButton
          type="button"
          variant="secondary"
          icon="i-mdi-folder-open-outline"
          isLoading={isOpeningDataDir}
          onClick={handleOpenDataDirectory}
        >
          {t("settings.appData.openDataDir")}
        </BetterButton>
      </div>
    </div>
  );
}
