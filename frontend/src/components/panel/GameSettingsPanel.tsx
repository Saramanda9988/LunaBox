import type { appconf } from "../../../src/bindings/models";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  SelectGameExecutable,
  SelectWineRunnerExecutable,
} from "../../../bindings/lunabox/internal/service/gameservice";
import { BetterActionInput } from "../ui/better/BetterActionInput";
import { BetterSelect } from "../ui/better/BetterSelect";
import { BetterSwitch } from "../ui/better/BetterSwitch";

const PROCESS_DETECTION_TIMEOUT_SECONDS = [60, 120, 180, 300, 600] as const;

interface GameSettingsPanelProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
  goos?: string;
  backgroundProcessMuteSupported?: boolean;
}

export function GameSettingsPanel({
  formData,
  onChange,
  goos,
  backgroundProcessMuteSupported = false,
}: GameSettingsPanelProps) {
  const { t } = useTranslation();
  const isDarwin = goos === "darwin";
  const isLinux = goos === "linux";
  const supportsWineLaunch = isDarwin || isLinux;
  const processDetectionTimeoutOptions = PROCESS_DETECTION_TIMEOUT_SECONDS.map(
    seconds => ({
      value: String(seconds),
      label: t("settings.game.processDetectionTimeoutMinutes", {
        minutes: seconds / 60,
      }),
    }),
  );

  const handleSelectLocaleEmulatorPath = async () => {
    try {
      const path = await SelectGameExecutable(
        formData.locale_emulator_path || "",
      );
      if (path) {
        onChange({
          ...formData,
          locale_emulator_path: path,
        } as appconf.AppConfig);
      }
    }
    catch (error) {
      console.error("Failed to select Locale Emulator:", error);
      toast.error(t("settings.game.toast.leSelectFailed"));
    }
  };

  const handleSelectMagpiePath = async () => {
    try {
      const path = await SelectGameExecutable(formData.magpie_path || "");
      if (path) {
        onChange({ ...formData, magpie_path: path } as appconf.AppConfig);
      }
    }
    catch (error) {
      console.error("Failed to select Magpie:", error);
      toast.error(t("settings.game.toast.magpieSelectFailed"));
    }
  };

  const handleSelectCompatibilityRunnerPath = async (
    field: "wine_runner_path" | "crossover_runner_path",
  ) => {
    try {
      const path = await SelectWineRunnerExecutable(formData[field] || "");
      if (path) {
        onChange({ ...formData, [field]: path } as appconf.AppConfig);
      }
    }
    catch (error) {
      console.error("Failed to select Wine runner:", error);
      toast.error(t("settings.game.toast.wineSelectFailed"));
    }
  };

  return (
    <>
      <div className="space-y-2">
        <div className="flex items-center justify-between gap-4">
          <div className="flex-1 space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              {t("settings.game.recordActiveOnly")}
            </label>
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("settings.game.recordActiveOnlyHint")}
            </p>
          </div>
          <BetterSwitch
            id="record_active_time_only"
            checked={formData.record_active_time_only || false}
            onCheckedChange={checked =>
              onChange({
                ...formData,
                record_active_time_only: checked,
              } as appconf.AppConfig)}
          />
        </div>
      </div>

      {backgroundProcessMuteSupported ? (
        <div className="mt-6 border-t border-brand-200 dark:border-brand-700 pt-6">
          <div className="flex items-center justify-between gap-4">
            <div className="flex-1 space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                {t("settings.game.muteInBackground")}
              </label>
              <p className="text-xs text-brand-500 dark:text-brand-400">
                {t(
                  isDarwin
                    ? "settings.game.muteInBackgroundHintMacOS"
                    : "settings.game.muteInBackgroundHint",
                )}
              </p>
            </div>
            <BetterSwitch
              id="mute_game_in_background"
              checked={formData.mute_game_in_background ?? false}
              onCheckedChange={checked =>
                onChange({
                  ...formData,
                  mute_game_in_background: checked,
                } as appconf.AppConfig)}
            />
          </div>
        </div>
      ) : null}

      <div className="mt-6 border-t border-brand-200 dark:border-brand-700 pt-6">
        <div className="space-y-2">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("settings.game.processDetectionTimeout")}
          </label>
          <BetterSelect
            name="process_detection_timeout_sec"
            value={String(formData.process_detection_timeout_sec || 60)}
            options={processDetectionTimeoutOptions}
            onChange={value =>
              onChange({
                ...formData,
                process_detection_timeout_sec: Number(value),
              } as appconf.AppConfig)}
            className="w-full"
          />
          <p className="text-xs text-brand-500 dark:text-brand-400">
            {t("settings.game.processDetectionTimeoutHint")}
          </p>
        </div>
        <div className="mt-4 rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-700 dark:bg-amber-900/20">
          <div className="flex items-start gap-2">
            <span className="i-mdi-alert mt-0.5 text-lg text-amber-600 dark:text-amber-400" />
            <div className="text-xs text-amber-700 dark:text-amber-300">
              <p className="mb-1 font-medium">
                {t("settings.game.warningTitle")}
              </p>
              <p>{t("settings.game.processDetectionTimeoutWarning")}</p>
            </div>
          </div>
        </div>
      </div>

      {/* Launch Tools Configuration */}
      <div className="mt-6 border-t border-brand-200 dark:border-brand-700 pt-6">
        <div className="mb-4 block text-sm font-semibold text-brand-700 dark:text-brand-300">
          {t("settings.game.launchTools")}
        </div>

        <div className="space-y-4">
          {supportsWineLaunch ? (
            <div className="space-y-5">
              <div className="space-y-4">
                <div className="space-y-2">
                  <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                    {t("settings.game.wineRunnerPath")}
                  </label>
                  <BetterActionInput
                    value={formData.wine_runner_path || ""}
                    onChange={e =>
                      onChange({
                        ...formData,
                        wine_runner_path: e.target.value,
                      } as appconf.AppConfig)}
                    placeholder={t("settings.game.wineRunnerPathPlaceholder")}
                    className="font-mono"
                    actions={[
                      {
                        ariaLabel: t("settings.game.selectBtn"),
                        icon: "i-mdi-file-search-outline",
                        onClick: () =>
                          handleSelectCompatibilityRunnerPath(
                            "wine_runner_path",
                          ),
                      },
                    ]}
                  />
                  <p className="text-xs text-brand-500 dark:text-brand-400">
                    {t("settings.game.wineRunnerPathHint")}
                  </p>
                </div>

                <div className="space-y-2">
                  <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                    {t("settings.game.winePrefix")}
                  </label>
                  <input
                    type="text"
                    value={formData.wine_prefix || ""}
                    onChange={e =>
                      onChange({
                        ...formData,
                        wine_prefix: e.target.value,
                      } as appconf.AppConfig)}
                    placeholder={t("settings.game.winePrefixPlaceholder")}
                    className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none font-mono"
                  />
                  <p className="text-xs text-brand-500 dark:text-brand-400">
                    {t("settings.game.winePrefixHint")}
                  </p>
                </div>
              </div>
              {isDarwin ? (
                <div className="space-y-4 border-t border-brand-200 pt-5 dark:border-brand-700">
                  <div className="space-y-2">
                    <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                      {t("settings.game.crossoverRunnerPath")}
                    </label>
                    <BetterActionInput
                      value={formData.crossover_runner_path || ""}
                      onChange={e =>
                        onChange({
                          ...formData,
                          crossover_runner_path: e.target.value,
                        } as appconf.AppConfig)}
                      placeholder={t(
                        "settings.game.crossoverRunnerPathPlaceholder",
                      )}
                      className="font-mono"
                      actions={[
                        {
                          ariaLabel: t("settings.game.selectBtn"),
                          icon: "i-mdi-file-search-outline",
                          onClick: () =>
                            handleSelectCompatibilityRunnerPath(
                              "crossover_runner_path",
                            ),
                        },
                      ]}
                    />
                    <p className="text-xs text-brand-500 dark:text-brand-400">
                      {t("settings.game.crossoverRunnerPathHint")}
                    </p>
                  </div>

                  <div className="space-y-2">
                    <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                      {t("settings.game.crossoverBottle")}
                    </label>
                    <input
                      type="text"
                      value={formData.crossover_bottle || ""}
                      onChange={e =>
                        onChange({
                          ...formData,
                          crossover_bottle: e.target.value,
                        } as appconf.AppConfig)}
                      placeholder={t(
                        "settings.game.crossoverBottlePlaceholder",
                      )}
                      className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none font-mono"
                    />
                    <p className="text-xs text-brand-500 dark:text-brand-400">
                      {t("settings.game.crossoverBottleHint")}
                    </p>
                  </div>
                </div>
              ) : null}
            </div>
          ) : (
            <>
              <div className="space-y-2">
                <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                  {t("settings.game.lePath")}
                </label>
                <BetterActionInput
                  value={formData.locale_emulator_path || ""}
                  onChange={e =>
                    onChange({
                      ...formData,
                      locale_emulator_path: e.target.value,
                    } as appconf.AppConfig)}
                  placeholder={t("settings.game.lePathPlaceholder")}
                  actions={[
                    {
                      ariaLabel: t("settings.game.selectBtn"),
                      icon: "i-mdi-file-search-outline",
                      onClick: handleSelectLocaleEmulatorPath,
                    },
                  ]}
                />
                <p className="text-xs text-brand-500 dark:text-brand-400">
                  {t("settings.game.lePathHint")}
                </p>
              </div>

              <div className="space-y-2">
                <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                  {t("settings.game.magpiePath")}
                </label>
                <BetterActionInput
                  value={formData.magpie_path || ""}
                  onChange={e =>
                    onChange({
                      ...formData,
                      magpie_path: e.target.value,
                    } as appconf.AppConfig)}
                  placeholder={t("settings.game.magpiePathPlaceholder")}
                  actions={[
                    {
                      ariaLabel: t("settings.game.selectBtn"),
                      icon: "i-mdi-file-search-outline",
                      onClick: handleSelectMagpiePath,
                    },
                  ]}
                />
                <p className="text-xs text-brand-500 dark:text-brand-400">
                  {t("settings.game.magpiePathHint")}
                </p>
              </div>
            </>
          )}
        </div>
      </div>
    </>
  );
}
