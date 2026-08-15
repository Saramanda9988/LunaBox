import type { appconf } from "../../../src/bindings/models";
import { useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  SaveCroppedBackgroundImage,
  SelectAndCropBackgroundImage,
} from "../../../bindings/lunabox/internal/service/configservice";
import { detectImageBrightness } from "../../utils/detectImageBrightness";
import { ImageCropperModal } from "../modal/ImageCropperModal";
import { BetterActionInput } from "../ui/better/BetterActionInput";
import { BetterNumberInput } from "../ui/better/BetterNumberInput";
import { BetterSelect } from "../ui/better/BetterSelect";
import { SettingSwitchRow } from "../ui/SettingSwitchRow";

interface BackgroundSettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

const DEFAULT_HOME_GAME_CAROUSEL_INTERVAL_SEC = 7;
const MIN_HOME_GAME_CAROUSEL_INTERVAL_SEC = 5;
const GAME_CARD_LAYOUTS = ["portrait", "landscape"] as const;

type GameCardLayout = (typeof GAME_CARD_LAYOUTS)[number];

export function BackgroundSettingsPanel({
  formData,
  onChange,
}: BackgroundSettingsProps) {
  const { t } = useTranslation();
  const [selectedImagePath, setSelectedImagePath] = useState<string>("");
  const [showCropper, setShowCropper] = useState(false);

  const handleSelectImage = async () => {
    try {
      const path = await SelectAndCropBackgroundImage();
      if (path) {
        setSelectedImagePath(path);
        setShowCropper(true);
      }
    }
    catch (err) {
      toast.error(
        t("settings.appearance.toast.selectFailed", {
          error: err instanceof Error ? err.message : String(err),
        }),
      );
      console.error("Failed to select background image:", err);
    }
  };

  const handleCropConfirm = async (crop: {
    x: number;
    y: number;
    width: number;
    height: number;
  }) => {
    try {
      const localPath = await SaveCroppedBackgroundImage(
        selectedImagePath,
        crop.x,
        crop.y,
        crop.width,
        crop.height,
      );

      if (localPath) {
        const isLight = await detectImageBrightness(localPath);
        onChange({
          ...formData,
          background_image: localPath,
          background_is_light: isLight,
        } as appconf.AppConfig);
      }

      setShowCropper(false);
      setSelectedImagePath("");
    }
    catch (err) {
      console.error("Failed to crop and save background image:", err);
      toast.error(
        t("settings.appearance.toast.cropFailed", {
          error: err instanceof Error ? err.message : String(err),
        }),
      );
    }
  };

  const handleCropCancel = () => {
    setShowCropper(false);
    setSelectedImagePath("");
  };

  const handleClearImage = () => {
    onChange({
      ...formData,
      background_image: "",
      background_enabled: false,
    } as appconf.AppConfig);
  };

  const handleBlurChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = Number.parseInt(e.target.value, 10);
    onChange({ ...formData, background_blur: value } as appconf.AppConfig);
  };

  const handleOpacityChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = Number.parseFloat(e.target.value);
    onChange({ ...formData, background_opacity: value } as appconf.AppConfig);
  };

  const handleCarouselIntervalChange = (value: number) => {
    onChange({
      ...formData,
      home_game_carousel_interval_sec: value,
    } as appconf.AppConfig);
  };

  const handleGameCardLayoutChange = (layout: GameCardLayout) => {
    onChange({
      ...formData,
      game_card_layout: layout,
    } as appconf.AppConfig);
  };

  const getFileName = (path: string) => {
    if (!path)
      return "";
    const parts = path.split(/[/\\]/);
    return parts[parts.length - 1];
  };

  const gameCardLayout: GameCardLayout
    = formData.game_card_layout === "landscape" ? "landscape" : "portrait";

  return (
    <>
      {/* Image Cropper Dialog */}
      {showCropper && selectedImagePath && (
        <ImageCropperModal
          imagePath={selectedImagePath}
          onConfirm={handleCropConfirm}
          onCancel={handleCropCancel}
          windowWidth={formData.window_width || 1134}
          windowHeight={formData.window_height || 750}
        />
      )}

      <div className="space-y-4">
        <section className="space-y-4" aria-labelledby="background-section">
          <div
            id="background-section"
            className="block text-sm font-semibold text-brand-700 dark:text-brand-300"
          >
            {t("settings.appearance.backgroundSection")}
          </div>

          <SettingSwitchRow
            id="background_enabled"
            label={t("settings.appearance.enableBg")}
            hint={t("settings.appearance.enableBgHint")}
            checked={formData.background_enabled || false}
            onCheckedChange={(checked) => {
              const newConfig = {
                ...formData,
                background_enabled: checked,
              } as appconf.AppConfig;
              if (checked && formData.background_is_light !== undefined) {
                newConfig.theme = formData.background_is_light
                  ? "light"
                  : "dark";
              }
              onChange(newConfig);
            }}
            disabled={!formData.background_image}
          />

          <div className="space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              {t("settings.appearance.bgImage")}
            </label>
            <BetterActionInput
              value={getFileName(formData.background_image || "")}
              placeholder={t("settings.appearance.noImageSelected")}
              readOnly
              className="truncate text-sm text-brand-600 dark:text-brand-400"
              actions={[
                {
                  ariaLabel: t("settings.appearance.selectBtn"),
                  icon: "i-mdi-image-search-outline",
                  onClick: handleSelectImage,
                },
                ...(formData.background_image
                  ? [
                      {
                        ariaLabel: t("settings.appearance.clearBtn"),
                        icon: "i-mdi-close",
                        onClick: handleClearImage,
                      },
                    ]
                  : []),
              ]}
            />
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("settings.appearance.bgImageHint")}
            </p>
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                {t("settings.appearance.blurLabel")}
              </label>
              <span className="text-sm text-brand-500 dark:text-brand-400">
                {formData.background_blur ?? 10}
                px
              </span>
            </div>
            <input
              type="range"
              min="0"
              max="30"
              step="1"
              value={formData.background_blur ?? 10}
              onChange={handleBlurChange}
              className="h-2 w-full cursor-pointer appearance-none rounded-lg bg-brand-200 accent-neutral-600 dark:bg-brand-700"
              disabled={!formData.background_image}
            />
            <div className="flex justify-between text-xs text-brand-400 dark:text-brand-500">
              <span>{t("settings.appearance.blurSharp")}</span>
              <span>{t("settings.appearance.blurBlurry")}</span>
            </div>
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                {t("settings.appearance.opacityLabel")}
              </label>
              <span className="text-sm text-brand-500 dark:text-brand-400">
                {Math.round((formData.background_opacity ?? 0.85) * 100)}
                %
              </span>
            </div>
            <input
              type="range"
              min="0.3"
              max="1"
              step="0.05"
              value={formData.background_opacity ?? 0.85}
              onChange={handleOpacityChange}
              className="h-2 w-full cursor-pointer appearance-none rounded-lg bg-brand-200 accent-neutral-600 dark:bg-brand-700"
              disabled={!formData.background_image}
            />
            <div className="flex justify-between text-xs text-brand-400 dark:text-brand-500">
              <span>{t("settings.appearance.opacityTransparent")}</span>
              <span>{t("settings.appearance.opacityOpaque")}</span>
            </div>
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("settings.appearance.opacityHint")}
            </p>
          </div>
        </section>

        <section
          className="space-y-4 border-t border-brand-200 pt-4 dark:border-brand-700"
          aria-labelledby="home-appearance-section"
        >
          <div
            id="home-appearance-section"
            className="block text-sm font-semibold text-brand-700 dark:text-brand-300"
          >
            {t("settings.appearance.homeSection")}
          </div>

          <SettingSwitchRow
            id="background_hide_game_cover"
            label={t("settings.appearance.hideGameCover")}
            hint={t("settings.appearance.hideGameCoverHint")}
            checked={formData.background_hide_game_cover || false}
            onCheckedChange={checked =>
              onChange({
                ...formData,
                background_hide_game_cover: checked,
              } as appconf.AppConfig)}
            disabled={!formData.background_enabled}
          />

          <SettingSwitchRow
            id="background_hide_game_hero_cover"
            label={t("settings.appearance.hideGameHeroCover")}
            hint={t("settings.appearance.hideGameHeroCoverHint")}
            checked={formData.background_hide_game_hero_cover || false}
            onCheckedChange={checked =>
              onChange({
                ...formData,
                background_hide_game_hero_cover: checked,
              } as appconf.AppConfig)}
            disabled={!formData.background_enabled}
          />

          <SettingSwitchRow
            id="home_game_carousel_enabled"
            label={t("settings.appearance.homeGameCarousel")}
            hint={t("settings.appearance.homeGameCarouselHint")}
            checked={formData.home_game_carousel_enabled !== false}
            onCheckedChange={checked =>
              onChange({
                ...formData,
                home_game_carousel_enabled: checked,
              } as appconf.AppConfig)}
          />

          <div className="space-y-2">
            <div className="flex items-center justify-between gap-4">
              <div className="flex-1 space-y-2">
                <label
                  htmlFor="home_game_carousel_interval_sec"
                  className="block text-sm font-medium text-brand-700 dark:text-brand-300"
                >
                  {t("settings.appearance.homeGameCarouselInterval")}
                </label>
                <p className="text-xs text-brand-500 dark:text-brand-400">
                  {t("settings.appearance.homeGameCarouselIntervalHint", {
                    seconds: MIN_HOME_GAME_CAROUSEL_INTERVAL_SEC,
                  })}
                </p>
              </div>
              <BetterNumberInput
                id="home_game_carousel_interval_sec"
                min={MIN_HOME_GAME_CAROUSEL_INTERVAL_SEC}
                step={1}
                value={
                  formData.home_game_carousel_interval_sec
                  || DEFAULT_HOME_GAME_CAROUSEL_INTERVAL_SEC
                }
                onValueChange={handleCarouselIntervalChange}
                disabled={formData.home_game_carousel_enabled === false}
                unit={t("settings.appearance.homeGameCarouselIntervalUnit")}
                size="sm"
                className="shrink-0"
              />
            </div>
          </div>
        </section>

        <section
          className="space-y-4 border-t border-brand-200 pt-4 dark:border-brand-700"
          aria-labelledby="library-appearance-section"
        >
          <div
            id="library-appearance-section"
            className="block text-sm font-semibold text-brand-700 dark:text-brand-300"
          >
            {t("settings.appearance.librarySection")}
          </div>

          <SettingSwitchRow
            id="blur_nsfw_game_covers"
            label={t("settings.appearance.blurNsfwCovers")}
            hint={t("settings.appearance.blurNsfwCoversHint")}
            checked={formData.blur_nsfw_game_covers !== false}
            onCheckedChange={checked =>
              onChange({
                ...formData,
                blur_nsfw_game_covers: checked,
              } as appconf.AppConfig)}
          />

          <div className="space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              {t("settings.appearance.gameCardLayout")}
            </label>
            <BetterSelect
              name="game_card_layout"
              value={gameCardLayout}
              onChange={value =>
                handleGameCardLayoutChange(value as GameCardLayout)}
              options={GAME_CARD_LAYOUTS.map(layout => ({
                value: layout,
                label: t(`settings.appearance.gameCardLayout_${layout}`),
              }))}
            />
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("settings.appearance.gameCardLayoutHint")}
            </p>
          </div>
        </section>
      </div>
    </>
  );
}
