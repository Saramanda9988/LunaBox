import type { appconf, models } from "../../../wailsjs/go/models";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterSwitch } from "../ui/better/BetterSwitch";

interface GameLaunchPanelProps {
  game: models.Game;
  config?: appconf.AppConfig;
  onGameChange: (game: models.Game) => void;
  onSelectProcessExecutable: () => void;
  onExportShortcut: () => void;
}

export function GameLaunchPanel({
  game,
  config,
  onGameChange,
  onSelectProcessExecutable,
  onExportShortcut,
}: GameLaunchPanelProps) {
  const { t } = useTranslation();
  const hasLocaleEmulatorPath
    = config?.locale_emulator_path && config?.locale_emulator_path.length > 0;
  const hasMagpiePath = config?.magpie_path && config?.magpie_path.length > 0;
  const executableName = game.path
    ? game.path.split(/[\\/]/).pop()
    : t("gameLaunch.noPathSet");

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
    <div className="space-y-6">
      {/* Process Monitor */}
      <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
        <div className="space-y-6">
          <div className="border-brand-200 dark:border-brand-700">
            <h3 className="text-lg font-semibold text-brand-900 dark:text-white">
              {t("gameLaunch.processMonitor")}
            </h3>
            <p className="text-sm text-brand-500 dark:text-brand-400 mt-1"></p>
          </div>

          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              {t("gameLaunch.executable")}
            </label>
            <div className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-brand-50 dark:bg-brand-700 text-brand-900 dark:text-white font-mono break-all text-sm">
              {executableName}
            </div>
            <p className="mt-1 text-xs text-brand-500">
              {t("gameLaunch.executableHint")}
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              {t("gameLaunch.actualProcess")}
            </label>
            <div className="flex gap-2">
              <input
                type="text"
                value={game.process_name || ""}
                onChange={e =>
                  onGameChange({
                    ...game,
                    process_name: e.target.value,
                  } as models.Game)}
                className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none font-mono"
              />
              <BetterButton
                onClick={onSelectProcessExecutable}
                icon="i-mdi-file"
                title={t("gameLaunch.selectProcessFile")}
              />
            </div>
            <p className="mt-1 text-xs text-brand-500">
              {t("gameLaunch.processHint")}
            </p>
          </div>

          <div className="glass-panel rounded-xl border border-brand-200/80 bg-brand-50/70 p-4 dark:border-brand-700 dark:bg-brand-900/30">
            <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <div className="min-w-0">
                <p className="text-sm font-medium text-brand-800 dark:text-brand-200">
                  {t("gameLaunch.exportShortcut")}
                </p>
                <p className="mt-1 text-xs leading-relaxed text-brand-500 dark:text-brand-400">
                  {t("gameLaunch.exportShortcutHint")}
                </p>
              </div>
              <BetterButton
                variant="primary"
                icon="i-mdi-link-variant"
                onClick={onExportShortcut}
              >
                {t("gameLaunch.exportShortcut")}
              </BetterButton>
            </div>
          </div>
        </div>
      </div>

      {/* Enhancement Tools */}
      <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
        <div className="space-y-6">
          <div className="border-brand-200 dark:border-brand-700 pb-2">
            <h3 className="text-lg font-semibold text-brand-900 dark:text-white">
              {t("gameLaunch.enhancementTools")}
            </h3>
          </div>

          <div className="flex items-center justify-between">
            <div className="min-w-0 pr-4">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-brand-700 dark:text-brand-300">
                  Locale Emulator
                </span>
                <span className="px-1.5 py-0.5 text-[10px] font-medium bg-brand-100 dark:bg-brand-600 text-brand-800 dark:text-brand-100 rounded">
                  {t("gameLaunch.leLabel")}
                </span>
              </div>
              <p className="mt-1 text-xs text-brand-500 dark:text-brand-400">
                {t("gameLaunch.leDesc")}
              </p>
              {!hasLocaleEmulatorPath && (
                <p className="mt-1 flex items-center gap-1 text-xs text-error-500">
                  <div className="i-mdi-alert-circle text-sm shrink-0" />
                  <span>{t("gameLaunch.leNotConfigured")}</span>
                </p>
              )}
            </div>
            <div className="shrink-0">
              <BetterSwitch
                id="use_locale_emulator"
                checked={game.use_locale_emulator || false}
                onCheckedChange={handleLocaleEmulatorToggle}
                disabled={!hasLocaleEmulatorPath}
              />
            </div>
          </div>

          <div className="flex items-center justify-between">
            <div className="min-w-0 pr-4">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-brand-700 dark:text-brand-300">
                  Magpie
                </span>
                <span className="px-1.5 py-0.5 text-[10px] font-medium bg-brand-100 dark:bg-brand-600 text-brand-800 dark:text-brand-100 rounded">
                  {t("gameLaunch.magpieLabel")}
                </span>
              </div>
              <p className="mt-1 text-xs text-brand-500 dark:text-brand-400">
                {t("gameLaunch.magpieDesc")}
              </p>
              {!hasMagpiePath && (
                <p className="mt-1 flex items-center gap-1 text-xs text-error-500">
                  <div className="i-mdi-alert-circle text-sm shrink-0" />
                  <span>{t("gameLaunch.magpieNotConfigured")}</span>
                </p>
              )}
            </div>
            <div className="shrink-0">
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
    </div>
  );
}
