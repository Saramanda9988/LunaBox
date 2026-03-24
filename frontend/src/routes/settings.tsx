import type { appconf } from "../../wailsjs/go/models";
import { createRoute } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";

import { useTranslation } from "react-i18next";

import { GetVersionInfo } from "../../wailsjs/go/service/VersionService";
import { AISettingsPanel } from "../components/panel/AISettingsPanel";
import { AutoBackupSettingsPanel } from "../components/panel/AutoBackupSettingsPanel";
import { BackgroundSettingsPanel } from "../components/panel/BackgroundSettingsPanel";
import { BasicSettingsPanel } from "../components/panel/BasicSettingsPanel";
import { CloudBackupSettingsPanel } from "../components/panel/CloudBackupSettingsPanel";
import { DBBackupPanel } from "../components/panel/DBBackupPanel";
import { DownloadSettingsPanel } from "../components/panel/DownloadSettingsPanel";
import { FullDataBackupPanel } from "../components/panel/FullDataBackupPanel";
import { GameSettingsPanel } from "../components/panel/GameSettingsPanel";
import { MetadataSettingsPanel } from "../components/panel/MetadataSettingsPanel";
import { UpdateSettingsPanel } from "../components/panel/UpdateSettingsPanel";
import { SettingsSkeleton } from "../components/skeleton/SettingsSkeleton";
import { CollapsibleSection } from "../components/ui/CollapsibleSection";
import { useAppStore } from "../store";
import { Route as rootRoute } from "./__root";

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/settings",
  component: SettingsPage,
});

function SettingsPage() {
  const { t } = useTranslation();
  const config = useAppStore(state => state.config);
  const draftConfig = useAppStore(state => state.draftConfig);
  const fetchConfig = useAppStore(state => state.fetchConfig);
  const patchLiveConfig = useAppStore(state => state.patchLiveConfig);
  const resetDraftConfig = useAppStore(state => state.resetDraftConfig);
  const saveDraftConfig = useAppStore(state => state.saveDraftConfig);
  const setDraftConfig = useAppStore(state => state.setDraftConfig);
  const [isLoading, setIsLoading] = useState(true);
  const [showSkeleton, setShowSkeleton] = useState(false);
  const [versionInfo, setVersionInfo] = useState<Record<string, string> | null>(null);
  const isInitialMount = useRef(true);

  useEffect(() => {
    const init = async () => {
      setIsLoading(true);
      await fetchConfig();
      try {
        const info = await GetVersionInfo();
        setVersionInfo(info);
      }
      catch (err) {
        console.error("Failed to fetch version info:", err);
      }
      setIsLoading(false);
      isInitialMount.current = false;
    };
    init();
  }, [fetchConfig]);

  useEffect(() => {
    let timer: number;
    if (isLoading) {
      timer = window.setTimeout(() => {
        setShowSkeleton(true);
      }, 300);
    }
    else {
      setShowSkeleton(false);
    }
    return () => clearTimeout(timer);
  }, [isLoading]);

  useEffect(() => {
    if (!config || isInitialMount.current) {
      return;
    }

    resetDraftConfig();
  }, [config, resetDraftConfig]);

  useEffect(() => {
    if (!draftConfig || !config || isInitialMount.current) {
      return;
    }

    const hasChanges = JSON.stringify(draftConfig) !== JSON.stringify(config);
    if (!hasChanges) {
      return;
    }

    const timer = setTimeout(() => {
      void saveDraftConfig();
    }, 250);

    return () => clearTimeout(timer);
  }, [config, draftConfig, saveDraftConfig]);

  const handleDraftChange = (newData: appconf.AppConfig) => {
    setDraftConfig(newData);
  };

  const handleZoomChange = (zoomFactor: number) => {
    void patchLiveConfig({ window_zoom_factor: zoomFactor });
  };

  if (isLoading && (!config || !draftConfig)) {
    if (!showSkeleton) {
      return null;
    }
    return <SettingsSkeleton />;
  }

  if (!config || !draftConfig) {
    return (
      <div className="flex min-h-[80vh] flex-col items-center justify-center space-y-4 text-brand-500">
        <div className="i-mdi-cog-outline animate-spin-slow text-6xl" />
        <p className="text-xl">{t("settings.preparingSettings")}</p>
      </div>
    );
  }

  return (
    <div className={`mx-auto max-w-8xl space-y-6 p-8 transition-opacity duration-300 ${isLoading ? "pointer-events-none opacity-50" : "opacity-100"}`}>
      <div className="flex items-center justify-between">
        <h1 className="text-4xl font-bold text-brand-900 dark:text-white">{t("settings.title")}</h1>
      </div>

      <div className="mx-auto w-full max-w-5xl space-y-6">
        <CollapsibleSection title={t("settings.sections.basic")} icon="i-mdi-database-settings" defaultOpen={true}>
          <BasicSettingsPanel formData={draftConfig} onChange={handleDraftChange} onZoomChange={handleZoomChange} />
        </CollapsibleSection>

        <CollapsibleSection title={t("settings.sections.metadata")} icon="i-mdi-database-search" defaultOpen={false}>
          <MetadataSettingsPanel formData={draftConfig} onChange={handleDraftChange} />
        </CollapsibleSection>

        <CollapsibleSection title={t("settings.sections.appearance")} icon="i-mdi-palette" defaultOpen={false}>
          <BackgroundSettingsPanel formData={draftConfig} onChange={handleDraftChange} />
        </CollapsibleSection>

        <CollapsibleSection title={t("settings.sections.game")} icon="i-mdi-timer-play-outline" defaultOpen={false}>
          <GameSettingsPanel formData={draftConfig} onChange={handleDraftChange} />
        </CollapsibleSection>

        <CollapsibleSection title={t("settings.sections.download")} icon="i-mdi-download" defaultOpen={false}>
          <DownloadSettingsPanel formData={draftConfig} onChange={handleDraftChange} />
        </CollapsibleSection>

        <CollapsibleSection title={t("settings.sections.cloudBackup")} icon="i-mdi-cloud-upload" defaultOpen={false}>
          <CloudBackupSettingsPanel formData={draftConfig} onChange={handleDraftChange} />
        </CollapsibleSection>

        <CollapsibleSection title={t("settings.sections.autoBackup")} icon="i-mdi-backup-restore" defaultOpen={false}>
          <AutoBackupSettingsPanel formData={draftConfig} onChange={handleDraftChange} />
        </CollapsibleSection>

        <CollapsibleSection title={t("settings.sections.ai")} icon="i-mdi-robot-happy" defaultOpen={false}>
          <AISettingsPanel formData={draftConfig} onChange={handleDraftChange} />
        </CollapsibleSection>

        <CollapsibleSection title={t("settings.sections.dbBackup")} icon="i-mdi-database-refresh" defaultOpen={false}>
          <DBBackupPanel />
        </CollapsibleSection>

        <CollapsibleSection title={t("settings.sections.fullDataBackup")} icon="i-mdi-package-variant" defaultOpen={false}>
          <FullDataBackupPanel />
        </CollapsibleSection>

        <CollapsibleSection title={t("settings.sections.update")} icon="i-mdi-update" defaultOpen={false}>
          <UpdateSettingsPanel formData={draftConfig} onChange={handleDraftChange} />
        </CollapsibleSection>
      </div>

      <div className="pt-4">
        <p className="text-xs text-brand-500 dark:text-brand-400">
          Lunabox made with LunaRain_079 &amp; Contributors.
        </p>
        {versionInfo && (
          <p className="mt-1 text-xs text-brand-400 dark:text-brand-500">
            Version
            {" "}
            {versionInfo.version}
            {" "}
            (
            {versionInfo.commit}
            )
            |
            {" "}
            {versionInfo.buildMode}
            {" "}
            | Built at
            {" "}
            {versionInfo.buildTime}
          </p>
        )}
      </div>
    </div>
  );
}
