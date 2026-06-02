import { useTranslation } from "react-i18next";
import { BetterButton } from "../ui/better/BetterButton";
import { ModalPortal } from "../ui/ModalPortal";

export interface MetadataRefreshProgress {
  status: string;
  current: number;
  total: number;
  game_name: string;
  updated_games: number;
  skipped_games: number;
  failed_games: number;
  locked_games: number;
  failed_game_ids?: string[];
  failed_game_names?: string[];
}

interface MetadataRefreshProgressModalProps {
  isOpen: boolean;
  progress: MetadataRefreshProgress;
  isRefreshing: boolean;
  onRetryFailed?: () => void;
  onClose?: () => void;
}

export function MetadataRefreshProgressModal({
  isOpen,
  progress,
  isRefreshing,
  onRetryFailed,
  onClose,
}: MetadataRefreshProgressModalProps) {
  const { t } = useTranslation();

  if (!isOpen) {
    return null;
  }

  const progressWidth
    = progress.total > 0
      ? `${Math.min(100, Math.max(0, (progress.current / progress.total) * 100))}%`
      : "0%";
  const failedGameIDs = progress.failed_game_ids || [];
  const failedCount = progress.failed_games || failedGameIDs.length;
  const canRetryFailed = Boolean(
    !isRefreshing && failedGameIDs.length > 0 && onRetryFailed,
  );
  const iconClassName = isRefreshing
    ? "i-mdi-loading animate-spin text-info-500"
    : failedCount > 0
      ? "i-mdi-alert-circle-outline text-warning-500"
      : "i-mdi-check-circle-outline text-success-500";
  const title = isRefreshing
    ? t("settings.metadata.progress.title", "正在更新游戏元数据")
    : failedCount > 0
      ? t("settings.metadata.progress.failedTitle", "部分游戏元数据更新失败")
      : t("settings.metadata.progress.doneTitle", "游戏元数据更新完成");

  return (
    <ModalPortal>
      <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
        <div className="w-full max-w-xl rounded-xl border border-brand-200 bg-white p-6 shadow-2xl dark:border-brand-700 dark:bg-brand-800">
          <div className="py-8 text-center">
            <div className={`${iconClassName} mx-auto mb-4 text-5xl`} />
            <p className="text-lg text-brand-600 dark:text-brand-300">
              {title}
            </p>
            <p className="mt-2 text-sm text-brand-400 dark:text-brand-500">
              {progress.current}
              {" "}
              /
              {progress.total}
            </p>
            <p className="mt-2 min-h-5 truncate text-sm text-neutral-500 dark:text-neutral-400">
              {progress.game_name
                || t("settings.metadata.progress.preparing", "准备中...")}
            </p>
            <div className="mx-auto mt-4 h-2 w-full max-w-md rounded-full bg-brand-200 dark:bg-brand-700">
              <div
                className={`h-2 rounded-full transition-all duration-300 ${
                  failedCount > 0 && !isRefreshing
                    ? "bg-warning-500"
                    : "bg-info-500"
                }`}
                style={{ width: progressWidth }}
              />
            </div>
            <div className="mx-auto mt-5 grid max-w-md grid-cols-2 gap-2 sm:grid-cols-4">
              <div className="rounded-lg bg-success-50 px-3 py-2 dark:bg-success-900/20">
                <div className="text-base font-semibold text-success-700 dark:text-success-300">
                  {progress.updated_games}
                </div>
                <div className="text-xs text-success-700/80 dark:text-success-300/80">
                  {t("settings.metadata.progress.updated", "成功")}
                </div>
              </div>
              <div className="rounded-lg bg-warning-50 px-3 py-2 dark:bg-warning-900/20">
                <div className="text-base font-semibold text-warning-700 dark:text-warning-300">
                  {failedCount}
                </div>
                <div className="text-xs text-warning-700/80 dark:text-warning-300/80">
                  {t("settings.metadata.progress.failed", "失败")}
                </div>
              </div>
              <div className="rounded-lg bg-brand-50 px-3 py-2 dark:bg-brand-900/30">
                <div className="text-base font-semibold text-brand-700 dark:text-brand-300">
                  {progress.skipped_games}
                </div>
                <div className="text-xs text-brand-500 dark:text-brand-400">
                  {t("settings.metadata.progress.skipped", "跳过")}
                </div>
              </div>
              <div className="rounded-lg bg-brand-50 px-3 py-2 dark:bg-brand-900/30">
                <div className="text-base font-semibold text-brand-700 dark:text-brand-300">
                  {progress.locked_games}
                </div>
                <div className="text-xs text-brand-500 dark:text-brand-400">
                  {t("settings.metadata.progress.locked", "锁定")}
                </div>
              </div>
            </div>
            {isRefreshing ? (
              <p className="mt-4 text-xs text-brand-400">
                {t(
                  "settings.metadata.progress.hint",
                  "请等待当前批量更新完成。图片会在更新后加入下载管理。",
                )}
              </p>
            ) : (
              <div className="mt-6 flex flex-col-reverse justify-center gap-3 sm:flex-row">
                {onClose && (
                  <BetterButton variant="secondary" onClick={onClose}>
                    {t("common.complete")}
                  </BetterButton>
                )}
                {canRetryFailed && (
                  <BetterButton
                    variant="primary"
                    icon="i-mdi-refresh"
                    onClick={onRetryFailed}
                  >
                    {t("settings.metadata.progress.retryFailed", {
                      count: failedGameIDs.length,
                    })}
                  </BetterButton>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </ModalPortal>
  );
}
