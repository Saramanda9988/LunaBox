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
  const { config, fetchConfig, updateConfig } = useAppStore();
  const [formData, setFormData] = useState<appconf.AppConfig | null>(null);
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
    };
    init();
  }, [fetchConfig]);

  // 延迟显示骨架屏
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
    if (config && isInitialMount.current) {
      setFormData({ ...config } as appconf.AppConfig);
      isInitialMount.current = false;
    }
  }, [config]);

  // 自动保存逻辑
  useEffect(() => {
    if (!formData || isInitialMount.current)
      return;

    const hasChanges = JSON.stringify(formData) !== JSON.stringify(config);
    if (!hasChanges)
      return;

    const timer = setTimeout(() => {
      updateConfig(formData);
    }, 250);

    return () => clearTimeout(timer);
  }, [formData, updateConfig, config]);

  const handleFormChange = (newData: appconf.AppConfig) => {
    setFormData(newData);
  };

  if (isLoading && (!config || !formData)) {
    if (!showSkeleton) {
      return null;
    }
    return <SettingsSkeleton />;
  }

  if (!config || !formData) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[80vh] space-y-4 text-brand-500">
        <div className="i-mdi-cog-outline text-6xl animate-spin-slow" />
        <p className="text-xl">{t("settings.preparingSettings")}</p>
      </div>
    );
  }

  return (
    <div className={`space-y-6 max-w-8xl mx-auto p-8 transition-opacity duration-300 ${isLoading ? "opacity-50 pointer-events-none" : "opacity-100"}`}>
      <div className="flex items-center justify-between">
        <h1 className="text-4xl font-bold text-brand-900 dark:text-white">{t("settings.title")}</h1>
      </div>

      <CollapsibleSection title={t("settings.sections.basic")} icon="i-mdi-database-settings" defaultOpen={true}>
        <BasicSettingsPanel formData={formData} onChange={handleFormChange} />
      </CollapsibleSection>

      <CollapsibleSection title={t("settings.sections.appearance")} icon="i-mdi-palette" defaultOpen={false}>
        <BackgroundSettingsPanel formData={formData} onChange={handleFormChange} />
      </CollapsibleSection>

      <CollapsibleSection title={t("settings.sections.game")} icon="i-mdi-timer-play-outline" defaultOpen={false}>
        <GameSettingsPanel formData={formData} onChange={handleFormChange} />
      </CollapsibleSection>

      <CollapsibleSection title={t("settings.sections.download")} icon="i-mdi-download" defaultOpen={false}>
        <DownloadSettingsPanel formData={formData} onChange={handleFormChange} />
      </CollapsibleSection>

      <CollapsibleSection title={t("settings.sections.cloudBackup")} icon="i-mdi-cloud-upload" defaultOpen={false}>
        <CloudBackupSettingsPanel formData={formData} onChange={handleFormChange} />
      </CollapsibleSection>

      <CollapsibleSection title={t("settings.sections.autoBackup")} icon="i-mdi-backup-restore" defaultOpen={false}>
        <AutoBackupSettingsPanel formData={formData} onChange={handleFormChange} />
      </CollapsibleSection>

      <CollapsibleSection title={t("settings.sections.ai")} icon="i-mdi-robot-happy" defaultOpen={false}>
        <AISettingsPanel formData={formData} onChange={handleFormChange} />
      </CollapsibleSection>

      <CollapsibleSection title={t("settings.sections.dbBackup")} icon="i-mdi-database-refresh" defaultOpen={false}>
        <DBBackupPanel />
      </CollapsibleSection>

      <CollapsibleSection title={t("settings.sections.fullDataBackup")} icon="i-mdi-package-variant" defaultOpen={false}>
        <FullDataBackupPanel />
      </CollapsibleSection>

      <CollapsibleSection title={t("settings.sections.update")} icon="i-mdi-update" defaultOpen={false}>
        <UpdateSettingsPanel formData={formData} onChange={handleFormChange} />
      </CollapsibleSection>

      <div className="pt-4">
        <p className="text-xs text-brand-500 dark:text-brand-400">
          Lunabox made with LunaRain_079 &amp; Contributors.
        </p>
        {versionInfo && (
          <p className="text-xs text-brand-400 dark:text-brand-500 mt-1">
            Version
            {" "}
            {versionInfo.version}
            {" "}
            (
            {versionInfo.commit}
            ) |
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
