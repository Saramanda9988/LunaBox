import { Link } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { BrowserOpenURL, EventsOff, EventsOn } from "../../../wailsjs/runtime/runtime";
import { useAppStore } from "../../store";

interface SideBarProps {
  bgEnabled?: boolean;
  bgOpacity?: number;
}

export function SideBar({ bgEnabled = false, bgOpacity = 0.85 }: SideBarProps) {
  const { t } = useTranslation();
  const { isSidebarOpen, toggleSidebar } = useAppStore();
  const [activeDownloads, setActiveDownloads] = useState(0);

  // 监听下载进度事件，统计进行中的任务数
  useEffect(() => {
    const counts: Record<string, string> = {};
    EventsOn("download:progress", (evt: { id: string; status: string }) => {
      counts[evt.id] = evt.status;
      const active = Object.values(counts).filter(s => s === "downloading" || s === "pending").length;
      setActiveDownloads(active);
    });
    return () => EventsOff("download:progress");
  }, []);

  const navItems = [
    { to: "/", label: t("sideBar.home"), icon: "i-mdi-home" },
    { to: "/library", label: t("sideBar.library"), icon: "i-mdi-gamepad-variant" },
    { to: "/stats", label: t("sideBar.stats"), icon: "i-mdi-chart-bar" },
    { to: "/categories", label: t("sideBar.categories"), icon: "i-mdi-format-list-bulleted" },
  ];

  const sidebarBgClass = bgEnabled
    ? "border-r border-white/20 dark:border-white/10"
    : "bg-white dark:bg-brand-800 border-r border-brand-200 dark:border-brand-700";

  const sidebarStyle = bgEnabled
    ? { backgroundColor: `rgba(var(--sidebar-bg-rgb), ${bgOpacity})` }
    : undefined;

  return (
    <aside
      className={`flex flex-col transition-all duration-300 ${sidebarBgClass} ${isSidebarOpen ? "w-64" : "w-16"
      }`}
      style={sidebarStyle}
    >
      <div className={`flex items-center h-16 ${bgEnabled ? "border-white/20 dark:border-white/10" : "border-brand-200 dark:border-brand-700"} ${isSidebarOpen ? "justify-between px-4" : "justify-center"}`}>
        {isSidebarOpen && (
          <div className="flex items-center gap-1 select-none">
            <img src="/appicon.png" className="w-8 h-8 dark:hidden pointer-events-none" draggable="false" />
            <img src="/appicon-dark.png" className="w-8 h-8 hidden dark:block pointer-events-none" draggable="false" />
            <img src="/topbar-title-dark.png" className="h-6 dark:hidden pointer-events-none " />
            <img src="/topbar-title.png" className="h-6 hidden dark:block pointer-events-none " />
          </div>
        )}
        <button
          type="button"
          onClick={toggleSidebar}
          className="p-2 rounded hover:bg-brand-100 dark:hover:bg-brand-700 focus:outline-none select-none data-glass:hover:bg-white/10 data-glass:hover:dark:bg-black/10"
          onDragStart={e => e.preventDefault()}
        >
          <div className="i-mdi-menu text-xl pointer-events-none" />
        </button>
      </div>

      <nav className="flex-1 py-4">
        <ul className="space-y-2 px-2">
          {navItems.map(item => (
            <li key={item.to}>
              <Link
                to={item.to}
                className={`flex items-center p-2 rounded hover:bg-brand-100 dark:hover:bg-brand-700 text-brand-700 dark:text-brand-300 no-underline [&.active]:bg-brand-200 [&.active]:text-brand-900 dark:[&.active]:bg-brand-700 dark:[&.active]:text-brand-100 select-none data-glass:hover:bg-white/10 data-glass:hover:dark:bg-black/10 data-glass:[&.active]:bg-white/20 data-glass:[&.active]:dark:bg-black/20 ${isSidebarOpen ? "" : "justify-center"}`}
                onDragStart={e => e.preventDefault()}
              >
                <div className="relative shrink-0">
                  <div className={`${item.icon} text-xl pointer-events-none`} />
                </div>
                {isSidebarOpen && <span className="ml-3 pointer-events-none flex-1">{item.label}</span>}
              </Link>
            </li>
          ))}
        </ul>
      </nav>

      <div className={`p-4 ${bgEnabled ? "border-white/20 dark:border-white/10" : "border-brand-200 dark:border-brand-700"} flex ${isSidebarOpen ? "flex-row items-center justify-end gap-1" : "flex-col items-center gap-2"}`}>
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
          onClick={() => BrowserOpenURL("https://github.com/Saramanda9988/LunaBox")}
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
