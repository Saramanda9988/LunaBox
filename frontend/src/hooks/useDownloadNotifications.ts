import type { i18n as I18nInstance } from "i18next";

import { useEffect, useRef } from "react";
import { toast } from "react-hot-toast";

import { onWailsEvent } from "../../src/bindings/runtime";
import { invalidateAllGameLists } from "../cache/gameCache";
import { useAppStore } from "../store";

type DownloadProgressEvent = {
  id: string;
  request?: {
    title?: string;
    download_source?: string;
  };
  status: string;
  error?: string;
};

type DownloadTaskErrorEvent = {
  task_id?: string;
  title?: string;
  meta_source?: string;
  meta_id?: string;
  error?: string;
};

const IMAGE_DOWNLOAD_SOURCE = "cover-image-batch";

export function useDownloadNotifications(i18n: I18nInstance) {
  const downloadStatusRef = useRef<Record<string, string>>({});

  useEffect(() => {
    const unsubscribeProgress = onWailsEvent(
      "download:progress",
      (evt: DownloadProgressEvent) => {
        const previousStatus = downloadStatusRef.current[evt.id];
        downloadStatusRef.current[evt.id] = evt.status;

        const title
          = evt.request?.title?.trim()
            || i18n.t("downloads.unknownTitle", "未知标题");
        const isImageDownloadTask
          = evt.request?.download_source === IMAGE_DOWNLOAD_SOURCE;

        if (evt.status === "done" && previousStatus !== "done") {
          if (isImageDownloadTask) {
            const message = i18n.t(
              "downloads.imageTask.toastDone",
              "批量图片下载任务已完成",
            );
            toast.success(message, { id: `download-done-${evt.id}` });
            return;
          }

          const message
            = evt.error === "manual_extract_required"
              ? `${i18n.t("downloads.toast.downloadCompleted", { title, defaultValue: "{{title}} 下载完成" })}\n${i18n.t("downloads.toast.manualExtractRequired", "自动解压失败，请手动解压后再导入或启动")}`
              : i18n.t("downloads.toast.downloadCompleted", {
                  title,
                  defaultValue: "{{title}} 下载完成",
                });

          toast.success(message, { id: `download-done-${evt.id}` });
          return;
        }

        if (evt.status === "error" && previousStatus !== "error") {
          const message = evt.error?.trim()
            ? `${i18n.t("downloads.toast.downloadFailed", { title, defaultValue: "{{title}} 下载失败" })}\n${evt.error.trim()}`
            : i18n.t("downloads.toast.downloadFailed", {
                title,
                defaultValue: "{{title}} 下载失败",
              });

          toast.error(message, { id: `download-error-${evt.id}` });
        }
      },
    );

    const unsubscribeGameImported = onWailsEvent(
      "download:game-imported",
      () => {
        invalidateAllGameLists();
        void useAppStore.getState().fetchHomeData({
          showLoading: false,
          syncRuntime: false,
        });
      },
    );

    const unsubscribeGameImportFailed = onWailsEvent(
      "download:game-import-failed",
      (evt: DownloadTaskErrorEvent) => {
        const title
          = evt.title?.trim() || i18n.t("downloads.unknownTitle", "未知标题");
        toast.error(
          i18n.t("downloads.toast.autoImportFailed", {
            title,
            error:
              evt.error?.trim() || i18n.t("common.unknownError", "未知错误"),
            defaultValue: "{{title}} 自动导入失败：{{error}}",
          }),
          { id: `download-import-failed-${evt.task_id || title}` },
        );
      },
    );

    const unsubscribeMetadataFailed = onWailsEvent(
      "download:metadata-failed",
      (evt: DownloadTaskErrorEvent) => {
        const title
          = evt.title?.trim() || i18n.t("downloads.unknownTitle", "未知标题");
        toast(
          i18n.t("downloads.toast.metadataFailed", {
            title,
            error:
              evt.error?.trim() || i18n.t("common.unknownError", "未知错误"),
            defaultValue: "{{title}} 已导入，但元数据获取失败：{{error}}",
          }),
          {
            icon: "⚠️",
            id: `download-metadata-failed-${evt.task_id || title}`,
          },
        );
      },
    );

    return () => {
      unsubscribeProgress();
      unsubscribeGameImported();
      unsubscribeGameImportFailed();
      unsubscribeMetadataFailed();
    };
  }, [i18n]);
}
