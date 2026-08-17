import { Browser } from "@wailsio/runtime";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { DownloadAndApplyUpdate } from "../../../bindings/lunabox/internal/service/updateservice";
import { onWailsEvent } from "../../bindings/runtime";
import { useAppStore } from "../../store";
import { formatFileSize } from "../../utils/size";
import { ModalPortal } from "./ModalPortal";

interface UpdateInfo {
  has_update: boolean;
  current_ver: string;
  latest_ver: string;
  release_date: string;
  changelog: string[];
  downloads: Record<string, string | undefined>;
  update_manifest_url: string;
}

interface UpdateProgress {
  phase: "downloading" | "preparing" | "fallback" | "ready";
  file?: string;
  downloaded: number;
  total: number;
  percent: number;
  fallback: boolean;
}

interface UpdateDialogProps {
  updateInfo: UpdateInfo | null;
  onClose: () => void;
  onSkip: (version: string) => void;
}

interface DownloadSample {
  downloaded: number;
  timestamp: number;
}

export function UpdateDialog({
  updateInfo,
  onClose,
  onSkip,
}: UpdateDialogProps) {
  const { t } = useTranslation();
  const platformGOOS = useAppStore(state => state.platformGOOS);
  const [isVisible, setIsVisible] = useState(false);
  const [isUpdating, setIsUpdating] = useState(false);
  const [progress, setProgress] = useState<UpdateProgress | null>(null);
  const [downloadSpeed, setDownloadSpeed] = useState(0);
  const [updateError, setUpdateError] = useState("");
  const downloadSampleRef = useRef<DownloadSample | null>(null);

  useEffect(() => {
    if (updateInfo?.has_update) {
      setTimeout(() => setIsVisible(true), 100);
    }
  }, [updateInfo]);

  useEffect(() => {
    return onWailsEvent("update:progress", (value: UpdateProgress) => {
      if (value.phase === "downloading") {
        const timestamp = performance.now();
        const previousSample = downloadSampleRef.current;
        if (
          previousSample
          && value.downloaded >= previousSample.downloaded
          && timestamp > previousSample.timestamp
        ) {
          const elapsedSeconds = (timestamp - previousSample.timestamp) / 1000;
          setDownloadSpeed(
            (value.downloaded - previousSample.downloaded) / elapsedSeconds,
          );
        }
        else {
          setDownloadSpeed(0);
        }
        downloadSampleRef.current = {
          downloaded: value.downloaded,
          timestamp,
        };
      }
      else {
        downloadSampleRef.current = null;
        setDownloadSpeed(0);
      }
      setProgress(value);
    });
  }, []);

  if (!updateInfo?.has_update) {
    return null;
  }

  const supportsInAppUpdate = platformGOOS === "windows";
  const usesManualUpdate
    = platformGOOS === "darwin" || platformGOOS === "linux";
  const showGitHubDownload
    = usesManualUpdate && Boolean(updateInfo.downloads.github);
  const showGiteeDownload
    = usesManualUpdate && Boolean(updateInfo.downloads.gitee);

  const handleClose = () => {
    if (isUpdating) {
      return;
    }
    setIsVisible(false);
    setTimeout(onClose, 200);
  };

  const handleSkip = () => {
    onSkip(updateInfo.latest_ver);
    handleClose();
  };

  const handleDownload = (source: string) => {
    const url = updateInfo.downloads[source];
    if (url) {
      void Browser.OpenURL(url);
    }
  };

  const handleInAppUpdate = async () => {
    if (!supportsInAppUpdate || !updateInfo.update_manifest_url || isUpdating) {
      return;
    }
    setIsUpdating(true);
    setUpdateError("");
    setDownloadSpeed(0);
    downloadSampleRef.current = null;
    setProgress({
      phase: "downloading",
      downloaded: 0,
      total: 0,
      percent: 0,
      fallback: false,
    });
    try {
      const result = await DownloadAndApplyUpdate(
        updateInfo.update_manifest_url,
      );
      if (!result?.started) {
        throw new Error(t("updateDialog.noFilesToUpdate"));
      }
    }
    catch (error) {
      setIsUpdating(false);
      setUpdateError(error instanceof Error ? error.message : String(error));
    }
  };

  const progressText = (() => {
    switch (progress?.phase) {
      case "preparing":
        return t("updateDialog.preparing");
      case "fallback":
        return t("updateDialog.fallback");
      case "ready":
        return t("updateDialog.restarting");
      default:
        return t("updateDialog.downloading");
    }
  })();

  return (
    <ModalPortal>
      <div
        className={`absolute inset-0 z-50 flex items-center justify-center transition-all duration-200 ${isVisible ? "opacity-100" : "opacity-0"}`}
        onClick={handleClose}
      >
        {/* Background overlay */}
        <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" />

        {/* Dialog */}
        <div
          className={`relative bg-white dark:bg-brand-800 rounded-xl shadow-2xl border border-brand-200 dark:border-brand-700 max-w-md w-full mx-4 transition-all duration-200 ${isVisible ? "scale-100" : "scale-95"}`}
          onClick={e => e.stopPropagation()}
        >
          {/* Header */}
          <div className="flex items-start gap-4 p-6 pb-4">
            <div className="flex-shrink-0 w-12 h-12 bg-accent-100 dark:bg-accent-900/30 rounded-full flex items-center justify-center">
              <span className="i-mdi-download-circle text-3xl text-accent-600 dark:text-accent-400" />
            </div>
            <div className="flex-1 min-w-0">
              <h2 className="text-xl font-bold text-brand-900 dark:text-white mb-1">
                {t("updateDialog.newVersion")}
              </h2>
              <p className="text-sm text-brand-600 dark:text-brand-400">
                {t("updateDialog.available")}
              </p>
            </div>
            <button
              type="button"
              onClick={handleClose}
              disabled={isUpdating}
              aria-label={t("updateDialog.close")}
              className="flex-shrink-0 w-8 h-8 flex items-center justify-center rounded-lg hover:bg-brand-100 dark:hover:bg-brand-700 transition-colors text-brand-500 dark:text-brand-400"
            >
              <span className="i-mdi-close text-xl" />
            </button>
          </div>

          {/* Content */}
          <div className="px-6 pb-6 space-y-4">
            {/* Version Info */}
            <div className="p-4 bg-brand-50 dark:bg-brand-900/50 rounded-lg space-y-2">
              <div className="flex justify-between items-center text-sm">
                <span className="text-brand-600 dark:text-brand-400">
                  {t("updateDialog.currentVersion")}
                </span>
                <span className="font-mono font-medium text-brand-900 dark:text-white">
                  v
                  {updateInfo.current_ver}
                </span>
              </div>
              <div className="flex justify-between items-center text-sm">
                <span className="text-brand-600 dark:text-brand-400">
                  {t("updateDialog.latestVersion")}
                </span>
                <span className="font-mono font-semibold text-accent-600 dark:text-accent-400">
                  v
                  {updateInfo.latest_ver}
                </span>
              </div>
              <div className="flex justify-between items-center text-sm">
                <span className="text-brand-600 dark:text-brand-400">
                  {t("updateDialog.releaseDate")}
                </span>
                <span className="text-brand-900 dark:text-white">
                  {updateInfo.release_date}
                </span>
              </div>
            </div>

            {/* Changelog */}
            <div className="max-h-64 overflow-y-auto p-4 bg-brand-50 dark:bg-brand-900/50 rounded-lg">
              <h3 className="text-sm font-semibold text-brand-900 dark:text-white mb-2">
                {t("updateDialog.changelog")}
              </h3>
              <div className="text-xs text-brand-700 dark:text-brand-300 space-y-1 whitespace-pre-wrap">
                {updateInfo.changelog.map((line, index) => (
                  <div key={index}>{line}</div>
                ))}
              </div>
            </div>

            {/* Actions */}
            <div className="space-y-2">
              {isUpdating && progress && (
                <div className="space-y-2 px-0.5 py-1">
                  <div className="flex items-center justify-between gap-3 text-xs text-brand-600 dark:text-brand-300">
                    <span className="truncate">{progressText}</span>
                    {progress.phase === "downloading" && (
                      <span className="shrink-0 font-mono font-medium text-brand-800 dark:text-brand-100">
                        {progress.percent}
                        %
                      </span>
                    )}
                  </div>
                  <div className="h-1.5 overflow-hidden rounded-full bg-brand-200 dark:bg-brand-700">
                    <div
                      className={`h-full rounded-full bg-accent-500 transition-all duration-300 ${progress.phase !== "downloading" ? "animate-pulse" : ""}`}
                      style={{
                        width: `${progress.phase === "downloading" ? Math.max(2, progress.percent) : 100}%`,
                      }}
                    />
                  </div>
                  <div className="flex items-center justify-between gap-3 text-xs text-brand-500 dark:text-brand-400">
                    <span className="min-w-0 truncate">
                      {progress.file ?? ""}
                    </span>
                    {progress.phase === "downloading" && (
                      <span className="shrink-0 tabular-nums">
                        {t("updateDialog.currentSpeed", {
                          speed: `${formatFileSize(downloadSpeed)}/s`,
                        })}
                      </span>
                    )}
                  </div>
                </div>
              )}

              {supportsInAppUpdate && updateInfo.update_manifest_url && (
                <button
                  type="button"
                  onClick={handleInAppUpdate}
                  disabled={isUpdating}
                  aria-busy={isUpdating}
                  className="w-full px-4 py-2.5 text-sm font-medium text-white bg-accent-600 hover:bg-accent-700 disabled:cursor-wait disabled:opacity-70 rounded-lg transition-colors flex items-center justify-center gap-2"
                >
                  <span
                    className={`${isUpdating ? "i-mdi-loading animate-spin" : "i-mdi-update"} text-lg`}
                  />
                  {isUpdating
                    ? t("updateDialog.downloadingButton")
                    : t("updateDialog.updateNow")}
                </button>
              )}

              {updateError && (
                <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-xs text-red-700 dark:border-red-700 dark:bg-red-900/20 dark:text-red-300">
                  <p className="font-medium">
                    {t("updateDialog.updateFailed")}
                  </p>
                  <p className="mt-1 break-words">{updateError}</p>
                  {(showGitHubDownload || showGiteeDownload) && (
                    <p className="mt-1">{t("updateDialog.manualFallback")}</p>
                  )}
                </div>
              )}

              {(showGitHubDownload || showGiteeDownload) && (
                <div
                  className={`grid gap-2 ${showGitHubDownload && showGiteeDownload ? "grid-cols-2" : "grid-cols-1"}`}
                >
                  {showGitHubDownload && (
                    <button
                      type="button"
                      onClick={() => handleDownload("github")}
                      disabled={isUpdating}
                      className="w-full px-4 py-2.5 text-sm font-medium text-white bg-brand-700 hover:bg-brand-800 dark:bg-brand-200 dark:hover:bg-white dark:text-brand-900 rounded-lg transition-colors flex items-center justify-center gap-2"
                    >
                      <span className="i-mdi-github text-lg" />
                      {t("updateDialog.githubDownload")}
                    </button>
                  )}
                  {showGiteeDownload && (
                    <button
                      type="button"
                      onClick={() => handleDownload("gitee")}
                      disabled={isUpdating}
                      className="w-full px-4 py-2.5 text-sm font-medium text-white bg-neutral-600 hover:bg-neutral-700 rounded-lg transition-colors flex items-center justify-center gap-2"
                    >
                      <span className="i-mdi-cloud-download text-lg" />
                      {t("updateDialog.giteeDownload")}
                    </button>
                  )}
                </div>
              )}
              <button
                type="button"
                onClick={handleSkip}
                disabled={isUpdating}
                className="w-full px-4 py-2.5 text-sm font-medium text-brand-600 dark:text-brand-400 hover:bg-brand-100 dark:hover:bg-brand-700 border border-brand-300 dark:border-brand-600 rounded-lg transition-colors"
              >
                {t("updateDialog.skipVersion")}
              </button>
            </div>
          </div>
        </div>
      </div>
    </ModalPortal>
  );
}
