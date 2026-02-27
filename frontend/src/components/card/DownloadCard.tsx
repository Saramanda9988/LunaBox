import type { service } from "../../../wailsjs/go/models";
import { useTranslation } from "react-i18next";
import { formatFileSize } from "../../utils/size";

function StatusBadge({ status }: { status: service.DownloadTask["status"] }) {
  const { t } = useTranslation();
  const map: Record<service.DownloadTask["status"], { cls: string; label: string }> = {
    pending: { cls: "bg-warning-100 text-warning-700 dark:bg-warning-900/40 dark:text-warning-300", label: t("downloads.status.pending", "等待中") },
    downloading: { cls: "bg-info-100 text-info-700 dark:bg-info-900/40 dark:text-info-300", label: t("downloads.status.downloading", "下载中") },
    done: { cls: "bg-success-100 text-success-700 dark:bg-success-900/40 dark:text-success-300", label: t("downloads.status.done", "已完成") },
    error: { cls: "bg-error-100 text-error-700 dark:bg-error-900/40 dark:text-error-300", label: t("downloads.status.error", "错误") },
    cancelled: { cls: "bg-brand-100 text-brand-600 dark:bg-brand-800 dark:text-brand-400", label: t("downloads.status.cancelled", "已取消") },
  };
  const { cls, label } = map[status] ?? map.pending;
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${cls}`}>{label}</span>
  );
}

export function DownloadCard({
  task,
  onCancel,
  onDelete,
  onCopyURL,
  onOpenFolder,
  onImportAsGame,
  importing,
  imported,
}: {
  task: service.DownloadTask;
  onCancel: (id: string) => void;
  onDelete: (id: string) => void;
  onCopyURL: (url: string) => void;
  onOpenFolder: (id: string) => void;
  onImportAsGame: (id: string) => void;
  importing?: boolean;
  imported?: boolean;
}) {
  const { t } = useTranslation();
  const isActive = task.status === "pending" || task.status === "downloading";
  const progress = Math.max(0, Math.min(100, task.progress ?? 0));
  const canOpenFolder = !!task.file_path;

  return (
    <div className="glass-card flex flex-col gap-3 rounded-xl border border-brand-200 bg-white/90 p-4 shadow-sm transition-all duration-300 hover:shadow-md dark:border-brand-700 dark:bg-brand-800/80">
      <div className="flex items-start justify-between gap-2">
        <div className="flex-1 min-w-0">
          <p className="truncate text-sm font-semibold text-brand-900 dark:text-white" title={task.request.title}>
            {task.request.title || t("downloads.unknownTitle", "未知标题")}
          </p>
          <p className="mt-0.5 truncate text-xs text-brand-500 dark:text-brand-400">
            {task.request.download_source || t("downloads.unknownSource", "未知来源")}
          </p>
        </div>
        <div className="shrink-0 flex items-center gap-2">
          <StatusBadge status={task.status} />
          <div className="flex h-10 items-center overflow-hidden rounded-xl border border-brand-300 bg-brand-100/90 shadow-sm dark:border-brand-600 dark:bg-brand-700/70">
            <button
              type="button"
              title={t("downloads.copyURL", "复制下载地址")}
              onClick={() => onCopyURL(task.request.url)}
              className="flex h-10 w-10 items-center justify-center text-brand-700 transition-colors hover:bg-brand-200 hover:text-brand-900 dark:text-brand-200 dark:hover:bg-brand-600 dark:hover:text-white"
            >
              <span className="i-mdi-link text-xl" />
            </button>
            <button
              type="button"
              title={t("downloads.openFolder", "打开所在文件夹")}
              onClick={() => onOpenFolder(task.id)}
              disabled={!canOpenFolder}
              className="border-l border-brand-300 flex h-10 w-10 items-center justify-center text-brand-700 transition-colors hover:bg-brand-200 hover:text-brand-900 disabled:cursor-not-allowed disabled:opacity-40 dark:border-brand-600 dark:text-brand-200 dark:hover:bg-brand-600 dark:hover:text-white"
            >
              <span className="i-mdi-folder-open-outline text-xl" />
            </button>
            {isActive
              ? (
                  <button
                    type="button"
                    title={t("downloads.cancel", "取消下载")}
                    onClick={() => onCancel(task.id)}
                    className="border-l border-brand-300 flex h-10 w-10 items-center justify-center text-error-500 transition-colors hover:bg-error-100 dark:border-brand-600 dark:hover:bg-error-900/40"
                  >
                    <span className="i-mdi-close text-xl" />
                  </button>
                )
              : (
                  <button
                    type="button"
                    title={t("downloads.delete", "删除记录")}
                    onClick={() => onDelete(task.id)}
                    className="border-l border-brand-300 flex h-10 w-10 items-center justify-center text-error-500 transition-colors hover:bg-error-100 dark:border-brand-600 dark:hover:bg-error-900/40"
                  >
                    <span className="i-mdi-delete text-xl" />
                  </button>
                )}
          </div>
        </div>
      </div>

      {(task.status === "downloading" || task.status === "pending") && (
        <div className="space-y-1">
          <div className="h-2 overflow-hidden rounded-full bg-brand-200 dark:bg-brand-700">
            <div
              className="h-full rounded-full bg-info-500 transition-all duration-300"
              style={{ width: `${progress}%` }}
            />
          </div>
          <div className="flex justify-between text-xs text-brand-500 dark:text-brand-400">
            <span>
              {progress.toFixed(1)}
              %
            </span>
            <span>
              {formatFileSize(task.downloaded)}
              {task.total > 0 ? ` / ${formatFileSize(task.total)}` : ""}
            </span>
          </div>
        </div>
      )}

      {task.status === "done" && task.file_path && (
        <>
          <div className="flex items-center gap-1 text-xs text-brand-500 dark:text-brand-400">
            <span className="i-mdi-folder-check shrink-0" />
            <span className="truncate" title={task.file_path}>{task.file_path}</span>
          </div>
          <div className="flex items-center justify-end">
            <button
              type="button"
              onClick={() => onImportAsGame(task.id)}
              disabled={importing || imported}
              className="inline-flex items-center gap-1 rounded-lg border border-neutral-200 bg-neutral-50 px-3 py-1.5 text-xs font-medium text-neutral-700 transition-colors hover:bg-neutral-100 disabled:cursor-not-allowed disabled:opacity-60 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-200 dark:hover:bg-neutral-700"
            >
              <span className={importing ? "i-mdi-loading animate-spin" : imported ? "i-mdi-check-circle-outline" : "i-mdi-gamepad-variant-outline"} />
              {imported
                ? t("downloads.imported", "已导入")
                : importing
                  ? t("downloads.importing", "导入中...")
                  : t("downloads.importAsGame", "导入为游戏")}
            </button>
          </div>
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
