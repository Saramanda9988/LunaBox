import type { i18n as I18nInstance } from "i18next";

import { useEffect } from "react";
import { toast } from "react-hot-toast";

import { onWailsEvent } from "../../src/bindings/runtime";

type CoverImageDownloadEvent = {
  game_id: string;
  game_name: string;
  status: "started" | "done" | "failed";
  error?: string;
};

export function useCoverImageDownloadNotifications(i18n: I18nInstance) {
  useEffect(() => {
    const unsubscribe = onWailsEvent(
      "cover-image:download",
      (evt: CoverImageDownloadEvent) => {
        const toastID = `cover-image-${evt.game_id}`;

        if (evt.status === "started") {
          toast.loading(
            i18n.t(
              "coverImageDownload.toast.started",
              "正在尝试下载图片，请勿关闭",
            ),
            { id: toastID },
          );
          return;
        }

        if (evt.status === "done") {
          toast.success(
            i18n.t("coverImageDownload.toast.success", "图片下载成功"),
            { id: toastID },
          );
          return;
        }

        const message = evt.error?.trim()
          ? `${i18n.t("coverImageDownload.toast.failed", "图片下载失败")}\n${evt.error.trim()}`
          : i18n.t("coverImageDownload.toast.failed", "图片下载失败");
        toast.error(message, { id: toastID });
      },
    );

    return unsubscribe;
  }, [i18n]);
}
