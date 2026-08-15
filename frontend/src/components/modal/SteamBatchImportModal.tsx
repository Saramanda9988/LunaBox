import type { service } from "../../../src/bindings/models";
import { useTranslation } from "react-i18next";
import { BetterButton } from "../ui/better/BetterButton";
import { ModalPortal } from "../ui/ModalPortal";

interface SteamBatchImportModalProps {
  isOpen: boolean;
  selectedCount: number;
  status: service.SteamLaunchStatus | null;
  isChecking: boolean;
  isImporting: boolean;
  onClose: () => void;
  onImport: () => void;
  onRetry: () => void;
}

type SteamBatchImportModalState
  = | "checking"
    | "confirm"
    | "steam_running"
    | "steam_not_installed";

export function SteamBatchImportModal({
  isOpen,
  selectedCount,
  status,
  isChecking,
  isImporting,
  onClose,
  onImport,
  onRetry,
}: SteamBatchImportModalProps) {
  const { t } = useTranslation();

  if (!isOpen)
    return null;

  let state: SteamBatchImportModalState = "confirm";
  if (isChecking || !status) {
    state = "checking";
  }
  else if (!status.steam_installed) {
    state = "steam_not_installed";
  }
  else if (status.steam_running) {
    state = "steam_running";
  }

  const content = {
    checking: {
      icon: "i-mdi-loading animate-spin",
      title: t("steamImport.checkingTitle"),
      message: t("steamImport.checkingMessage"),
    },
    confirm: {
      icon: "i-mdi-steam",
      title: t("library.batchImportToSteam"),
      message: t("library.toast.batchSteamImportConfirmMsg", {
        count: selectedCount,
      }),
    },
    steam_running: {
      icon: "i-mdi-steam",
      title: t("steamImport.steamRunningTitle"),
      message: t("steamImport.steamRunning"),
    },
    steam_not_installed: {
      icon: "i-mdi-alert-circle-outline",
      title: t("steamImport.steamMissingTitle"),
      message: t("steamImport.steamMissing"),
    },
  }[state];

  const busy = isChecking || isImporting;

  return (
    <ModalPortal>
      <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
        <div className="glass-card w-full max-w-lg rounded-2xl border border-brand-200 bg-white p-6 shadow-2xl dark:border-brand-700 dark:bg-brand-800">
          <div className="flex items-start gap-4">
            <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-neutral-100 text-neutral-700 dark:bg-neutral-900/30 dark:text-neutral-300">
              <div className={`${content.icon} text-2xl`} />
            </div>
            <div className="min-w-0 flex-1">
              <h3 className="text-xl font-bold text-brand-900 dark:text-white">
                {content.title}
              </h3>
              <p className="mt-2 text-sm leading-relaxed text-brand-600 dark:text-brand-400">
                {content.message}
              </p>
            </div>
          </div>

          <div className="mt-7 flex flex-wrap justify-end gap-3">
            <BetterButton variant="secondary" disabled={busy} onClick={onClose}>
              {t("common.cancel")}
            </BetterButton>

            {state === "confirm" && (
              <BetterButton
                variant="primary"
                icon="i-mdi-plus"
                isLoading={isImporting}
                onClick={onImport}
              >
                {t("steamImport.import")}
              </BetterButton>
            )}

            {state === "steam_running" && (
              <BetterButton
                variant="primary"
                icon="i-mdi-refresh"
                isLoading={isChecking}
                onClick={onRetry}
              >
                {t("steamImport.retry")}
              </BetterButton>
            )}
          </div>
        </div>
      </div>
    </ModalPortal>
  );
}
