import type { appconf, vo } from "../../../wailsjs/go/models";
import { useEffect, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { appZoomOptions } from "../../consts/options";
import {
  disconnectBangumiAuthorization,
  fetchBangumiAuthStatus,
  fetchBangumiProfile,
  mergeBangumiAuthStatus,
  startBangumiAuthorization,
} from "../../utils/bangumiAuth";
import { ConfirmModal } from "../modal/ConfirmModal";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterSelect } from "../ui/better/BetterSelect";
import { BetterSwitch } from "../ui/better/BetterSwitch";

interface BetterSelectOption {
  value: string;
  label: string;
}

type BangumiStatusPushConfig = appconf.AppConfig & {
  bangumi_status_push_enabled?: boolean;
};

interface BasicSettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
  onZoomChange: (zoomFactor: number) => void;
  onConfigRefresh: () => Promise<void>;
}

export function BasicSettingsPanel({
  formData,
  onChange,
  onZoomChange,
  onConfigRefresh,
}: BasicSettingsProps) {
  const { t } = useTranslation();
  const [bangumiSnapshot, setBangumiSnapshot]
    = useState<vo.BangumiAuthStatus | null>(null);
  const [bangumiProfile, setBangumiProfile]
    = useState<vo.BangumiProfile | null>(null);
  const [isBangumiStatusLoading, setIsBangumiStatusLoading] = useState(false);
  const [isBangumiProfileLoading, setIsBangumiProfileLoading] = useState(false);
  const [isBangumiAuthorizing, setIsBangumiAuthorizing] = useState(false);
  const [isBangumiDisconnecting, setIsBangumiDisconnecting] = useState(false);
  const [showBangumiDisconnectConfirm, setShowBangumiDisconnectConfirm]
    = useState(false);

  const COMMON_TIMEZONES: BetterSelectOption[] = [
    { value: "Asia/Shanghai", label: "China Standard Time (UTC+8)" },
    { value: "Asia/Tokyo", label: "Japan Standard Time (UTC+9)" },
    { value: "Asia/Seoul", label: "Korea Standard Time (UTC+9)" },
    { value: "Asia/Hong_Kong", label: "Hong Kong Time (UTC+8)" },
    { value: "Asia/Taipei", label: "Taipei Time (UTC+8)" },
    { value: "Asia/Singapore", label: "Singapore Time (UTC+8)" },
    { value: "Asia/Bangkok", label: "Bangkok Time (UTC+7)" },
    { value: "Asia/Dubai", label: "Dubai Time (UTC+4)" },
    { value: "Europe/London", label: "London Time (UTC+0)" },
    { value: "Europe/Paris", label: "Paris Time (UTC+1)" },
    { value: "Europe/Berlin", label: "Berlin Time (UTC+1)" },
    { value: "Europe/Moscow", label: "Moscow Time (UTC+3)" },
    { value: "America/New_York", label: "New York Time (UTC-5)" },
    { value: "America/Chicago", label: "Chicago Time (UTC-6)" },
    { value: "America/Denver", label: "Denver Time (UTC-7)" },
    { value: "America/Los_Angeles", label: "Los Angeles Time (UTC-8)" },
    { value: "America/Sao_Paulo", label: "São Paulo Time (UTC-3)" },
    { value: "Australia/Sydney", label: "Sydney Time (UTC+10)" },
    { value: "Pacific/Auckland", label: "Auckland Time (UTC+12)" },
    { value: "UTC", label: "Coordinated Universal Time (UTC)" },
  ];

  const bangumiAuth = mergeBangumiAuthStatus(formData, bangumiSnapshot);
  const bangumiConfig = formData as BangumiStatusPushConfig;
  const bangumiDisplayName
    = bangumiProfile?.nickname?.trim()
      || bangumiProfile?.username?.trim()
      || bangumiAuth.identity;
  const bangumiUsername = bangumiProfile?.username?.trim() || "";
  const bangumiAvatarUrl
    = bangumiProfile?.avatar_large?.trim()
      || bangumiProfile?.avatar_medium?.trim()
      || bangumiProfile?.avatar_small?.trim()
      || "";
  const isBangumiAuthorized = bangumiAuth.state === "authorized";
  const shouldShowBangumiProfile
    = isBangumiAuthorized && Boolean(bangumiDisplayName);
  const bangumiAvatarFallback
    = bangumiDisplayName.trim().charAt(0).toUpperCase() || "B";

  const refreshBangumiProfile = async (status: vo.BangumiAuthStatus | null) => {
    const isAuthorized
      = Boolean(status?.authorized) && !status?.needs_reauthorization;
    if (!isAuthorized) {
      setBangumiProfile(null);
      return;
    }

    setIsBangumiProfileLoading(true);
    try {
      const nextProfile = await fetchBangumiProfile();
      setBangumiProfile(nextProfile);
    }
    catch (err) {
      console.error("Failed to fetch Bangumi profile:", err);
      setBangumiProfile(null);
    }
    finally {
      setIsBangumiProfileLoading(false);
    }
  };

  const refreshBangumiStatus = async () => {
    setIsBangumiStatusLoading(true);
    try {
      const nextSnapshot = await fetchBangumiAuthStatus();
      setBangumiSnapshot(nextSnapshot);
      await refreshBangumiProfile(nextSnapshot);
    }
    catch (err) {
      console.error("Failed to fetch Bangumi auth status:", err);
      setBangumiSnapshot(null);
      setBangumiProfile(null);
    }
    finally {
      setIsBangumiStatusLoading(false);
    }
  };

  useEffect(() => {
    const loadBangumiStatus = async () => {
      await refreshBangumiStatus();
    };

    void loadBangumiStatus();
  }, []);

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>,
  ) => {
    const { name, value, type } = e.target;
    const newValue
      = type === "checkbox" ? (e.target as HTMLInputElement).checked : value;
    onChange({ ...formData, [name]: newValue } as appconf.AppConfig);
  };

  const handleBangumiAuthorize = async () => {
    setIsBangumiAuthorizing(true);
    try {
      await startBangumiAuthorization();
      await onConfigRefresh();
      await refreshBangumiStatus();
      toast.success(t("settings.basic.bangumiAuthSuccess"));
    }
    catch (err) {
      toast.error(
        t("settings.basic.bangumiAuthActionFailed", {
          error: err instanceof Error ? err.message : String(err),
        }),
      );
      await refreshBangumiStatus();
    }
    finally {
      setIsBangumiAuthorizing(false);
    }
  };

  const handleBangumiDisconnect = async () => {
    setIsBangumiDisconnecting(true);
    try {
      await disconnectBangumiAuthorization();
      await onConfigRefresh();
      await refreshBangumiStatus();
      toast.success(t("settings.basic.bangumiDisconnectSuccess"));
    }
    catch (err) {
      toast.error(
        t("settings.basic.bangumiAuthActionFailed", {
          error: err instanceof Error ? err.message : String(err),
        }),
      );
    }
    finally {
      setIsBangumiDisconnecting(false);
    }
  };

  return (
    <>
      <div className="space-y-2">
        <div className="block text-sm font-semibold text-brand-700 dark:text-brand-300">
          {t("settings.basic.bangumiSectionLabel")}
        </div>
        <div className="glass-panel flex items-center justify-between gap-4 rounded-2xl border border-brand-200/80 bg-white/55 p-4 dark:border-brand-700/80 dark:bg-brand-900/25">
          <div className="min-w-0 flex flex-1 items-center gap-3">
            {shouldShowBangumiProfile ? (
              bangumiAvatarUrl ? (
                <img
                  src={bangumiAvatarUrl}
                  alt=""
                  width={48}
                  height={48}
                  className="h-12 w-12 shrink-0 rounded-2xl border border-brand-200/70 object-cover shadow-sm dark:border-brand-700/70"
                />
              ) : (
                <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-brand-200/80 text-sm font-semibold text-brand-700 dark:bg-brand-700/80 dark:text-brand-200">
                  {bangumiAvatarFallback}
                </div>
              )
            ) : (
              <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border border-brand-200/80 bg-white/70 dark:border-brand-700/80 dark:bg-brand-800/70">
                <img
                  src="/bangumi-logo.png"
                  alt=""
                  width={28}
                  height={28}
                  className="h-7 w-7 object-contain opacity-90"
                />
              </div>
            )}

            <div className="min-w-0 space-y-1">
              <div className="flex flex-wrap items-center gap-2">
                <div className="truncate text-sm font-semibold text-brand-800 dark:text-brand-100">
                  {shouldShowBangumiProfile ? bangumiDisplayName : "Bangumi"}
                </div>
                {(isBangumiStatusLoading || isBangumiProfileLoading) && (
                  <span
                    aria-hidden="true"
                    className="i-mdi-loading animate-spin text-brand-400"
                  />
                )}
                {bangumiAuth.state === "needs_reauth" && (
                  <span className="rounded-full bg-warning-100 px-2 py-0.5 text-[11px] font-semibold text-warning-700 dark:bg-warning-900/30 dark:text-warning-300">
                    {t("settings.basic.bangumiAuthNeedsReauth")}
                  </span>
                )}
              </div>

              {shouldShowBangumiProfile ? (
                <>
                  {bangumiUsername
                    && bangumiUsername !== bangumiDisplayName && (
                    <p className="truncate text-xs text-brand-500 dark:text-brand-400">
                      @
                      {bangumiUsername}
                    </p>
                  )}
                  <p className="text-xs text-brand-500 dark:text-brand-400">
                    {t("settings.basic.bangumiAuthAuthorized")}
                  </p>
                </>
              ) : (
                <p className="text-xs text-brand-500 dark:text-brand-400">
                  {bangumiAuth.state === "needs_reauth"
                    ? t("settings.basic.bangumiAuthReconnectHint")
                    : t("settings.basic.bangumiAuthHint")}
                </p>
              )}

              {bangumiAuth.lastError && (
                <p className="text-xs text-warning-700 dark:text-warning-300">
                  {t("settings.basic.bangumiAuthLastErrorLabel")}
                  {": "}
                  {bangumiAuth.lastError}
                </p>
              )}
            </div>
          </div>

          {isBangumiAuthorized ? (
            <BetterButton
              variant="danger"
              icon="i-mdi-link-off"
              isLoading={isBangumiDisconnecting}
              onClick={() => setShowBangumiDisconnectConfirm(true)}
            >
              {t("settings.basic.bangumiDisconnect")}
            </BetterButton>
          ) : (
            <BetterButton
              variant="primary"
              icon="i-mdi-account-key-outline"
              isLoading={isBangumiAuthorizing}
              onClick={handleBangumiAuthorize}
            >
              {bangumiAuth.state === "needs_reauth"
                ? t("settings.basic.bangumiReauthorize")
                : t("settings.basic.bangumiAuthorize")}
            </BetterButton>
          )}
        </div>

        <div className="flex items-center justify-between gap-4 rounded-xl border border-brand-200 bg-white/60 p-3 dark:border-brand-700 dark:bg-brand-900/20">
          <div className="flex-1 space-y-2">
            <div className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              {t("settings.basic.bangumiStatusPushLabel")}
            </div>
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("settings.basic.bangumiStatusPushHint")}
            </p>
          </div>
          <BetterSwitch
            id="bangumi_status_push_enabled"
            checked={bangumiConfig.bangumi_status_push_enabled ?? true}
            onCheckedChange={checked =>
              onChange({
                ...formData,
                bangumi_status_push_enabled: checked,
              } as appconf.AppConfig)}
          />
        </div>
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          VNDB Access Token
        </label>
        <input
          type="text"
          name="vndb_access_token"
          value={formData.vndb_access_token || ""}
          onChange={handleChange}
          className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
        />
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.basic.themeLabel")}
        </label>
        <BetterSelect
          name="theme"
          value={formData.theme}
          onChange={value =>
            onChange({ ...formData, theme: value } as appconf.AppConfig)}
          options={[
            { value: "light", label: t("settings.basic.themeLight") },
            { value: "dark", label: t("settings.basic.themeDark") },
            { value: "system", label: t("settings.basic.themeSystem") },
          ]}
        />
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.basic.languageLabel")}
        </label>
        <BetterSelect
          name="language"
          value={formData.language}
          onChange={value =>
            onChange({ ...formData, language: value } as appconf.AppConfig)}
          options={[
            { value: "zh-CN", label: "简体中文" },
            { value: "en-US", label: "English" },
            { value: "ja-JP", label: "日本語" },
          ]}
        />
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.basic.zoomLabel")}
        </label>
        <BetterSelect
          name="window_zoom_factor"
          value={String(formData.window_zoom_factor || 1)}
          onChange={value => onZoomChange(Number(value))}
          options={appZoomOptions}
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.basic.zoomHint")}
        </p>
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.basic.timezoneLabel")}
        </label>
        <BetterSelect
          name="timezone"
          value={formData.time_zone || "Asia/Shanghai"}
          onChange={value =>
            onChange({ ...formData, time_zone: value } as appconf.AppConfig)}
          options={COMMON_TIMEZONES}
          placeholder={t("settings.basic.timezonePlaceholder")}
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.basic.timezoneHint")}
        </p>
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between gap-4">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("settings.basic.closeToTray")}
          </label>
          <BetterSwitch
            id="close_to_tray"
            checked={formData.close_to_tray || false}
            onCheckedChange={checked =>
              onChange({
                ...formData,
                close_to_tray: checked,
              } as appconf.AppConfig)}
          />
        </div>
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between gap-4">
          <label
            htmlFor="launch_at_login"
            className="block cursor-pointer text-sm font-medium text-brand-700 dark:text-brand-300"
          >
            {t("settings.basic.launchAtLogin")}
          </label>
          <BetterSwitch
            id="launch_at_login"
            checked={formData.launch_at_login || false}
            onCheckedChange={checked =>
              onChange({
                ...formData,
                launch_at_login: checked,
              } as appconf.AppConfig)}
          />
        </div>
      </div>

      <ConfirmModal
        isOpen={showBangumiDisconnectConfirm}
        title={t("settings.basic.bangumiDisconnectConfirmTitle")}
        message={t("settings.basic.bangumiDisconnectConfirmMsg")}
        confirmText={t("settings.basic.bangumiDisconnect")}
        type="danger"
        onClose={() => setShowBangumiDisconnectConfirm(false)}
        onConfirm={() => {
          void handleBangumiDisconnect();
        }}
      />
    </>
  );
}
