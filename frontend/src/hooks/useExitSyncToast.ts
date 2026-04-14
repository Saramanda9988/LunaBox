import { useEffect } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";

import type { QuitSyncRequest } from "./useAppRuntimeEffects";

import { CreateAndUploadDBBackupForQuit } from "../../wailsjs/go/service/BackupService";
import { SafeQuit } from "../../wailsjs/go/service/ConfigService";

const EXIT_SYNC_TOAST_ID = "exit-sync";
const EXIT_SYNC_TIMEOUT_MS = 15000;
const EXIT_SUCCESS_DELAY_MS = 700;
const EXIT_ERROR_DELAY_MS = 1600;

type UseExitSyncToastOptions = {
  quitSyncRequest: QuitSyncRequest | null;
};

export function useExitSyncToast({ quitSyncRequest }: UseExitSyncToastOptions) {
  const { t } = useTranslation();

  useEffect(() => {
    if (!quitSyncRequest) {
      return;
    }

    let cancelled = false;
    let quitTimer: number | undefined;
    let syncTimeoutTimer: number | undefined;

    const requestQuit = (delay: number) => {
      quitTimer = window.setTimeout(() => {
        if (cancelled) {
          return;
        }

        SafeQuit();
      }, delay);
    };

    const runQuitSync = async () => {
      toast.loading(
        `${t("exitSyncToast.syncingTitle")}\n${t("exitSyncToast.syncingMessage")}`,
        {
          duration: Infinity,
          id: EXIT_SYNC_TOAST_ID,
        },
      );

      try {
        await Promise.race([
          CreateAndUploadDBBackupForQuit(),
          new Promise<never>((_, reject) => {
            syncTimeoutTimer = window.setTimeout(() => {
              reject(new Error(t("exitSyncToast.timeoutMessage")));
            }, EXIT_SYNC_TIMEOUT_MS);
          }),
        ]);

        if (cancelled) {
          return;
        }

        toast.success(
          `${t("exitSyncToast.successTitle")}\n${t("exitSyncToast.successMessage")}`,
          {
            duration: EXIT_SUCCESS_DELAY_MS + 1000,
            id: EXIT_SYNC_TOAST_ID,
          },
        );
        requestQuit(EXIT_SUCCESS_DELAY_MS);
      }
      catch (error) {
        if (cancelled) {
          return;
        }

        const errorMessage
          = error instanceof Error ? error.message : String(error);
        toast.error(
          `${t("exitSyncToast.errorTitle")}\n${t("exitSyncToast.errorMessage", { error: errorMessage })}`,
          {
            duration: EXIT_ERROR_DELAY_MS + 1200,
            id: EXIT_SYNC_TOAST_ID,
          },
        );
        requestQuit(EXIT_ERROR_DELAY_MS);
      }
      finally {
        if (syncTimeoutTimer !== undefined) {
          window.clearTimeout(syncTimeoutTimer);
        }
      }
    };

    void runQuitSync();

    return () => {
      cancelled = true;
      if (quitTimer !== undefined) {
        window.clearTimeout(quitTimer);
      }
      if (syncTimeoutTimer !== undefined) {
        window.clearTimeout(syncTimeoutTimer);
      }
    };
  }, [quitSyncRequest, t]);
}
