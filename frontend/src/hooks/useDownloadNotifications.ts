import type { i18n as I18nInstance } from "i18next";

import { useEffect, useRef } from "react";
import { toast } from "react-hot-toast";

import { EventsOn } from "../../wailsjs/runtime/runtime";

type DownloadProgressEvent = {
  id: string;
  request?: {
    title?: string;
  };
  status: string;
  error?: string;
};

export function useDownloadNotifications(i18n: I18nInstance) {
  const downloadStatusRef = useRef<Record<string, string>>({});

  useEffect(() => {
    const unsubscribe = EventsOn("download:progress", (evt: DownloadProgressEvent) => {
      const previousStatus = downloadStatusRef.current[evt.id];
      downloadStatusRef.current[evt.id] = evt.status;

      const title = evt.request?.title?.trim() || i18n.t("downloads.unknownTitle", "未知标题");

      if (evt.status === "done" && previousStatus !== "done") {
        const message = evt.error === "manual_extract_required"
          ? `${i18n.t("downloads.toast.downloadCompleted", { title, defaultValue: "{{title}} 下载完成" })}\n${i18n.t("downloads.toast.manualExtractRequired", "自动解压失败，请手动解压后再导入或启动")}`
          : i18n.t("downloads.toast.downloadCompleted", { title, defaultValue: "{{title}} 下载完成" });

        toast.success(message, { id: `download-done-${evt.id}` });
        return;
      }

      if (evt.status === "error" && previousStatus !== "error") {
        const message = evt.error?.trim()
          ? `${i18n.t("downloads.toast.downloadFailed", { title, defaultValue: "{{title}} 下载失败" })}\n${evt.error.trim()}`
          : i18n.t("downloads.toast.downloadFailed", { title, defaultValue: "{{title}} 下载失败" });

        toast.error(message, { id: `download-error-${evt.id}` });
      }
    });

    return unsubscribe;
  }, [i18n]);
}
