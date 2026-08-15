import type { service } from "../../../src/bindings/models";
import { useTranslation } from "react-i18next";
import { BetterButton } from "../ui/better/BetterButton";
import { ModalPortal } from "../ui/ModalPortal";

interface SteamImportModalProps {
  isOpen: boolean;
  gameName: string;
  status: service.SteamLaunchStatus | null;
  isChecking: boolean;
  isImporting: boolean;
  onClose: () => void;
  onImport: () => void;
  onRetry: () => void;
  onSelectExecutable: () => void;
}

export function SteamImportModal({
  isOpen,
  gameName,
  status,
  isChecking,
  isImporting,
  onClose,
  onImport,
  onRetry,
  onSelectExecutable,
}: SteamImportModalProps) {
  const { t } = useTranslation();

  if (!isOpen)
    return null;

  const state = status?.state || "checking";
  const content = {
    checking: {
      icon: "i-mdi-loading animate-spin",
      title: t("steamImport.checkingTitle"),
      message: t("steamImport.checkingMessage"),
    },
    needs_import: {
      icon: "i-mdi-steam",
      title: t("steamImport.title"),
      message: t("steamImport.needsImport", { name: gameName }),
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
    executable_required: {
      icon: "i-mdi-file-alert-outline",
      title: t("steamImport.executableRequiredTitle"),
      message: t("steamImport.executableRequired"),
    },
    user_unavailable: {
      icon: "i-mdi-account-alert-outline",
      title: t("steamImport.userUnavailableTitle"),
      message: t("steamImport.userUnavailable"),
    },
    ready: {
      icon: "i-mdi-check-circle-outline",
      title: t("steamImport.readyTitle"),
      message: t("steamImport.ready"),
    },
  }[state] ?? {
    icon: "i-mdi-alert-circle-outline",
    title: t("steamImport.title"),
    message: t("steamImport.unknownState"),
  };

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

          {state === "needs_import" && (
            <div className="mt-5 rounded-xl border border-brand-200 bg-brand-50/80 p-4 dark:border-brand-700 dark:bg-brand-900/30">
              <div className="flex gap-3">
                <div className="i-mdi-information-outline mt-0.5 shrink-0 text-lg text-brand-500" />
                <p className="text-xs leading-relaxed text-brand-500 dark:text-brand-400">
                  {t("steamImport.localIdentityNote")}
                </p>
              </div>
            </div>
          )}

          <div className="mt-7 flex flex-wrap justify-end gap-3">
            <BetterButton variant="secondary" disabled={busy} onClick={onClose}>
              {t("common.cancel")}
            </BetterButton>

            {state === "needs_import" && (
              <BetterButton
                variant="primary"
                icon="i-mdi-plus"
                isLoading={isImporting}
                onClick={onImport}
              >
                {t("steamImport.import")}
              </BetterButton>
            )}

            {state === "executable_required" && (
              <BetterButton
                variant="primary"
                icon="i-mdi-file-search-outline"
                disabled={busy}
                onClick={onSelectExecutable}
              >
                {t("steamImport.selectExecutable")}
              </BetterButton>
            )}

            {(state === "steam_running" || state === "user_unavailable") && (
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
