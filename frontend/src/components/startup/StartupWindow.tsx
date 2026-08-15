import type { CSSProperties } from "react";

import { Window } from "@wailsio/runtime";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { GetFailure } from "../../../bindings/lunabox/internal/service/startupservice";
import appIconDarkUrl from "../../assets/branding/appicon-dark.png";
import appIconUrl from "../../assets/branding/appicon.png";
import topbarTitleDarkUrl from "../../assets/branding/topbar-title-dark.png";
import topbarTitleUrl from "../../assets/branding/topbar-title.png";
import { onWailsEvent } from "../../bindings/runtime";

interface StartupFailure {
  message: string;
}

function applyStoredTheme() {
  const root = window.document.documentElement;
  const storedTheme = localStorage.getItem("lunabox-theme");
  const prefersDark = window.matchMedia("(prefers-color-scheme: dark)");
  const theme
    = storedTheme === "dark" || (storedTheme !== "light" && prefersDark.matches)
      ? "dark"
      : "light";

  root.classList.remove("light", "dark");
  root.classList.add(theme);

  return { prefersDark, storedTheme };
}

function StartupWindow() {
  const { t } = useTranslation();
  const [failure, setFailure] = useState<StartupFailure>({ message: "" });
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    const { prefersDark, storedTheme } = applyStoredTheme();
    if (storedTheme === "light" || storedTheme === "dark") {
      return;
    }

    const handleSystemThemeChange = () => applyStoredTheme();
    prefersDark.addEventListener("change", handleSystemThemeChange);
    return () =>
      prefersDark.removeEventListener("change", handleSystemThemeChange);
  }, []);

  useEffect(() => {
    let active = true;
    const unsubscribe = onWailsEvent<StartupFailure>(
      "startup:failed",
      nextFailure => setFailure(nextFailure),
    );

    void GetFailure().then((latestFailure) => {
      if (active) {
        setFailure(latestFailure);
      }
    });

    return () => {
      active = false;
      unsubscribe();
    };
  }, []);

  const errorMessage = failure.message || t("startup.unknownError");

  const copyError = async () => {
    await navigator.clipboard.writeText(errorMessage);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  };

  const nonDraggableStyle = {
    "--wails-draggable": "no-drag",
  } as CSSProperties;

  return (
    <main
      className="startup-backdrop flex h-screen w-screen select-none overflow-hidden text-brand-900 dark:text-white"
      style={nonDraggableStyle}
    >
      <aside className="relative flex w-[282px] shrink-0 items-center justify-center overflow-hidden border-r border-brand-200 bg-brand-100 dark:border-white/8 dark:bg-brand-900">
        <div
          className="pointer-events-none absolute -left-30 -top-34 h-72 w-72 rounded-full border border-primary-300/25 dark:border-primary-400/12"
          aria-hidden="true"
        />
        <div
          className="pointer-events-none absolute -bottom-38 -right-32 h-72 w-72 rounded-full border border-primary-300/18 dark:border-primary-400/10"
          aria-hidden="true"
        />

        <div className="relative flex -translate-y-1 flex-col items-center gap-5">
          <img
            src={appIconUrl}
            className="h-24 w-24 rounded-[23px] object-contain dark:hidden"
            draggable="false"
            alt=""
          />
          <img
            src={appIconDarkUrl}
            className="hidden h-24 w-24 rounded-[23px] object-contain dark:block"
            draggable="false"
            alt=""
          />
          <img
            src={topbarTitleDarkUrl}
            className="h-auto w-38 dark:hidden"
            draggable="false"
            alt="LunaBox"
          />
          <img
            src={topbarTitleUrl}
            className="hidden h-auto w-38 dark:block"
            draggable="false"
            alt="LunaBox"
          />
        </div>
      </aside>

      <section className="flex min-w-0 flex-1 flex-col bg-white px-9 pb-7 pt-8 dark:bg-brand-800">
        <header className="flex items-start gap-3.5">
          <span
            className="i-mdi-alert-circle-outline mt-0.5 shrink-0 text-[28px] text-error-500 dark:text-error-400"
            aria-hidden="true"
          />
          <div className="min-w-0">
            <h1 className="text-[23px] font-semibold leading-8 tracking-tight">
              {t("startup.title")}
            </h1>
            <p className="mt-1 text-[13px] leading-5 text-brand-500 dark:text-brand-400">
              {t("startup.detail")}
            </p>
          </div>
        </header>

        <div className="mt-6 min-h-0 flex-1" style={nonDraggableStyle}>
          <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-xl border border-brand-200 bg-brand-50 dark:border-brand-700 dark:bg-brand-900/70">
            <div className="flex h-11 shrink-0 items-center justify-between border-b border-brand-200 px-4 dark:border-brand-700">
              <span className="text-xs font-medium text-brand-500 dark:text-brand-400">
                {t("startup.errorSubtitle")}
              </span>

              <div className="group relative">
                <button
                  type="button"
                  className="grid h-8 w-8 place-items-center rounded-lg text-brand-500 transition-colors hover:bg-brand-200 hover:text-brand-800 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary-500 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white"
                  onClick={() => void copyError()}
                  aria-label={
                    copied ? t("startup.copied") : t("startup.copyError")
                  }
                >
                  <span
                    className={`${copied ? "i-mdi-check text-success-500" : "i-mdi-content-copy"} text-base`}
                    aria-hidden="true"
                  />
                </button>
                <span className="pointer-events-none absolute right-0 top-10 z-10 whitespace-nowrap rounded-md bg-brand-900 px-2 py-1 text-[10px] text-white opacity-0 shadow-lg transition-opacity group-hover:opacity-100 dark:bg-white dark:text-brand-900">
                  {copied ? t("startup.copied") : t("startup.copyError")}
                </span>
              </div>
            </div>

            <pre className="scrollbar-stable min-h-0 flex-1 overflow-auto whitespace-pre-wrap break-words px-4 py-3.5 font-mono text-[11px] leading-[1.65] text-error-700 select-text dark:text-error-300">
              {errorMessage}
            </pre>
          </div>
        </div>

        <footer className="mt-5 flex justify-end" style={nonDraggableStyle}>
          <button
            type="button"
            className="shrink-0 rounded-lg bg-brand-900 px-4 py-2 text-xs font-medium text-white transition-colors hover:bg-brand-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500 dark:bg-white dark:text-brand-900 dark:hover:bg-brand-200"
            onClick={() => void Window.Close()}
          >
            {t("startup.exit")}
          </button>
        </footer>
      </section>
    </main>
  );
}

export default StartupWindow;
