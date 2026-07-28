import type { appconf, vo } from "../../../src/bindings/models";
import { useCallback, useEffect, useRef, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  CancelUmbraAuth,
  GetUmbraUserProfile,
  LogoutUmbra,
  SetupCloudBackup,
  StartOneDriveAuth,
  StartUmbraAuth,
  TestOneDriveConnection,
  TestS3Connection,
  TestUmbraConnection,
  TestWebDAVConnection,
} from "../../../bindings/lunabox/internal/service/backupservice";
import { GetAppConfig } from "../../../bindings/lunabox/internal/service/configservice";
import { formatFileSize } from "../../utils/size";
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
  const [testingUmbra, setTestingUmbra] = useState(false);
  const [testingWebDAV, setTestingWebDAV] = useState(false);
  const [authorizingOneDrive, setAuthorizingOneDrive] = useState(false);
  const [authorizingUmbra, setAuthorizingUmbra] = useState(false);
  const [cancellingUmbra, setCancellingUmbra] = useState(false);
  const [revokingUmbra, setRevokingUmbra] = useState(false);
  const [umbraProfile, setUmbraProfile] = useState<vo.UmbraUserProfile | null>(
    null,
  );
  const [loadingUmbraProfile, setLoadingUmbraProfile] = useState(false);
  const [umbraProfileError, setUmbraProfileError] = useState("");
  const [showPasswordModal, setShowPasswordModal] = useState(false);
  const oneDriveClientID = formData.onedrive_client_id?.trim() || "";
  const hasOneDriveClientID = oneDriveClientID.length > 0;
  const requiresBackupPassword
    = formData.cloud_backup_provider === "s3"
      || formData.cloud_backup_provider === "onedrive"
      || formData.cloud_backup_provider === "webdav";
  const umbraStoragePercent
    = umbraProfile && umbraProfile.quota_bytes > 0
      ? Math.min(
          100,
          (umbraProfile.used_bytes / umbraProfile.quota_bytes) * 100,
        )
      : 0;
  const mountedRef = useRef(true);
  const formDataRef = useRef(formData);
  const umbraAuthActiveRef = useRef(false);
  const umbraAuthCancelRequestedRef = useRef(false);
  const umbraProfileRequestRef = useRef(0);

  formDataRef.current = formData;

  const loadUmbraProfile = useCallback(async (config: appconf.AppConfig) => {
    const requestID = ++umbraProfileRequestRef.current;
    setLoadingUmbraProfile(true);
    setUmbraProfile(null);
    setUmbraProfileError("");
    try {
      const profile = await GetUmbraUserProfile(config);
      if (!mountedRef.current || requestID !== umbraProfileRequestRef.current)
        return;
      setUmbraProfile(profile);
    }
    catch (err: unknown) {
      if (!mountedRef.current || requestID !== umbraProfileRequestRef.current)
        return;
      setUmbraProfileError(err instanceof Error ? err.message : String(err));
    }
    finally {
      if (mountedRef.current && requestID === umbraProfileRequestRef.current)
        setLoadingUmbraProfile(false);
    }
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      if (umbraAuthActiveRef.current) {
        umbraAuthCancelRequestedRef.current = true;
        void CancelUmbraAuth();
      }
    };
  }, []);

  useEffect(() => {
    if (
      formData.cloud_backup_provider !== "umbra"
      || !formData.umbra_authenticated
      || !formData.umbra_base_url?.trim()
    ) {
      umbraProfileRequestRef.current += 1;
      return;
    }

    void loadUmbraProfile(formDataRef.current);
  }, [
    formData.cloud_backup_provider,
    formData.network_proxy_mode,
    formData.network_proxy_url,
    formData.umbra_authenticated,
    formData.umbra_base_url,
    loadUmbraProfile,
  ]);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    if (name === "onedrive_client_id") {
      const previousClientID = formData.onedrive_client_id?.trim() || "";
      const nextClientID = value.trim();
      onChange({
        ...formData,
        [name]: value,
        ...(previousClientID !== nextClientID
          ? { onedrive_refresh_token: "" }
          : {}),
      } as appconf.AppConfig);
      return;
    }

    if (name === "umbra_base_url") {
      umbraProfileRequestRef.current += 1;
      setUmbraProfile(null);
      setUmbraProfileError("");
      onChange({
        ...formData,
        [name]: value,
        umbra_authenticated: false,
      } as appconf.AppConfig);
      return;
    }

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

  const handleTestWebDAV = async () => {
    setTestingWebDAV(true);
    try {
      await TestWebDAVConnection(formData);
      toast.success(t("settings.cloudBackup.toast.webdavTestSuccess"));
    }
    catch (err: any) {
      toast.error(
        t("settings.cloudBackup.toast.webdavTestFailed", { error: err }),
      );
    }
    finally {
      setTestingWebDAV(false);
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
    if (!hasOneDriveClientID) {
      toast.error(t("settings.cloudBackup.toast.oneDriveClientIdRequired"));
      return;
    }

    setAuthorizingOneDrive(true);
    try {
      const refreshToken = await StartOneDriveAuth(oneDriveClientID);
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

  const handleUmbraAuth = async () => {
    if (!formData.umbra_base_url?.trim()) {
      toast.error(t("settings.cloudBackup.toast.umbraConfigRequired"));
      return;
    }

    umbraAuthActiveRef.current = true;
    umbraAuthCancelRequestedRef.current = false;
    setAuthorizingUmbra(true);
    try {
      await StartUmbraAuth(formData);
      if (!mountedRef.current)
        return;
      onChange({
        ...formData,
        umbra_authenticated: true,
      } as appconf.AppConfig);
      toast.success(t("settings.cloudBackup.toast.umbraAuthSuccess"));
    }
    catch (err: any) {
      if (!mountedRef.current)
        return;
      if (umbraAuthCancelRequestedRef.current) {
        toast.success(t("settings.cloudBackup.toast.umbraAuthCancelled"));
        return;
      }
      toast.error(
        t("settings.cloudBackup.toast.umbraAuthFailed", { error: err }),
      );
    }
    finally {
      umbraAuthActiveRef.current = false;
      if (mountedRef.current) {
        setAuthorizingUmbra(false);
        setCancellingUmbra(false);
      }
    }
  };

  const handleCancelUmbraAuth = async () => {
    umbraAuthCancelRequestedRef.current = true;
    setCancellingUmbra(true);
    try {
      const cancelled = await CancelUmbraAuth();
      if (!cancelled) {
        umbraAuthCancelRequestedRef.current = false;
        setCancellingUmbra(false);
      }
    }
    catch (err: any) {
      umbraAuthCancelRequestedRef.current = false;
      setCancellingUmbra(false);
      toast.error(
        t("settings.cloudBackup.toast.umbraAuthCancelFailed", { error: err }),
      );
    }
  };

  const handleUmbraLogout = async () => {
    setRevokingUmbra(true);
    try {
      await LogoutUmbra(formData);
      onChange({
        ...formData,
        umbra_authenticated: false,
      } as appconf.AppConfig);
      umbraProfileRequestRef.current += 1;
      setUmbraProfile(null);
      setUmbraProfileError("");
      toast.success(t("settings.cloudBackup.toast.umbraLogoutSuccess"));
    }
    catch (err: any) {
      toast.error(
        t("settings.cloudBackup.toast.umbraLogoutFailed", { error: err }),
      );
    }
    finally {
      setRevokingUmbra(false);
    }
  };

  const handleTestUmbra = async () => {
    setTestingUmbra(true);
    try {
      await TestUmbraConnection(formData);
      toast.success(t("settings.cloudBackup.toast.umbraTestSuccess"));
    }
    catch (err: any) {
      toast.error(
        t("settings.cloudBackup.toast.umbraTestFailed", { error: err }),
      );
    }
    finally {
      setTestingUmbra(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <div className="flex items-center justify-between gap-4">
          <div className="flex-1 space-y-2">
            <label
              htmlFor="cloud_backup_enabled"
              className="block cursor-pointer text-sm font-medium text-brand-700 dark:text-brand-300"
            >
              {t("settings.cloudBackup.serviceEnableLabel")}
            </label>
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("settings.cloudBackup.serviceEnableHint")}
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
            { value: "webdav", label: "WebDAV" },
            { value: "umbra", label: t("settings.cloudBackup.providerUmbra") },
          ]}
        />
      </div>

      {requiresBackupPassword ? (
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
      ) : null}

      {formData.cloud_backup_provider === "s3" && (
        <div className="glass-card space-y-4 rounded-lg bg-brand-100 p-4 dark:bg-brand-800">
          <div className="block text-sm font-semibold text-brand-700 dark:text-brand-300">
            {t("settings.cloudBackup.s3Section")}
          </div>
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

      {formData.cloud_backup_provider === "webdav" && (
        <div className="glass-card space-y-4 rounded-lg bg-brand-100 p-4 dark:bg-brand-800">
          <div className="block text-sm font-semibold text-brand-700 dark:text-brand-300">
            {t("settings.cloudBackup.webdavSection")}
          </div>
          <div className="space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              URL
            </label>
            <input
              type="text"
              name="webdav_url"
              value={formData.webdav_url || ""}
              onChange={handleChange}
              placeholder="https://dav.example.com/remote.php/dav/files/user"
              className="glass-input w-full rounded-md border border-brand-300 px-3 py-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
            />
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("settings.cloudBackup.webdavUrlHint")}
            </p>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                {t("settings.cloudBackup.webdavUsername")}
              </label>
              <input
                type="text"
                name="webdav_username"
                value={formData.webdav_username || ""}
                onChange={handleChange}
                className="glass-input w-full rounded-md border border-brand-300 px-3 py-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
              />
            </div>
            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                {t("settings.cloudBackup.webdavPassword")}
              </label>
              <input
                type="password"
                name="webdav_password"
                value={formData.webdav_password || ""}
                onChange={handleChange}
                className="glass-input w-full rounded-md border border-brand-300 px-3 py-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
              />
            </div>
          </div>
          <div className="flex justify-end">
            <button
              type="button"
              onClick={handleTestWebDAV}
              disabled={testingWebDAV}
              className="glass-btn-neutral rounded-md bg-brand-100 px-3 py-1.5 text-sm text-brand-700 hover:bg-brand-200 disabled:opacity-50 dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600"
            >
              {testingWebDAV
                ? t("settings.cloudBackup.testing")
                : t("settings.cloudBackup.testConnection")}
            </button>
          </div>
        </div>
      )}

      {formData.cloud_backup_provider === "onedrive" && (
        <div className="glass-card space-y-4 rounded-lg bg-brand-100 p-4 dark:bg-brand-800">
          <div className="block text-sm font-semibold text-brand-700 dark:text-brand-300">
            {t("settings.cloudBackup.oneDriveSection")}
          </div>

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
            </label>
            <input
              type="text"
              name="onedrive_client_id"
              value={formData.onedrive_client_id || ""}
              onChange={handleChange}
              placeholder="your-app-client-id"
              className="glass-input w-full rounded-md border border-brand-300 px-3 py-2 font-mono text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
            />
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("settings.cloudBackup.clientIdHint")}
              {" "}
              <a
                href="https://box.lunarain.site/configuration/onedrive-cloud-backup.html"
                target="_blank"
                rel="noopener noreferrer"
                className="underline hover:text-brand-600 dark:hover:text-brand-300"
              >
                {t("settings.cloudBackup.clientIdHintLink")}
              </a>
              {t("settings.cloudBackup.clientIdHint2")}
            </p>
            <p className="text-xs text-warning-600 dark:text-warning-400">
              {t("settings.cloudBackup.clientIdChangeHint")}
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
                  disabled={authorizingOneDrive || !hasOneDriveClientID}
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

      {formData.cloud_backup_provider === "umbra" && (
        <div className="glass-card space-y-4 rounded-lg bg-brand-100 p-4 dark:bg-brand-800">
          <div className="block text-sm font-semibold text-brand-700 dark:text-brand-300">
            {t("settings.cloudBackup.umbraSection")}
          </div>

          <div className="space-y-2">
            <div className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              {t("settings.cloudBackup.authStatusLabel")}
            </div>
            {formData.umbra_authenticated ? (
              <div className="space-y-3">
                {(loadingUmbraProfile || umbraProfile) && (
                  <div className="glass-panel overflow-hidden rounded-lg border border-brand-200 dark:border-brand-700">
                    {loadingUmbraProfile && !umbraProfile ? (
                      <div className="flex items-center gap-3 px-4 py-5 text-sm text-brand-500 dark:text-brand-400">
                        <span className="i-mdi-loading animate-spin text-xl" />
                        {t("settings.cloudBackup.umbraProfileLoading")}
                      </div>
                    ) : umbraProfile ? (
                      <>
                        <div className="flex items-center gap-3 px-4 py-3">
                          <div className="flex h-10 w-10 flex-none items-center justify-center rounded-full bg-neutral-600 text-lg font-semibold text-white shadow-sm">
                            {umbraProfile.username
                              .trim()
                              .charAt(0)
                              .toUpperCase() || (
                              <span className="i-mdi-account" />
                            )}
                          </div>
                          <div className="min-w-0 flex-1">
                            <div className="truncate text-sm font-semibold text-brand-800 dark:text-brand-200">
                              {umbraProfile.username}
                            </div>
                          </div>
                          <span className="flex flex-none items-center gap-1 rounded-full bg-success-100 px-2 py-1 text-xs font-medium text-success-700 dark:bg-success-900 dark:text-success-300">
                            <span className="i-mdi-check-decagram" />
                            {t("settings.cloudBackup.authorized")}
                          </span>
                        </div>

                        <div className="space-y-2 border-t border-brand-200 px-4 py-3 dark:border-brand-700">
                          <div className="flex items-center justify-between gap-3 text-xs">
                            <span className="font-medium text-brand-700 dark:text-brand-300">
                              {t("settings.cloudBackup.umbraStorage")}
                            </span>
                            <span className="text-brand-500 dark:text-brand-400">
                              {formatFileSize(umbraProfile.used_bytes)}
                              {" / "}
                              {formatFileSize(umbraProfile.quota_bytes)}
                            </span>
                          </div>
                          <div
                            role="progressbar"
                            aria-label={t("settings.cloudBackup.umbraStorage")}
                            aria-valuemin={0}
                            aria-valuemax={100}
                            aria-valuenow={Math.round(umbraStoragePercent)}
                            className="h-1.5 overflow-hidden rounded-full bg-brand-200 dark:bg-brand-700"
                          >
                            <div
                              className="h-full rounded-full bg-neutral-600 transition-[width] duration-300 dark:bg-neutral-400"
                              style={{ width: `${umbraStoragePercent}%` }}
                            />
                          </div>
                          <p className="text-right text-xs text-brand-500 dark:text-brand-400">
                            {t("settings.cloudBackup.umbraStorageAvailable", {
                              size: formatFileSize(
                                umbraProfile.available_bytes,
                              ),
                            })}
                          </p>
                        </div>
                      </>
                    ) : null}
                  </div>
                )}

                {umbraProfileError && (
                  <div className="flex items-start justify-between gap-3 rounded-md border border-error-200 bg-error-50 px-3 py-2 text-xs text-error-700 dark:border-error-800 dark:bg-error-900 dark:text-error-300 data-glass:bg-error-900/20">
                    <span className="flex items-start gap-2">
                      <span className="i-mdi-alert-circle-outline mt-0.5 flex-none" />
                      <span>
                        {t("settings.cloudBackup.umbraProfileLoadFailed", {
                          error: umbraProfileError,
                        })}
                      </span>
                    </span>
                    <button
                      type="button"
                      onClick={() => void loadUmbraProfile(formData)}
                      disabled={loadingUmbraProfile}
                      className="flex-none rounded px-2 py-1 font-medium hover:bg-error-100 disabled:opacity-50 dark:hover:bg-error-800"
                    >
                      {t("settings.cloudBackup.retry")}
                    </button>
                  </div>
                )}

                <div className="flex flex-wrap justify-end gap-2">
                  <button
                    type="button"
                    onClick={() => void loadUmbraProfile(formData)}
                    disabled={loadingUmbraProfile}
                    className="glass-btn-neutral flex items-center gap-1.5 rounded-md bg-brand-100 px-3 py-1.5 text-sm text-brand-700 hover:bg-brand-200 disabled:opacity-50 dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600"
                  >
                    <span
                      className={
                        loadingUmbraProfile
                          ? "i-mdi-loading animate-spin"
                          : "i-mdi-refresh"
                      }
                    />
                    {t("settings.cloudBackup.refresh")}
                  </button>
                  <button
                    type="button"
                    onClick={handleTestUmbra}
                    disabled={testingUmbra}
                    className="glass-btn-neutral rounded-md bg-brand-100 px-3 py-1.5 text-sm text-brand-700 hover:bg-brand-200 disabled:opacity-50 dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600"
                  >
                    {testingUmbra
                      ? t("settings.cloudBackup.testing")
                      : t("settings.cloudBackup.testConnection")}
                  </button>
                  <button
                    type="button"
                    onClick={handleUmbraLogout}
                    disabled={revokingUmbra}
                    className="rounded-md px-3 py-1.5 text-sm text-error-600 hover:bg-error-100 disabled:opacity-50 dark:text-error-400 dark:hover:bg-error-900"
                  >
                    {revokingUmbra
                      ? t("settings.cloudBackup.waitingAuth")
                      : t("settings.cloudBackup.revokeAuth")}
                  </button>
                </div>
              </div>
            ) : (
              <div className="space-y-3">
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={handleUmbraAuth}
                    disabled={authorizingUmbra}
                    className="glass-btn-neutral flex items-center gap-2 rounded-md bg-neutral-600 px-3 py-1.5 text-sm text-white hover:bg-neutral-700 disabled:opacity-50"
                  >
                    <span
                      className={
                        authorizingUmbra
                          ? "i-mdi-loading animate-spin"
                          : "i-mdi-shield-key-outline"
                      }
                    />
                    {authorizingUmbra
                      ? t("settings.cloudBackup.waitingAuth")
                      : t("settings.cloudBackup.authUmbraBtn")}
                  </button>
                  {authorizingUmbra && (
                    <button
                      type="button"
                      onClick={handleCancelUmbraAuth}
                      disabled={cancellingUmbra}
                      className="rounded-md px-3 py-1.5 text-sm text-error-600 hover:bg-error-100 disabled:opacity-50 dark:text-error-400 dark:hover:bg-error-900"
                    >
                      {t("settings.cloudBackup.cancelAuth")}
                    </button>
                  )}
                </div>
                {authorizingUmbra && (
                  <p className="text-xs text-brand-500 dark:text-brand-400">
                    {t("settings.cloudBackup.authWaitHint")}
                  </p>
                )}
              </div>
            )}
          </div>

          <details className="glass-panel group overflow-hidden rounded-lg border border-brand-200 dark:border-brand-700">
            <summary className="flex cursor-pointer list-none items-center justify-between gap-3 px-3 py-2.5 text-sm font-medium text-brand-700 outline-none transition-colors hover:bg-brand-200/60 focus-visible:ring-2 focus-visible:ring-neutral-500 dark:text-brand-300 dark:hover:bg-brand-700/60">
              <span className="flex items-center gap-2">
                <span className="i-mdi-tune-variant" />
                {t("settings.cloudBackup.advancedOptions")}
              </span>
              <span className="i-mdi-chevron-down transition-transform duration-200 group-open:rotate-180" />
            </summary>
            <div className="space-y-2 border-t border-brand-200 px-3 py-3 dark:border-brand-700">
              <label
                htmlFor="umbra_base_url"
                className="block text-sm font-medium text-brand-700 dark:text-brand-300"
              >
                {t("settings.cloudBackup.umbraBaseURL")}
              </label>
              <input
                id="umbra_base_url"
                type="url"
                name="umbra_base_url"
                value={formData.umbra_base_url || ""}
                onChange={handleChange}
                placeholder="https://umbra.example.com"
                className="glass-input w-full rounded-md border border-brand-300 px-3 py-2 font-mono text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
              />
              <p className="text-xs text-brand-500 dark:text-brand-400">
                {t("settings.cloudBackup.umbraBaseURLHint")}
              </p>
            </div>
          </details>
        </div>
      )}

      {requiresBackupPassword ? (
        <PasswordInputModal
          isOpen={showPasswordModal}
          onClose={() => setShowPasswordModal(false)}
          onConfirm={handleSetupBackupPassword}
        />
      ) : null}
    </div>
  );
}
