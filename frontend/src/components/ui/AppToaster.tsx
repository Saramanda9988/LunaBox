import type { Toast } from "react-hot-toast";
import { useEffect, useMemo, useState } from "react";
import { resolveValue, toast as toastApi, Toaster, useToasterStore } from "react-hot-toast";
import { useTranslation } from "react-i18next";

const MAX_VISIBLE_TOASTS = 4;
const DEFAULT_TOAST_DURATION = 4000;

function splitToastMessage(message: string) {
  const normalized = message.trim();
  if (!normalized)
    return { title: "", details: "" };

  const newlineParts = normalized
    .split(/\r?\n+/)
    .map(part => part.trim())
    .filter(Boolean);

  if (newlineParts.length > 1) {
    return {
      title: newlineParts[0],
      details: newlineParts.slice(1).join("\n"),
    };
  }

  const separatorMatch = normalized.match(/^(.{4,90}?)[：:](.+)$/);
  if (separatorMatch) {
    const title = separatorMatch[1].trim();
    const details = separatorMatch[2].trim();
    if (title && details.length > 4) {
      return { title, details };
    }
  }

  if (normalized.length > 110) {
    return {
      title: `${normalized.slice(0, 92).trimEnd()}...`,
      details: normalized,
    };
  }

  return { title: normalized, details: "" };
}

function getToastTone(type: Toast["type"]) {
  switch (type) {
    case "success": {
      return {
        iconWrap: "bg-success-500/15 text-success-600 dark:bg-success-500/20 dark:text-success-400",
        progress: "bg-gradient-to-r from-success-400 via-success-500 to-success-600",
      };
    }
    case "error": {
      return {
        iconWrap: "bg-error-500/15 text-error-600 dark:bg-error-500/20 dark:text-error-400",
        progress: "bg-gradient-to-r from-error-400 via-error-500 to-error-600",
      };
    }
    case "loading": {
      return {
        iconWrap: "bg-info-500/15 text-info-600 dark:bg-info-500/20 dark:text-info-400",
        progress: "bg-gradient-to-r from-info-400 via-info-500 to-info-600",
      };
    }
    default: {
      return {
        iconWrap: "bg-primary-500/15 text-primary-600 dark:bg-primary-500/20 dark:text-primary-400",
        progress: "bg-gradient-to-r from-primary-400 via-primary-500 to-primary-600",
      };
    }
  }
}

function ToastGlyph({ toast }: { toast: Toast }) {
  if (toast.icon) {
    return <span className="text-base leading-none">{toast.icon}</span>;
  }

  switch (toast.type) {
    case "success":
      return (
        <svg viewBox="0 0 20 20" className="h-4.5 w-4.5 fill-current" aria-hidden="true">
          <path d="M16.704 5.29a1 1 0 0 1 .006 1.414l-7.2 7.262a1 1 0 0 1-1.42 0L3.29 9.127a1 1 0 0 1 1.42-1.406l4.09 4.127 6.49-6.545a1 1 0 0 1 1.414-.013Z" />
        </svg>
      );
    case "error":
      return (
        <svg viewBox="0 0 20 20" className="h-4.5 w-4.5 fill-current" aria-hidden="true">
          <path d="M10 2.5A7.5 7.5 0 1 0 10 17.5 7.5 7.5 0 0 0 10 2.5Zm2.85 9.15a.9.9 0 1 1-1.27 1.27L10 11.27 8.42 12.92a.9.9 0 1 1-1.27-1.27L8.73 10 7.08 8.42a.9.9 0 1 1 1.27-1.27L10 8.73l1.58-1.65a.9.9 0 1 1 1.27 1.27L11.27 10l1.58 1.65Z" />
        </svg>
      );
    case "loading":
      return <span className="i-mdi-loading h-4.5 w-4.5 animate-spin" aria-hidden="true" />;
    default:
      return (
        <svg viewBox="0 0 20 20" className="h-4.5 w-4.5 fill-current" aria-hidden="true">
          <path d="M10 3a7 7 0 1 0 0 14 7 7 0 0 0 0-14Zm.75 9.5a.75.75 0 0 1-1.5 0v-3a.75.75 0 0 1 1.5 0v3Zm0-5.5a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Z" />
        </svg>
      );
  }
}

function ToastCard({ toast }: { toast: Toast }) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);
  const renderedMessage = resolveValue(toast.message, toast);
  const tone = getToastTone(toast.type);
  const duration = toast.duration ?? DEFAULT_TOAST_DURATION;

  const content = useMemo(() => {
    if (typeof renderedMessage !== "string") {
      return {
        title: renderedMessage,
        details: "",
        hasDetails: false,
      };
    }

    const parsed = splitToastMessage(renderedMessage);
    return {
      title: parsed.title,
      details: parsed.details,
      hasDetails: Boolean(parsed.details),
    };
  }, [renderedMessage]);

  return (
    <div
      className="group relative flex w-[min(380px,calc(100vw-24px))] flex-col overflow-hidden rounded-xl bg-white/90 text-brand-900 shadow-xl backdrop-blur-xl dark:bg-brand-800/90 dark:text-brand-50"
      style={{
        animation: toast.visible
          ? "toast-enter 0.3s cubic-bezier(0.16, 1, 0.3, 1) forwards"
          : "toast-leave 0.2s cubic-bezier(0.7, 0, 0.84, 0) forwards",
      }}
      role={toast.type === "error" ? "alert" : "status"}
    >
      <div className="flex items-start gap-3.5 p-4 pr-11">
        <div className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-xl ${tone.iconWrap}`}>
          <ToastGlyph toast={toast} />
        </div>

        <div className="min-w-0 flex-1 flex flex-col justify-center min-h-9">
          <div className="text-[14px] leading-snug">
            {typeof content.title === "string"
              ? <p className="break-words font-medium">{content.title}</p>
              : content.title}
          </div>

          {content.hasDetails && (
            <div className="mt-2.5">
              <button
                type="button"
                className="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-medium text-primary-600 transition-colors hover:bg-brand-500/10 dark:text-primary-400 dark:hover:bg-brand-400/10"
                onClick={() => setExpanded(value => !value)}
              >
                <span className={expanded ? "i-mdi-chevron-up h-3.5 w-3.5" : "i-mdi-chevron-down h-3.5 w-3.5"} aria-hidden="true" />
                {expanded ? t("common.toast.hideDetails") : t("common.toast.showDetails")}
              </button>

              {expanded && (
                <div className="mt-2 max-h-40 overflow-auto rounded-lg border border-brand-500/10 bg-brand-500/5 p-3 text-[12px] text-brand-700 scrollbar-hide dark:border-brand-400/10 dark:bg-brand-400/5 dark:text-brand-300">
                  <pre className="whitespace-pre-wrap break-words font-mono leading-relaxed">
                    {content.details}
                  </pre>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      <button
        type="button"
        aria-label={t("common.toast.dismiss")}
        className="absolute right-3 top-3 flex h-7 w-7 items-center justify-center rounded-lg text-brand-400 transition-colors hover:bg-brand-500/10 hover:text-brand-700 dark:text-brand-500 dark:hover:bg-brand-400/10 dark:hover:text-brand-200"
        onClick={() => toastApi.dismiss(toast.id)}
      >
        <span className="i-mdi-close h-4 w-4" aria-hidden="true" />
      </button>

      <div className="h-1 w-full bg-brand-500/10 dark:bg-brand-400/10">
        <div
          className={`toast-progress h-full origin-left ${tone.progress}`}
          style={{ animationDuration: `${duration}ms` }}
        />
      </div>
    </div>
  );
}

export function AppToaster({ topOffset = 16 }: { topOffset?: number }) {
  const { toasts } = useToasterStore();

  useEffect(() => {
    const visibleToasts = toasts
      .filter(toast => toast.visible)
      .sort((first, second) => first.createdAt - second.createdAt);

    if (visibleToasts.length <= MAX_VISIBLE_TOASTS)
      return;

    for (const toast of visibleToasts.slice(0, visibleToasts.length - MAX_VISIBLE_TOASTS)) {
      toastApi.dismiss(toast.id);
    }
  }, [toasts]);

  return (
    <Toaster
      position="top-right"
      gutter={10}
      containerStyle={{
        top: topOffset,
        right: 16,
        zIndex: 40,
      }}
      toastOptions={{
        duration: DEFAULT_TOAST_DURATION,
        style: {
          background: "transparent",
          boxShadow: "none",
          padding: 0,
          margin: 0,
          maxWidth: "380px",
        },
      }}
    >
      {toast => <ToastCard toast={toast} />}
    </Toaster>
  );
}
