import type { Dispatch, SetStateAction } from "react";

import { useEffect } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";

import type { appconf, vo } from "../../wailsjs/go/models";

import { ShouldShowMainWindowOnReady } from "../../wailsjs/go/service/ConfigService";
import { GetPendingInstall } from "../../wailsjs/go/service/DownloadService";
import { EventsOn, WindowShow } from "../../wailsjs/runtime/runtime";

export type ProcessSelectData = {
  isOpen: boolean;
  gameID: string;
  launcherExeName: string;
};

export type QuitSyncRequest = {
  reason: string;
  requestedAt: number;
};

type BangumiStatusPushFailureEvent = {
  game_id?: string;
  game_name?: string;
  subject_id?: string;
  local_status?: string;
  error?: string;
};

type UseAppRuntimeEffectsOptions = {
  config: appconf.AppConfig | null;
  refreshConfig: () => Promise<void>;
  refreshHomeData: () => Promise<void>;
  setProcessSelectData: Dispatch<SetStateAction<ProcessSelectData>>;
  setInstallRequest: Dispatch<SetStateAction<vo.InstallRequest | null>>;
  setQuitSyncRequest: Dispatch<SetStateAction<QuitSyncRequest | null>>;
};

export function useAppRuntimeEffects({
  config,
  refreshConfig,
  refreshHomeData,
  setProcessSelectData,
  setInstallRequest,
  setQuitSyncRequest,
}: UseAppRuntimeEffectsOptions) {
  const { t } = useTranslation();

  useEffect(() => {
    const unsubscribe = EventsOn(
      "process-select-required",
      (data: {
        gameID: string;
        sessionID: string;
        launcherExeName: string;
      }) => {
        console.warn("Process select required:", data);
        WindowShow();
        setProcessSelectData({
          isOpen: true,
          gameID: data.gameID,
          launcherExeName: data.launcherExeName,
        });
      },
    );

    return unsubscribe;
  }, [setProcessSelectData]);

  useEffect(() => {
    if (!config) {
      return;
    }

    let cancelled = false;

    document.getElementById("root")?.classList.add("ready");
    void ShouldShowMainWindowOnReady()
      .then((shouldShow) => {
        if (cancelled || !shouldShow) {
          return;
        }
        WindowShow();
      })
      .catch((error) => {
        console.error("Failed to resolve initial window visibility:", error);
        if (!cancelled) {
          WindowShow();
        }
      });

    GetPendingInstall().then((req) => {
      if (cancelled || !req) {
        return;
      }

      setInstallRequest(req);
      WindowShow();
    });

    return () => {
      cancelled = true;
    };
  }, [config, setInstallRequest]);

  useEffect(() => {
    const unsubscribe = EventsOn(
      "install:pending",
      (req: vo.InstallRequest) => {
        setInstallRequest(req);
        WindowShow();
      },
    );

    return unsubscribe;
  }, [setInstallRequest]);

  useEffect(() => {
    const unsubscribe = EventsOn(
      "app:quit-sync-requested",
      (payload?: { reason?: string }) => {
        setQuitSyncRequest({
          reason: payload?.reason ?? "unknown",
          requestedAt: Date.now(),
        });
      },
    );

    return unsubscribe;
  }, [setQuitSyncRequest]);

  useEffect(() => {
    const unsubscribe = EventsOn(
      "protocol-launch:error",
      (payload?: { message?: string; detail?: string }) => {
        const message = payload?.message?.trim() || "快捷启动失败";
        const detail = payload?.detail?.trim();
        WindowShow();
        toast.error(detail ? `${message}\n${detail}` : message, {
          id: "protocol-launch-error",
        });
      },
    );

    return unsubscribe;
  }, []);

  useEffect(() => {
    const unsubscribe = EventsOn("bangumi:auth-status-changed", () => {
      void refreshConfig();
    });

    return unsubscribe;
  }, [refreshConfig]);

  useEffect(() => {
    const unsubscribe = EventsOn(
      "bangumi:status-push-failed",
      (payload?: BangumiStatusPushFailureEvent) => {
        const gameName
          = payload?.game_name?.trim()
            || t("settings.basic.bangumiStatusPushFailedUnknownGame");
        const error
          = payload?.error?.trim()
            || t("settings.basic.bangumiStatusPushFailedUnknownReason");
        toast.error(
          t("settings.basic.bangumiStatusPushFailed", {
            game: gameName,
            error,
          }),
          {
            id: `bangumi-status-push-failed-${payload?.game_id || "unknown"}`,
          },
        );
      },
    );

    return unsubscribe;
  }, [t]);

  useEffect(() => {
    const unsubscribe = EventsOn("app:main-window-shown", () => {
      void refreshHomeData();
    });

    return unsubscribe;
  }, [refreshHomeData]);

  useEffect(() => {
    const unsubscribe = EventsOn("home:refresh-requested", () => {
      void refreshHomeData();
    });

    return unsubscribe;
  }, [refreshHomeData]);
}
