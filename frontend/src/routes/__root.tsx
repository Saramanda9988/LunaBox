import { createRootRoute, Outlet } from "@tanstack/react-router";
import { SideBar } from "../components/bar/SideBar";
import { TopBar } from "../components/bar/TopBar";
import { useBackgroundBrightness } from "../hooks/useBackgroundBrightness";
import { useAppStore } from "../store";

function RootLayout() {
  const { config } = useAppStore();

  // 背景图相关配置
  const bgEnabled = config?.background_enabled && config?.background_image;
  const bgBlur = config?.background_blur ?? 10;
  const bgOpacity = config?.background_opacity ?? 0.85;

  // 检测背景图亮度
  const { isLight } = useBackgroundBrightness(config?.background_image, !!bgEnabled);

  // 智能决定文字颜色主题
  // isLight === true: 亮色背景 → 使用深色文字（移除 dark 类）
  // isLight === false: 暗色背景 → 使用浅色文字（保持 dark 类）
  // isLight === null: 未检测或未启用 → 使用系统主题
  const shouldUseDarkText = bgEnabled && isLight === true;
  const shouldUseLightText = bgEnabled && isLight === false;

  return (
    <div
      className={`relative h-screen w-full overflow-hidden ${bgEnabled ? "custom-bg-enabled" : ""} ${
        shouldUseDarkText ? "" : shouldUseLightText ? "dark" : ""
      }`}
      data-glass={bgEnabled ? "true" : "false"}
      data-bg-brightness={isLight === null ? "unknown" : isLight ? "light" : "dark"}
    >
      {/* 背景图层 */}
      {bgEnabled && (
        <div
          key={`bg-${bgBlur}-${config.background_image}`}
          className="absolute inset-0 bg-cover bg-center bg-no-repeat transition-all duration-300"
          style={{
            backgroundImage: `url("${config.background_image}")`,
            filter: `blur(${bgBlur}px)`,
            transform: "scale(1.1)", // 防止模糊边缘出现空白
          }}
        />
      )}

      {/* 主内容容器 */}
      <div
        className={`relative flex h-full w-full flex-col ${
          bgEnabled
            ? ""
            : "bg-brand-100 dark:bg-brand-900"
        } text-brand-900 dark:text-brand-100`}
      >
        {/* 顶部栏 */}
        <TopBar bgEnabled={!!bgEnabled} bgOpacity={bgOpacity} />

        {/* 内容区域 */}
        <div className="flex flex-1 overflow-hidden">
          <SideBar bgEnabled={!!bgEnabled} bgOpacity={bgOpacity} />
          <main
            className="flex-1 overflow-auto"
            style={bgEnabled ? {
              backgroundColor: `rgba(var(--main-bg-rgb), ${bgOpacity})`,
            } : undefined}
          >
            <Outlet />
          </main>
        </div>
      </div>
    </div>
  );
}

export const Route = createRootRoute({
  component: RootLayout,
});
