import type { appconf } from "../../../wailsjs/go/models";
import { useState } from "react";
import toast from "react-hot-toast";
import { SaveCroppedBackgroundImage, SelectAndCropBackgroundImage } from "../../../wailsjs/go/service/ConfigService";
import { detectImageBrightness } from "../../utils/detectImageBrightness";
import { ImageCropperModal } from "../modal/ImageCropperModal";
import { BetterSwitch } from "../ui/BetterSwitch";

interface BackgroundSettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

export function BackgroundSettingsPanel({ formData, onChange }: BackgroundSettingsProps) {
  const [selectedImagePath, setSelectedImagePath] = useState<string>("");
  const [showCropper, setShowCropper] = useState(false);

  const handleSelectImage = async () => {
    try {
      const path = await SelectAndCropBackgroundImage();
      if (path) {
        // 打开裁剪对话框
        setSelectedImagePath(path);
        setShowCropper(true);
      }
    }
    catch (err) {
      toast.error(`选择背景图片失败: ${err instanceof Error ? err.message : String(err)}`);
      console.error("Failed to select background image:", err);
    }
  };

  const handleCropConfirm = async (crop: { x: number; y: number; width: number; height: number }) => {
    try {
      // 调用后端裁剪并保存图片
      const localPath = await SaveCroppedBackgroundImage(
        selectedImagePath,
        crop.x,
        crop.y,
        crop.width,
        crop.height,
      );

      if (localPath) {
        // 立即检测图片亮度并缓存
        const isLight = await detectImageBrightness(localPath);
        onChange({
          ...formData,
          background_image: localPath,
          background_is_light: isLight,
        } as appconf.AppConfig);
      }

      // 关闭裁剪对话框
      setShowCropper(false);
      setSelectedImagePath("");
    }
    catch (err) {
      console.error("Failed to crop and save background image:", err);
      toast.error(`裁剪保存失败: ${err instanceof Error ? err.message : String(err)}`);
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

  // 从完整路径中提取文件名
  const getFileName = (path: string) => {
    if (!path)
      return "";
    const parts = path.split(/[/\\]/);
    return parts[parts.length - 1];
  };

  return (
    <>
      {/* 图片裁剪对话框 */}
      {showCropper && selectedImagePath && (
        <ImageCropperModal
          imagePath={selectedImagePath}
          onConfirm={handleCropConfirm}
          onCancel={handleCropCancel}
          windowWidth={formData.window_width || 1134}
          windowHeight={formData.window_height || 750}
        />
      )}

      {/* 启用开关 */}
      <div className="flex items-center justify-between p-2">
        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            启用自定义背景
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
            开启后将使用自定义图片作为应用背景，并自动调整主题配色
          </p>
        </div>
        <BetterSwitch
          id="background_enabled"
          checked={formData.background_enabled || false}
          onCheckedChange={(checked) => {
            const newConfig = { ...formData, background_enabled: checked } as appconf.AppConfig;

            // 启用背景图时，根据缓存的亮度自动切换主题
            if (checked && formData.background_is_light !== undefined) {
              // 暗色背景 → 使用暗色主题
              // 亮色背景 → 使用亮色主题
              newConfig.theme = formData.background_is_light ? "light" : "dark";
            }

            onChange(newConfig);
          }}
          disabled={!formData.background_image}
        />
      </div>

      {/* 隐藏游戏封面开关 */}
      <div className="flex items-center justify-between p-2">
        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            隐藏首页游戏封面
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
            启用自定义背景时，隐藏首页上一次游玩游戏的封面图片
          </p>
        </div>
        <BetterSwitch
          id="background_hide_game_cover"
          checked={formData.background_hide_game_cover || false}
          onCheckedChange={checked => onChange({ ...formData, background_hide_game_cover: checked } as appconf.AppConfig)}
          disabled={!formData.background_enabled}
        />
      </div>

      {/* 背景图片选择 */}
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          背景图片
        </label>
        <div className="flex gap-2">
          <div className="flex-1 flex items-center px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-brand-50 dark:bg-brand-800 text-sm text-brand-600 dark:text-brand-400 truncate">
            {formData.background_image ? getFileName(formData.background_image) : "未选择图片"}
          </div>
          <button
            type="button"
            onClick={handleSelectImage}
            className="glass-btn-neutral px-4 py-2 bg-neutral-600 text-white rounded-md hover:bg-neutral-700 transition-colors text-sm font-medium"
          >
            选择
          </button>
          {formData.background_image && (
            <button
              type="button"
              onClick={handleClearImage}
              className="glass-btn-error px-4 py-2 bg-error-500 text-white rounded-md hover:bg-error-600 transition-colors text-sm font-medium"
            >
              清除
            </button>
          )}
        </div>
        <p className="text-xs text-brand-500 dark:text-brand-400">
          支持 PNG、JPG、JPEG、GIF、WebP、BMP 格式，选择后可裁剪所需区域
        </p>
      </div>

      {/* 背景图片预览 */}
      {formData.background_image && (
        <div className="space-y-2">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            预览
          </label>
          <div className="relative w-full h-40 rounded-lg overflow-hidden border border-brand-300 dark:border-brand-600">
            <img
              src={formData.background_image}
              alt="背景预览"
              className="w-full h-full object-cover"
              style={{
                filter: `blur(${(formData.background_blur ?? 10) / 2}px)`,
              }}
            />
            <div
              className="absolute inset-0 bg-brand-100 dark:bg-brand-900"
              style={{
                opacity: formData.background_opacity ?? 0.85,
              }}
            />
            <div className="absolute inset-0 flex items-center justify-center">
              <span className="text-brand-700 dark:text-brand-300 text-sm font-medium">
                效果预览
              </span>
            </div>
          </div>
        </div>
      )}

      {/* 模糊度调节 */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            背景模糊度
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
          <span>清晰</span>
          <span>模糊</span>
        </div>
      </div>

      {/* 遮罩不透明度调节 */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            遮罩不透明度
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
          <span>透明</span>
          <span>不透明</span>
        </div>
        <p className="text-xs text-brand-500 dark:text-brand-400">
          调节内容区域的遮罩层不透明度，数值越低背景图越明显
        </p>
      </div>
    </>
  );
}
