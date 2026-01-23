import { FastAverageColor } from "fast-average-color";
import { useEffect, useState } from "react";

/**
 * 检测背景图的亮度，返回是否为亮色背景
 * @param imageUrl - 背景图片URL
 * @param enabled - 是否启用背景图
 * @returns isLight - true 表示亮色背景（应使用深色文字），false 表示暗色背景（应使用浅色文字）
 */
export function useBackgroundBrightness(imageUrl: string | undefined, enabled: boolean) {
  const [isLight, setIsLight] = useState<boolean | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    if (!imageUrl || !enabled) {
      setIsLight(null);
      return;
    }

    const fac = new FastAverageColor();
    setIsLoading(true);

    // 创建临时图片元素进行分析
    const img = new Image();
    img.crossOrigin = "Anonymous";

    img.onload = () => {
      try {
        const color = fac.getColor(img);

        // 计算相对亮度 (根据 WCAG 标准)
        // https://www.w3.org/TR/WCAG20-TECHS/G17.html
        const luminance = (0.299 * color.value[0] + 0.587 * color.value[1] + 0.114 * color.value[2]) / 255;

        // 亮度 > 0.5 认为是亮色背景，应使用深色文字
        setIsLight(luminance > 0.5);
        setIsLoading(false);
      }
      catch (error) {
        console.error("Failed to analyze image brightness:", error);
        setIsLight(null);
        setIsLoading(false);
      }
    };

    img.onerror = () => {
      console.error("Failed to load image for brightness analysis");
      setIsLight(null);
      setIsLoading(false);
    };

    img.src = imageUrl;

    return () => {
      fac.destroy();
    };
  }, [imageUrl, enabled]);

  return { isLight, isLoading };
}
