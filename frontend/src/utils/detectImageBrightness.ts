import { FastAverageColor } from "fast-average-color";

/**
 * 检测图片的亮度，返回是否为亮色背景
 * @param imageUrl - 图片URL或路径
 * @returns Promise<boolean> - true 表示亮色背景，false 表示暗色背景
 */
export async function detectImageBrightness(imageUrl: string): Promise<boolean> {
  return new Promise((resolve, reject) => {
    const fac = new FastAverageColor();
    const img = new Image();
    img.crossOrigin = "Anonymous";

    img.onload = () => {
      try {
        const color = fac.getColor(img);

        // 计算相对亮度 (根据 WCAG 标准)
        // https://www.w3.org/TR/WCAG20-TECHS/G17.html
        const luminance = (0.299 * color.value[0] + 0.587 * color.value[1] + 0.114 * color.value[2]) / 255;

        // 亮度 > 0.5 认为是亮色背景
        fac.destroy();
        resolve(luminance > 0.5);
      }
      catch (error) {
        console.error("Failed to analyze image brightness:", error);
        fac.destroy();
        reject(error);
      }
    };

    img.onerror = (error) => {
      console.error("Failed to load image for brightness analysis");
      fac.destroy();
      reject(error);
    };

    img.src = imageUrl;
  });
}
