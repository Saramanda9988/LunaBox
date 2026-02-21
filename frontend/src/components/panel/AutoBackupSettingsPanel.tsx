import type { appconf } from "../../../wailsjs/go/models";
import { useTranslation } from "react-i18next";
import { BetterSwitch } from "../ui/BetterSwitch";

interface AutoBackupSettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

export function AutoBackupSettingsPanel({ formData, onChange }: AutoBackupSettingsProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex-1">
          <label htmlFor="auto_backup_db" className="text-sm font-medium text-brand-700 dark:text-brand-300 cursor-pointer">
            {t("settings.autoBackup.backupDbOnExit")}
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
            {t("settings.autoBackup.backupDbOnExitHint")}
          </p>
        </div>
        <BetterSwitch
          id="auto_backup_db"
          checked={formData.auto_backup_db || false}
          onCheckedChange={checked => onChange({ ...formData, auto_backup_db: checked } as appconf.AppConfig)}
        />
      </div>

      <div className="flex items-center justify-between">
        <div className="flex-1">
          <label htmlFor="auto_backup_game_save" className="text-sm font-medium text-brand-700 dark:text-brand-300 cursor-pointer">
            {t("settings.autoBackup.backupSaveOnExit")}
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
            {t("settings.autoBackup.backupSaveOnExitHint")}
          </p>
        </div>
        <BetterSwitch
          id="auto_backup_game_save"
          checked={formData.auto_backup_game_save || false}
          onCheckedChange={checked => onChange({ ...formData, auto_backup_game_save: checked } as appconf.AppConfig)}
        />
      </div>

      <div className="flex items-center justify-between">
        <div className="flex-1">
          <label htmlFor="auto_upload_db_to_cloud" className={`text-sm font-medium cursor-pointer ${formData.cloud_backup_enabled ? "text-brand-700 dark:text-brand-300" : "text-brand-400 dark:text-brand-500"}`}>
            {t("settings.autoBackup.autoUploadDb")}
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
            {t("settings.autoBackup.autoUploadDbHint")}
          </p>
        </div>
        <BetterSwitch
          id="auto_upload_db_to_cloud"
          checked={formData.auto_upload_db_to_cloud || false}
          onCheckedChange={checked => onChange({ ...formData, auto_upload_db_to_cloud: checked } as appconf.AppConfig)}
          disabled={!formData.cloud_backup_enabled}
        />
      </div>

      <div className="flex items-center justify-between">
        <div className="flex-1">
          <label htmlFor="auto_upload_game_save_to_cloud" className={`text-sm font-medium cursor-pointer ${formData.cloud_backup_enabled ? "text-brand-700 dark:text-brand-300" : "text-brand-400 dark:text-brand-500"}`}>
            {t("settings.autoBackup.autoUploadSave")}
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
            {t("settings.autoBackup.autoUploadSaveHint")}
          </p>
        </div>
        <BetterSwitch
          id="auto_upload_game_save_to_cloud"
          checked={formData.auto_upload_game_save_to_cloud || false}
          onCheckedChange={checked => onChange({ ...formData, auto_upload_game_save_to_cloud: checked } as appconf.AppConfig)}
          disabled={!formData.cloud_backup_enabled}
        />
      </div>

      <div className="pt-4 border-t border-brand-300 dark:border-brand-700 grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="space-y-2">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">{t("settings.autoBackup.localGameRetention")}</label>
          <input
            type="number"
            name="local_backup_retention"
            value={formData.local_backup_retention || 10}
            onChange={e => onChange({ ...formData, local_backup_retention: Number.parseInt(e.target.value) || 0 } as appconf.AppConfig)}
            className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
          />
          <p className="text-xs text-brand-500 dark:text-brand-400">{t("settings.autoBackup.localGameRetentionHint")}</p>
        </div>

        <div className="space-y-2">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">{t("settings.autoBackup.localDbRetention")}</label>
          <input
            type="number"
            name="local_db_backup_retention"
            value={formData.local_db_backup_retention || 5}
            onChange={e => onChange({ ...formData, local_db_backup_retention: Number.parseInt(e.target.value) || 0 } as appconf.AppConfig)}
            className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
          />
          <p className="text-xs text-brand-500 dark:text-brand-400">{t("settings.autoBackup.localDbRetentionHint")}</p>
        </div>
      </div>
    </div>
  );
}
