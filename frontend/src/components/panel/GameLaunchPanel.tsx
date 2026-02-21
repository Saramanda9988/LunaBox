import type { appconf, models } from "../../../wailsjs/go/models";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { BetterButton } from "../ui/BetterButton";
import { BetterSwitch } from "../ui/BetterSwitch";

interface GameLaunchPanelProps {
  game: models.Game;
  config?: appconf.AppConfig;
  onGameChange: (game: models.Game) => void;
  onSelectProcessExecutable: () => void;
}

export function GameLaunchPanel({ game, config, onGameChange, onSelectProcessExecutable }: GameLaunchPanelProps) {
  const { t } = useTranslation();
  const hasLocaleEmulatorPath = config?.locale_emulator_path && config?.locale_emulator_path.length > 0;
  const hasMagpiePath = config?.magpie_path && config?.magpie_path.length > 0;

  const handleLocaleEmulatorToggle = (checked: boolean) => {
    if (checked && !hasLocaleEmulatorPath) {
      toast.error(t("gameLaunch.toast.lePathRequired"));
      return;
    }
    onGameChange({ ...game, use_locale_emulator: checked } as models.Game);
  };

  const handleMagpieToggle = (checked: boolean) => {
    if (checked && !hasMagpiePath) {
      toast.error(t("gameLaunch.toast.magpiePathRequired"));
      return;
    }
    onGameChange({ ...game, use_magpie: checked } as models.Game);
  };

  return (
    <div className="glass-panel mx-auto bg-white dark:bg-brand-800 p-8 rounded-lg shadow-sm">
      <div className="space-y-8">
        {/* 进程监控配置 */}
        <div className="flex items-center gap-2">
          <div className="i-mdi-monitor text-xl text-brand-600 dark:text-brand-400" />
          <h3 className="text-sm font-semibold text-brand-900 dark:text-white">{t("gameLaunch.processMonitor")}</h3>
        </div>

        <div className="rounded-lg space-y-4">
          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-2">
              {t("gameLaunch.executable")}
            </label>
            <div className="text-sm text-neutral-600 dark:text-neutral-400 font-mono bg-white dark:bg-brand-800 px-3 py-1.5 rounded border border-brand-200 dark:border-brand-700">
              {game.path ? game.path.split(/[\\/]/).pop() : t("gameLaunch.noPathSet")}
            </div>
            <p className="mt-1 text-xs text-brand-400">
              {t("gameLaunch.executableHint")}
            </p>
          </div>

          <div className="border-t border-brand-200 dark:border-brand-700 my-4" />

          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-2">
              {t("gameLaunch.actualProcess")}
            </label>
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={game.process_name || ""}
                onChange={e => onGameChange({ ...game, process_name: e.target.value } as models.Game)}
                className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none font-mono"
              />
              <BetterButton
                onClick={onSelectProcessExecutable}
                icon="i-mdi-file"
                title={t("gameLaunch.selectProcessFile")}
              />
            </div>
            <p className="mt-2 text-xs text-brand-400 leading-relaxed whitespace-pre-line">
              {t("gameLaunch.processHint")}
            </p>
          </div>
        </div>

        {/* 启动工具配置 */}
        <div>
          <div className="flex items-center gap-2 mb-4">
            <div className="i-mdi-tools text-xl text-brand-600 dark:text-brand-400" />
            <h3 className="text-sm font-semibold text-brand-900 dark:text-white">{t("gameLaunch.enhancementTools")}</h3>
          </div>

          <div className="flex items-center justify-between p-4">
            <div className="flex-1 mr-4">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-brand-700 dark:text-brand-300">Locale Emulator</span>
                <span className="px-1.5 py-0.5 text-[10px] font-medium bg-brand-200 dark:bg-brand-600 text-brand-800 dark:text-brand-100 rounded">{t("gameLaunch.leLabel")}</span>
              </div>
              <p className="text-xs text-neutral-500 dark:text-neutral-400 mt-1">
                {t("gameLaunch.leDesc")}
              </p>
              {!hasLocaleEmulatorPath && (
                <p className="text-xs text-error-500 mt-1 flex items-center gap-1">
                  <div className="i-mdi-alert-circle text-sm" />
                  {" "}
                  {t("gameLaunch.leNotConfigured")}
                </p>
              )}
            </div>
            <BetterSwitch
              id="use_locale_emulator"
              checked={game.use_locale_emulator || false}
              onCheckedChange={handleLocaleEmulatorToggle}
              disabled={!hasLocaleEmulatorPath}
            />
          </div>

          <div className="flex items-center justify-between p-4">
            <div className="flex-1 mr-4">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-brand-700 dark:text-brand-300">Magpie</span>
                <span className="px-1.5 py-0.5 text-[10px] font-medium bg-brand-200 dark:bg-brand-600 text-brand-800 dark:text-brand-100 rounded">{t("gameLaunch.magpieLabel")}</span>
              </div>
              <p className="text-xs text-neutral-500 dark:text-neutral-400 mt-1">
                {t("gameLaunch.magpieDesc")}
              </p>
              {!hasMagpiePath && (
                <p className="text-xs text-error-500 mt-1 flex items-center gap-1">
                  <div className="i-mdi-alert-circle text-sm" />
                  {" "}
                  {t("gameLaunch.magpieNotConfigured")}
                </p>
              )}
            </div>
            <BetterSwitch
              id="use_magpie"
              checked={game.use_magpie || false}
              onCheckedChange={handleMagpieToggle}
              disabled={!hasMagpiePath}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
