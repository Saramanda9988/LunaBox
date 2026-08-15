import type { appconf, vo } from "../../../src/bindings/models";

import { useCallback, useEffect, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";

import bangumiIconUrl from "../../assets/providers/bangumi-icon.png";
import bangumiLogoUrl from "../../assets/providers/bangumi-logo.png";
import { useRemoteStatusSync } from "../../hooks/useRemoteStatusSync";
import {
  BANGUMI_STATUS_SYNC_PROGRESS_EVENT,
  disconnectBangumiAuthorization,
  fetchBangumiAuthStatus,
  fetchBangumiProfile,
  mergeBangumiAuthStatus,
  startBangumiAuthorization,
  syncAllBangumiGameStatuses,
} from "../../utils/bangumiAuth";
import { ConfirmModal } from "../modal/ConfirmModal";
import { RemoteStatusSyncProgressModal } from "../modal/RemoteStatusSyncProgressModal";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterSwitch } from "../ui/better/BetterSwitch";

type BangumiStatusPushConfig = appconf.AppConfig & {
  bangumi_status_push_enabled?: boolean;
};

interface BangumiAccountSettingsProps {
  formData: appconf.AppConfig;
  isContentVisible: boolean;
  isExpanded: boolean;
  onChange: (data: appconf.AppConfig) => void;
  onConfigRefresh: () => Promise<void>;
  onExpand: () => void;
}

export function BangumiAccountSettings({
  formData,
  isContentVisible,
  isExpanded,
  onChange,
  onConfigRefresh,
  onExpand,
}: BangumiAccountSettingsProps) {
  const { t } = useTranslation();
  const [snapshot, setSnapshot] = useState<vo.BangumiAuthStatus | null>(null);
  const [profile, setProfile] = useState<vo.BangumiProfile | null>(null);
  const [isStatusLoading, setIsStatusLoading] = useState(false);
  const [isProfileLoading, setIsProfileLoading] = useState(false);
  const [isAuthorizing, setIsAuthorizing] = useState(false);
  const [isDisconnecting, setIsDisconnecting] = useState(false);
  const [showDisconnectConfirm, setShowDisconnectConfirm] = useState(false);
  const [showSyncConfirm, setShowSyncConfirm] = useState(false);

  const auth = mergeBangumiAuthStatus(formData, snapshot);
  const config = formData as BangumiStatusPushConfig;
  const displayName
    = profile?.nickname?.trim() || profile?.username?.trim() || auth.identity;
  const username = profile?.username?.trim() || auth.identity;
  const avatarURL
    = profile?.avatar_url?.trim()
      || auth.avatarUrl?.trim()
      || profile?.avatar_large?.trim()
      || profile?.avatar_medium?.trim()
      || profile?.avatar_small?.trim()
      || "";
  const isAuthorized = auth.state === "authorized";
  const shouldShowProfile = isAuthorized && Boolean(displayName);
  const avatarFallback = displayName.trim().charAt(0).toUpperCase() || "B";
  const statusSync = useRemoteStatusSync(
    "Bangumi",
    BANGUMI_STATUS_SYNC_PROGRESS_EVENT,
    syncAllBangumiGameStatuses,
  );

  const refreshProfile = useCallback(
    async (status: vo.BangumiAuthStatus | null) => {
      const canLoadProfile
        = Boolean(status?.authorized) && !status?.needs_reauthorization;
      if (!canLoadProfile) {
        setProfile(null);
        return;
      }

      setIsProfileLoading(true);
      try {
        setProfile(await fetchBangumiProfile());
      }
      catch (error) {
        console.error("Failed to fetch Bangumi profile:", error);
        setProfile(null);
      }
      finally {
        setIsProfileLoading(false);
      }
    },
    [],
  );

  const refreshStatus = useCallback(async () => {
    setIsStatusLoading(true);
    try {
      const nextSnapshot = await fetchBangumiAuthStatus();
      setSnapshot(nextSnapshot);
      await refreshProfile(nextSnapshot);
    }
    catch (error) {
      console.error("Failed to fetch Bangumi auth status:", error);
      setSnapshot(null);
      setProfile(null);
    }
    finally {
      setIsStatusLoading(false);
    }
  }, [refreshProfile]);

  useEffect(() => {
    void refreshStatus();
  }, [refreshStatus]);

  const handleAuthorize = async () => {
    setIsAuthorizing(true);
    try {
      await startBangumiAuthorization();
      await onConfigRefresh();
      await refreshStatus();
      toast.success(t("settings.basic.bangumiAuthSuccess"));
    }
    catch (error) {
      toast.error(
        t("settings.basic.bangumiAuthActionFailed", {
          error: error instanceof Error ? error.message : String(error),
        }),
      );
      await refreshStatus();
    }
    finally {
      setIsAuthorizing(false);
    }
  };

  const handleDisconnect = async () => {
    setIsDisconnecting(true);
    try {
      await disconnectBangumiAuthorization();
      await onConfigRefresh();
      await refreshStatus();
      toast.success(t("settings.basic.bangumiDisconnectSuccess"));
    }
    catch (error) {
      toast.error(
        t("settings.basic.bangumiAuthActionFailed", {
          error: error instanceof Error ? error.message : String(error),
        }),
      );
    }
    finally {
      setIsDisconnecting(false);
    }
  };

  return (
    <>
      <div
        className={`glass-panel relative isolate min-h-[132px] min-w-0 overflow-hidden rounded-2xl border transition-colors duration-200 sm:h-[190px] lg:h-[160px] ${
          isExpanded
            ? "border-brand-300/90 bg-brand-50/70 shadow-sm dark:border-brand-600/90 dark:bg-brand-900/35"
            : "border-brand-200/80 bg-white/55 hover:border-brand-300/80 dark:border-brand-700/80 dark:bg-brand-900/25 dark:hover:border-brand-600/80"
        }`}
      >
        <img
          src={bangumiLogoUrl}
          alt=""
          aria-hidden="true"
          className="pointer-events-none absolute bottom-4 right-4 z-0 h-auto w-48 object-contain opacity-30 dark:opacity-25"
        />

        {isExpanded ? (
          <div
            className={`account-choice-content-transition relative z-10 flex h-full flex-col gap-3 overflow-y-auto p-3 motion-reduce:transition-none ${
              isContentVisible ? "opacity-100" : "pointer-events-none opacity-0"
            }`}
          >
            <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
              <div className="min-w-0 flex flex-1 items-center gap-3">
                {shouldShowProfile ? (
                  avatarURL ? (
                    <img
                      src={avatarURL}
                      alt=""
                      width={48}
                      height={48}
                      className="h-12 w-12 shrink-0 rounded-2xl border border-brand-200/70 object-cover shadow-sm dark:border-brand-700/70"
                    />
                  ) : (
                    <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-brand-200/80 text-sm font-semibold text-brand-700 dark:bg-brand-700/80 dark:text-brand-200">
                      {avatarFallback}
                    </div>
                  )
                ) : (
                  <div className="h-12 w-12 shrink-0 overflow-hidden rounded-2xl border border-brand-200/80 bg-white/70 dark:border-brand-700/80 dark:bg-brand-800/70">
                    <img
                      src={bangumiIconUrl}
                      alt=""
                      width={48}
                      height={48}
                      className="h-full w-full object-cover"
                    />
                  </div>
                )}

                <div className="min-w-0 space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <div className="truncate text-sm font-semibold text-brand-800 dark:text-brand-100">
                      {shouldShowProfile ? displayName : "Bangumi"}
                    </div>
                    {isStatusLoading || isProfileLoading ? (
                      <span
                        aria-hidden="true"
                        className="i-mdi-loading animate-spin text-brand-400"
                      />
                    ) : null}
                    {auth.state === "needs_reauth" ? (
                      <span className="rounded-full bg-warning-100 px-2 py-0.5 text-[11px] font-semibold text-warning-700 dark:bg-warning-900/30 dark:text-warning-300">
                        {t("settings.basic.bangumiAuthNeedsReauth")}
                      </span>
                    ) : null}
                  </div>

                  {shouldShowProfile ? (
                    <p className="truncate text-xs text-brand-500 dark:text-brand-400">
                      @
                      {username}
                    </p>
                  ) : (
                    <p className="text-xs text-brand-500 dark:text-brand-400">
                      {auth.state === "needs_reauth"
                        ? t("settings.basic.bangumiAuthReconnectHint")
                        : t("settings.basic.bangumiAuthHint")}
                    </p>
                  )}

                  {auth.lastError ? (
                    <p className="text-xs text-warning-700 dark:text-warning-300">
                      {t("settings.basic.bangumiAuthLastErrorLabel")}
                      {": "}
                      {auth.lastError}
                    </p>
                  ) : null}
                </div>
              </div>

              <div className="flex self-end gap-2 lg:self-auto">
                {isAuthorized ? (
                  <>
                    <BetterButton
                      variant="secondary"
                      size="sm"
                      icon="i-mdi-cloud-sync-outline"
                      isLoading={statusSync.isSyncing}
                      disabled={isDisconnecting}
                      className="!rounded-full"
                      aria-label={t("settings.basic.bangumiSyncAllLabel")}
                      onClick={() => setShowSyncConfirm(true)}
                    />
                    <BetterButton
                      variant="danger"
                      size="sm"
                      icon="i-mdi-link-off"
                      isLoading={isDisconnecting}
                      disabled={statusSync.isSyncing}
                      className="!rounded-full"
                      aria-label={t("settings.basic.bangumiDisconnect")}
                      onClick={() => setShowDisconnectConfirm(true)}
                    />
                  </>
                ) : (
                  <BetterButton
                    variant="primary"
                    icon="i-mdi-account-key-outline"
                    isLoading={isAuthorizing}
                    onClick={handleAuthorize}
                  >
                    {auth.state === "needs_reauth"
                      ? t("settings.basic.bangumiReauthorize")
                      : t("settings.basic.bangumiAuthorize")}
                  </BetterButton>
                )}
              </div>
            </div>

            {isAuthorized ? (
              <>
                <div className="h-px w-full shrink-0 bg-brand-200/50 dark:bg-brand-700/50" />
                <div className="flex items-center justify-between gap-4">
                  <div className="flex-1 space-y-1">
                    <div className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                      {t("settings.basic.bangumiStatusPushLabel")}
                    </div>
                    <p className="text-xs text-brand-500 dark:text-brand-400">
                      {t("settings.basic.bangumiStatusPushHint")}
                    </p>
                  </div>
                  <BetterSwitch
                    id="bangumi_status_push_enabled"
                    checked={config.bangumi_status_push_enabled ?? true}
                    onCheckedChange={checked =>
                      onChange({
                        ...formData,
                        bangumi_status_push_enabled: checked,
                      } as appconf.AppConfig)}
                  />
                </div>
              </>
            ) : null}
          </div>
        ) : (
          <button
            type="button"
            className={`account-choice-content-transition relative z-10 flex h-full min-h-[132px] w-full items-center p-3 text-left motion-reduce:transition-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-brand-400 ${
              isContentVisible ? "opacity-100" : "pointer-events-none opacity-0"
            }`}
            aria-expanded="false"
            onClick={onExpand}
          >
            <span className="flex min-w-0 flex-1 items-center gap-3">
              {shouldShowProfile ? (
                avatarURL ? (
                  <img
                    src={avatarURL}
                    alt=""
                    width={48}
                    height={48}
                    className="h-12 w-12 shrink-0 rounded-2xl border border-brand-200/70 object-cover shadow-sm dark:border-brand-700/70"
                  />
                ) : (
                  <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-brand-200/80 text-sm font-semibold text-brand-700 dark:bg-brand-700/80 dark:text-brand-200">
                    {avatarFallback}
                  </span>
                )
              ) : (
                <span className="h-12 w-12 shrink-0 overflow-hidden rounded-2xl border border-brand-200/80 bg-white/70 dark:border-brand-700/80 dark:bg-brand-800/70">
                  <img
                    src={bangumiIconUrl}
                    alt=""
                    width={48}
                    height={48}
                    className="h-full w-full object-cover"
                  />
                </span>
              )}

              <span className="min-w-0 flex-1 space-y-1">
                <span className="flex min-w-0 items-center gap-2">
                  <span className="truncate text-sm font-semibold text-brand-800 dark:text-brand-100">
                    {shouldShowProfile ? displayName : "Bangumi"}
                  </span>
                  {isStatusLoading || isProfileLoading ? (
                    <span
                      aria-hidden="true"
                      className="i-mdi-loading shrink-0 animate-spin text-brand-400"
                    />
                  ) : null}
                </span>

                {shouldShowProfile ? (
                  <span className="block truncate text-xs text-brand-500 dark:text-brand-400">
                    @
                    {username}
                  </span>
                ) : (
                  <span className="block text-xs text-brand-500 dark:text-brand-400">
                    {t(
                      auth.state === "needs_reauth"
                        ? "settings.basic.bangumiAuthNeedsReauth"
                        : "settings.basic.bangumiAuthUnauthorized",
                    )}
                  </span>
                )}
              </span>

              <span
                aria-hidden="true"
                className="i-mdi-chevron-right shrink-0 text-lg text-brand-400"
              />
            </span>
          </button>
        )}
      </div>

      <ConfirmModal
        isOpen={showSyncConfirm}
        title={t("settings.basic.bangumiSyncAllConfirmTitle")}
        message={t("settings.basic.bangumiSyncAllConfirmMsg")}
        confirmText={t("settings.basic.remoteStatusSyncStart")}
        onClose={() => setShowSyncConfirm(false)}
        onConfirm={() => {
          void statusSync.startSync();
        }}
      />

      <RemoteStatusSyncProgressModal
        isOpen={statusSync.isProgressOpen}
        isSyncing={statusSync.isSyncing}
        progress={statusSync.progress}
        providerName="Bangumi"
        onClose={statusSync.closeProgress}
      />

      <ConfirmModal
        isOpen={showDisconnectConfirm}
        title={t("settings.basic.bangumiDisconnectConfirmTitle")}
        message={t("settings.basic.bangumiDisconnectConfirmMsg")}
        confirmText={t("settings.basic.bangumiDisconnect")}
        type="danger"
        onClose={() => setShowDisconnectConfirm(false)}
        onConfirm={() => {
          void handleDisconnect();
        }}
      />
    </>
  );
}
