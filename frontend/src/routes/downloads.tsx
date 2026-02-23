import type { service } from "../../wailsjs/go/models";
import { createRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { CancelDownload, DeleteDownloadTask, GetDownloadTasks, OpenDownloadTaskLocation } from "../../wailsjs/go/service/DownloadService";
import { ClipboardSetText, EventsOff, EventsOn } from "../../wailsjs/runtime/runtime";
import { DownloadCard } from "../components/card/DownloadCard";
import { Route as rootRoute } from "./__root";

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/downloads",
  component: DownloadsPage,
});

function DownloadsPage() {
  const { t } = useTranslation();
  const [tasks, setTasks] = useState<service.DownloadTask[]>([]);

  // 加载已有任务
  const loadTasks = async () => {
    const list = await GetDownloadTasks();
    setTasks((list as service.DownloadTask[]) ?? []);
  };

  useEffect(() => {
    loadTasks();
  }, []);

  useEffect(() => {
    EventsOn("download:progress", (evt: service.DownloadTask) => {
      setTasks((prev) => {
        const idx = prev.findIndex(t => t.id === evt.id);
        if (idx === -1) {
          // 新任务
          return [...prev, evt];
        }
        const next = [...prev];
        next[idx] = { ...next[idx], ...evt } as service.DownloadTask;
        return next;
      });
    });
    return () => EventsOff("download:progress");
  }, []);

  const handleCancel = async (id: string) => {
    await CancelDownload(id);
  };

  const handleDelete = async (id: string) => {
    await DeleteDownloadTask(id);
    setTasks(prev => prev.filter(task => task.id !== id));
  };

  const handleCopyURL = async (url: string) => {
    if (!url)
      return;
    const ok = await ClipboardSetText(url);
    if (ok)
      toast.success(t("downloads.toast.copyURLSuccess", "下载地址已复制"));
    else
      toast.error(t("downloads.toast.copyURLFailed", "复制失败"));
  };

  const handleOpenFolder = async (id: string) => {
    try {
      await OpenDownloadTaskLocation(id);
    }
    catch {
      toast.error(t("downloads.toast.openFolderFailed", "打开文件夹失败"));
    }
  };

  // 排序：活跃任务在前
  const sorted = [...tasks].sort((a, b) => {
    const order: Record<string, number> = { downloading: 0, pending: 1, error: 2, done: 3, cancelled: 4 };
    return (order[a.status] ?? 5) - (order[b.status] ?? 5);
  });

  const activeCount = tasks.filter(t => t.status === "downloading" || t.status === "pending").length;

  return (
    <div className="mx-auto flex h-full max-w-8xl flex-col space-y-6 p-8">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div>
            <h1 className="text-4xl font-bold text-brand-900 dark:text-white">
              {t("downloads.title", "下载管理")}
            </h1>
            <p className="mt-1 text-sm text-brand-500 dark:text-brand-400">
              {activeCount > 0
                ? t("downloads.activeCount", "{{count}} 个任务进行中", { count: activeCount })
                : t("downloads.noActive", "暂无进行中的任务")}
            </p>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto pr-1">
        {sorted.length === 0
          ? (
              <div className="glass-panel mx-auto flex h-full w-full max-w-5xl flex-col items-center justify-center gap-3 rounded-2xl border border-dashed border-brand-300 text-brand-400 select-none dark:border-brand-700 dark:text-brand-500">
                <span className="i-mdi-download-off text-5xl" />
                <p className="text-sm">{t("downloads.empty", "暂无下载任务")}</p>
                <p className="text-xs text-center max-w-xs">
                  {t("downloads.emptyHint", "点击 GAL 游戏站的「下载到 LunaBox」按钮后，任务将出现在这里")}
                </p>
              </div>
            )
          : (
              <div className="mx-auto grid w-full max-w-5xl grid-cols-1 gap-3">
                {sorted.map(task => (
                  <DownloadCard
                    key={task.id}
                    task={task}
                    onCancel={handleCancel}
                    onDelete={handleDelete}
                    onCopyURL={handleCopyURL}
                    onOpenFolder={handleOpenFolder}
                  />
                ))}
              </div>
            )}
      </div>
    </div>
  );
}
