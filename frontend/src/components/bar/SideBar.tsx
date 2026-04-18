import { Link } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import { useCloudSync } from "../../hooks/useCloudSync";
import { useAppStore } from "../../store";
import {
  formatCloudSyncTime,
  getCloudSyncStatusClass,
  getCloudSyncStatusLabel,
} from "../../utils/cloudSync";

interface SideBarProps {
  bgEnabled?: boolean;
  bgOpacity?: number;
}

export function SideBar({ bgEnabled = false, bgOpacity = 0.85 }: SideBarProps) {
  const { t } = useTranslation();
  const config = useAppStore(state => state.config);
  const isSidebarOpen = useAppStore(state => state.isSidebarOpen);
  const toggleSidebar = useAppStore(state => state.toggleSidebar);
  const [activeDownloads, setActiveDownloads] = useState(0);
  const {
    canSyncNow,
    effectiveSyncStatus,
    handleSyncNow,
    refreshSyncStatus,
    syncBusy,
    syncConfigured,
  } = useCloudSync({ config });

  // 监听下载进度事件，统计进行中的任务数
  useEffect(() => {
    const counts: Record<string, string> = {};
    const unsubscribe = EventsOn(
      "download:progress",
      (evt: { id: string; status: string }) => {
        counts[evt.id] = evt.status;
        const active = Object.values(counts).filter(
          s => s === "downloading" || s === "pending" || s === "extracting",
        ).length;
        setActiveDownloads(active);
      },
    );
    return unsubscribe;
  }, []);

  const navItems = [
    { to: "/", label: t("sideBar.home"), icon: "i-mdi-home" },
    {
      to: "/library",
      label: t("sideBar.library"),
      icon: "i-mdi-gamepad-variant",
    },
    { to: "/stats", label: t("sideBar.stats"), icon: "i-mdi-chart-bar" },
    {
      to: "/categories",
      label: t("sideBar.categories"),
      icon: "i-mdi-format-list-bulleted",
    },
  ];

  const sidebarBgClass = bgEnabled
    ? "border-r border-white/20 dark:border-white/10"
    : "bg-white dark:bg-brand-800 border-r border-brand-200 dark:border-brand-700";

  const sidebarStyle = bgEnabled
    ? {
        backgroundColor: `rgba(var(--sidebar-bg-rgb), ${bgOpacity})`,
        transition: "width 300ms ease",
        width: isSidebarOpen ? "16rem" : "4rem",
      }
    : {
        transition: "width 300ms ease",
        width: isSidebarOpen ? "16rem" : "4rem",
      };
  const navItemClass
    = "flex items-center rounded-xl p-2 text-brand-700 no-underline transition-colors hover:bg-brand-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400/70 dark:text-brand-300 dark:hover:bg-brand-700 [&.active]:bg-brand-200 [&.active]:text-brand-900 dark:[&.active]:bg-brand-700 dark:[&.active]:text-brand-100 data-glass:hover:bg-white/10 data-glass:hover:dark:bg-black/10 data-glass:[&.active]:bg-white/20 data-glass:[&.active]:dark:bg-black/20";
  const footerActionClass
    = "relative flex items-center justify-center rounded-xl p-2.5 text-brand-700 transition-colors hover:bg-brand-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400/70 dark:text-brand-300 dark:hover:bg-brand-700 data-glass:hover:bg-white/10 data-glass:hover:dark:bg-black/10";
  const cloudServiceEnabled = Boolean(config?.cloud_backup_enabled);
  const cloudSyncEnabled = Boolean(
    cloudServiceEnabled && config?.cloud_sync_enabled,
  );
  const cloudSyncStatusLabel = getCloudSyncStatusLabel(
    effectiveSyncStatus.last_sync_status,
    t,
  );
  const cloudSyncStatusClass = getCloudSyncStatusClass(
    effectiveSyncStatus.last_sync_status,
  );
  const cloudSyncLastTime = formatCloudSyncTime(
    effectiveSyncStatus.last_sync_time,
    config?.time_zone,
    t("settings.cloudBackup.syncNever"),
  );
  const cloudSyncIconClass = (() => {
    if (!cloudServiceEnabled) {
      return "i-mdi-cloud-off-outline text-brand-400 dark:text-brand-500";
    }

    if (!cloudSyncEnabled) {
      return "i-mdi-cloud-outline text-brand-400 dark:text-brand-500";
    }

    if (syncBusy) {
      return "i-mdi-loading animate-spin";
    }

    switch (effectiveSyncStatus.last_sync_status) {
      case "success":
        return "i-mdi-cloud-check-outline text-success-500";
      case "failed":
        return "i-mdi-cloud-alert-outline text-error-500";
      default:
        return "i-mdi-cloud-sync-outline";
    }
  })();

  const handleCloudSyncClick = () => {
    if (!canSyncNow) {
      return;
    }

    void handleSyncNow();
  };

  return (
    <aside
      className={`relative z-30 flex shrink-0 flex-col ${sidebarBgClass}`}
      style={sidebarStyle}
    >
      <div
        className={`flex items-center h-16 px-3 ${bgEnabled ? "border-white/20 dark:border-white/10" : "border-brand-200 dark:border-brand-700"}`}
      >
        <div
          className={`flex items-center gap-1 select-none overflow-hidden transition-all duration-300 ${isSidebarOpen ? "opacity-100 max-w-[200px] pl-1" : "opacity-0 max-w-0 pl-0"}`}
        >
          <img
            src="/appicon.png"
            className="w-8 h-8 dark:hidden pointer-events-none shrink-0"
            draggable="false"
          />
          <img
            src="/appicon-dark.png"
            className="w-8 h-8 hidden dark:block pointer-events-none shrink-0"
            draggable="false"
          />
          <img
            src="/topbar-title-dark.png"
            className="h-6 dark:hidden pointer-events-none shrink-0"
          />
          <img
            src="/topbar-title.png"
            className="h-6 hidden dark:block pointer-events-none shrink-0"
          />
        </div>
        <div
          className={`flex-1 flex items-center min-w-[40px] ${isSidebarOpen ? "justify-end" : "justify-center"}`}
        >
          <button
            type="button"
            onClick={toggleSidebar}
            className="shrink-0 rounded-xl p-2 text-brand-700 transition-colors hover:bg-brand-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400/70 dark:text-brand-300 dark:hover:bg-brand-700 select-none data-glass:hover:bg-white/10 data-glass:hover:dark:bg-black/10"
            aria-label={t("sideBar.toggle")}
            onDragStart={e => e.preventDefault()}
          >
            <div
              className="i-mdi-menu text-xl pointer-events-none"
              aria-hidden="true"
            />
          </button>
        </div>
      </div>

      <nav className="flex-1 py-4">
        <ul className="space-y-2 px-2">
          {navItems.map(item => (
            <li key={item.to}>
              <Link
                to={item.to}
                className={`${navItemClass} select-none`}
                onDragStart={e => e.preventDefault()}
              >
                <div className="relative shrink-0 flex items-center justify-center w-8 h-8">
                  <div
                    className={`${item.icon} text-xl pointer-events-none`}
                    aria-hidden="true"
                  />
                </div>
                <div
                  className={`overflow-hidden transition-all duration-300 flex items-center ${isSidebarOpen ? "w-[120px] ml-2 opacity-100" : "w-0 ml-0 opacity-0"}`}
                >
                  <span className="pointer-events-none whitespace-nowrap shrink-0 break-keep">
                    {item.label}
                  </span>
                </div>
              </Link>
            </li>
          ))}
        </ul>
      </nav>

      <div
        className={`p-4 ${bgEnabled ? "border-white/20 dark:border-white/10" : "border-brand-200 dark:border-brand-700"} flex items-center ${isSidebarOpen ? "justify-end gap-1" : "flex-col gap-2"}`}
      >
        <div
          className="group relative"
          onMouseEnter={() => {
            if (cloudSyncEnabled) {
              void refreshSyncStatus();
            }
          }}
          onFocusCapture={() => {
            if (cloudSyncEnabled) {
              void refreshSyncStatus();
            }
          }}
        >
          <button
            type="button"
            onClick={handleCloudSyncClick}
            disabled={!cloudSyncEnabled}
            aria-disabled={!canSyncNow || !cloudSyncEnabled}
            aria-label={t("sideBar.cloudSync")}
            title={t("sideBar.cloudSync")}
            className={`${footerActionClass} ${canSyncNow && cloudSyncEnabled ? "" : "opacity-75"} ${syncBusy ? "cursor-wait" : ""}`}
          >
            <div className="relative shrink-0">
              <div
                className={`${cloudSyncIconClass} text-xl pointer-events-none`}
                aria-hidden="true"
              />
            </div>
          </button>

          {cloudSyncEnabled && (
            <div
              role="tooltip"
              aria-live="polite"
              className={`pointer-events-none absolute z-50 w-44 opacity-0 transition-all duration-200 group-hover:pointer-events-auto group-hover:opacity-100 group-focus-within:pointer-events-auto group-focus-within:opacity-100 ${
                isSidebarOpen
                  ? "bottom-full left-1/2 mb-3 -translate-x-1/2 translate-y-2 group-hover:-translate-x-1/2 group-hover:translate-y-0 group-focus-within:-translate-x-1/2 group-focus-within:translate-y-0"
                  : "bottom-0 left-full ml-3 translate-y-0 translate-x-2 group-hover:translate-x-0 group-hover:translate-y-0 group-focus-within:translate-x-0 group-focus-within:translate-y-0"
              }`}
            >
              <div className="glass-panel flex flex-col gap-2 rounded-lg border border-brand-200/80 bg-white/92 p-2.5 shadow-lg backdrop-blur-xl dark:border-brand-700/80 dark:bg-brand-900/88 data-glass:bg-white/78 data-glass:dark:bg-black/42">
                <div className="flex flex-col items-start gap-1.5">
                  <span className="text-[10px] font-medium text-brand-500 dark:text-brand-400 whitespace-nowrap">
                    {t("sideBar.cloudSync")}
                  </span>
                  <div className="flex items-center gap-1.5">
                    {!syncConfigured && (
                      <span className="text-[9px] font-medium text-warning-600 dark:text-warning-400 whitespace-normal break-words">
                        {t("settings.cloudBackup.syncNotConfigured")}
                      </span>
                    )}
                    <span
                      className={`rounded-full px-1.5 py-0.5 text-[9px] font-semibold text-center whitespace-normal break-words ${cloudSyncStatusClass}`}
                    >
                      {cloudSyncStatusLabel}
                    </span>
                  </div>
                </div>
                <div className="flex flex-col items-start gap-1">
                  <span className="text-brand-400 dark:text-brand-500 text-[10px] whitespace-nowrap">
                    {t("settings.cloudBackup.syncLastTimeLabel")}
                  </span>
                  <span className="text-[10px] font-medium text-brand-700 dark:text-brand-100 whitespace-nowrap">
                    {cloudSyncLastTime}
                  </span>
                </div>
                {effectiveSyncStatus.last_sync_error && (
                  <p className="mt-0.5 max-w-[12rem] whitespace-normal break-words text-[9px] leading-3 text-error-600 dark:text-error-400">
                    {effectiveSyncStatus.last_sync_error}
                  </p>
                )}
              </div>
            </div>
          )}
        </div>

        <Link
          to="/downloads"
          className={`${footerActionClass} no-underline [&.active]:bg-brand-200 [&.active]:text-brand-900 dark:[&.active]:bg-brand-700 dark:[&.active]:text-brand-100 select-none data-glass:[&.active]:bg-white/20 data-glass:[&.active]:dark:bg-black/20`}
          title={t("sideBar.downloads", "下载")}
          onDragStart={e => e.preventDefault()}
        >
          <div className="relative shrink-0">
            <div
              className="i-mdi-download text-xl pointer-events-none"
              aria-hidden="true"
            />
            {activeDownloads > 0 && (
              <span className="absolute -top-1.5 -right-1.5 min-w-[16px] h-4 px-1 flex items-center justify-center bg-blue-500 text-white text-[10px] font-bold rounded-full leading-none pointer-events-none">
                {activeDownloads > 99 ? "99+" : activeDownloads}
              </span>
            )}
          </div>
        </Link>
        <Link
          to="/settings"
          className={`${footerActionClass} no-underline [&.active]:bg-brand-200 [&.active]:text-brand-900 dark:[&.active]:bg-brand-700 dark:[&.active]:text-brand-100 select-none data-glass:[&.active]:bg-white/20 data-glass:[&.active]:dark:bg-black/20`}
          title={t("sideBar.settings")}
          onDragStart={e => e.preventDefault()}
        >
          <div
            className="i-mdi-cog text-xl pointer-events-none"
            aria-hidden="true"
          />
        </Link>
      </div>
    </aside>
  );
}
