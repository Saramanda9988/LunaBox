import type { appconf } from "../../../wailsjs/go/models";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { SelectDirectory } from "../../../wailsjs/go/service/ConfigService";
import { BetterButton } from "../ui/BetterButton";

interface GameLibrarySettingsPanelProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

export function DownloadSettingsPanel({ formData, onChange }: GameLibrarySettingsPanelProps) {
  const { t } = useTranslation();

  const handleSelectGameLibraryPath = async () => {
    try {
      const path = await SelectDirectory(t("settings.game.selectGameLibraryTitle", "选择游戏库目录"));
      if (path) {
        onChange({ ...formData, game_library_path: path } as appconf.AppConfig);
      }
    }
    catch (error) {
      console.error("Failed to select game library path:", error);
      toast.error(t("settings.game.toast.selectFailed", "选择目录失败"));
    }
  };

  const handleClearGameLibraryPath = () => {
    onChange({ ...formData, game_library_path: "" } as appconf.AppConfig);
  };

  return (
    <>
      {/* 游戏库目录 */}
      <div className="p-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
          {t("settings.game.gameLibraryPath", "游戏库目录")}
        </label>
        <p className="text-xs text-brand-500 dark:text-brand-400 mb-2">
          {t(
            "settings.game.gameLibraryPathHint",
            "通过 lunabox:// 协议下载的游戏将解压到此目录。留空则使用 ~/Games"
          )}
        </p>
        <div className="flex gap-2">
          <input
            type="text"
            value={formData.game_library_path || ""}
            onChange={e => onChange({ ...formData, game_library_path: e.target.value } as appconf.AppConfig)}
            placeholder={t("settings.game.gameLibraryPathPlaceholder", "例如 D:\\Games 或 /home/user/games")}
            className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none text-sm"
          />
          <BetterButton onClick={handleSelectGameLibraryPath} icon="i-mdi-folder-open">
            {t("settings.game.selectBtn", "选择")}
          </BetterButton>
          {formData.game_library_path && (
            <BetterButton onClick={handleClearGameLibraryPath} icon="i-mdi-close">
              {t("common.clear", "清除")}
            </BetterButton>
          )}
        </div>

        {/* 当前生效路径提示 */}
        <p className="mt-1.5 text-xs text-brand-400 dark:text-brand-500 flex items-center gap-1">
          <span className="i-mdi-information-outline" />
          {formData.game_library_path
            ? t("settings.game.effectivePath", "游戏库路径：{{path}}", { path: formData.game_library_path })
            : t("settings.game.defaultPath", "游戏库路径：~/Games（默认）")}
        </p>
      </div>
    </>
  );
}
