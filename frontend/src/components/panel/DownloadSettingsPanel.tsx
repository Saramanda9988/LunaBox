import type { appconf } from "../../../wailsjs/go/models";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { SelectDirectory } from "../../../wailsjs/go/service/ConfigService";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterSelect } from "../ui/better/BetterSelect";

interface GameLibrarySettingsPanelProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

export function DownloadSettingsPanel({ formData, onChange }: GameLibrarySettingsPanelProps) {
  const { t } = useTranslation();
  const proxyMode = formData.download_proxy_mode || "system";

  const handleSelectGameLibraryPath = async () => {
    try {
      const path = await SelectDirectory(t("settings.download.selectGameLibraryTitle", "选择游戏库目录"));
      if (path) {
        onChange({ ...formData, game_library_path: path } as appconf.AppConfig);
      }
    }
    catch (error) {
      console.error("Failed to select game library path:", error);
      toast.error(t("settings.download.toast.selectFailed", "选择目录失败"));
    }
  };

  const handleClearGameLibraryPath = () => {
    onChange({ ...formData, game_library_path: "" } as appconf.AppConfig);
  };

  return (
    <div className="space-y-4">
      {/* 下载代理 */}
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.download.proxyMode", "下载代理")}
        </label>
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t(
            "settings.download.proxyModeHint",
            "默认自动跟随系统代理；如果下载仍未走代理，可切换为手动并填写代理 URL。",
          )}
        </p>
        <BetterSelect
          value={proxyMode}
          onChange={val => onChange({ ...formData, download_proxy_mode: val } as appconf.AppConfig)}
          options={[
            { value: "system", label: t("settings.download.proxyModeSystem", "自动跟随系统代理") },
            { value: "manual", label: t("settings.download.proxyModeManual", "手动代理") },
            { value: "direct", label: t("settings.download.proxyModeDirect", "直连，不使用代理") },
          ]}
        />
        <p className="text-xs text-brand-400 dark:text-brand-500">
          {t(
            "settings.download.proxyModeNote",
            "Windows 下会优先读取系统代理。PAC 或仅 TUN 的场景如果无法识别，可改为手动代理。",
          )}
        </p>
      </div>

      {proxyMode === "manual" && (
        <div className="space-y-2 px-4 py-3 bg-brand-50/50 dark:bg-brand-800/30 rounded-lg border border-brand-200 dark:border-brand-700">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("settings.download.manualProxyURL", "手动代理 URL")}
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400">
            {t(
              "settings.download.manualProxyURLHint",
              "支持 http://、https://、socks5://，也可直接填写 127.0.0.1:7890 这类地址。",
            )}
          </p>
          <input
            type="text"
            value={formData.download_proxy_url || ""}
            onChange={e => onChange({ ...formData, download_proxy_url: e.target.value } as appconf.AppConfig)}
            placeholder={t("settings.download.manualProxyURLPlaceholder", "例如 http://127.0.0.1:7890")}
            className="glass-input w-full px-3 py-2 text-sm border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
          />
        </div>
      )}

      {/* 游戏库目录 */}
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.download.gameLibraryPath", "游戏库目录")}
        </label>
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t(
            "settings.download.gameLibraryPathHint",
            "下载的游戏将解压到此目录。留空则使用 ~/Games",
          )}
        </p>
        <div className="flex gap-2">
          <input
            type="text"
            value={formData.game_library_path || ""}
            onChange={e => onChange({ ...formData, game_library_path: e.target.value } as appconf.AppConfig)}
            placeholder={t("settings.download.gameLibraryPathPlaceholder", "例如 D:\\Games 或 /home/user/games")}
            className="glass-input flex-1 px-3 py-2 text-sm border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
          />
          <BetterButton onClick={handleSelectGameLibraryPath} icon="i-mdi-folder-open" variant="secondary" />
          {formData.game_library_path && (
            <BetterButton onClick={handleClearGameLibraryPath} icon="i-mdi-close" variant="secondary" />
          )}
        </div>

        {/* 当前生效路径提示 */}
        <p className="flex items-center gap-1 text-xs text-brand-400 dark:text-brand-500">
          <span className="i-mdi-information-outline" />
          {formData.game_library_path
            ? t("settings.download.effectivePath", "游戏库路径：{{path}}", { path: formData.game_library_path })
            : t("settings.download.defaultPath", "游戏库路径：~/Games（默认）")}
        </p>
      </div>
    </div>
  );
}
