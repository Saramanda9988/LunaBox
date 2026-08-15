import type { service, vo } from "../../src/bindings/models";
import { createRoute } from "@tanstack/react-router";
import { Clipboard } from "@wailsio/runtime";
import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";

import {
  CancelDownload,
  CheckDownloadImportStates,
  DeleteDownloadTask,
  GetDownloadTasks,
  ImportDownloadTaskAsGame,
  OpenDownloadTaskLocation,
  PauseDownload,
  ResumeDownload,
  RetryDownload,
} from "../../bindings/lunabox/internal/service/downloadservice";
import { onWailsEvent } from "../../src/bindings/runtime";
import { DownloadCard } from "../components/card/DownloadCard";
import { useAppStore } from "../store";
import { formatLocalDate, formatLocalDateKey, parseTime } from "../utils/time";
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
  created_at?: string;
  progress: number;
  downloaded: number;
  total: number;
  error?: string;
  file_path?: string;
}

const IMAGE_DOWNLOAD_SOURCE = "cover-image-batch";
const DOWNLOAD_STATUS_ORDER: Record<string, number> = {
  downloading: 0,
  extracting: 1,
  paused: 2,
  pending: 3,
  error: 4,
  done: 5,
  cancelled: 6,
};

interface DownloadTaskGroup {
  key: string;
  label: string;
  tasks: DownloadTaskVM[];
}

interface DownloadGameImportedEvent {
  task_id?: string;
}

function compareTasksWithinDate(a: DownloadTaskVM, b: DownloadTaskVM) {
  const statusOrder
    = (DOWNLOAD_STATUS_ORDER[a.status] ?? 5)
      - (DOWNLOAD_STATUS_ORDER[b.status] ?? 5);
  if (statusOrder !== 0) {
    return statusOrder;
  }

  const aImageTask = a.request.download_source === IMAGE_DOWNLOAD_SOURCE;
  const bImageTask = b.request.download_source === IMAGE_DOWNLOAD_SOURCE;
  if (aImageTask !== bImageTask) {
    return aImageTask ? -1 : 1;
  }

  return 0;
}

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/downloads",
  component: DownloadsPage,
});

function DownloadsPage() {
  const { t } = useTranslation();
  const timezone = useAppStore(state => state.config?.time_zone);
  const [tasks, setTasks] = useState<DownloadTaskVM[]>([]);
  const [importingTaskId, setImportingTaskId] = useState<string | null>(null);
  const [importedTaskIds, setImportedTaskIds] = useState<
    Record<string, boolean>
  >({});

  const markImportedTasks = useCallback(
    async (targetTasks: DownloadTaskVM[]) => {
      const requests: vo.DownloadImportStateRequest[] = targetTasks.map(
        task => ({
          task_id: task.id,
          file_path: task.file_path || "",
          meta_source: task.request.meta_source || "",
          meta_id: task.request.meta_id || "",
        }),
      );
      const states = await CheckDownloadImportStates(requests);
      const nextImported = Object.fromEntries(
        (states || [])
          .filter(state => state.imported)
          .map(state => [state.task_id, true]),
      );
      setImportedTaskIds(nextImported);
    },
    [],
  );

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
    const unsubscribeProgress = onWailsEvent(
      "download:progress",
      async (evt: DownloadTaskVM) => {
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
          const normalized = (latest as DownloadTaskVM[]) ?? [];
          setTasks(normalized);
        }
      },
    );

    const unsubscribeGameImported = onWailsEvent(
      "download:game-imported",
      (evt: DownloadGameImportedEvent) => {
        const taskID = evt?.task_id?.trim();
        if (!taskID) {
          return;
        }
        setImportedTaskIds(prev =>
          prev[taskID] ? prev : { ...prev, [taskID]: true },
        );
      },
    );

    return () => {
      unsubscribeProgress();
      unsubscribeGameImported();
    };
  }, []);

  const handleCancel = async (id: string) => {
    await CancelDownload(id);
  };

  const handlePause = async (id: string) => {
    try {
      await PauseDownload(id);
    }
    catch {
      toast.error(t("downloads.toast.pauseFailed", "暂停下载失败"));
    }
  };

  const handleResume = async (id: string) => {
    try {
      await ResumeDownload(id);
    }
    catch {
      toast.error(t("downloads.toast.resumeFailed", "继续下载失败"));
    }
  };

  const handleRetry = async (id: string) => {
    try {
      await RetryDownload(id);
    }
    catch {
      toast.error(t("downloads.toast.retryFailed", "重试下载失败"));
    }
  };

  const handleDelete = async (id: string) => {
    await DeleteDownloadTask(id);
    setTasks(prev => prev.filter(task => task.id !== id));
  };

  const handleCopyURL = async (url: string) => {
    if (!url)
      return;
    try {
      await Clipboard.SetText(url);
      toast.success(t("downloads.toast.copyURLSuccess", "下载地址已复制"));
    }
    catch {
      toast.error(t("downloads.toast.copyURLFailed", "复制失败"));
    }
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
    if (task.request.download_source === "cover-image-batch") {
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
      if (
        error instanceof Error
        && error.message.includes("select executable cancelled")
      ) {
        toast(
          t(
            "downloads.toast.selectExecutableCancelled",
            "已取消选择可执行文件",
          ),
          { icon: "⚠️" },
        );
        return;
      }
      console.error("Failed to import game from download task:", error);
      toast.error(t("downloads.toast.importGameFailed", "导入为游戏失败"));
    }
    finally {
      setImportingTaskId(null);
    }
  };

  const taskGroups = useMemo<DownloadTaskGroup[]>(() => {
    const now = new Date();
    const todayKey = formatLocalDateKey(now, timezone);
    const yesterdayKey = formatLocalDateKey(
      new Date(now.getTime() - 24 * 60 * 60 * 1000),
      timezone,
    );
    const datedTasks = tasks.map((task) => {
      const timestamp = task.created_at
        ? parseTime(task.created_at).getTime()
        : 0;
      return {
        task,
        timestamp: Number.isNaN(timestamp) ? 0 : timestamp,
        dateKey: task.created_at
          ? formatLocalDateKey(task.created_at, timezone)
          : "",
      };
    });

    datedTasks.sort((a, b) => {
      if (a.dateKey !== b.dateKey) {
        return b.timestamp - a.timestamp;
      }

      const taskOrder = compareTasksWithinDate(a.task, b.task);
      return taskOrder !== 0 ? taskOrder : b.timestamp - a.timestamp;
    });

    const groups = new Map<string, DownloadTaskGroup>();
    for (const { dateKey, task } of datedTasks) {
      const key = dateKey || "earlier";
      let group = groups.get(key);
      if (!group) {
        const label
          = dateKey === todayKey
            ? t("downloads.dateGroup.today", "今天")
            : dateKey === yesterdayKey
              ? t("downloads.dateGroup.yesterday", "昨天")
              : task.created_at
                ? formatLocalDate(task.created_at, timezone, {
                    year: "numeric",
                    month: "long",
                    day: "numeric",
                  })
                : t("downloads.dateGroup.earlier", "更早");
        group = { key, label, tasks: [] };
        groups.set(key, group);
      }
      group.tasks.push(task);
    }

    return [...groups.values()];
  }, [t, tasks, timezone]);

  const activeCount = tasks.filter(
    t =>
      t.status === "downloading"
      || t.status === "pending"
      || t.status === "extracting",
  ).length;

  return (
    <div className="h-full w-full overflow-y-auto scrollbar-stable p-8">
      <div className="mx-auto max-w-8xl space-y-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div>
              <h1 className="text-4xl font-bold text-brand-900 dark:text-white">
                {t("downloads.title", "下载管理")}
              </h1>
              <p className="mt-1 text-sm text-brand-500 dark:text-brand-400">
                {activeCount > 0
                  ? t("downloads.activeCount", "{{count}} 个任务进行中", {
                      count: activeCount,
                    })
                  : t("downloads.noActive", "暂无进行中的任务")}
              </p>
            </div>
          </div>
        </div>

        {taskGroups.length === 0 ? (
          <div className="glass-panel mx-auto flex min-h-[24rem] w-full max-w-5xl flex-col items-center justify-center gap-3 rounded-2xl border border-dashed border-brand-300 text-brand-400 select-none dark:border-brand-700 dark:text-brand-500">
            <span className="i-mdi-download-off text-5xl" />
            <p className="text-sm">{t("downloads.empty", "暂无下载任务")}</p>
          </div>
        ) : (
          <div className="mx-auto w-full max-w-5xl space-y-8">
            {taskGroups.map(group => (
              <section
                key={group.key}
                aria-labelledby={`downloads-group-${group.key}`}
              >
                <h2
                  id={`downloads-group-${group.key}`}
                  className="mb-3 px-1 text-base font-medium text-brand-600 dark:text-brand-300 data-glass:text-brand-700 data-glass:dark:text-brand-200"
                >
                  {group.label}
                </h2>
                <div className="grid grid-cols-1 gap-3">
                  {group.tasks.map(task => (
                    <DownloadCard
                      key={task.id}
                      task={task as unknown as service.DownloadTask}
                      onCancel={handleCancel}
                      onPause={handlePause}
                      onResume={handleResume}
                      onRetry={handleRetry}
                      onDelete={handleDelete}
                      onCopyURL={handleCopyURL}
                      onOpenFolder={handleOpenFolder}
                      onImportAsGame={handleImportAsGame}
                      importing={importingTaskId === task.id}
                      imported={!!importedTaskIds[task.id]}
                    />
                  ))}
                </div>
              </section>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
