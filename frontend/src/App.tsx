import type { vo } from "../wailsjs/go/models";
import type {
  ProcessSelectData,
  QuitSyncRequest,
} from "./hooks/useAppRuntimeEffects";
import { createRouter, RouterProvider } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { SafeQuit } from "../wailsjs/go/service/ConfigService";
import { InstallConfirmModal } from "./components/modal/InstallConfirmModal";
import { ProcessSelectModal } from "./components/modal/ProcessSelectModal";
import { TimezoneSelectModal } from "./components/modal/TimezoneSelectModal";
import { UpdateDialog } from "./components/ui/UpdateDialog";
import { useAppRuntimeEffects } from "./hooks/useAppRuntimeEffects";
import { useAppTheme } from "./hooks/useAppTheme";
import { useAppZoom } from "./hooks/useAppZoom";
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

const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

function App() {
  const config = useAppStore(state => state.config);
  const fetchConfig = useAppStore(state => state.fetchConfig);
  const fetchHomeData = useAppStore(state => state.fetchHomeData);
  const patchLiveConfig = useAppStore(state => state.patchLiveConfig);
  const {
    updateInfo,
    showUpdateDialog,
    setShowUpdateDialog,
    handleSkipVersion,
  } = useUpdateCheck();
  const [processSelectData, setProcessSelectData] = useState<ProcessSelectData>(
    { isOpen: false, gameID: "", launcherExeName: "" },
  );
  const [installRequest, setInstallRequest]
    = useState<vo.InstallRequest | null>(null);
  const [quitSyncRequest, setQuitSyncRequest]
    = useState<QuitSyncRequest | null>(null);
  const { i18n } = useTranslation();
  const showTimezoneModal = Boolean(
    config && (!config.time_zone || config.time_zone === ""),
  );

  useEffect(() => {
    fetchConfig();
  }, [fetchConfig]);

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
    refreshHomeData: fetchHomeData,
    setProcessSelectData,
    setInstallRequest,
    setQuitSyncRequest,
  });
  useExitSyncToast({ quitSyncRequest });
  useDownloadNotifications(i18n);

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
      <ProcessSelectModal
        isOpen={processSelectData.isOpen}
        gameID={processSelectData.gameID}
        launcherExeName={processSelectData.launcherExeName}
        onClose={() =>
          setProcessSelectData({
            isOpen: false,
            gameID: "",
            launcherExeName: "",
          })}
        onSelected={() =>
          setProcessSelectData({
            isOpen: false,
            gameID: "",
            launcherExeName: "",
          })}
      />
      <InstallConfirmModal
        request={installRequest}
        onClose={() => setInstallRequest(null)}
      />
    </>
  );
}

export default App;
