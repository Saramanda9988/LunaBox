import type { appconf } from "../../../wailsjs/go/models";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { SelectDirectory } from "../../../wailsjs/go/service/ConfigService";
import { BetterButton } from "../ui/BetterButton";

interface DownloadSettingsPanelProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

export function DownloadSettingsPanel({ formData, onChange }: DownloadSettingsPanelProps) {
  const { t } = useTranslation();

  const handleSelectDownloadDir = async () => {
    try {
      const path = await SelectDirectory(t("settings.download.selectDownloadDirTitle", "选择默认下载目录"));
      if (path) {
        onChange({ ...formData, download_dir: path } as appconf.AppConfig);
      }
    }
    catch (error) {
      console.error("Failed to select download dir:", error);
      toast.error(t("settings.download.toast.selectFailed", "选择目录失败"));
    }
  };

  const handleClearDownloadDir = () => {
    onChange({ ...formData, download_dir: "" } as appconf.AppConfig);
  };

  const handleSelectGameLibraryDir = async () => {
    try {
      const path = await SelectDirectory(t("settings.download.selectGameLibraryTitle", "选择游戏库目录"));
      if (path) {
        onChange({ ...formData, game_library_dir: path } as appconf.AppConfig);
      }
    }
    catch (error) {
      console.error("Failed to select game library dir:", error);
      toast.error(t("settings.download.toast.selectFailed", "选择目录失败"));
    }
  };

  const handleClearGameLibraryDir = () => {
    onChange({ ...formData, game_library_dir: "" } as appconf.AppConfig);
  };

  return (
    <>
      {/* 默认下载目录 */}
      <div className="p-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
          {t("settings.download.downloadDir", "默认下载目录")}
        </label>
        <p className="text-xs text-brand-500 dark:text-brand-400 mb-2">
          {t("settings.download.downloadDirHint", "通过 lunabox:// 协议触发的下载文件将保存到此目录。留空则使用 ~/Downloads/LunaBox")}
        </p>
        <div className="flex gap-2">
          <input
            type="text"
            value={formData.download_dir || ""}
            onChange={e => onChange({ ...formData, download_dir: e.target.value } as appconf.AppConfig)}
            placeholder={t("settings.download.downloadDirPlaceholder", "留空使用 ~/Downloads/LunaBox")}
            className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none text-sm"
          />
          <BetterButton onClick={handleSelectDownloadDir} icon="i-mdi-folder-open">
            {t("settings.game.selectBtn", "选择")}
          </BetterButton>
          {formData.download_dir && (
            <BetterButton onClick={handleClearDownloadDir} icon="i-mdi-close">
              {t("common.clear", "清除")}
            </BetterButton>
          )}
        </div>

        {/* 当前生效路径提示 */}
        <p className="mt-1.5 text-xs text-brand-400 dark:text-brand-500 flex items-center gap-1">
          <span className="i-mdi-information-outline" />
          {formData.download_dir
            ? t("settings.download.effectivePath", "下载路径：{{path}}", { path: formData.download_dir })
            : t("settings.download.defaultPath", "下载路径：~/Downloads/LunaBox（默认）")}
        </p>
      </div>

      {/* 游戏库目录 */}
      <div className="mt-6 border-t border-brand-200 dark:border-brand-700 pt-6 p-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
          {t("settings.download.gameLibraryDir", "游戏库目录")}
        </label>
        <p className="text-xs text-brand-500 dark:text-brand-400 mb-2">
          {t("settings.download.gameLibraryDirHint", "你的游戏安装根目录（仅作信息展示与快速导航用）")}
        </p>
        <div className="flex gap-2">
          <input
            type="text"
            value={formData.game_library_dir || ""}
            onChange={e => onChange({ ...formData, game_library_dir: e.target.value } as appconf.AppConfig)}
            placeholder={t("settings.download.gameLibraryDirPlaceholder", "例如 D:\\Games")}
            className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none text-sm"
          />
          <BetterButton onClick={handleSelectGameLibraryDir} icon="i-mdi-folder-open">
            {t("settings.game.selectBtn", "选择")}
          </BetterButton>
          {formData.game_library_dir && (
            <BetterButton onClick={handleClearGameLibraryDir} icon="i-mdi-close">
              {t("common.clear", "清除")}
            </BetterButton>
          )}
        </div>
      </div>
    </>
  );
}
