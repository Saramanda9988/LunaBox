import type { appconf } from "../../../wailsjs/go/models";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { SelectGameExecutable } from "../../../wailsjs/go/service/GameService";
import { BetterButton } from "../ui/BetterButton";
import { BetterSwitch } from "../ui/BetterSwitch";

interface GameSettingsPanelProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

export function GameSettingsPanel({ formData, onChange }: GameSettingsPanelProps) {
  const { t } = useTranslation();

  const handleSelectLocaleEmulatorPath = async () => {
    try {
      const path = await SelectGameExecutable();
      if (path) {
        onChange({ ...formData, locale_emulator_path: path } as appconf.AppConfig);
      }
    }
    catch (error) {
      console.error("Failed to select Locale Emulator:", error);
      toast.error(t("settings.game.toast.leSelectFailed"));
    }
  };

  const handleSelectMagpiePath = async () => {
    try {
      const path = await SelectGameExecutable();
      if (path) {
        onChange({ ...formData, magpie_path: path } as appconf.AppConfig);
      }
    }
    catch (error) {
      console.error("Failed to select Magpie:", error);
      toast.error(t("settings.game.toast.magpieSelectFailed"));
    }
  };

  return (
    <>
      <div className="flex items-center justify-between p-2">
        <div className="flex-1">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("settings.game.recordActiveOnly")}
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
            {t("settings.game.recordActiveOnlyHint")}
          </p>
        </div>
        <BetterSwitch
          id="record_active_time_only"
          checked={formData.record_active_time_only || false}
          onCheckedChange={checked =>
            onChange({ ...formData, record_active_time_only: checked } as appconf.AppConfig)}
        />
      </div>

      <div className="mt-4 p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700 rounded-lg">
        <div className="flex items-start gap-2">
          <span className="i-mdi-alert text-amber-600 dark:text-amber-400 text-lg mt-0.5" />
          <div className="text-xs text-amber-700 dark:text-amber-300">
            <p className="font-medium mb-1">{t("settings.game.warningTitle")}</p>
            <ul className="list-disc list-inside space-y-1 ml-2">
              <li>{t("settings.game.warningItem1")}</li>
              <li>{t("settings.game.warningItem2")}</li>
              <li>{t("settings.game.warningItem3")}</li>
            </ul>
          </div>
        </div>
      </div>

      {/* Auto Process Detection */}
      <div className="mt-6 border-t border-brand-200 dark:border-brand-700 pt-6">
        <div className="flex items-center justify-between p-2">
          <div className="flex-1">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              {t("settings.game.autoDetectProcess")}
            </label>
            <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
              {t("settings.game.autoDetectProcessHint")}
            </p>
          </div>
          <BetterSwitch
            id="auto_detect_game_process"
            checked={formData.auto_detect_game_process ?? true}
            onCheckedChange={checked =>
              onChange({ ...formData, auto_detect_game_process: checked } as appconf.AppConfig)}
          />
        </div>
      </div>

      {/* Launch Tools Configuration */}
      <div className="mt-6 border-t border-brand-200 dark:border-brand-700 pt-6">
        <h3 className="text-sm font-semibold text-brand-900 dark:text-white mb-4">{t("settings.game.launchTools")}</h3>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-2">
              {t("settings.game.lePath")}
            </label>
            <div className="flex gap-2">
              <input
                type="text"
                value={formData.locale_emulator_path || ""}
                onChange={e => onChange({ ...formData, locale_emulator_path: e.target.value } as appconf.AppConfig)}
                placeholder={t("settings.game.lePathPlaceholder")}
                className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
              />
              <BetterButton onClick={handleSelectLocaleEmulatorPath} icon="i-mdi-file">
                {t("settings.game.selectBtn")}
              </BetterButton>
            </div>
            <p className="mt-1 text-xs text-brand-500">
              {t("settings.game.lePathHint")}
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-2">
              {t("settings.game.magpiePath")}
            </label>
            <div className="flex gap-2">
              <input
                type="text"
                value={formData.magpie_path || ""}
                onChange={e => onChange({ ...formData, magpie_path: e.target.value } as appconf.AppConfig)}
                placeholder={t("settings.game.magpiePathPlaceholder")}
                className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
              />
              <BetterButton onClick={handleSelectMagpiePath} icon="i-mdi-file">
                {t("settings.game.selectBtn")}
              </BetterButton>
            </div>
            <p className="mt-1 text-xs text-brand-500">
              {t("settings.game.magpiePathHint")}
            </p>
          </div>
        </div>
      </div>
    </>
  );
}
