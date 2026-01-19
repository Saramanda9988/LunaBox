import type { appconf } from "../../wailsjs/go/models";
import { createRoute } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { GetVersionInfo } from "../../wailsjs/go/service/VersionService";
import { AISettingsPanel } from "../components/panel/AISettingsPanel";
import { AutoBackupSettingsPanel } from "../components/panel/AutoBackupSettingsPanel";
import { BasicSettingsPanel } from "../components/panel/BasicSettingsPanel";
import { CloudBackupSettingsPanel } from "../components/panel/CloudBackupSettingsPanel";
import { DBBackupPanel } from "../components/panel/DBBackupPanel";
import { GameSettingsPanel } from "../components/panel/GameSettingsPanel";
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
      return <div className="min-h-screen bg-brand-100 dark:bg-brand-900" />;
    }
    return <SettingsSkeleton />;
  }

  if (!config || !formData) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[80vh] space-y-4 text-brand-500">
        <div className="i-mdi-cog-outline text-6xl animate-spin-slow" />
        <p className="text-xl">正在准备设置...</p>
      </div>
    );
  }

  return (
    <div className={`space-y-6 max-w-8xl mx-auto p-8 transition-opacity duration-300 ${isLoading ? "opacity-50 pointer-events-none" : "opacity-100"}`}>
      <div className="flex items-center justify-between">
        <h1 className="text-4xl font-bold text-brand-900 dark:text-white">设置</h1>
      </div>

      <CollapsibleSection title="基础配置" icon="i-mdi-database-settings" defaultOpen={true}>
        <BasicSettingsPanel formData={formData} onChange={handleFormChange} />
      </CollapsibleSection>

      <CollapsibleSection title="游玩配置" icon="i-mdi-timer-play-outline" defaultOpen={false}>
        <GameSettingsPanel formData={formData} onChange={handleFormChange} />
      </CollapsibleSection>

      <CollapsibleSection title="云备份配置" icon="i-mdi-cloud-upload" defaultOpen={false}>
        <CloudBackupSettingsPanel formData={formData} onChange={handleFormChange} />
      </CollapsibleSection>

      <CollapsibleSection title="自动备份" icon="i-mdi-backup-restore" defaultOpen={false}>
        <AutoBackupSettingsPanel formData={formData} onChange={handleFormChange} />
      </CollapsibleSection>

      <CollapsibleSection title="AI 配置" icon="i-mdi-robot-happy" defaultOpen={false}>
        <AISettingsPanel formData={formData} onChange={handleFormChange} />
      </CollapsibleSection>

      <CollapsibleSection title="数据库备份" icon="i-mdi-database-refresh" defaultOpen={false}>
        <DBBackupPanel />
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
