import { createRootRoute, Outlet } from "@tanstack/react-router";
import { SideBar } from "../components/bar/SideBar";
import { TopBar } from "../components/bar/TopBar";
import { useAppStore } from "../store";

function RootLayout() {
  const { config } = useAppStore();

  // 背景图相关配置
  const bgEnabled = config?.background_enabled && config?.background_image;
  const bgBlur = config?.background_blur ?? 10;
  const bgOpacity = config?.background_opacity ?? 0.85;

  return (
    <div
      className="relative h-screen w-full overflow-hidden"
      data-glass={bgEnabled ? "true" : "false"}
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
      <div className="relative flex h-full w-full flex-col text-brand-900 dark:text-brand-100">
        {/* 顶部栏 */}
        <TopBar />

        {/* 内容区域 */}
        <div className="flex flex-1 overflow-hidden">
          <SideBar bgEnabled={!!bgEnabled} bgOpacity={bgOpacity} />
          <main
            className={`flex-1 overflow-auto ${
              bgEnabled ? "" : "bg-brand-100 dark:bg-brand-900"
            }`}
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
