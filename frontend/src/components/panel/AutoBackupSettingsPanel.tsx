import type { appconf } from "../../../wailsjs/go/models";
import { useTranslation } from "react-i18next";
import {
  getSyncIntervalSeconds,
  isCloudProviderConfigured,
} from "../../utils/cloudSync";
import { BetterSwitch } from "../ui/better/BetterSwitch";

interface AutoBackupSettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

interface SettingSwitchRowProps {
  id: string;
  label: string;
  hint: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
}

function SettingSwitchRow({
  id,
  label,
  hint,
  checked,
  onCheckedChange,
  disabled = false,
}: SettingSwitchRowProps) {
  const textClass = disabled
    ? "text-brand-400 dark:text-brand-500"
    : "text-brand-700 dark:text-brand-300";

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-4">
        <div className="flex-1 space-y-2">
          <label
            htmlFor={id}
            className={`block cursor-pointer text-sm font-medium ${textClass}`}
          >
            {label}
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400">{hint}</p>
        </div>
        <BetterSwitch
          id={id}
          checked={checked}
          onCheckedChange={onCheckedChange}
          disabled={disabled}
        />
      </div>
    </div>
  );
}

export function AutoBackupSettingsPanel({
  formData,
  onChange,
}: AutoBackupSettingsProps) {
  const { t } = useTranslation();
  const cloudServiceEnabled = formData.cloud_backup_enabled || false;
  const cloudConfigured = isCloudProviderConfigured(formData);
  const syncIntervalSeconds = getSyncIntervalSeconds(formData);

  return (
    <div className="space-y-4">
      <div className="space-y-4">
        <div>
          <div className="block text-sm font-semibold text-brand-700 dark:text-brand-300">
            {t("settings.autoBackup.localSection")}
          </div>
        </div>

        <SettingSwitchRow
          id="auto_backup_db"
          label={t("settings.autoBackup.backupDbOnExit")}
          hint={t("settings.autoBackup.backupDbOnExitHint")}
          checked={formData.auto_backup_db || false}
          onCheckedChange={checked =>
            onChange({
              ...formData,
              auto_backup_db: checked,
            } as appconf.AppConfig)}
        />

        <SettingSwitchRow
          id="auto_backup_game_save"
          label={t("settings.autoBackup.backupSaveOnExit")}
          hint={t("settings.autoBackup.backupSaveOnExitHint")}
          checked={formData.auto_backup_game_save || false}
          onCheckedChange={checked =>
            onChange({
              ...formData,
              auto_backup_game_save: checked,
            } as appconf.AppConfig)}
        />

        <div className="grid grid-cols-1 gap-4 border-t border-brand-200 pt-4 dark:border-brand-700 md:grid-cols-2">
          <div className="space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              {t("settings.autoBackup.localGameRetention")}
            </label>
            <input
              type="number"
              name="local_backup_retention"
              value={formData.local_backup_retention || 10}
              onChange={e =>
                onChange({
                  ...formData,
                  local_backup_retention:
                    Number.parseInt(e.target.value, 10) || 0,
                } as appconf.AppConfig)}
              className="glass-input w-full rounded-md border border-brand-300 px-3 py-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
            />
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("settings.autoBackup.localGameRetentionHint")}
            </p>
          </div>

          <div className="space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              {t("settings.autoBackup.localDbRetention")}
            </label>
            <input
              type="number"
              name="local_db_backup_retention"
              value={formData.local_db_backup_retention || 5}
              onChange={e =>
                onChange({
                  ...formData,
                  local_db_backup_retention:
                    Number.parseInt(e.target.value, 10) || 0,
                } as appconf.AppConfig)}
              className="glass-input w-full rounded-md border border-brand-300 px-3 py-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
            />
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("settings.autoBackup.localDbRetentionHint")}
            </p>
          </div>
        </div>
      </div>

      <div className="space-y-4 border-t border-brand-200 pt-4 dark:border-brand-700">
        <div>
          <div className="block text-sm font-semibold text-brand-700 dark:text-brand-300">
            {t("settings.autoBackup.cloudBackupSection")}
          </div>
        </div>

        <SettingSwitchRow
          id="auto_upload_db_to_cloud"
          label={t("settings.autoBackup.autoUploadDb")}
          hint={t("settings.autoBackup.autoUploadDbHint")}
          checked={formData.auto_upload_db_to_cloud || false}
          onCheckedChange={checked =>
            onChange({
              ...formData,
              auto_upload_db_to_cloud: checked,
            } as appconf.AppConfig)}
          disabled={!cloudServiceEnabled}
        />

        <SettingSwitchRow
          id="auto_upload_game_save_to_cloud"
          label={t("settings.autoBackup.autoUploadSave")}
          hint={t("settings.autoBackup.autoUploadSaveHint")}
          checked={formData.auto_upload_game_save_to_cloud || false}
          onCheckedChange={checked =>
            onChange({
              ...formData,
              auto_upload_game_save_to_cloud: checked,
            } as appconf.AppConfig)}
          disabled={!cloudServiceEnabled}
        />

        <div className="space-y-2 border-t border-brand-200 pt-4 dark:border-brand-700">
          <label
            className={`block text-sm font-medium ${
              cloudServiceEnabled
                ? "text-brand-700 dark:text-brand-300"
                : "text-brand-400 dark:text-brand-500"
            }`}
          >
            {t("settings.cloudBackup.retentionLabel")}
          </label>
          <input
            type="number"
            value={formData.cloud_backup_retention || 5}
            onChange={e =>
              onChange({
                ...formData,
                cloud_backup_retention:
                  Number.parseInt(e.target.value, 10) || 20,
              } as appconf.AppConfig)}
            min={1}
            max={100}
            disabled={!cloudServiceEnabled}
            className="glass-input w-32 rounded-md border border-brand-300 px-3 py-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 disabled:cursor-not-allowed disabled:opacity-60 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
          />
          <p className="text-xs text-brand-500 dark:text-brand-400">
            {t("settings.cloudBackup.retentionHint")}
          </p>
          {!cloudServiceEnabled && (
            <p className="text-xs text-warning-600 dark:text-warning-400">
              {t("settings.autoBackup.cloudServiceRequiredHint")}
            </p>
          )}
        </div>
      </div>

      <div className="space-y-4 border-t border-brand-200 pt-4 dark:border-brand-700">
        <div>
          <div className="block text-sm font-semibold text-brand-700 dark:text-brand-300">
            {t("settings.autoBackup.cloudSyncSection")}
          </div>
        </div>

        <SettingSwitchRow
          id="cloud_sync_enabled"
          label={t("settings.cloudBackup.syncEnableLabel")}
          hint={t("settings.cloudBackup.syncEnableHint")}
          checked={formData.cloud_sync_enabled || false}
          onCheckedChange={checked =>
            onChange({
              ...formData,
              cloud_sync_enabled: checked,
            } as appconf.AppConfig)}
          disabled={!cloudServiceEnabled}
        />

        <div className="space-y-2 border-t border-brand-200 pt-4 dark:border-brand-700">
          <label
            className={`block text-sm font-medium ${
              cloudServiceEnabled
                ? "text-brand-700 dark:text-brand-300"
                : "text-brand-400 dark:text-brand-500"
            }`}
          >
            {t("settings.cloudBackup.syncIntervalLabel")}
          </label>
          <div className="flex items-center gap-3">
            <input
              type="number"
              min={15}
              step={5}
              value={syncIntervalSeconds}
              disabled={!cloudServiceEnabled}
              onChange={(e) => {
                const raw = Number.parseInt(e.target.value, 10);
                const nextValue = Number.isNaN(raw) ? 60 : Math.max(15, raw);
                onChange({
                  ...formData,
                  cloud_sync_interval_sec: nextValue,
                } as appconf.AppConfig);
              }}
              className="glass-input w-32 rounded-md border border-brand-300 px-3 py-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 disabled:cursor-not-allowed disabled:opacity-60 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
            />
            <span className="text-sm text-brand-500 dark:text-brand-400">
              {t("settings.cloudBackup.syncIntervalUnit")}
            </span>
          </div>
          <p className="text-xs text-brand-500 dark:text-brand-400">
            {t("settings.cloudBackup.syncIntervalHint")}
          </p>
          {!cloudServiceEnabled && (
            <p className="text-xs text-warning-600 dark:text-warning-400">
              {t("settings.autoBackup.cloudServiceRequiredHint")}
            </p>
          )}
          {cloudServiceEnabled && !cloudConfigured && (
            <p className="text-xs text-warning-600 dark:text-warning-400">
              {t("settings.autoBackup.cloudSyncConfigRequiredHint")}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
