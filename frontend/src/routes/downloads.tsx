import type { models, service } from "../../wailsjs/go/models";
import { createRoute } from "@tanstack/react-router";
import { useCallback, useEffect, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";

import { CancelDownload, DeleteDownloadTask, GetDownloadTasks, ImportDownloadTaskAsGame, OpenDownloadTaskLocation } from "../../wailsjs/go/service/DownloadService";
import { GetGames } from "../../wailsjs/go/service/GameService";
import { ClipboardSetText, EventsOff, EventsOn } from "../../wailsjs/runtime/runtime";
import { DownloadCard } from "../components/card/DownloadCard";
import { Route as rootRoute } from "./__root";

interface DownloadTaskVM {
  id: string;
  request: {
    url: string;
    title: string;
    download_source: string;
    meta_source: string;
    meta_id: string;
    size: number;
  };
  status: string;
  progress: number;
  downloaded: number;
  total: number;
  error?: string;
  file_path?: string;
}

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/downloads",
  component: DownloadsPage,
});

function DownloadsPage() {
  const { t } = useTranslation();
  const [tasks, setTasks] = useState<DownloadTaskVM[]>([]);
  const [importingTaskId, setImportingTaskId] = useState<string | null>(null);
  const [importedTaskIds, setImportedTaskIds] = useState<Record<string, boolean>>({});

  const normalizeSource = useCallback((source: string) => source.trim().toLowerCase(), []);

  const markImportedTasks = useCallback(async (targetTasks: DownloadTaskVM[]) => {
    const games = await GetGames();
    const gameList = (games as models.Game[]) ?? [];

    const nextImported: Record<string, boolean> = {};
    for (const task of targetTasks) {
      const taskSource = normalizeSource(task.request.meta_source || "");
      const taskSourceID = (task.request.meta_id || "").trim();

      const imported = gameList.some((game) => {
        const byPath = !!task.file_path && game.path === task.file_path;
        const bySource = taskSource !== ""
          && taskSourceID !== ""
          && normalizeSource(game.source_type || "") === taskSource
          && (game.source_id || "").trim() === taskSourceID;
        return byPath || bySource;
      });

      if (imported) {
        nextImported[task.id] = true;
      }
    }

    setImportedTaskIds(nextImported);
  }, [normalizeSource]);

  // 加载已有任务
  const loadTasks = useCallback(async () => {
    const list = await GetDownloadTasks();
    const normalized = (list as DownloadTaskVM[]) ?? [];
    setTasks(normalized);
    await markImportedTasks(normalized);
  }, [markImportedTasks]);

  useEffect(() => {
    loadTasks();
  }, [loadTasks]);

  useEffect(() => {
    EventsOn("download:progress", async (evt: DownloadTaskVM) => {
      setTasks((prev) => {
        const idx = prev.findIndex(t => t.id === evt.id);
        if (idx === -1) {
          // 新任务
          return [...prev, evt];
        }
        const next = [...prev];
        next[idx] = { ...next[idx], ...evt };
        return next;
      });

      if (evt.status === "done") {
        const latest = await GetDownloadTasks();
        await markImportedTasks((latest as DownloadTaskVM[]) ?? []);
      }
    });

    return () => {
      EventsOff("download:progress");
    };
  }, [markImportedTasks]);

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

  const handleImportAsGame = async (id: string) => {
    const task = tasks.find(item => item.id === id);
    if (!task) {
      return;
    }
    if (!task.file_path) {
      toast.error(t("downloads.toast.noFilePath", "下载文件路径不存在"));
      return;
    }
    if (importedTaskIds[id]) {
      toast.success(t("downloads.toast.alreadyImported", "该任务已导入为游戏"));
      return;
    }

    setImportingTaskId(id);
    try {
      await ImportDownloadTaskAsGame(id);
      toast.success(t("downloads.toast.importGameSuccess", "导入为游戏成功"));
      const latest = await GetDownloadTasks();
      const normalized = (latest as DownloadTaskVM[]) ?? [];
      setTasks(normalized);
      await markImportedTasks(normalized);
    }
    catch (error) {
      if (error instanceof Error && error.message.includes("select executable cancelled")) {
        toast(t("downloads.toast.selectExecutableCancelled", "已取消选择可执行文件"), { icon: "⚠️" });
        return;
      }
      console.error("Failed to import game from download task:", error);
      toast.error(t("downloads.toast.importGameFailed", "导入为游戏失败"));
    }
    finally {
      setImportingTaskId(null);
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
              </div>
            )
          : (
              <div className="mx-auto grid w-full max-w-5xl grid-cols-1 gap-3">
                {sorted.map(task => (
                  <DownloadCard
                    key={task.id}
                    task={task as unknown as service.DownloadTask}
                    onCancel={handleCancel}
                    onDelete={handleDelete}
                    onCopyURL={handleCopyURL}
                    onOpenFolder={handleOpenFolder}
                    onImportAsGame={handleImportAsGame}
                    importing={importingTaskId === task.id}
                    imported={!!importedTaskIds[task.id]}
                  />
                ))}
              </div>
            )}
      </div>
    </div>
  );
}
