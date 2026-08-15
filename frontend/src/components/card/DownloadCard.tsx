import type { service } from "../../../src/bindings/models";
import { useTranslation } from "react-i18next";
import { formatFileSize } from "../../utils/size";

const IMAGE_DOWNLOAD_SOURCE = "cover-image-batch";
const DOWNLOAD_ACTION_BUTTON_CLASS
  = "flex size-9 shrink-0 items-center justify-center rounded-full border-0 bg-transparent transition-[color,background-color,transform] duration-150 hover:scale-105 active:scale-90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/60 disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:scale-100 disabled:active:scale-100";
const DOWNLOAD_ACTION_BUTTON_NEUTRAL_CLASS
  = "text-brand-600 hover:bg-brand-100 hover:text-brand-900 disabled:hover:bg-transparent dark:text-brand-300 dark:hover:bg-brand-700/70 dark:hover:text-white dark:disabled:hover:bg-transparent data-glass:hover:bg-white/15 data-glass:dark:hover:bg-black/20";

function StatusBadge({ status }: { status: service.DownloadTask["status"] }) {
  const { t } = useTranslation();
  const map: Record<string, { cls: string; label: string }> = {
    pending: {
      cls: "bg-warning-100 text-warning-700 dark:bg-warning-900/40 dark:text-warning-300",
      label: t("downloads.status.pending", "等待中"),
    },
    downloading: {
      cls: "bg-info-100 text-info-700 dark:bg-info-900/40 dark:text-info-300",
      label: t("downloads.status.downloading", "下载中"),
    },
    extracting: {
      cls: "bg-info-100 text-info-700 dark:bg-info-900/40 dark:text-info-300",
      label: t("downloads.status.extracting", "解压中"),
    },
    paused: {
      cls: "bg-warning-100 text-warning-700 dark:bg-warning-900/40 dark:text-warning-300",
      label: t("downloads.status.paused", "已暂停"),
    },
    done: {
      cls: "bg-success-100 text-success-700 dark:bg-success-900/40 dark:text-success-300",
      label: t("downloads.status.done", "已完成"),
    },
    error: {
      cls: "bg-error-100 text-error-700 dark:bg-error-900/40 dark:text-error-300",
      label: t("downloads.status.error", "错误"),
    },
    cancelled: {
      cls: "bg-brand-100 text-brand-600 dark:bg-brand-800 dark:text-brand-400",
      label: t("downloads.status.cancelled", "已取消"),
    },
  };
  const { cls, label } = map[status] ?? map.pending;
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${cls}`}>
      {label}
    </span>
  );
}

export function DownloadCard({
  task,
  onCancel,
  onPause,
  onResume,
  onRetry,
  onDelete,
  onCopyURL,
  onOpenFolder,
  onImportAsGame,
  importing,
  imported,
}: {
  task: service.DownloadTask;
  onCancel: (id: string) => void;
  onPause: (id: string) => void;
  onResume: (id: string) => void;
  onRetry: (id: string) => void;
  onDelete: (id: string) => void;
  onCopyURL: (url: string) => void;
  onOpenFolder: (id: string) => void;
  onImportAsGame: (id: string) => void;
  importing?: boolean;
  imported?: boolean;
}) {
  const { t } = useTranslation();
  const isActive = task.status === "pending" || task.status === "downloading";
  const isExtracting = task.status === "extracting";
  const isPaused = task.status === "paused";
  const isError = task.status === "error";
  const canCancel = isActive || isExtracting || isPaused;
  const progress = Math.max(0, Math.min(100, task.progress ?? 0));
  const canOpenFolder = !!task.file_path;
  const isImageDownloadTask
    = task.request.download_source === IMAGE_DOWNLOAD_SOURCE;
  const manualExtractRequired
    = task.status === "done" && task.error === "manual_extract_required";

  return (
    <div className="glass-card flex flex-col gap-3 rounded-xl border border-brand-200 bg-white/90 p-4 shadow-sm transition-all duration-300 hover:shadow-md dark:border-brand-700 dark:bg-brand-800/80">
      <div className="flex items-start justify-between gap-2">
        <div className="flex-1 min-w-0">
          <p className="truncate text-sm font-semibold text-brand-900 dark:text-white">
            {task.request.title || t("downloads.unknownTitle", "未知标题")}
          </p>
          <p className="mt-0.5 truncate text-xs text-brand-500 dark:text-brand-400">
            {task.request.download_source
              ? isImageDownloadTask
                ? t("downloads.imageTask.source", "图片缓存")
                : task.request.download_source
              : t("downloads.unknownSource", "未知来源")}
          </p>
        </div>
        <div className="shrink-0 flex items-center gap-2">
          <StatusBadge status={task.status} />
          <div className="flex items-center gap-1">
            {task.status === "done"
              && task.file_path
              && !isImageDownloadTask && (
              <button
                type="button"
                onClick={() => onImportAsGame(task.id)}
                disabled={importing || imported}
                aria-label={
                  imported
                    ? t("downloads.imported", "已导入")
                    : importing
                      ? t("downloads.importing", "导入中...")
                      : t("downloads.importAsGame", "导入为游戏")
                }
                className={`${DOWNLOAD_ACTION_BUTTON_CLASS} ${
                  imported
                    ? "text-success-600 disabled:opacity-70 dark:text-success-300"
                    : DOWNLOAD_ACTION_BUTTON_NEUTRAL_CLASS
                }`}
              >
                <span
                  className={`text-xl ${
                    importing
                      ? "i-mdi-loading animate-spin"
                      : imported
                        ? "i-mdi-check-circle-outline"
                        : "i-mdi-gamepad-variant-outline"
                  }`}
                  aria-hidden="true"
                />
              </button>
            )}
            <button
              type="button"
              onClick={() => onCopyURL(task.request.url)}
              disabled={!task.request.url}
              aria-label={t("downloads.copyURL", "复制下载地址")}
              className={`${DOWNLOAD_ACTION_BUTTON_CLASS} ${DOWNLOAD_ACTION_BUTTON_NEUTRAL_CLASS}`}
            >
              <span className="i-mdi-link text-xl" aria-hidden="true" />
            </button>
            <button
              type="button"
              onClick={() => onOpenFolder(task.id)}
              disabled={!canOpenFolder}
              aria-label={t("downloads.openFolder", "打开所在文件夹")}
              className={`${DOWNLOAD_ACTION_BUTTON_CLASS} ${DOWNLOAD_ACTION_BUTTON_NEUTRAL_CLASS}`}
            >
              <span
                className="i-mdi-folder-open-outline text-xl"
                aria-hidden="true"
              />
            </button>
            {task.status === "downloading" && !isImageDownloadTask && (
              <button
                type="button"
                onClick={() => onPause(task.id)}
                aria-label={t("downloads.pause", "暂停下载")}
                className={`${DOWNLOAD_ACTION_BUTTON_CLASS} text-warning-600 hover:bg-warning-100 dark:text-warning-300 dark:hover:bg-warning-900/40 data-glass:hover:bg-warning-500/15`}
              >
                <span className="i-mdi-pause text-xl" aria-hidden="true" />
              </button>
            )}
            {isPaused && !isImageDownloadTask && (
              <button
                type="button"
                onClick={() => onResume(task.id)}
                aria-label={t("downloads.resume", "继续下载")}
                className={`${DOWNLOAD_ACTION_BUTTON_CLASS} text-success-600 hover:bg-success-100 dark:text-success-300 dark:hover:bg-success-900/40 data-glass:hover:bg-success-500/15`}
              >
                <span className="i-mdi-play text-xl" aria-hidden="true" />
              </button>
            )}
            {isError && (
              <button
                type="button"
                onClick={() => onRetry(task.id)}
                aria-label={t("downloads.retry", "重试下载")}
                className={`${DOWNLOAD_ACTION_BUTTON_CLASS} text-info-600 hover:bg-info-100 dark:text-info-300 dark:hover:bg-info-900/40 data-glass:hover:bg-info-500/15`}
              >
                <span className="i-mdi-refresh text-xl" aria-hidden="true" />
              </button>
            )}
            {canCancel ? (
              <button
                type="button"
                onClick={() => onCancel(task.id)}
                aria-label={t("downloads.cancel", "取消下载")}
                className={`${DOWNLOAD_ACTION_BUTTON_CLASS} text-error-500 hover:bg-error-100 dark:text-error-400 dark:hover:bg-error-900/40 data-glass:hover:bg-error-500/15`}
              >
                <span className="i-mdi-close text-xl" aria-hidden="true" />
              </button>
            ) : (
              <button
                type="button"
                onClick={() => onDelete(task.id)}
                aria-label={t("downloads.delete", "删除记录")}
                className={`${DOWNLOAD_ACTION_BUTTON_CLASS} text-error-500 hover:bg-error-100 dark:text-error-400 dark:hover:bg-error-900/40 data-glass:hover:bg-error-500/15`}
              >
                <span className="i-mdi-delete text-xl" aria-hidden="true" />
              </button>
            )}
          </div>
        </div>
      </div>

      {(task.status === "downloading"
        || task.status === "pending"
        || task.status === "paused"
        || task.status === "extracting") && (
        <div className="space-y-1">
          <div className="h-2 overflow-hidden rounded-full bg-brand-200 dark:bg-brand-700">
            <div
              className={`h-full rounded-full transition-all duration-300 ${task.status === "extracting" ? "bg-info-500 animate-pulse" : "bg-info-500"}`}
              style={{ width: `${progress}%` }}
            />
          </div>
          <div className="flex justify-between text-xs text-brand-500 dark:text-brand-400">
            <span>
              {progress.toFixed(1)}
              %
            </span>
            <span>
              {isImageDownloadTask ? (
                `${task.downloaded}${task.total > 0 ? ` / ${task.total}` : ""} ${t("downloads.imageTask.unit", "张")}`
              ) : (
                <>
                  {formatFileSize(task.downloaded)}
                  {task.total > 0 ? ` / ${formatFileSize(task.total)}` : ""}
                </>
              )}
            </span>
          </div>
        </div>
      )}

      {task.status === "done" && isImageDownloadTask && (
        <div className="flex items-center gap-1 text-xs text-brand-500 dark:text-brand-400">
          <span className="i-mdi-image-multiple-outline shrink-0" />
          <span>
            {task.error
              ? t(
                  "downloads.imageTask.doneWithError",
                  "图片批量下载完成，部分失败",
                )
              : t("downloads.imageTask.done", "图片批量下载完成")}
          </span>
        </div>
      )}

      {task.status === "done" && task.file_path && !isImageDownloadTask && (
        <>
          <div className="flex items-center gap-1 text-xs text-brand-500 dark:text-brand-400">
            <span className="i-mdi-folder-check shrink-0" />
            <span className="truncate">{task.file_path}</span>
          </div>
          {manualExtractRequired && (
            <div className="flex items-start gap-1 text-xs text-warning-600 dark:text-warning-300">
              <span className="i-mdi-alert-outline shrink-0 mt-0.5" />
              <span>
                {t(
                  "downloads.manualExtractHint",
                  "自动解压失败，已保留压缩包。请手动解压到上述目录后再导入/启动。",
                )}
              </span>
            </div>
          )}
        </>
      )}

      {task.status === "error" && task.error && (
        <div className="flex items-start gap-1 text-xs text-error-500 dark:text-error-400">
          <span className="i-mdi-alert-circle shrink-0 mt-0.5" />
          <span>{task.error}</span>
        </div>
      )}
    </div>
  );
}
