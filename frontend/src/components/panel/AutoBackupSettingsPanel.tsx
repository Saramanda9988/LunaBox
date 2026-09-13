import type { appconf } from "../../../src/bindings/models";
import { useTranslation } from "react-i18next";
import {
  getSyncIntervalSeconds,
  isCloudProviderConfigured,
} from "../../utils/cloudSync";
import { BetterNumberInput } from "../ui/better/BetterNumberInput";
import { BetterSelect } from "../ui/better/BetterSelect";
import { BetterTimeWheelInput } from "../ui/better/BetterTimeWheelInput";
import { SettingSwitchRow } from "../ui/SettingSwitchRow";

interface AutoBackupSettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

export function AutoBackupSettingsPanel({
  formData,
  onChange,
}: AutoBackupSettingsProps) {
  const { t } = useTranslation();
  const cloudServiceEnabled = formData.cloud_backup_enabled || false;
  const cloudConfigured = isCloudProviderConfigured(formData);
  const syncIntervalSeconds = getSyncIntervalSeconds(formData);
  const scheduledDBBackupEnabled
    = formData.scheduled_db_backup_enabled ?? false;
  const scheduledDBBackupMode = formData.scheduled_db_backup_mode || "interval";
  const scheduledDBBackupModeOptions = [
    {
      value: "interval",
      label: t("settings.autoBackup.scheduleModeInterval"),
    },
    {
      value: "daily",
      label: t("settings.autoBackup.scheduleModeDaily"),
    },
  ];

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
          id="scheduled_db_backup_enabled"
          label={t("settings.autoBackup.scheduledDbBackup")}
          hint={t("settings.autoBackup.scheduledDbBackupHint")}
          checked={scheduledDBBackupEnabled}
          onCheckedChange={checked =>
            onChange({
              ...formData,
              scheduled_db_backup_enabled: checked,
            } as appconf.AppConfig)}
        />

        {scheduledDBBackupEnabled ? (
          <div className="space-y-4 border-l-2 border-brand-200 pl-4 dark:border-brand-700">
            <div className="flex items-center justify-between gap-4">
              <div className="flex-1 space-y-2">
                <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                  {t("settings.autoBackup.scheduleMode")}
                </label>
                <p className="text-xs text-brand-500 dark:text-brand-400">
                  {t("settings.autoBackup.scheduleModeHint")}
                </p>
              </div>
              <BetterSelect
                value={scheduledDBBackupMode}
                options={scheduledDBBackupModeOptions}
                onChange={value =>
                  onChange({
                    ...formData,
                    scheduled_db_backup_mode: value,
                  } as appconf.AppConfig)}
                className="w-44 shrink-0"
              />
            </div>

            {scheduledDBBackupMode === "daily" ? (
              <div className="flex items-center justify-between gap-4">
                <div className="flex-1 space-y-2">
                  <label
                    htmlFor="scheduled_db_backup_time"
                    className="block text-sm font-medium text-brand-700 dark:text-brand-300"
                  >
                    {t("settings.autoBackup.scheduleDailyTime")}
                  </label>
                  <p className="text-xs text-brand-500 dark:text-brand-400">
                    {t("settings.autoBackup.scheduleDailyTimeHint")}
                  </p>
                </div>
                <BetterTimeWheelInput
                  id="scheduled_db_backup_time"
                  value={formData.scheduled_db_backup_time || "03:00"}
                  actionLabel={t("settings.autoBackup.scheduleDailyTime")}
                  onChange={value =>
                    onChange({
                      ...formData,
                      scheduled_db_backup_time: value,
                    } as appconf.AppConfig)}
                  className="w-36 shrink-0"
                />
              </div>
            ) : (
              <div className="flex items-center justify-between gap-4">
                <div className="flex-1 space-y-2">
                  <label
                    htmlFor="scheduled_db_backup_interval_minutes"
                    className="block text-sm font-medium text-brand-700 dark:text-brand-300"
                  >
                    {t("settings.autoBackup.scheduleInterval")}
                  </label>
                  <p className="text-xs text-brand-500 dark:text-brand-400">
                    {t("settings.autoBackup.scheduleIntervalHint")}
                  </p>
                </div>
                <BetterNumberInput
                  id="scheduled_db_backup_interval_minutes"
                  min={15}
                  max={10080}
                  step={15}
                  value={formData.scheduled_db_backup_interval_minutes || 60}
                  onValueChange={value =>
                    onChange({
                      ...formData,
                      scheduled_db_backup_interval_minutes: value,
                    } as appconf.AppConfig)}
                  unit={t("settings.autoBackup.minutesUnit")}
                  size="sm"
                  className="shrink-0"
                />
              </div>
            )}
          </div>
        ) : null}

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

        <div className="space-y-4 border-brand-200 dark:border-brand-700">
          <div className="flex items-center justify-between gap-4">
            <div className="flex-1 space-y-2">
              <label
                htmlFor="local_backup_retention"
                className="block text-sm font-medium text-brand-700 dark:text-brand-300"
              >
                {t("settings.autoBackup.localGameRetention")}
              </label>
              <p className="text-xs text-brand-500 dark:text-brand-400">
                {t("settings.autoBackup.localGameRetentionHint")}
              </p>
            </div>
            <BetterNumberInput
              id="local_backup_retention"
              min={0}
              value={formData.local_backup_retention || 10}
              onValueChange={value =>
                onChange({
                  ...formData,
                  local_backup_retention: value,
                } as appconf.AppConfig)}
              size="sm"
              className="shrink-0"
            />
          </div>

          <div className="flex items-center justify-between gap-4">
            <div className="flex-1 space-y-2">
              <label
                htmlFor="local_db_backup_retention"
                className="block text-sm font-medium text-brand-700 dark:text-brand-300"
              >
                {t("settings.autoBackup.localDbRetention")}
              </label>
              <p className="text-xs text-brand-500 dark:text-brand-400">
                {t("settings.autoBackup.localDbRetentionHint")}
              </p>
            </div>
            <BetterNumberInput
              id="local_db_backup_retention"
              min={1}
              value={Math.max(1, formData.local_db_backup_retention)}
              onValueChange={value =>
                onChange({
                  ...formData,
                  local_db_backup_retention: Math.max(1, value),
                } as appconf.AppConfig)}
              size="sm"
              className="shrink-0"
            />
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

        <SettingSwitchRow
          id="auto_restore_cloud_save_before_launch"
          label={t("settings.autoBackup.restoreCloudSaveBeforeLaunch")}
          hint={t("settings.autoBackup.restoreCloudSaveBeforeLaunchHint")}
          checked={formData.auto_restore_cloud_save_before_launch ?? false}
          onCheckedChange={checked =>
            onChange({
              ...formData,
              auto_restore_cloud_save_before_launch: checked,
            } as appconf.AppConfig)}
          disabled={!cloudServiceEnabled || !cloudConfigured}
        />

        <div className="space-y-2 border-brand-200 dark:border-brand-700">
          <div className="flex items-center justify-between gap-4">
            <div className="flex-1 space-y-2">
              <label
                htmlFor="cloud_backup_retention"
                className={`block text-sm font-medium ${
                  cloudServiceEnabled
                    ? "text-brand-700 dark:text-brand-300"
                    : "text-brand-400 dark:text-brand-500"
                }`}
              >
                {t("settings.autoBackup.cloudGameRetention")}
              </label>
              <p className="text-xs text-brand-500 dark:text-brand-400">
                {t("settings.autoBackup.cloudGameRetentionHint")}
              </p>
              {!cloudServiceEnabled && (
                <p className="text-xs text-warning-600 dark:text-warning-400">
                  {t("settings.autoBackup.cloudServiceRequiredHint")}
                </p>
              )}
            </div>
            <BetterNumberInput
              id="cloud_backup_retention"
              value={formData.cloud_backup_retention || 5}
              onValueChange={value =>
                onChange({
                  ...formData,
                  cloud_backup_retention: value,
                } as appconf.AppConfig)}
              min={1}
              max={100}
              disabled={!cloudServiceEnabled}
              size="sm"
              className="shrink-0"
            />
          </div>

          <div className="flex items-center justify-between gap-4">
            <div className="flex-1 space-y-2">
              <label
                htmlFor="cloud_db_backup_retention"
                className={`block text-sm font-medium ${
                  cloudServiceEnabled
                    ? "text-brand-700 dark:text-brand-300"
                    : "text-brand-400 dark:text-brand-500"
                }`}
              >
                {t("settings.autoBackup.cloudDbRetention")}
              </label>
              <p className="text-xs text-brand-500 dark:text-brand-400">
                {t("settings.autoBackup.cloudDbRetentionHint")}
              </p>
            </div>
            <BetterNumberInput
              id="cloud_db_backup_retention"
              value={formData.cloud_db_backup_retention || 5}
              onValueChange={value =>
                onChange({
                  ...formData,
                  cloud_db_backup_retention: value,
                } as appconf.AppConfig)}
              min={1}
              max={100}
              disabled={!cloudServiceEnabled}
              size="sm"
              className="shrink-0"
            />
          </div>
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
              auto_cloud_sync_enabled: checked
                ? formData.auto_cloud_sync_enabled || false
                : false,
            } as appconf.AppConfig)}
          disabled={!cloudServiceEnabled}
        />

        <SettingSwitchRow
          id="auto_cloud_sync_enabled"
          label={t("settings.cloudBackup.autoSyncEnableLabel")}
          hint={t("settings.cloudBackup.autoSyncEnableHint")}
          checked={formData.auto_cloud_sync_enabled || false}
          onCheckedChange={checked =>
            onChange({
              ...formData,
              auto_cloud_sync_enabled: checked,
            } as appconf.AppConfig)}
          disabled={!cloudServiceEnabled || !formData.cloud_sync_enabled}
        />

        <div className="space-y-2 border-brand-200 dark:border-brand-700">
          <div className="flex items-center justify-between gap-4">
            <div className="flex-1 space-y-2">
              <label
                htmlFor="cloud_sync_interval_sec"
                className={`block text-sm font-medium ${
                  cloudServiceEnabled
                  && formData.cloud_sync_enabled
                  && formData.auto_cloud_sync_enabled
                    ? "text-brand-700 dark:text-brand-300"
                    : "text-brand-400 dark:text-brand-500"
                }`}
              >
                {t("settings.cloudBackup.syncIntervalLabel")}
              </label>
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
              {cloudServiceEnabled
                && formData.cloud_sync_enabled
                && !formData.auto_cloud_sync_enabled && (
                <p className="text-xs text-brand-500 dark:text-brand-400">
                  {t("settings.autoBackup.cloudSyncManualRecommendedHint")}
                </p>
              )}
            </div>
            <BetterNumberInput
              id="cloud_sync_interval_sec"
              min={15}
              step={5}
              value={syncIntervalSeconds}
              disabled={
                !cloudServiceEnabled
                || !formData.cloud_sync_enabled
                || !formData.auto_cloud_sync_enabled
              }
              onValueChange={(value) => {
                onChange({
                  ...formData,
                  cloud_sync_interval_sec: value,
                } as appconf.AppConfig);
              }}
              unit={t("settings.cloudBackup.syncIntervalUnit")}
              size="sm"
              className="shrink-0"
            />
          </div>
        </div>
      </div>
    </div>
  );
}
