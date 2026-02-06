import { useEffect, useState } from "react";
import { BrowserOpenURL } from "../../../wailsjs/runtime/runtime";

interface UpdateInfo {
  has_update: boolean;
  current_ver: string;
  latest_ver: string;
  release_date: string;
  changelog: string[];
  downloads: Record<string, string>;
}

interface UpdateDialogProps {
  updateInfo: UpdateInfo | null;
  onClose: () => void;
  onSkip: (version: string) => void;
}

export function UpdateDialog({ updateInfo, onClose, onSkip }: UpdateDialogProps) {
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    if (updateInfo?.has_update) {
      // 延迟显示，添加动画效果
      setTimeout(() => setIsVisible(true), 100);
    }
  }, [updateInfo]);

  if (!updateInfo?.has_update) {
    return null;
  }

  const handleClose = () => {
    setIsVisible(false);
    setTimeout(onClose, 200);
  };

  const handleSkip = () => {
    // 直接传递版本号，后端会统一处理格式
    onSkip(updateInfo.latest_ver);
    handleClose();
  };

  const handleDownload = (source: string) => {
    const url = updateInfo.downloads[source];
    if (url) {
      BrowserOpenURL(url);
    }
  };

  return (
    <div
      className={`fixed inset-0 z-50 flex items-center justify-center transition-all duration-200 ${isVisible ? "opacity-100" : "opacity-0"}`}
      onClick={handleClose}
    >
      {/* 背景遮罩 */}
      <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" />

      {/* 对话框 */}
      <div
        className={`glass-card relative bg-white dark:bg-brand-800 rounded-xl shadow-2xl border border-brand-200 dark:border-brand-700 max-w-md w-full mx-4 transition-all duration-200 ${isVisible ? "scale-100" : "scale-95"}`}
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-start gap-4 p-6 pb-4">
          <div className="flex-shrink-0 w-12 h-12 bg-accent-100 dark:bg-accent-900/30 rounded-full flex items-center justify-center">
            <span className="i-mdi-download-circle text-3xl text-accent-600 dark:text-accent-400" />
          </div>
          <div className="flex-1 min-w-0">
            <h2 className="text-xl font-bold text-brand-900 dark:text-white mb-1">
              发现新版本
            </h2>
            <p className="text-sm text-brand-600 dark:text-brand-400">
              LunaBox 有可用的更新
            </p>
          </div>
          <button
            type="button"
            onClick={handleClose}
            className="flex-shrink-0 w-8 h-8 flex items-center justify-center rounded-lg hover:bg-brand-100 dark:hover:bg-brand-700 transition-colors text-brand-500 dark:text-brand-400"
          >
            <span className="i-mdi-close text-xl" />
          </button>
        </div>

        {/* Content */}
        <div className="px-6 pb-6 space-y-4">
          {/* Version Info */}
          <div className="glass-card p-4 bg-brand-50 dark:bg-brand-900/50 rounded-lg space-y-2">
            <div className="flex justify-between items-center text-sm">
              <span className="text-brand-600 dark:text-brand-400">当前版本</span>
              <span className="font-mono font-medium text-brand-900 dark:text-white">
                v
                {updateInfo.current_ver}
              </span>
            </div>
            <div className="flex justify-between items-center text-sm">
              <span className="text-brand-600 dark:text-brand-400">最新版本</span>
              <span className="font-mono font-semibold text-accent-600 dark:text-accent-400">
                v
                {updateInfo.latest_ver}
              </span>
            </div>
            <div className="flex justify-between items-center text-sm">
              <span className="text-brand-600 dark:text-brand-400">发布日期</span>
              <span className="text-brand-900 dark:text-white">
                {updateInfo.release_date}
              </span>
            </div>
          </div>

          {/* Changelog */}
          <div className="max-h-64 overflow-y-auto p-4 bg-brand-50 dark:bg-brand-900/50 rounded-lg">
            <h3 className="text-sm font-semibold text-brand-900 dark:text-white mb-2">
              更新内容
            </h3>
            <div className="text-xs text-brand-700 dark:text-brand-300 space-y-1 whitespace-pre-wrap">
              {updateInfo.changelog.map((line, index) => (
                <div key={index}>{line}</div>
              ))}
            </div>
          </div>

          {/* Actions */}
          <div className="space-y-2">
            <div className="flex gap-2">
              {updateInfo.downloads.github && (
                <button
                  type="button"
                  onClick={() => handleDownload("github")}
                  className="glass-btn-neutral flex-1 px-4 py-2.5 text-sm font-medium text-white bg-neutral-600 hover:bg-neutral-700 rounded-lg transition-colors flex items-center justify-center gap-2"
                >
                  <span className="i-mdi-github text-lg" />
                  GitHub 下载
                </button>
              )}
              {updateInfo.downloads.gitee && (
                <button
                  type="button"
                  onClick={() => handleDownload("gitee")}
                  className="glass-btn-neutral flex-1 px-4 py-2.5 text-sm font-medium text-white bg-neutral-600 hover:bg-neutral-700 rounded-lg transition-colors flex items-center justify-center gap-2"
                >
                  <span className="i-mdi-cloud-download text-lg" />
                  Gitee 下载
                </button>
              )}
            </div>
            <button
              type="button"
              onClick={handleSkip}
              className="w-full px-4 py-2.5 text-sm font-medium text-brand-600 dark:text-brand-400 hover:bg-brand-100 dark:hover:bg-brand-700 border border-brand-300 dark:border-brand-600 rounded-lg transition-colors"
            >
              跳过此版本
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
