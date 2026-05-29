import { useTranslation } from "react-i18next";
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
}

interface MetadataRefreshProgressModalProps {
  isOpen: boolean;
  progress: MetadataRefreshProgress;
}

export function MetadataRefreshProgressModal({
  isOpen,
  progress,
}: MetadataRefreshProgressModalProps) {
  const { t } = useTranslation();

  if (!isOpen) {
    return null;
  }

  const progressWidth
    = progress.total > 0
      ? `${Math.min(100, Math.max(0, (progress.current / progress.total) * 100))}%`
      : "0%";

  return (
    <ModalPortal>
      <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
        <div className="w-full max-w-xl rounded-xl border border-brand-200 bg-white p-6 shadow-2xl dark:border-brand-700 dark:bg-brand-800">
          <div className="py-8 text-center">
            <div className="i-mdi-loading mx-auto mb-4 animate-spin text-5xl text-info-500" />
            <p className="text-lg text-brand-600 dark:text-brand-300">
              {t("settings.metadata.progress.title", "正在更新游戏元数据")}
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
                className="h-2 rounded-full bg-info-500 transition-all duration-300"
                style={{ width: progressWidth }}
              />
            </div>
            <p className="mt-4 text-xs text-brand-400">
              {t(
                "settings.metadata.progress.hint",
                "请等待当前批量更新完成。图片会在更新后加入下载管理。",
              )}
            </p>
          </div>
        </div>
      </div>
    </ModalPortal>
  );
}
