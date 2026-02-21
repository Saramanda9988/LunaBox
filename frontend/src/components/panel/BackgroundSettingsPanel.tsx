import type { appconf } from "../../../wailsjs/go/models";
import { useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { SaveCroppedBackgroundImage, SelectAndCropBackgroundImage } from "../../../wailsjs/go/service/ConfigService";
import { detectImageBrightness } from "../../utils/detectImageBrightness";
import { ImageCropperModal } from "../modal/ImageCropperModal";
import { BetterSwitch } from "../ui/BetterSwitch";

interface BackgroundSettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

export function BackgroundSettingsPanel({ formData, onChange }: BackgroundSettingsProps) {
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
      toast.error(t("settings.appearance.toast.selectFailed", { error: err instanceof Error ? err.message : String(err) }));
      console.error("Failed to select background image:", err);
    }
  };

  const handleCropConfirm = async (crop: { x: number; y: number; width: number; height: number }) => {
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
      toast.error(t("settings.appearance.toast.cropFailed", { error: err instanceof Error ? err.message : String(err) }));
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

  const getFileName = (path: string) => {
    if (!path)
      return "";
    const parts = path.split(/[/\\]/);
    return parts[parts.length - 1];
  };

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

      {/* Enable Toggle */}
      <div className="flex items-center justify-between p-2">
        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("settings.appearance.enableBg")}
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
            {t("settings.appearance.enableBgHint")}
          </p>
        </div>
        <BetterSwitch
          id="background_enabled"
          checked={formData.background_enabled || false}
          onCheckedChange={(checked) => {
            const newConfig = { ...formData, background_enabled: checked } as appconf.AppConfig;
            if (checked && formData.background_is_light !== undefined) {
              newConfig.theme = formData.background_is_light ? "light" : "dark";
            }
            onChange(newConfig);
          }}
          disabled={!formData.background_image}
        />
      </div>

      {/* Hide Game Cover Toggle */}
      <div className="flex items-center justify-between p-2">
        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("settings.appearance.hideGameCover")}
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
            {t("settings.appearance.hideGameCoverHint")}
          </p>
        </div>
        <BetterSwitch
          id="background_hide_game_cover"
          checked={formData.background_hide_game_cover || false}
          onCheckedChange={checked => onChange({ ...formData, background_hide_game_cover: checked } as appconf.AppConfig)}
          disabled={!formData.background_enabled}
        />
      </div>

      {/* Background Image Selection */}
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.appearance.bgImage")}
        </label>
        <div className="flex gap-2">
          <div className="flex-1 flex items-center px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-brand-50 dark:bg-brand-800 text-sm text-brand-600 dark:text-brand-400 truncate">
            {formData.background_image ? getFileName(formData.background_image) : t("settings.appearance.noImageSelected")}
          </div>
          <button
            type="button"
            onClick={handleSelectImage}
            className="glass-btn-neutral px-4 py-2 bg-neutral-600 text-white rounded-md hover:bg-neutral-700 transition-colors text-sm font-medium"
          >
            {t("settings.appearance.selectBtn")}
          </button>
          {formData.background_image && (
            <button
              type="button"
              onClick={handleClearImage}
              className="glass-btn-error px-4 py-2 bg-error-500 text-white rounded-md hover:bg-error-600 transition-colors text-sm font-medium"
            >
              {t("settings.appearance.clearBtn")}
            </button>
          )}
        </div>
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.appearance.bgImageHint")}
        </p>
      </div>

      {/* Background Image Preview */}
      {formData.background_image && (
        <div className="space-y-2">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("settings.appearance.preview")}
          </label>
          <div className="relative w-full h-40 rounded-lg overflow-hidden border border-brand-300 dark:border-brand-600">
            <img
              src={formData.background_image}
              alt={t("settings.appearance.bgPreviewAlt")}
              className="w-full h-full object-cover"
              style={{
                filter: `blur(${(formData.background_blur ?? 10) / 2}px)`,
              }}
              draggable="false"
              onDragStart={e => e.preventDefault()}
            />
            <div
              className="absolute inset-0 bg-brand-100 dark:bg-brand-900"
              style={{
                opacity: formData.background_opacity ?? 0.85,
              }}
            />
            <div className="absolute inset-0 flex items-center justify-center">
              <span className="text-brand-700 dark:text-brand-300 text-sm font-medium">
                {t("settings.appearance.effectPreview")}
              </span>
            </div>
          </div>
        </div>
      )}

      {/* Blur Adjustment */}
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
          className="w-full h-2 bg-brand-200 dark:bg-brand-700 rounded-lg appearance-none cursor-pointer accent-neutral-600"
          disabled={!formData.background_image}
        />
        <div className="flex justify-between text-xs text-brand-400 dark:text-brand-500">
          <span>{t("settings.appearance.blurSharp")}</span>
          <span>{t("settings.appearance.blurBlurry")}</span>
        </div>
      </div>

      {/* Opacity Adjustment */}
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
          className="w-full h-2 bg-brand-200 dark:bg-brand-700 rounded-lg appearance-none cursor-pointer accent-neutral-600"
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
    </>
  );
}
