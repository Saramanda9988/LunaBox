import type { Dispatch, SetStateAction } from "react";

import { useEffect } from "react";
import { toast } from "react-hot-toast";

import type { appconf, vo } from "../../wailsjs/go/models";

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

type UseAppRuntimeEffectsOptions = {
  config: appconf.AppConfig | null;
  setProcessSelectData: Dispatch<SetStateAction<ProcessSelectData>>;
  setInstallRequest: Dispatch<SetStateAction<vo.InstallRequest | null>>;
  setQuitSyncRequest: Dispatch<SetStateAction<QuitSyncRequest | null>>;
};

export function useAppRuntimeEffects({
  config,
  setProcessSelectData,
  setInstallRequest,
  setQuitSyncRequest,
}: UseAppRuntimeEffectsOptions) {
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
    WindowShow();

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
}
