import { useEffect } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";

import type { appconf } from "../../wailsjs/go/models";

import { DEFAULT_APP_ZOOM, getNextAppZoomFactor, normalizeAppZoomFactor } from "../consts/options";

function isEditableTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) {
    return false;
  }

  return target.isContentEditable
    || target.tagName === "INPUT"
    || target.tagName === "TEXTAREA"
    || target.tagName === "SELECT";
}

type UseAppZoomOptions = {
  config: appconf.AppConfig | null;
  updateConfig: (config: appconf.AppConfig) => Promise<void>;
  setWindowZoomFactor: (zoomFactor: number) => void;
};

export function useAppZoom({ config, updateConfig, setWindowZoomFactor }: UseAppZoomOptions) {
  const { t } = useTranslation();

  useEffect(() => {
    if (!config) {
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if ((!event.ctrlKey && !event.metaKey) || event.altKey || isEditableTarget(event.target)) {
        return;
      }

      const currentZoom = normalizeAppZoomFactor(config.window_zoom_factor);
      let nextZoom = currentZoom;

      if (event.key === "0") {
        nextZoom = DEFAULT_APP_ZOOM;
      }
      else if (event.key === "=" || event.key === "+" || event.code === "NumpadAdd") {
        nextZoom = getNextAppZoomFactor(currentZoom, 1);
      }
      else if (event.key === "-" || event.key === "_" || event.code === "NumpadSubtract") {
        nextZoom = getNextAppZoomFactor(currentZoom, -1);
      }
      else {
        return;
      }

      if (nextZoom === currentZoom) {
        return;
      }

      event.preventDefault();
      setWindowZoomFactor(nextZoom);
      void updateConfig({ ...config, window_zoom_factor: nextZoom });
      toast.success(t("settings.basic.zoomToast", { value: `${Math.round(nextZoom * 100)}%` }), {
        id: "app-zoom-changed",
      });
    };

    window.addEventListener("keydown", handleKeyDown);

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [config, setWindowZoomFactor, t, updateConfig]);
}
