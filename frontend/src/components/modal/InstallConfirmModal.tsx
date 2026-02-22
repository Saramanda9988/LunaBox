import { useState } from "react";
import { useTranslation } from "react-i18next";
import { StartDownload } from "../../../wailsjs/go/service/DownloadService";

interface InstallRequest {
  url: string;
  title: string;
  download_source: string;
  meta_source: string;
  meta_id: string;
  size: number;
}

const META_SOURCE_LABELS: Record<string, string> = {
  vndb: "VNDB",
  bangumi: "Bangumi",
  ymgal: "月幕Galgame",
};

function metaUrl(source: string, id: string): string {
  switch (source) {
    case "vndb": return `https://vndb.org/${id}`;
    case "bangumi": return `https://bgm.tv/subject/${id}`;
    case "ymgal": return `https://www.ymgal.games/ga/${id}`;
    default: return "";
  }
}

interface InstallConfirmModalProps {
  request: InstallRequest | null;
  onClose: () => void;
}

function formatSize(bytes: number): string {
  if (!bytes || bytes <= 0)
    return "";
  if (bytes >= 1024 ** 3)
    return `${(bytes / 1024 ** 3).toFixed(2)} GB`;
  if (bytes >= 1024 ** 2)
    return `${(bytes / 1024 ** 2).toFixed(1)} MB`;
  return `${(bytes / 1024).toFixed(0)} KB`;
}

export function InstallConfirmModal({ request, onClose }: InstallConfirmModalProps) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);

  if (!request)
    return null;

  const handleConfirm = async () => {
    setLoading(true);
    try {
      await StartDownload(request);
    }
    finally {
      setLoading(false);
      onClose();
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div className="w-full max-w-md rounded-xl bg-white dark:bg-brand-800 border border-brand-200 dark:border-brand-700 shadow-2xl overflow-hidden">
        {/* Header */}
        <div className="flex items-center gap-3 px-6 pt-6 pb-4">
          <div className="p-2.5 rounded-xl bg-primary-100 dark:bg-primary-900/30 text-primary-600 dark:text-primary-400 shrink-0">
            <div className="i-mdi-download-circle text-2xl" />
          </div>
          <div className="min-w-0">
            <h3 className="text-lg font-bold text-brand-900 dark:text-white leading-tight">
              {t("installModal.title", "安装游戏")}
            </h3>
            <p className="text-sm text-brand-500 dark:text-brand-400 mt-0.5">
              {t("installModal.subtitle", "来自外部链接的安装请求")}
            </p>
          </div>
        </div>

        {/* Game info */}
        <div className="mx-6 mb-5 rounded-lg bg-brand-50 dark:bg-brand-900/50 border border-brand-200 dark:border-brand-700 p-4 space-y-2">
          <div className="flex items-start gap-2">
            <span className="text-xs text-brand-500 dark:text-brand-400 w-14 shrink-0 pt-0.5">
              {t("installModal.name", "游戏名")}
            </span>
            <span className="text-sm font-semibold text-brand-900 dark:text-white break-all leading-snug">
              {request.title || t("installModal.unknown", "未知")}
            </span>
          </div>

          {request.download_source && (
            <div className="flex items-center gap-2">
              <span className="text-xs text-brand-500 dark:text-brand-400 w-14 shrink-0">
                {t("installModal.source", "来源")}
              </span>
              <span className="text-sm font-medium text-primary-600 dark:text-primary-400">
                {request.download_source}
              </span>
            </div>
          )}

          {request.meta_source && request.meta_id && (() => {
            const label = META_SOURCE_LABELS[request.meta_source] ?? request.meta_source;
            const href = metaUrl(request.meta_source, request.meta_id);
            return (
              <div className="flex items-center gap-2">
                <span className="text-xs text-brand-500 dark:text-brand-400 w-14 shrink-0">{label}</span>
                {href
                  ? (
                      <a
                        href={href}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-sm text-primary-600 dark:text-primary-400 hover:underline font-mono"
                      >
                        {request.meta_id}
                      </a>
                    )
                  : <span className="text-sm text-brand-700 dark:text-brand-300 font-mono">{request.meta_id}</span>}
              </div>
            );
          })()}

          {request.size > 0 && (
            <div className="flex items-center gap-2">
              <span className="text-xs text-brand-500 dark:text-brand-400 w-14 shrink-0">
                {t("installModal.size", "大小")}
              </span>
              <span className="text-sm text-brand-700 dark:text-brand-300 font-mono">
                {formatSize(request.size)}
              </span>
            </div>
          )}
        </div>

        {/* Warning */}
        <div className="mx-6 mb-5 flex items-start gap-2 text-xs text-amber-700 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800/50 rounded-lg px-3 py-2.5">
          <div className="i-mdi-alert-outline mt-0.5 shrink-0" />
          <span>{t("installModal.warning", "请确认来源可信后再继续。下载完成后需手动配置启动路径。")}</span>
        </div>

        {/* Actions */}
        <div className="flex justify-end gap-3 px-6 pb-6">
          <button
            type="button"
            onClick={onClose}
            disabled={loading}
            className="px-4 py-2 text-sm rounded-lg border border-brand-300 dark:border-brand-600 text-brand-700 dark:text-brand-300 hover:bg-brand-100 dark:hover:bg-brand-700 transition-colors disabled:opacity-50"
          >
            {t("common.cancel", "取消")}
          </button>
          <button
            type="button"
            onClick={handleConfirm}
            disabled={loading}
            className="px-4 py-2 text-sm rounded-lg bg-primary-600 hover:bg-primary-700 text-white font-medium transition-colors disabled:opacity-50 flex items-center gap-2"
          >
            {loading && <div className="i-mdi-loading animate-spin" />}
            {t("installModal.confirm", "确认下载")}
          </button>
        </div>
      </div>
    </div>
  );
}
