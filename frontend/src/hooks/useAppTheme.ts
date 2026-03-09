import { useEffect } from "react";

import type { appconf } from "../../wailsjs/go/models";

export function useAppTheme(config: appconf.AppConfig | null) {
  useEffect(() => {
    if (!config) {
      return;
    }

    const root = window.document.documentElement;
    let frameId: number | undefined;
    let timeoutId: number | undefined;

    const clearScheduledCleanup = () => {
      if (frameId !== undefined) {
        window.cancelAnimationFrame(frameId);
      }
      if (timeoutId !== undefined) {
        window.clearTimeout(timeoutId);
      }
    };

    const applyTheme = (theme: string) => {
      clearScheduledCleanup();
      root.classList.add("theme-transitioning");
      root.classList.remove("light", "dark");
      root.classList.add(theme);

      frameId = window.requestAnimationFrame(() => {
        timeoutId = window.setTimeout(() => {
          root.classList.remove("theme-transitioning");
          timeoutId = undefined;
        }, 0);
        frameId = undefined;
      });
    };

    localStorage.setItem("lunabox-theme", config.theme);

    if (config.theme === "system") {
      const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
      applyTheme(mediaQuery.matches ? "dark" : "light");

      const handler = (event: MediaQueryListEvent) => {
        applyTheme(event.matches ? "dark" : "light");
      };

      mediaQuery.addEventListener("change", handler);
      return () => {
        mediaQuery.removeEventListener("change", handler);
        clearScheduledCleanup();
      };
    }

    applyTheme(config.theme);
    return clearScheduledCleanup;
  }, [config]);
}
