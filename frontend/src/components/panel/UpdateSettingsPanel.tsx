import type { appconf } from "../../../wailsjs/go/models";
import { useState } from "react";
import { toast } from "react-hot-toast";
import { CheckForUpdates, SkipVersion } from "../../../wailsjs/go/service/UpdateService";
import { BetterSwitch } from "../ui/BetterSwitch";
import { UpdateDialog } from "../ui/UpdateDialog";

interface UpdateSettingsPanelProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

interface UpdateInfo {
  has_update: boolean;
  current_ver: string;
  latest_ver: string;
  release_date: string;
  changelog: string[];
  downloads: Record<string, string>;
}

export function UpdateSettingsPanel({ formData, onChange }: UpdateSettingsPanelProps) {
  const [isChecking, setIsChecking] = useState(false);
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null);
  const [showDialog, setShowDialog] = useState(false);
  const [error, setError] = useState<string>("");

  const handleCheckUpdate = async () => {
    setIsChecking(true);
    setError("");
    setUpdateInfo(null);

    try {
      const result = await CheckForUpdates();
      if (result) {
        setUpdateInfo(result);
        if (result.has_update) {
          setShowDialog(true);
        }
      }
    }
    catch (err) {
      setError(err instanceof Error ? err.message : "检查更新失败");
    }
    finally {
      setIsChecking(false);
    }
  };

  const handleSkipVersion = async (version: string) => {
    try {
      await SkipVersion(version);
      if (updateInfo) {
        setUpdateInfo({ ...updateInfo, has_update: false });
      }
      setShowDialog(false);
    }
    catch (err) {
      const msg = `修改失败：${err}`;
      toast.error(msg);
    }
  };

  return (
    <>
      <div className="space-y-4">
        {/* 启动时自动检查更新 */}
        <div className="flex items-center justify-between">
          <div className="flex-1">
            <label htmlFor="check_update_on_startup" className="text-sm font-medium text-brand-700 dark:text-brand-300 cursor-pointer">
              启动时自动检查更新
            </label>
            <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
              应用启动时自动检查是否有新版本（每天最多检查一次）
            </p>
          </div>
          <BetterSwitch
            id="check_update_on_startup"
            checked={formData.check_update_on_startup || false}
            onCheckedChange={checked => onChange({ ...formData, check_update_on_startup: checked } as appconf.AppConfig)}
          />
        </div>

        {/* 手动检查更新按钮 */}
        <div className="pt-2">
          <button
            type="button"
            onClick={handleCheckUpdate}
            disabled={isChecking}
            className="w-full flex items-center justify-center gap-2 px-4 py-2.5 bg-neutral-600 hover:bg-neutral-700 disabled:bg-brand-300 disabled:cursor-not-allowed text-white rounded-lg transition-colors text-sm font-medium"
          >
            <span className={`i-mdi-update ${isChecking ? "animate-spin" : ""}`} />
            {isChecking ? "检查中..." : "手动检查更新"}
          </button>
        </div>

        {/* 错误信息 */}
        {error && (
          <div className="p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 rounded-lg">
            <div className="flex items-start gap-2">
              <span className="i-mdi-alert-circle text-red-600 dark:text-red-400 text-lg mt-0.5" />
              <div className="text-xs text-red-700 dark:text-red-300">
                <p className="font-medium">检查更新失败</p>
                <p className="mt-1">{error}</p>
              </div>
            </div>
          </div>
        )}

        {/* 发现新版本提示（对话框关闭但有更新时显示） */}
        {updateInfo && updateInfo.has_update && !showDialog && !error && (
          <button
            type="button"
            onClick={() => setShowDialog(true)}
            className="w-full p-3 bg-accent-50 dark:bg-accent-900/20 border border-accent-200 dark:border-accent-700 rounded-lg hover:bg-accent-100 dark:hover:bg-accent-900/30 transition-colors text-left"
          >
            <div className="flex items-center gap-2">
              <span className="i-mdi-update text-accent-600 dark:text-accent-400 text-xl" />
              <div className="flex-1 text-sm text-accent-700 dark:text-accent-300">
                <span className="font-medium">发现新版本</span>
                <span className="ml-2 font-mono font-semibold">
                  v
                  {updateInfo.latest_ver}
                </span>
              </div>
              <span className="i-mdi-chevron-right text-accent-500 dark:text-accent-400 text-lg" />
            </div>
          </button>
        )}

        {/* 已是最新版本提示 */}
        {updateInfo && !updateInfo.has_update && !error && (
          <div className="p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-700 rounded-lg">
            <div className="flex items-center gap-2">
              <span className="i-mdi-check-circle text-green-600 dark:text-green-400 text-xl" />
              <div className="text-sm text-green-700 dark:text-green-300">
                <span className="font-medium">已是最新版本</span>
                <span className="ml-2 font-mono">{updateInfo.current_ver}</span>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* 更新对话框 */}
      {showDialog && updateInfo && (
        <UpdateDialog
          updateInfo={updateInfo}
          onClose={() => setShowDialog(false)}
          onSkip={() => handleSkipVersion(updateInfo.latest_ver)}
        />
      )}
    </>
  );
}
