import type { vo } from "../../bindings/models";

import { useTranslation } from "react-i18next";

import { BetterButton } from "../ui/better/BetterButton";
import { ModalPortal } from "../ui/ModalPortal";

interface RemoteStatusSyncProgressModalProps {
  isOpen: boolean;
  isSyncing: boolean;
  progress: vo.RemoteStatusSyncProgress;
  providerName: string;
  onClose: () => void;
}

export function RemoteStatusSyncProgressModal({
  isOpen,
  isSyncing,
  progress,
  providerName,
  onClose,
}: RemoteStatusSyncProgressModalProps) {
  const { t } = useTranslation();

  if (!isOpen) {
    return null;
  }

  const isFailed = progress.status === "failed";
  const isDone = progress.status === "done";
  const hasPartialFailure = progress.failed_games > 0;
  const progressPercent
    = progress.total > 0
      ? Math.min(100, Math.max(0, (progress.current / progress.total) * 100))
      : isDone
        ? 100
        : 0;
  const iconClassName = isSyncing
    ? "i-mdi-cloud-sync-outline animate-pulse text-info-500"
    : isFailed || hasPartialFailure
      ? "i-mdi-alert-circle-outline text-warning-500"
      : "i-mdi-check-circle-outline text-success-500";
  const title = isSyncing
    ? t("settings.basic.remoteStatusSyncRunningTitle", {
        provider: providerName,
      })
    : isFailed || hasPartialFailure
      ? t("settings.basic.remoteStatusSyncFailedTitle", {
          provider: providerName,
        })
      : t("settings.basic.remoteStatusSyncDoneTitle", {
          provider: providerName,
        });

  return (
    <ModalPortal>
      <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
        <div className="w-full max-w-lg rounded-xl border border-brand-200 bg-white p-6 shadow-2xl dark:border-brand-700 dark:bg-brand-800">
          <div className="py-5 text-center">
            <div className={`${iconClassName} mx-auto mb-4 text-5xl`} />
            <h3 className="text-lg font-semibold text-brand-900 dark:text-brand-100">
              {title}
            </h3>
            <p className="mt-2 text-sm text-brand-500 dark:text-brand-400">
              {progress.current}
              {" "}
              /
              {progress.total}
            </p>
            <p className="mt-2 min-h-5 truncate text-sm text-brand-500 dark:text-brand-400">
              {progress.game_name
                || (progress.total === 0 && isDone
                  ? t("settings.basic.remoteStatusSyncEmpty", {
                      provider: providerName,
                    })
                  : t("settings.basic.remoteStatusSyncPreparing"))}
            </p>

            <div className="mx-auto mt-4 h-2 w-full max-w-md overflow-hidden rounded-full bg-brand-200 dark:bg-brand-700">
              <div
                className={`h-full rounded-full transition-[width] duration-300 ${
                  isFailed || hasPartialFailure
                    ? "bg-warning-500"
                    : "bg-info-500"
                }`}
                style={{ width: `${progressPercent}%` }}
              />
            </div>

            <div className="mx-auto mt-5 grid max-w-sm grid-cols-2 gap-3">
              <div className="rounded-lg bg-success-50 px-3 py-2 dark:bg-success-900/20">
                <div className="text-base font-semibold text-success-700 dark:text-success-300">
                  {progress.succeeded_games}
                </div>
                <div className="text-xs text-success-700/80 dark:text-success-300/80">
                  {t("settings.basic.remoteStatusSyncSucceeded")}
                </div>
              </div>
              <div className="rounded-lg bg-warning-50 px-3 py-2 dark:bg-warning-900/20">
                <div className="text-base font-semibold text-warning-700 dark:text-warning-300">
                  {progress.failed_games}
                </div>
                <div className="text-xs text-warning-700/80 dark:text-warning-300/80">
                  {t("settings.basic.remoteStatusSyncFailed")}
                </div>
              </div>
            </div>

            {progress.last_error ? (
              <p className="mx-auto mt-4 max-w-md break-words text-xs text-warning-700 dark:text-warning-300">
                {progress.last_error}
              </p>
            ) : null}

            {!isSyncing ? (
              <div className="mt-6 flex justify-center">
                <BetterButton variant="secondary" onClick={onClose}>
                  {t("common.complete")}
                </BetterButton>
              </div>
            ) : null}
          </div>
        </div>
      </div>
    </ModalPortal>
  );
}
