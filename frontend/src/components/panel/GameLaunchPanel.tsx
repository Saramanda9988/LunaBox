import type { appconf, models, service } from "../../../src/bindings/models";
import type { SteamCompatibilityInfo } from "../../bindings/integration";
import { useEffect, useMemo, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { OpenLocalPath } from "../../../bindings/lunabox/internal/service/gameservice";
import { enums } from "../../../src/bindings/models";
import {
  GetGameSteamCompatibility,
  OpenGameSteamProtonPrefix,
  RestartSteamClient,
  SetGameSteamCompatibilityTool,
} from "../../bindings/integration";
import { ConfirmModal } from "../modal/ConfirmModal";
import { BetterActionInput } from "../ui/better/BetterActionInput";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterSelect } from "../ui/better/BetterSelect";
import { BetterSwitch } from "../ui/better/BetterSwitch";

interface GameLaunchPanelProps {
  game: models.Game;
  config?: appconf.AppConfig;
  onGameChange: (game: models.Game) => void;
  onLaunchModeChange: (mode: enums.LaunchMode) => void;
  onRefreshSteamSettings?: () => Promise<void>;
  onSaveSteamLaunchOptions?: (
    launchOptions: string,
  ) => Promise<service.SteamLaunchStatus | void>;
  onSelectProcessExecutable: () => void;
  onSelectRunningProcess: () => void;
  onExportShortcut: () => void;
  goos?: string;
}

type GameWithSteamLaunchOptions = models.Game & {
  steam_launch_options?: string;
};

const localeLaunchOptionPresets = [
  {
    key: "chineseLocale",
    value: "LANG=zh_CN.UTF-8 %command%",
  },
  {
    key: "fullChineseLocale",
    value: "LC_ALL=zh_CN.UTF-8 LANG=zh_CN.UTF-8 %command%",
  },
] as const;

const steamLaunchOptionPresets = [
  ...localeLaunchOptionPresets,
  {
    key: "protonLog",
    value: "PROTON_LOG=1 %command%",
  },
] as const;

const wineLaunchOptionPresets = localeLaunchOptionPresets;

function getSteamLaunchOptions(game: models.Game): string {
  return (game as GameWithSteamLaunchOptions).steam_launch_options || "";
}

function withSteamLaunchOptions(
  game: models.Game,
  steamLaunchOptions: string,
): models.Game {
  return {
    ...game,
    steam_launch_options: steamLaunchOptions,
  } as GameWithSteamLaunchOptions as models.Game;
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

function isWindowsExecutablePath(path?: string): boolean {
  return /\.(?:exe|bat)$/i.test((path || "").trim());
}

export function GameLaunchPanel({
  game,
  config,
  onGameChange,
  onLaunchModeChange,
  onRefreshSteamSettings,
  onSaveSteamLaunchOptions,
  onSelectProcessExecutable,
  onSelectRunningProcess,
  onExportShortcut,
  goos,
}: GameLaunchPanelProps) {
  const { t } = useTranslation();
  const isDarwin = goos === "darwin";
  const isLinux = goos === "linux";
  const supportsWineLaunch = isDarwin || isLinux;
  const supportsWindowsEnhancements = goos === "windows";
  const hasLocaleEmulatorPath
    = config?.locale_emulator_path && config?.locale_emulator_path.length > 0;
  const hasMagpiePath = config?.magpie_path && config?.magpie_path.length > 0;
  const executableName = game.path
    ? game.path.split(/[\\/]/).pop()
    : t("gameLaunch.noPathSet");
  const supportsSteamLaunch
    = goos === "windows"
      || isLinux
      || (isDarwin
        && game.steam_launch_kind === "native"
        && Boolean(game.steam_launch_id));
  const launchModeOptions = [
    {
      value: enums.LaunchMode.LaunchModeNormal,
      label: t("gameLaunch.launchModeNormal"),
    },
    ...(isDarwin
      ? [
          {
            value: enums.LaunchMode.LaunchModeCompatibility,
            label: t("gameLaunch.launchModeCompatibility"),
          },
        ]
      : []),
    ...(supportsSteamLaunch
      ? [
          {
            value: enums.LaunchMode.LaunchModeSteam,
            label: t("gameLaunch.launchModeSteam"),
          },
        ]
      : []),
  ];
  const launchMode
    = (game.launch_mode === enums.LaunchMode.LaunchModeSteam
      && !supportsSteamLaunch)
    || (game.launch_mode === enums.LaunchMode.LaunchModeCompatibility && !isDarwin)
      ? enums.LaunchMode.LaunchModeNormal
      : game.launch_mode || enums.LaunchMode.LaunchModeNormal;
  const isSteamLaunch = launchMode === enums.LaunchMode.LaunchModeSteam;
  const isCompatibilityLaunch
    = launchMode === enums.LaunchMode.LaunchModeCompatibility;
  const defaultsToSystemWineRunner
    = isDarwin || (isLinux && isWindowsExecutablePath(game.path));
  const configuredWineRunner
    = isLinux && game.wine_runner === "crossover" ? "" : game.wine_runner;
  const selectedWineRunner
    = configuredWineRunner || (defaultsToSystemWineRunner ? "system" : "");
  const hasWineCompatibilityLayer = selectedWineRunner !== "";
  const effectiveWinePrefixPath
    = selectedWineRunner === "crossover"
      ? ""
      : game.wine_prefix || config?.wine_prefix || "";
  const supportsSteamCompatibility = isLinux;
  const [steamCompatibility, setSteamCompatibility]
    = useState<SteamCompatibilityInfo | null>(null);
  const [isSteamCompatibilityLoading, setIsSteamCompatibilityLoading]
    = useState(false);
  const [isSteamCompatibilitySaving, setIsSteamCompatibilitySaving]
    = useState(false);
  const [isSteamProtonPrefixOpening, setIsSteamProtonPrefixOpening]
    = useState(false);
  const [isSteamLaunchOptionsSaving, setIsSteamLaunchOptionsSaving]
    = useState(false);
  const [isSteamRestarting, setIsSteamRestarting] = useState(false);
  const [isSteamRestartConfirmOpen, setIsSteamRestartConfirmOpen]
    = useState(false);
  const [steamCompatibilityError, setSteamCompatibilityError] = useState("");
  const steamLaunchOptions = getSteamLaunchOptions(game);

  const handleRefreshSteamSettings = async () => {
    if (!supportsSteamLaunch) {
      return;
    }
    if (supportsSteamCompatibility) {
      setIsSteamCompatibilityLoading(true);
    }
    setSteamCompatibilityError("");
    try {
      const [info] = await Promise.all([
        supportsSteamCompatibility
          ? GetGameSteamCompatibility(game.id)
          : Promise.resolve(null),
        onRefreshSteamSettings?.() ?? Promise.resolve(),
      ]);
      if (supportsSteamCompatibility) {
        setSteamCompatibility(info);
      }
    }
    catch (error) {
      console.error("Failed to refresh Steam settings:", error);
      if (supportsSteamCompatibility) {
        setSteamCompatibility(null);
      }
      setSteamCompatibilityError(errorMessage(error));
    }
    finally {
      if (supportsSteamCompatibility) {
        setIsSteamCompatibilityLoading(false);
      }
    }
  };

  useEffect(() => {
    let cancelled = false;
    async function run() {
      if (!supportsSteamCompatibility) {
        setSteamCompatibility(null);
        setSteamCompatibilityError("");
        return;
      }
      setIsSteamCompatibilityLoading(true);
      setSteamCompatibilityError("");
      try {
        const info = await GetGameSteamCompatibility(game.id);
        if (!cancelled) {
          setSteamCompatibility(info);
        }
      }
      catch (error) {
        if (!cancelled) {
          console.error("Failed to load Steam compatibility tools:", error);
          setSteamCompatibility(null);
          setSteamCompatibilityError(errorMessage(error));
        }
      }
      finally {
        if (!cancelled) {
          setIsSteamCompatibilityLoading(false);
        }
      }
    }
    run();
    return () => {
      cancelled = true;
    };
  }, [
    game.id,
    game.source_id,
    game.steam_launch_id,
    game.steam_launch_kind,
    supportsSteamCompatibility,
  ]);

  useEffect(() => {
    if (!isLinux || game.wine_runner !== "crossover") {
      return;
    }
    onGameChange({
      ...game,
      wine_runner: defaultsToSystemWineRunner ? "system" : "",
      wine_prefix: "",
    } as models.Game);
  }, [defaultsToSystemWineRunner, game, isLinux, onGameChange]);

  const steamCompatibilityOptions = useMemo(() => {
    const defaultTool
      = steamCompatibility?.default_tool || t("gameLaunch.steamProtonAuto");
    const options = [
      {
        value: "",
        label: t("gameLaunch.steamProtonDefault", { tool: defaultTool }),
      },
    ];
    for (const tool of steamCompatibility?.tools || []) {
      const label
        = tool.display_name && tool.display_name !== tool.name
          ? `${tool.display_name} (${tool.name})`
          : tool.display_name || tool.name;
      options.push({ value: tool.name, label });
    }
    return options;
  }, [steamCompatibility, t]);

  const handleSteamCompatibilityChange = async (toolName: string) => {
    if (!steamCompatibility || isSteamCompatibilitySaving) {
      return;
    }
    setIsSteamCompatibilitySaving(true);
    setSteamCompatibilityError("");
    try {
      const info = await SetGameSteamCompatibilityTool(game.id, toolName);
      setSteamCompatibility(info);
      toast.success(t("gameLaunch.toast.steamProtonSaved"));
    }
    catch (error) {
      console.error("Failed to save Steam compatibility tool:", error);
      const message = errorMessage(error);
      setSteamCompatibilityError(message);
      toast.error(
        t("gameLaunch.toast.steamProtonSaveFailed", { error: message }),
      );
    }
    finally {
      setIsSteamCompatibilitySaving(false);
    }
  };

  const handleOpenSteamProtonPrefix = async () => {
    if (!steamCompatibility?.proton_prefix || isSteamProtonPrefixOpening) {
      return;
    }
    setIsSteamProtonPrefixOpening(true);
    try {
      await OpenGameSteamProtonPrefix(game.id);
    }
    catch (error) {
      console.error("Failed to open Steam Proton Prefix:", error);
      toast.error(
        t("gameLaunch.toast.steamProtonPrefixOpenFailed", {
          error: errorMessage(error),
        }),
      );
    }
    finally {
      setIsSteamProtonPrefixOpening(false);
    }
  };

  const handleOpenWinePrefix = async () => {
    if (!effectiveWinePrefixPath) {
      return;
    }
    try {
      await OpenLocalPath(effectiveWinePrefixPath);
    }
    catch {
      toast.error(t("gameEdit.openPathFailed"));
    }
  };

  const handleRestartSteam = async () => {
    if (isSteamRestarting) {
      return;
    }
    setIsSteamRestarting(true);
    try {
      await RestartSteamClient();
      toast.success(t("gameLaunch.toast.steamRestarted"));
      await handleRefreshSteamSettings();
    }
    catch (error) {
      console.error("Failed to restart Steam:", error);
      toast.error(
        t("gameLaunch.toast.steamRestartFailed", {
          error: errorMessage(error),
        }),
      );
    }
    finally {
      setIsSteamRestarting(false);
    }
  };

  const handleSteamLaunchOptionsChange = (value: string) => {
    onGameChange(withSteamLaunchOptions(game, value));
  };

  const handleApplySteamLaunchOptionsPreset = (value: string) => {
    handleSteamLaunchOptionsChange(value);
  };

  const handleWineArgsChange = (value: string) => {
    onGameChange({
      ...game,
      wine_args: value,
    } as models.Game);
  };

  const handleApplyWineArgsPreset = (value: string) => {
    handleWineArgsChange(value);
  };

  const promptSteamRestartIfNeeded = (
    status?: service.SteamLaunchStatus | void,
  ) => {
    if (status?.steam_running) {
      setIsSteamRestartConfirmOpen(true);
    }
  };

  const handleSaveSteamLaunchOptions = async () => {
    if (!onSaveSteamLaunchOptions || isSteamLaunchOptionsSaving) {
      return;
    }
    setIsSteamLaunchOptionsSaving(true);
    try {
      const status = await onSaveSteamLaunchOptions(steamLaunchOptions);
      toast.success(t("gameLaunch.toast.steamLaunchOptionsSaved"));
      promptSteamRestartIfNeeded(status);
    }
    catch (error) {
      toast.error(
        t("gameLaunch.toast.steamLaunchOptionsSaveFailed", {
          error: errorMessage(error),
        }),
      );
    }
    finally {
      setIsSteamLaunchOptionsSaving(false);
    }
  };

  const handleLocaleEmulatorToggle = (checked: boolean) => {
    if (checked && supportsWindowsEnhancements && !hasLocaleEmulatorPath) {
      toast.error(t("gameLaunch.toast.lePathRequired"));
      return;
    }
    onGameChange({
      ...game,
      use_locale_emulator: checked,
    } as models.Game);
  };

  const handleMagpieToggle = (checked: boolean) => {
    if (checked && !hasMagpiePath) {
      toast.error(t("gameLaunch.toast.magpiePathRequired"));
      return;
    }
    onGameChange({ ...game, use_magpie: checked } as models.Game);
  };
  const wineRunnerOptions = [
    ...(isLinux && !defaultsToSystemWineRunner
      ? [{ value: "", label: t("gameLaunch.wineRunnerNone") }]
      : []),
    { value: "system", label: t("gameLaunch.wineRunnerSystem") },
    ...(isDarwin
      ? [{ value: "crossover", label: t("gameLaunch.wineRunnerCrossover") }]
      : []),
    ...(isLinux
      ? [{ value: "custom", label: t("gameLaunch.wineRunnerCustom") }]
      : []),
  ];
  const isSteamCompatibilityPending
    = supportsSteamCompatibility
      && !steamCompatibility
      && !steamCompatibilityError;
  const steamCompatibilityDisabled
    = isSteamCompatibilityLoading
      || isSteamCompatibilityPending
      || isSteamCompatibilitySaving
      || !steamCompatibility?.supported
      || !steamCompatibility?.steam_installed
      || !steamCompatibility?.app_id
      || !!steamCompatibilityError;
  const steamRestartDisabled
    = isSteamCompatibilityLoading
      || isSteamCompatibilityPending
      || isSteamCompatibilitySaving
      || isSteamRestarting
      || !steamCompatibility?.supported
      || !steamCompatibility?.steam_installed
      || !!steamCompatibilityError;
  const steamCompatibilityNotice
    = isSteamCompatibilityLoading || isSteamCompatibilityPending
      ? t("gameLaunch.steamProtonLoading")
      : steamCompatibilityError
        ? t("gameLaunch.steamProtonError", { error: steamCompatibilityError })
        : !steamCompatibility?.steam_installed
            ? t("gameLaunch.steamProtonNotInstalled")
            : !steamCompatibility?.app_id
                ? t("gameLaunch.steamProtonNotAssociated")
                : t("gameLaunch.steamProtonHint");
  const steamProtonPrefixPath = steamCompatibility?.proton_prefix || "";
  const steamProtonPrefixDisabled
    = isSteamCompatibilityLoading
      || isSteamCompatibilityPending
      || isSteamProtonPrefixOpening
      || !steamProtonPrefixPath
      || !!steamCompatibilityError;
  const showSteamLaunchConfiguration = supportsSteamLaunch && isSteamLaunch;
  const showWineLaunchConfiguration
    = supportsWineLaunch && !isSteamLaunch && (isLinux || isCompatibilityLaunch);
  const showCompatibilityLauncher
    = showSteamLaunchConfiguration || showWineLaunchConfiguration;
  const compatibilityLauncherTitle
    = showSteamLaunchConfiguration && !supportsSteamCompatibility
      ? t("gameLaunch.steamTools")
      : t("gameLaunch.compatibilityTools");
  const compatibilityLauncherHint = showSteamLaunchConfiguration
    ? supportsSteamCompatibility
      ? t("gameLaunch.compatibilityToolsSteamHint")
      : t("gameLaunch.steamToolsHint")
    : t("gameLaunch.compatibilityToolsWineHint");

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
              {t("gameLaunch.launchMode")}
            </label>
            <BetterSelect
              value={launchMode}
              options={launchModeOptions}
              onChange={value =>
                onLaunchModeChange(value as enums.LaunchMode)}
            />
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
            <div className="flex flex-col gap-2 sm:flex-row">
              <BetterActionInput
                value={game.process_name || ""}
                placeholder={
                  supportsWineLaunch
                    ? t("gameLaunch.processPlaceholderDarwin")
                    : undefined
                }
                onChange={e =>
                  onGameChange({
                    ...game,
                    process_name: e.target.value,
                  } as models.Game)}
                readOnly={supportsWineLaunch}
                disabled={supportsWineLaunch}
                className="font-mono"
                containerClassName="flex-1"
                actions={
                  supportsWineLaunch
                    ? []
                    : [
                        {
                          ariaLabel: t("gameLaunch.selectProcessFile"),
                          icon: "i-mdi-file-search-outline",
                          onClick: onSelectProcessExecutable,
                        },
                      ]
                }
              />
              {supportsWineLaunch ? null : (
                <BetterButton
                  variant="secondary"
                  icon="i-mdi-application-search-outline"
                  onClick={onSelectRunningProcess}
                >
                  {t("gameLaunch.selectRunningProcess")}
                </BetterButton>
              )}
            </div>
            <p className="mt-1 text-xs text-brand-500">
              {supportsWineLaunch
                ? t("gameLaunch.processHintDarwin")
                : t("gameLaunch.processHint")}
            </p>
          </div>

          {supportsWindowsEnhancements && (
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
          )}
        </div>
      </div>

      {showCompatibilityLauncher && (
        <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
          <div className="space-y-5">
            <div className="border-brand-200 dark:border-brand-700 pb-2">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <h3 className="text-lg font-semibold text-brand-900 dark:text-white">
                    {compatibilityLauncherTitle}
                  </h3>
                  <p className="mt-1 text-xs text-brand-500 dark:text-brand-400">
                    {compatibilityLauncherHint}
                  </p>
                </div>
                {showSteamLaunchConfiguration && (
                  <BetterButton
                    variant="ghost"
                    size="sm"
                    icon="i-mdi-refresh"
                    onClick={handleRefreshSteamSettings}
                    isLoading={isSteamCompatibilityLoading}
                    disabled={isSteamCompatibilitySaving || isSteamRestarting}
                    aria-label={t("gameLaunch.steamProtonRefresh")}
                  />
                )}
              </div>
            </div>

            {showSteamLaunchConfiguration && (
              <>
                {supportsSteamCompatibility && (
                  <div className="glass-panel rounded-xl border border-brand-200/80 bg-brand-50/70 p-4 dark:border-brand-700 dark:bg-brand-900/30">
                    <p className="text-sm font-medium text-brand-800 dark:text-brand-200">
                      {t("gameLaunch.compatibilityEngine")}
                    </p>
                    <div className="mt-2 flex items-center gap-2 text-sm text-brand-900 dark:text-white">
                      <div className="i-mdi-steam text-lg text-brand-500 dark:text-brand-300" />
                      <span className="font-medium">Proton</span>
                    </div>
                    <p className="mt-1 text-xs leading-relaxed text-brand-500 dark:text-brand-400">
                      {t("gameLaunch.steamProtonLockedHint")}
                    </p>
                  </div>
                )}

                <div className="space-y-3">
                  <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                    {t("gameLaunch.steamLaunchOptions")}
                  </label>
                  <div className="flex flex-col gap-2 md:flex-row">
                    <input
                      type="text"
                      value={steamLaunchOptions}
                      onChange={e =>
                        handleSteamLaunchOptionsChange(e.target.value)}
                      placeholder={t(
                        "gameLaunch.steamLaunchOptionsPlaceholder",
                      )}
                      className="glass-input min-w-0 flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none font-mono"
                    />
                    <BetterButton
                      variant="secondary"
                      icon="i-mdi-content-save-outline"
                      onClick={handleSaveSteamLaunchOptions}
                      isLoading={isSteamLaunchOptionsSaving}
                      disabled={!onSaveSteamLaunchOptions}
                    >
                      {t("gameLaunch.steamLaunchOptionsSave")}
                    </BetterButton>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {steamLaunchOptionPresets.map(preset => (
                      <BetterButton
                        key={preset.key}
                        variant="ghost"
                        size="sm"
                        icon="i-mdi-plus-circle-outline"
                        onClick={() =>
                          handleApplySteamLaunchOptionsPreset(preset.value)}
                      >
                        {t(
                          `gameLaunch.steamLaunchOptionsPresets.${preset.key}`,
                        )}
                      </BetterButton>
                    ))}
                  </div>
                  <p className="text-xs leading-relaxed text-brand-500 dark:text-brand-400">
                    {t("gameLaunch.steamLaunchOptionsHint")}
                  </p>
                </div>

                {supportsSteamCompatibility && (
                  <div className="space-y-2">
                    <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                      {t("gameLaunch.steamProton")}
                    </label>
                    <div className="flex flex-col gap-2 md:flex-row">
                      <BetterSelect
                        value={steamCompatibility?.current_tool || ""}
                        options={steamCompatibilityOptions}
                        onChange={handleSteamCompatibilityChange}
                        disabled={steamCompatibilityDisabled}
                        className="min-w-0 flex-1"
                      />
                      <BetterButton
                        variant="secondary"
                        icon="i-mdi-restart"
                        onClick={handleRestartSteam}
                        isLoading={isSteamRestarting}
                        disabled={steamRestartDisabled}
                      >
                        {t("gameLaunch.steamRestart")}
                      </BetterButton>
                    </div>
                    <p
                      className={[
                        "text-xs dark:text-brand-400",
                        steamCompatibilityError
                          ? "text-error-500"
                          : "text-brand-500",
                      ].join(" ")}
                    >
                      {steamCompatibilityNotice}
                    </p>
                    {steamCompatibility?.steam_root && (
                      <p className="text-xs text-brand-400 dark:text-brand-500 font-mono break-all">
                        {steamCompatibility.steam_root}
                      </p>
                    )}
                  </div>
                )}

                {supportsSteamCompatibility && (
                  <div className="glass-panel rounded-xl border border-brand-200/80 bg-brand-50/70 p-4 dark:border-brand-700 dark:bg-brand-900/30">
                    <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                      <div className="min-w-0">
                        <p className="text-sm font-medium text-brand-800 dark:text-brand-200">
                          {t("gameLaunch.steamProtonPrefix")}
                        </p>
                        <p
                          className={[
                            "mt-1 break-all text-xs",
                            steamProtonPrefixPath
                              ? "font-mono text-brand-500 dark:text-brand-400"
                              : "text-brand-500 dark:text-brand-400",
                          ].join(" ")}
                        >
                          {steamProtonPrefixPath
                            || t("gameLaunch.steamProtonPrefixNotFound")}
                        </p>
                        <p className="mt-1 text-xs leading-relaxed text-brand-500 dark:text-brand-400">
                          {t("gameLaunch.steamProtonPrefixHint")}
                        </p>
                      </div>
                      <BetterButton
                        variant="secondary"
                        size="sm"
                        icon="i-mdi-folder-open-outline"
                        onClick={handleOpenSteamProtonPrefix}
                        isLoading={isSteamProtonPrefixOpening}
                        disabled={steamProtonPrefixDisabled}
                      >
                        {t("gameLaunch.steamProtonPrefixOpen")}
                      </BetterButton>
                    </div>
                  </div>
                )}
              </>
            )}

            {showWineLaunchConfiguration && (
              <>
                <div className="space-y-2">
                  <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                    {t("gameLaunch.compatibilityEngine")}
                  </label>
                  <BetterSelect
                    value={selectedWineRunner}
                    options={wineRunnerOptions}
                    onChange={value =>
                      onGameChange({
                        ...game,
                        wine_runner: value,
                        wine_prefix:
                          value === game.wine_runner ? game.wine_prefix : "",
                      } as models.Game)}
                  />
                  <p className="text-xs text-brand-500 dark:text-brand-400">
                    {t("gameLaunch.wineRunnerHint")}
                  </p>
                </div>

                {hasWineCompatibilityLayer && (
                  <div className="space-y-2">
                    <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                      {t("gameLaunch.wineArgs")}
                    </label>
                    <input
                      type="text"
                      value={game.wine_args || ""}
                      onChange={e => handleWineArgsChange(e.target.value)}
                      placeholder={t("gameLaunch.wineArgsPlaceholder")}
                      className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none font-mono"
                    />
                    <div className="flex flex-wrap gap-2">
                      {wineLaunchOptionPresets.map(preset => (
                        <BetterButton
                          key={preset.key}
                          variant="ghost"
                          size="sm"
                          icon="i-mdi-plus-circle-outline"
                          onClick={() =>
                            handleApplyWineArgsPreset(preset.value)}
                        >
                          {t(
                            `gameLaunch.steamLaunchOptionsPresets.${preset.key}`,
                          )}
                        </BetterButton>
                      ))}
                    </div>
                    <p className="text-xs leading-relaxed text-brand-500 dark:text-brand-400">
                      {t("gameLaunch.wineArgsHint")}
                    </p>
                  </div>
                )}

                {hasWineCompatibilityLayer && (
                  <div className="space-y-2">
                    <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                      {selectedWineRunner === "crossover"
                        ? t("gameLaunch.crossoverBottle")
                        : t("gameLaunch.winePrefix")}
                    </label>
                    <BetterActionInput
                      value={game.wine_prefix || ""}
                      onChange={e =>
                        onGameChange({
                          ...game,
                          wine_prefix: e.target.value,
                        } as models.Game)}
                      placeholder={
                        selectedWineRunner === "crossover"
                          ? t("gameLaunch.crossoverBottlePlaceholder")
                          : t("gameLaunch.winePrefixPlaceholder")
                      }
                      className="font-mono"
                      actions={
                        selectedWineRunner === "crossover"
                          ? []
                          : [
                              {
                                ariaLabel: t("gameEdit.openInExplorer"),
                                disabled: !effectiveWinePrefixPath,
                                icon: "i-mdi-folder-open-outline",
                                onClick: handleOpenWinePrefix,
                              },
                            ]
                      }
                    />
                    <p className="text-xs text-brand-500 dark:text-brand-400">
                      {selectedWineRunner === "crossover"
                        ? t("gameLaunch.crossoverBottleHint")
                        : t("gameLaunch.winePrefixHint")}
                    </p>
                  </div>
                )}
              </>
            )}
          </div>
        </div>
      )}

      {supportsWindowsEnhancements && (
        <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
          <div className="space-y-6">
            <div className="border-brand-200 dark:border-brand-700 pb-2">
              <h3 className="text-lg font-semibold text-brand-900 dark:text-white">
                {t("gameLaunch.enhancementTools")}
              </h3>
            </div>

            {!isSteamLaunch && (
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
            )}

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
      )}
      <ConfirmModal
        isOpen={isSteamRestartConfirmOpen}
        title={t("gameLaunch.steamRestartConfirmTitle")}
        message={t("gameLaunch.steamRestartConfirmMessage")}
        confirmText={t("gameLaunch.steamRestart")}
        type="info"
        onClose={() => setIsSteamRestartConfirmOpen(false)}
        onConfirm={() => {
          void handleRestartSteam();
        }}
      />
    </div>
  );
}
