import type { Dispatch, SetStateAction } from "react";

import { Window } from "@wailsio/runtime";
import { createElement, useEffect, useRef } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";

import type { appconf, vo } from "../../src/bindings/models";
import type { FetchHomeDataOptions, GameRuntimeChangedEvent } from "../store";

import { ShouldShowMainWindowOnReady } from "../../bindings/lunabox/internal/service/configservice";
import { GetPendingInstall } from "../../bindings/lunabox/internal/service/downloadservice";
import { onWailsEvent } from "../../src/bindings/runtime";
import { useAppStore } from "../store";

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

type HikarinagiStatusPushFailureEvent = {
  game_id?: string;
  game_name?: string;
  work_id?: string;
  local_status?: string;
  error?: string;
};

type UseAppRuntimeEffectsOptions = {
  config: appconf.AppConfig | null;
  refreshConfig: () => Promise<void>;
  refreshHomeData: (options?: FetchHomeDataOptions) => Promise<void>;
  setInstallRequest: Dispatch<SetStateAction<vo.InstallRequest | null>>;
  setQuitSyncRequest: Dispatch<SetStateAction<QuitSyncRequest | null>>;
  openGameLaunchSettings?: (gameID: string) => void;
};

const WAILS_RESIZE_BORDER_THICKNESS = 5;

function renderProtocolLaunchConfigToast({
  visible,
  id,
  message,
  detail,
  actionLabel,
  onAction,
}: {
  visible: boolean;
  id: string;
  message: string;
  detail?: string;
  actionLabel: string;
  onAction: () => void;
}) {
  return createElement(
    "div",
    {
      className: `rounded-lg border border-brand-200 bg-white px-4 py-3 shadow-lg dark:border-brand-700 dark:bg-brand-800 ${visible ? "animate-enter" : "animate-leave"}`,
    },
    createElement(
      "div",
      { className: "space-y-3" },
      createElement(
        "div",
        null,
        createElement(
          "p",
          { className: "text-sm font-medium text-brand-900 dark:text-white" },
          message,
        ),
        detail
          ? createElement(
              "p",
              {
                className:
                  "mt-1 max-w-md whitespace-pre-wrap text-xs text-brand-500 dark:text-brand-400",
              },
              detail,
            )
          : null,
      ),
      createElement(
        "button",
        {
          type: "button",
          className:
            "inline-flex items-center gap-1 rounded-md bg-neutral-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-neutral-700",
          onClick: () => {
            toast.dismiss(id);
            onAction();
          },
        },
        createElement("span", {
          className: "i-mdi-cog-play-outline text-sm",
        }),
        actionLabel,
      ),
    ),
  );
}

export function useAppRuntimeEffects({
  config,
  refreshConfig,
  refreshHomeData,
  setInstallRequest,
  setQuitSyncRequest,
  openGameLaunchSettings,
}: UseAppRuntimeEffectsOptions) {
  const { t } = useTranslation();
  const applyGameRuntimeEvent = useAppStore(
    state => state.applyGameRuntimeEvent,
  );
  const initialWindowReadyCheckedRef = useRef(false);
  const skipNextLaunchHomeRefreshRef = useRef(false);

  useEffect(() => {
    if (window.wails?.flags) {
      window.wails.flags.borderThickness = WAILS_RESIZE_BORDER_THICKNESS;
    }
  }, []);

  useEffect(() => {
    if (!config || initialWindowReadyCheckedRef.current) {
      return;
    }

    initialWindowReadyCheckedRef.current = true;
    let cancelled = false;

    void ShouldShowMainWindowOnReady()
      .then((shouldShow) => {
        if (cancelled || !shouldShow) {
          return;
        }
        void Window.Show();
      })
      .catch((error) => {
        console.error("Failed to resolve initial window visibility:", error);
        if (!cancelled) {
          void Window.Show();
        }
      });

    GetPendingInstall().then((req) => {
      if (cancelled || !req) {
        return;
      }

      setInstallRequest(req);
      void Window.Show();
    });

    return () => {
      cancelled = true;
    };
  }, [config, setInstallRequest]);

  useEffect(() => {
    const unsubscribe = onWailsEvent(
      "install:pending",
      (req: vo.InstallRequest) => {
        setInstallRequest(req);
        void Window.Show();
      },
    );

    return unsubscribe;
  }, [setInstallRequest]);

  useEffect(() => {
    const unsubscribe = onWailsEvent(
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
    const unsubscribe = onWailsEvent(
      "protocol-launch:error",
      (payload?: {
        message?: string;
        detail?: string;
        game_id?: string;
        kind?: string;
        config_key?: string;
      }) => {
        const message = payload?.message?.trim() || "快捷启动失败";
        const detail = payload?.detail?.trim();
        void Window.Show();
        if (
          payload?.kind === "missing-config"
          && payload?.config_key === "wine_runner"
          && payload?.game_id
          && openGameLaunchSettings
        ) {
          toast.custom(
            toastItem =>
              renderProtocolLaunchConfigToast({
                visible: toastItem.visible,
                id: toastItem.id,
                message,
                detail,
                actionLabel: t("gameLaunch.openLaunchConfig"),
                onAction: () => openGameLaunchSettings(payload.game_id || ""),
              }),
            { id: "protocol-launch-error" },
          );
          return;
        }
        toast.error(detail ? `${message}\n${detail}` : message, {
          id: "protocol-launch-error",
        });
      },
    );

    return unsubscribe;
  }, [openGameLaunchSettings, t]);

  useEffect(() => {
    const unsubscribe = onWailsEvent("bangumi:auth-status-changed", () => {
      void refreshConfig();
    });

    return unsubscribe;
  }, [refreshConfig]);

  useEffect(() => {
    const unsubscribe = onWailsEvent("hikarinagi:auth-status-changed", () => {
      void refreshConfig();
    });

    return unsubscribe;
  }, [refreshConfig]);

  useEffect(() => {
    const unsubscribe = onWailsEvent(
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
    const unsubscribe = onWailsEvent(
      "hikarinagi:status-push-failed",
      (payload?: HikarinagiStatusPushFailureEvent) => {
        const gameName
          = payload?.game_name?.trim()
            || t("settings.basic.hikarinagiStatusPushFailedUnknownGame");
        const error
          = payload?.error?.trim()
            || t("settings.basic.hikarinagiStatusPushFailedUnknownReason");
        toast.error(
          t("settings.basic.hikarinagiStatusPushFailed", {
            game: gameName,
            error,
          }),
          {
            id: `hikarinagi-status-push-failed-${payload?.game_id || "unknown"}`,
          },
        );
      },
    );

    return unsubscribe;
  }, [t]);

  useEffect(() => {
    const unsubscribe = onWailsEvent("app:main-window-shown", () => {
      void refreshHomeData();
    });

    return unsubscribe;
  }, [refreshHomeData]);

  useEffect(() => {
    const unsubscribe = onWailsEvent("home:refresh-requested", () => {
      if (skipNextLaunchHomeRefreshRef.current) {
        skipNextLaunchHomeRefreshRef.current = false;
        return;
      }

      void refreshHomeData({ showLoading: false, syncRuntime: false });
    });

    return unsubscribe;
  }, [refreshHomeData]);

  useEffect(() => {
    const unsubscribe = onWailsEvent(
      "game-runtime:changed",
      (event?: GameRuntimeChangedEvent) => {
        if (!event) {
          return;
        }

        applyGameRuntimeEvent(event);

        if (event.state === "launching" && event.reason === "launched") {
          skipNextLaunchHomeRefreshRef.current = true;
          return;
        }

        if (event.state === "playing" || event.state === "ending") {
          return;
        }

        void refreshHomeData({ showLoading: false, syncRuntime: false });
      },
    );

    return unsubscribe;
  }, [applyGameRuntimeEvent, refreshHomeData]);
}
