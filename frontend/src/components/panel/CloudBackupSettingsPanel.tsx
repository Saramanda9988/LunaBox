import type { appconf } from "../../../wailsjs/go/models";
import { useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  SetupCloudBackup,
  StartOneDriveAuth,
  TestOneDriveConnection,
  TestS3Connection,
} from "../../../wailsjs/go/service/BackupService";
import { GetAppConfig } from "../../../wailsjs/go/service/ConfigService";
import { getSyncIntervalSeconds } from "../../utils/cloudSync";
import { PasswordInputModal } from "../modal/PasswordInputModal";
import { BetterSelect } from "../ui/better/BetterSelect";
import { BetterSwitch } from "../ui/better/BetterSwitch";

interface CloudBackupSettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

export function CloudBackupSettingsPanel({
  formData,
  onChange,
}: CloudBackupSettingsProps) {
  const { t } = useTranslation();
  const [testingS3, setTestingS3] = useState(false);
  const [testingOneDrive, setTestingOneDrive] = useState(false);
  const [authorizingOneDrive, setAuthorizingOneDrive] = useState(false);
  const [showPasswordModal, setShowPasswordModal] = useState(false);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    onChange({ ...formData, [name]: value } as appconf.AppConfig);
  };

  const handleSetupBackupPassword = async (
    password: string,
    confirmPassword: string,
  ) => {
    if (password.length < 6) {
      toast.error(t("settings.cloudBackup.toast.passwordTooShort"));
      return;
    }

    if (password !== confirmPassword) {
      toast.error(t("settings.cloudBackup.toast.passwordMismatch"));
      return;
    }

    try {
      const userID = await SetupCloudBackup(password);
      toast.success(
        t("settings.cloudBackup.toast.passwordSetSuccess", {
          id: userID.substring(0, 8),
        }),
      );
      const updatedConfig = await GetAppConfig();
      onChange(updatedConfig);
    }
    catch (err: any) {
      toast.error(t("settings.cloudBackup.toast.setupFailed", { error: err }));
    }
  };

  const handleTestS3 = async () => {
    setTestingS3(true);
    try {
      await TestS3Connection(formData);
      toast.success(t("settings.cloudBackup.toast.s3TestSuccess"));
    }
    catch (err: any) {
      toast.error(t("settings.cloudBackup.toast.s3TestFailed", { error: err }));
    }
    finally {
      setTestingS3(false);
    }
  };

  const handleTestOneDrive = async () => {
    setTestingOneDrive(true);
    try {
      await TestOneDriveConnection(formData);
      toast.success(t("settings.cloudBackup.toast.oneDriveTestSuccess"));
    }
    catch (err: any) {
      toast.error(
        t("settings.cloudBackup.toast.oneDriveTestFailed", { error: err }),
      );
    }
    finally {
      setTestingOneDrive(false);
    }
  };

  const handleOneDriveAuth = async () => {
    setAuthorizingOneDrive(true);
    try {
      const refreshToken = await StartOneDriveAuth();
      onChange({
        ...formData,
        onedrive_refresh_token: refreshToken,
      } as appconf.AppConfig);
      toast.success(t("settings.cloudBackup.toast.oneDriveAuthSuccess"));
    }
    catch (err: any) {
      toast.error(
        t("settings.cloudBackup.toast.oneDriveAuthFailed", { error: err }),
      );
    }
    finally {
      setAuthorizingOneDrive(false);
    }
  };
  const syncIntervalSeconds = getSyncIntervalSeconds(formData);

  return (
    <div className="space-y-4">
      <div className="glass-card flex items-center justify-between rounded-lg bg-brand-50 p-4 dark:bg-brand-800/50">
        <div className="flex flex-col">
          <label
            htmlFor="cloud_backup_enabled"
            className="cursor-pointer text-sm font-medium text-brand-700 dark:text-brand-300"
          >
            {t("settings.cloudBackup.enableLabel")}
          </label>
          <p className="mt-1 text-xs text-brand-500 dark:text-brand-400">
            {t("settings.cloudBackup.enableHint")}
          </p>
        </div>
        <BetterSwitch
          id="cloud_backup_enabled"
          checked={formData.cloud_backup_enabled || false}
          onCheckedChange={checked =>
            onChange({
              ...formData,
              cloud_backup_enabled: checked,
            } as appconf.AppConfig)}
        />
      </div>

      <div className="glass-card flex items-center justify-between rounded-lg bg-brand-50 p-4 dark:bg-brand-800/50">
        <div className="flex flex-col">
          <label
            htmlFor="cloud_sync_enabled"
            className="cursor-pointer text-sm font-medium text-brand-700 dark:text-brand-300"
          >
            {t("settings.cloudBackup.syncEnableLabel")}
          </label>
          <p className="mt-1 text-xs text-brand-500 dark:text-brand-400">
            {t("settings.cloudBackup.syncEnableHint")}
          </p>
        </div>
        <BetterSwitch
          id="cloud_sync_enabled"
          checked={formData.cloud_sync_enabled || false}
          onCheckedChange={checked =>
            onChange({
              ...formData,
              cloud_sync_enabled: checked,
            } as appconf.AppConfig)}
        />
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.cloudBackup.providerLabel")}
        </label>
        <BetterSelect
          value={formData.cloud_backup_provider || "s3"}
          onChange={value =>
            onChange({
              ...formData,
              cloud_backup_provider: value,
            } as appconf.AppConfig)}
          options={[
            { value: "s3", label: t("settings.cloudBackup.providerS3") },
            { value: "onedrive", label: "OneDrive" },
          ]}
        />
      </div>

      <div className="space-y-2">
        <div className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.cloudBackup.backupPasswordLabel")}
        </div>
        {formData.backup_user_id ? (
          <div className="space-y-2">
            <div className="glass-panel rounded-md border border-brand-300 px-3 py-2 text-brand-600 dark:border-brand-600 dark:text-brand-300">
              ********
            </div>
            <p className="text-xs text-brand-500 dark:text-brand-400">
              <span className="text-success-600 dark:text-success-400">
                {t("settings.cloudBackup.passwordSet")}
                {" "}
                {formData.backup_user_id?.substring(0, 8)}
                ...
              </span>
            </p>
          </div>
        ) : (
          <div className="space-y-2">
            <button
              type="button"
              onClick={() => setShowPasswordModal(true)}
              className="glass-btn-neutral flex w-full items-center justify-center gap-2 rounded-md bg-brand-600 px-4 py-2 text-white transition-colors hover:bg-brand-700"
            >
              <span className="i-mdi-lock-plus text-lg" />
              {t("settings.cloudBackup.setPasswordBtn")}
            </button>
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("settings.cloudBackup.passwordForIdHint")}
            </p>
            <p className="text-xs text-warning-600 dark:text-warning-400">
              {t("settings.cloudBackup.passwordWarning")}
            </p>
          </div>
        )}
      </div>

      {formData.cloud_backup_provider === "s3" && (
        <div className="glass-card space-y-4 rounded-lg bg-brand-100 p-4 dark:bg-brand-800">
          <h3 className="text-sm font-medium text-brand-800 dark:text-brand-200">
            {t("settings.cloudBackup.s3Section")}
          </h3>
          <div className="space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              S3 Endpoint
            </label>
            <input
              type="text"
              name="s3_endpoint"
              value={formData.s3_endpoint || ""}
              onChange={handleChange}
              placeholder="https://s3.example.com"
              className="glass-input w-full rounded-md border border-brand-300 px-3 py-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                Region
              </label>
              <input
                type="text"
                name="s3_region"
                value={formData.s3_region || ""}
                onChange={handleChange}
                placeholder="us-east-1"
                className="glass-input w-full rounded-md border border-brand-300 px-3 py-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
              />
            </div>
            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                Bucket
              </label>
              <input
                type="text"
                name="s3_bucket"
                value={formData.s3_bucket || ""}
                onChange={handleChange}
                placeholder="lunabox-backup"
                className="glass-input w-full rounded-md border border-brand-300 px-3 py-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
              />
            </div>
          </div>
          <div className="space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              Access Key
            </label>
            <input
              type="text"
              name="s3_access_key"
              value={formData.s3_access_key || ""}
              onChange={handleChange}
              className="glass-input w-full rounded-md border border-brand-300 px-3 py-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
            />
          </div>
          <div className="space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              Secret Key
            </label>
            <input
              type="password"
              name="s3_secret_key"
              value={formData.s3_secret_key || ""}
              onChange={handleChange}
              className="glass-input w-full rounded-md border border-brand-300 px-3 py-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
            />
          </div>
          <div className="flex justify-end">
            <button
              type="button"
              onClick={handleTestS3}
              disabled={testingS3}
              className="glass-btn-neutral rounded-md bg-brand-100 px-3 py-1.5 text-sm text-brand-700 hover:bg-brand-200 disabled:opacity-50 dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600"
            >
              {testingS3
                ? t("settings.cloudBackup.testing")
                : t("settings.cloudBackup.testConnection")}
            </button>
          </div>
        </div>
      )}

      {formData.cloud_backup_provider === "onedrive" && (
        <div className="space-y-4 rounded-lg bg-brand-100 p-4 dark:bg-brand-800">
          <h3 className="text-sm font-medium text-brand-800 dark:text-brand-200">
            {t("settings.cloudBackup.oneDriveSection")}
          </h3>

          <div className="rounded-md border border-brand-300 bg-brand-100 p-3 dark:border-brand-600 dark:bg-brand-700">
            <div className="flex items-start gap-2">
              <span className="i-mdi-information-outline mt-0.5 flex-shrink-0 text-lg text-warning-500 dark:text-brand-400" />
              <div className="space-y-1 text-xs text-brand-600 dark:text-brand-400">
                <p className="font-medium">
                  {t("settings.cloudBackup.oneDriveNote")}
                </p>
                <ul className="list-disc list-inside space-y-0.5 pl-2">
                  <li>{t("settings.cloudBackup.oneDriveNoteItem1")}</li>
                  <li>{t("settings.cloudBackup.oneDriveNoteItem2")}</li>
                  <li>{t("settings.cloudBackup.oneDriveNoteItem3")}</li>
                  <li>{t("settings.cloudBackup.oneDriveNoteItem4")}</li>
                  <li>{t("settings.cloudBackup.oneDriveNoteItem5")}</li>
                </ul>
              </div>
            </div>
          </div>

          <div className="space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              Client ID
              {(!formData.onedrive_client_id
                || formData.onedrive_client_id
                === "26fcab6e-41ea-49ff-8ec9-063983cae3ef") && (
                <span className="ml-2 text-xs text-brand-500 dark:text-brand-400">
                  {t("settings.cloudBackup.clientIdUsingDefault")}
                </span>
              )}
            </label>
            <input
              type="text"
              name="onedrive_client_id"
              value={formData.onedrive_client_id || ""}
              onChange={handleChange}
              placeholder="26fcab6e-41ea-49ff-8ec9-063983cae3ef (default)"
              className="glass-input w-full rounded-md border border-brand-300 px-3 py-2 font-mono text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
            />
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("settings.cloudBackup.clientIdHint")}
              {" "}
              <a
                href="https://portal.azure.com/#view/Microsoft_AAD_RegisteredApps/ApplicationsListBlade"
                target="_blank"
                rel="noopener noreferrer"
                className="underline hover:text-brand-600 dark:hover:text-brand-300"
              >
                Microsoft Entra
              </a>
            </p>
          </div>

          <div className="space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              {t("settings.cloudBackup.authStatusLabel")}
            </label>
            {formData.onedrive_refresh_token ? (
              <div className="flex items-center gap-2">
                <span className="flex items-center gap-1 text-success-600 dark:text-success-400">
                  <span className="i-mdi-check-circle text-lg" />
                  {t("settings.cloudBackup.authorized")}
                </span>
                <button
                  type="button"
                  onClick={() =>
                    onChange({
                      ...formData,
                      onedrive_refresh_token: "",
                    } as appconf.AppConfig)}
                  className="rounded px-2 py-1 text-xs text-error-600 hover:bg-error-100 dark:hover:bg-error-900"
                >
                  {t("settings.cloudBackup.revokeAuth")}
                </button>
              </div>
            ) : (
              <div className="space-y-3">
                <button
                  type="button"
                  onClick={handleOneDriveAuth}
                  disabled={authorizingOneDrive}
                  className="glass-btn-neutral flex items-center gap-2 rounded-md bg-neutral-600 px-3 py-1.5 text-sm text-white hover:bg-neutral-700 disabled:opacity-50"
                >
                  {authorizingOneDrive ? (
                    <>
                      <span className="i-mdi-loading animate-spin" />
                      {t("settings.cloudBackup.waitingAuth")}
                    </>
                  ) : (
                    <>
                      <span className="i-mdi-microsoft" />
                      {t("settings.cloudBackup.authOneDriveBtn")}
                    </>
                  )}
                </button>
                {authorizingOneDrive && (
                  <p className="text-xs text-brand-500 dark:text-brand-400">
                    {t("settings.cloudBackup.authWaitHint")}
                  </p>
                )}
              </div>
            )}
          </div>

          {formData.onedrive_refresh_token && (
            <div className="flex justify-end">
              <button
                type="button"
                onClick={handleTestOneDrive}
                disabled={testingOneDrive}
                className="glass-btn-neutral rounded-md bg-brand-100 px-3 py-1.5 text-sm text-brand-700 hover:bg-brand-200 disabled:opacity-50 dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600"
              >
                {testingOneDrive
                  ? t("settings.cloudBackup.testing")
                  : t("settings.cloudBackup.testConnection")}
              </button>
            </div>
          )}
        </div>
      )}

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.cloudBackup.retentionLabel")}
        </label>
        <input
          type="number"
          value={formData.cloud_backup_retention || 5}
          onChange={e =>
            onChange({
              ...formData,
              cloud_backup_retention: Number.parseInt(e.target.value) || 20,
            } as appconf.AppConfig)}
          min={1}
          max={100}
          className="glass-input w-32 rounded-md border border-brand-300 px-3 py-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.cloudBackup.retentionHint")}
        </p>
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.cloudBackup.syncIntervalLabel")}
        </label>
        <div className="flex items-center gap-3">
          <input
            type="number"
            min={15}
            step={5}
            value={syncIntervalSeconds}
            onChange={(e) => {
              const raw = Number.parseInt(e.target.value, 10);
              const nextValue = Number.isNaN(raw) ? 60 : Math.max(15, raw);
              onChange({
                ...formData,
                cloud_sync_interval_sec: nextValue,
              } as appconf.AppConfig);
            }}
            className="glass-input w-32 rounded-md border border-brand-300 px-3 py-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
          />
          <span className="text-sm text-brand-500 dark:text-brand-400">
            {t("settings.cloudBackup.syncIntervalUnit")}
          </span>
        </div>
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.cloudBackup.syncIntervalHint")}
        </p>
      </div>

      <PasswordInputModal
        isOpen={showPasswordModal}
        onClose={() => setShowPasswordModal(false)}
        onConfirm={handleSetupBackupPassword}
      />
    </div>
  );
}
