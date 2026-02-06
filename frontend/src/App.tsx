import { createRouter, RouterProvider } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { Toaster } from "react-hot-toast";
import { SafeQuit } from "../wailsjs/go/service/ConfigService";
import { EventsOff, EventsOn, WindowShow } from "../wailsjs/runtime/runtime";
import { ProcessSelectModal } from "./components/modal/ProcessSelectModal";
import { TimezoneSelectModal } from "./components/modal/TimezoneSelectModal";
import { UpdateDialog } from "./components/ui/UpdateDialog";
import { useUpdateCheck } from "./hooks/useUpdateCheck";
import { Route as rootRoute } from "./routes/__root";
import { Route as categoriesRoute } from "./routes/categories";
import { Route as categoryRoute } from "./routes/category";
import { Route as gameRoute } from "./routes/game";
import { Route as indexRoute } from "./routes/index";
import { Route as libraryRoute } from "./routes/library";
import { Route as settingsRoute } from "./routes/settings";
import { Route as statsRoute } from "./routes/stats";
import { useAppStore } from "./store";

const routeTree = rootRoute.addChildren([indexRoute, libraryRoute, gameRoute, statsRoute, categoriesRoute, categoryRoute, settingsRoute]);

const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

function App() {
  const { config, fetchConfig, updateConfig } = useAppStore();
  const { updateInfo, showUpdateDialog, setShowUpdateDialog, handleSkipVersion } = useUpdateCheck();
  const [showTimezoneModal, setShowTimezoneModal] = useState(false);
  const [processSelectData, setProcessSelectData] = useState<{
    isOpen: boolean;
    gameID: string;
    launcherExeName: string;
  }>({ isOpen: false, gameID: "", launcherExeName: "" });

  useEffect(() => {
    fetchConfig();
  }, [fetchConfig]);

  // 监听后端发送的进程选择事件
  useEffect(() => {
    const handleProcessSelectRequired = (data: { gameID: string; sessionID: string; launcherExeName: string }) => {
      console.warn("Process select required:", data);

      // 将窗口显示到前台
      WindowShow();

      setProcessSelectData({
        isOpen: true,
        gameID: data.gameID,
        launcherExeName: data.launcherExeName,
      });
    };

    EventsOn("process-select-required", handleProcessSelectRequired);

    return () => {
      EventsOff("process-select-required");
    };
  }, []);

  // 检查时区配置，如果未设置则显示选择弹窗
  useEffect(() => {
    if (config && (!config.time_zone || config.time_zone === "")) {
      setShowTimezoneModal(true);
    }
  }, [config]);

  const handleTimezoneConfirm = async (timezone: string) => {
    if (!config)
      return;

    // 更新配置
    const newConfig = { ...config, time_zone: timezone };
    await updateConfig(newConfig);

    // 关闭弹窗
    setShowTimezoneModal(false);

    // 延迟 500ms 后重启应用
    setTimeout(() => {
      SafeQuit();
    }, 500);
  };

  useEffect(() => {
    if (!config)
      return;

    const root = window.document.documentElement;
    const applyTheme = (theme: string) => {
      // 切换主题时临时禁用所有 transition，避免闪烁
      root.classList.add("theme-transitioning");
      root.classList.remove("light", "dark");
      root.classList.add(theme);

      // 在下一帧移除禁用类，让 hover 等交互恢复 transition
      requestAnimationFrame(() => {
        setTimeout(() => {
          root.classList.remove("theme-transitioning");
        }, 0);
      });
    };

    // 缓存主题设置到 localStorage，供下次启动时预加载
    localStorage.setItem("lunabox-theme", config.theme);

    if (config.theme === "system") {
      const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
      applyTheme(mediaQuery.matches ? "dark" : "light");

      const handler = (e: MediaQueryListEvent) => {
        applyTheme(e.matches ? "dark" : "light");
      };

      mediaQuery.addEventListener("change", handler);
      return () => mediaQuery.removeEventListener("change", handler);
    }
    else {
      applyTheme(config.theme);
    }
  }, [config?.theme]);

  // 配置加载完成后显示窗口
  useEffect(() => {
    if (config) {
      // 标记内容已准备好，触发淡入动画
      document.getElementById("root")?.classList.add("ready");
      // 显示窗口
      WindowShow();
    }
  }, [config]);

  return (
    <>
      <RouterProvider router={router} />
      <Toaster
        position="top-center"
        toastOptions={{
          duration: 3000,
          style: {
            background: "var(--toast-bg, #fff)",
            color: "var(--toast-color, #374151)",
          },
          success: {
            iconTheme: {
              primary: "#10b981",
              secondary: "#fff",
            },
          },
          error: {
            iconTheme: {
              primary: "#ef4444",
              secondary: "#fff",
            },
          },
        }}
      />
      {showUpdateDialog && updateInfo && (
        <UpdateDialog
          updateInfo={updateInfo}
          onClose={() => setShowUpdateDialog(false)}
          onSkip={handleSkipVersion}
        />
      )}
      <TimezoneSelectModal
        isOpen={showTimezoneModal}
        onConfirm={handleTimezoneConfirm}
      />
      <ProcessSelectModal
        isOpen={processSelectData.isOpen}
        gameID={processSelectData.gameID}
        launcherExeName={processSelectData.launcherExeName}
        onClose={() => setProcessSelectData({ isOpen: false, gameID: "", launcherExeName: "" })}
        onSelected={() => setProcessSelectData({ isOpen: false, gameID: "", launcherExeName: "" })}
      />
    </>
  );
}

export default App;
