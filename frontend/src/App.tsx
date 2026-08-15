import type { vo } from "../src/bindings/models";
import type { QuitSyncRequest } from "./hooks/useAppRuntimeEffects";
import { createRouter, RouterProvider } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { SafeQuit } from "../bindings/lunabox/internal/service/configservice";
import { InstallConfirmModal } from "./components/modal/InstallConfirmModal";
import { TimezoneSelectModal } from "./components/modal/TimezoneSelectModal";
import { UpdateDialog } from "./components/ui/UpdateDialog";
import { useAppRuntimeEffects } from "./hooks/useAppRuntimeEffects";
import { useAppTheme } from "./hooks/useAppTheme";
import { useAppZoom } from "./hooks/useAppZoom";
import { useCoverImageDownloadNotifications } from "./hooks/useCoverImageDownloadNotifications";
import { useDownloadNotifications } from "./hooks/useDownloadNotifications";
import { useExitSyncToast } from "./hooks/useExitSyncToast";
import { useUpdateCheck } from "./hooks/useUpdateCheck";
import { Route as rootRoute } from "./routes/__root";
import { Route as categoriesRoute } from "./routes/categories";
import { Route as categoryRoute } from "./routes/category";
import { Route as downloadsRoute } from "./routes/downloads";
import { Route as gameRoute } from "./routes/game";
import { Route as indexRoute } from "./routes/index";
import { Route as libraryRoute } from "./routes/library";
import { Route as settingsRoute } from "./routes/settings";
import { Route as statsRoute } from "./routes/stats";
import { useAppStore } from "./store";

const routeTree = rootRoute.addChildren([
  indexRoute,
  libraryRoute,
  gameRoute,
  statsRoute,
  categoriesRoute,
  categoryRoute,
  settingsRoute,
  downloadsRoute,
]);

const router = createRouter({
  routeTree,
  scrollRestoration: true,
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

function App() {
  const config = useAppStore(state => state.config);
  const fetchConfig = useAppStore(state => state.fetchConfig);
  const fetchHomeData = useAppStore(state => state.fetchHomeData);
  const fetchPlatformGOOS = useAppStore(state => state.fetchPlatformGOOS);
  const patchLiveConfig = useAppStore(state => state.patchLiveConfig);
  const {
    updateInfo,
    showUpdateDialog,
    setShowUpdateDialog,
    handleSkipVersion,
  } = useUpdateCheck();
  const [installRequest, setInstallRequest]
    = useState<vo.InstallRequest | null>(null);
  const [quitSyncRequest, setQuitSyncRequest]
    = useState<QuitSyncRequest | null>(null);
  const { i18n } = useTranslation();
  const showTimezoneModal = Boolean(
    config && (!config.time_zone || config.time_zone === ""),
  );
  const openGameLaunchSettings = (gameID: string) => {
    void router.navigate({ to: "/game/$gameId", params: { gameId: gameID } });
    window.setTimeout(() => {
      window.location.hash = "launch";
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    }, 0);
  };

  useEffect(() => {
    fetchConfig();
    fetchPlatformGOOS();
  }, [fetchConfig, fetchPlatformGOOS]);

  useEffect(() => {
    if (config?.language && i18n.language !== config.language) {
      i18n.changeLanguage(config.language);
    }
  }, [config, i18n]);

  const handleTimezoneConfirm = async (timezone: string) => {
    if (!config)
      return;

    await patchLiveConfig({ time_zone: timezone });

    // 延迟 500ms 后重启应用
    setTimeout(() => {
      SafeQuit();
    }, 500);
  };

  useAppTheme(config);
  useAppZoom({ config, patchLiveConfig });
  useAppRuntimeEffects({
    config,
    refreshConfig: fetchConfig,
    refreshHomeData: fetchHomeData,
    setInstallRequest,
    setQuitSyncRequest,
    openGameLaunchSettings,
  });
  useExitSyncToast({ quitSyncRequest });
  useDownloadNotifications(i18n);
  useCoverImageDownloadNotifications(i18n);

  return (
    <>
      <RouterProvider router={router} />
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
      <InstallConfirmModal
        request={installRequest}
        onClose={() => setInstallRequest(null)}
      />
    </>
  );
}

export default App;
