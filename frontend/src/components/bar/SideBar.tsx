import { Link } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { BrowserOpenURL, EventsOn } from "../../../wailsjs/runtime/runtime";
import { useAppStore } from "../../store";

interface SideBarProps {
  bgEnabled?: boolean;
  bgOpacity?: number;
}

export function SideBar({ bgEnabled = false, bgOpacity = 0.85 }: SideBarProps) {
  const { t } = useTranslation();
  const isSidebarOpen = useAppStore(state => state.isSidebarOpen);
  const toggleSidebar = useAppStore(state => state.toggleSidebar);
  const [activeDownloads, setActiveDownloads] = useState(0);

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

  return (
    <aside
      className={`flex shrink-0 flex-col overflow-hidden ${sidebarBgClass}`}
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
            className="p-2 rounded hover:bg-brand-100 dark:hover:bg-brand-700 focus:outline-none select-none data-glass:hover:bg-white/10 data-glass:hover:dark:bg-black/10 shrink-0"
            onDragStart={e => e.preventDefault()}
          >
            <div className="i-mdi-menu text-xl pointer-events-none" />
          </button>
        </div>
      </div>

      <nav className="flex-1 py-4">
        <ul className="space-y-2 px-2">
          {navItems.map(item => (
            <li key={item.to}>
              <Link
                to={item.to}
                className="flex items-center p-2 rounded hover:bg-brand-100 dark:hover:bg-brand-700 text-brand-700 dark:text-brand-300 no-underline [&.active]:bg-brand-200 [&.active]:text-brand-900 dark:[&.active]:bg-brand-700 dark:[&.active]:text-brand-100 select-none data-glass:hover:bg-white/10 data-glass:hover:dark:bg-black/10 data-glass:[&.active]:bg-white/20 data-glass:[&.active]:dark:bg-black/20"
                onDragStart={e => e.preventDefault()}
              >
                <div className="relative shrink-0 flex items-center justify-center w-8 h-8">
                  <div className={`${item.icon} text-xl pointer-events-none`} />
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
        className={`p-4 ${bgEnabled ? "border-white/20 dark:border-white/10" : "border-brand-200 dark:border-brand-700"} flex ${isSidebarOpen ? "flex-row items-center justify-end gap-1" : "flex-col items-center gap-2"}`}
      >
        <Link
          to="/downloads"
          className="flex items-center relative p-2 rounded hover:bg-brand-100 dark:hover:bg-brand-700 text-brand-700 dark:text-brand-300 no-underline [&.active]:bg-brand-200 [&.active]:text-brand-900 dark:[&.active]:bg-brand-700 dark:[&.active]:text-brand-100 select-none data-glass:hover:bg-white/10 data-glass:hover:dark:bg-black/10 data-glass:[&.active]:bg-white/20 data-glass:[&.active]:dark:bg-black/20"
          title={t("sideBar.downloads", "下载")}
          onDragStart={e => e.preventDefault()}
        >
          <div className="relative shrink-0">
            <div className="i-mdi-download text-xl pointer-events-none" />
            {activeDownloads > 0 && (
              <span className="absolute -top-1.5 -right-1.5 min-w-[16px] h-4 px-1 flex items-center justify-center bg-blue-500 text-white text-[10px] font-bold rounded-full leading-none pointer-events-none">
                {activeDownloads > 99 ? "99+" : activeDownloads}
              </span>
            )}
          </div>
        </Link>
        <div
          onClick={() =>
            BrowserOpenURL("https://github.com/Saramanda9988/LunaBox")}
          className="flex items-center p-2 rounded hover:bg-brand-100 dark:hover:bg-brand-700 text-brand-700 dark:text-brand-300 cursor-pointer select-none data-glass:hover:bg-white/10 data-glass:hover:dark:bg:black/10"
          title={t("sideBar.github")}
          onDragStart={e => e.preventDefault()}
        >
          <div className="i-mdi-github text-xl pointer-events-none" />
        </div>
        <Link
          to="/settings"
          className="flex items-center p-2 rounded hover:bg-brand-100 dark:hover:bg-brand-700 text-brand-700 dark:text-brand-300 no-underline [&.active]:bg-brand-200 [&.active]:text-brand-900 dark:[&.active]:bg-brand-700 dark:[&.active]:text-brand-100 select-none data-glass:hover:bg-white/10 data-glass:hover:dark:bg:black/10 data-glass:[&.active]:bg-white/20 data-glass:[&.active]:dark:bg:black/20"
          title={t("sideBar.settings")}
          onDragStart={e => e.preventDefault()}
        >
          <div className="i-mdi-cog text-xl pointer-events-none" />
        </Link>
      </div>
    </aside>
  );
}
