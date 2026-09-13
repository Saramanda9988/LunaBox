import type { CSSProperties } from "react";
import type { Toast, ToastPosition } from "react-hot-toast";
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { resolveValue, toast as toastApi, useToaster } from "react-hot-toast";
import { useTranslation } from "react-i18next";

const MAX_VISIBLE_TOASTS = 4;
const DEFAULT_TOAST_DURATION = 4000;
const TOAST_GAP = 12;
const TOAST_PEEK = 10;
const TOAST_POSITIONS: ToastPosition[] = [
  "top-left",
  "top-center",
  "top-right",
  "bottom-left",
  "bottom-center",
  "bottom-right",
];
const TOAST_OPTIONS = {
  duration: DEFAULT_TOAST_DURATION,
  removeDelay: 280,
  loading: { duration: Infinity },
};

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
        iconWrap:
          "bg-success-500/15 text-success-600 dark:bg-success-500/20 dark:text-success-400",
        progress: "text-success-500 dark:text-success-400",
      };
    }
    case "error": {
      return {
        iconWrap:
          "bg-error-500/15 text-error-600 dark:bg-error-500/20 dark:text-error-400",
        progress: "text-error-500 dark:text-error-400",
      };
    }
    case "loading": {
      return {
        iconWrap:
          "bg-info-500/15 text-info-600 dark:bg-info-500/20 dark:text-info-400",
        progress: "text-info-500 dark:text-info-400",
      };
    }
    default: {
      return {
        iconWrap:
          "bg-primary-500/15 text-primary-600 dark:bg-primary-500/20 dark:text-primary-400",
        progress: "text-primary-500 dark:text-primary-400",
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
        <svg
          viewBox="0 0 20 20"
          className="h-4.5 w-4.5 fill-current"
          aria-hidden="true"
        >
          <path d="M16.704 5.29a1 1 0 0 1 .006 1.414l-7.2 7.262a1 1 0 0 1-1.42 0L3.29 9.127a1 1 0 0 1 1.42-1.406l4.09 4.127 6.49-6.545a1 1 0 0 1 1.414-.013Z" />
        </svg>
      );
    case "error":
      return (
        <svg
          viewBox="0 0 20 20"
          className="h-4.5 w-4.5 fill-current"
          aria-hidden="true"
        >
          <path d="M10 2.5A7.5 7.5 0 1 0 10 17.5 7.5 7.5 0 0 0 10 2.5Zm2.85 9.15a.9.9 0 1 1-1.27 1.27L10 11.27 8.42 12.92a.9.9 0 1 1-1.27-1.27L8.73 10 7.08 8.42a.9.9 0 1 1 1.27-1.27L10 8.73l1.58-1.65a.9.9 0 1 1 1.27 1.27L11.27 10l1.58 1.65Z" />
        </svg>
      );
    case "loading":
      return (
        <span
          className="i-mdi-loading h-4.5 w-4.5 animate-spin"
          aria-hidden="true"
        />
      );
    default:
      return (
        <svg
          viewBox="0 0 20 20"
          className="h-4.5 w-4.5 fill-current"
          aria-hidden="true"
        >
          <path d="M10 3a7 7 0 1 0 0 14 7 7 0 0 0 0-14Zm.75 9.5a.75.75 0 0 1-1.5 0v-3a.75.75 0 0 1 1.5 0v3Zm0-5.5a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Z" />
        </svg>
      );
  }
}

function ToastCard({ toast, paused }: { toast: Toast; paused: boolean }) {
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
      className={`app-toast-card ${toast.className ?? ""}`}
      style={toast.style}
      {...toast.ariaProps}
      role={toast.type === "error" ? "alert" : toast.ariaProps.role}
    >
      <div className="flex items-start gap-3.5 p-4 pr-11">
        <div
          className={`relative flex h-9 w-9 shrink-0 items-center justify-center rounded-full ${tone.iconWrap}`}
        >
          {Number.isFinite(duration) && toast.type !== "loading" && (
            <svg
              viewBox="0 0 44 44"
              className={`pointer-events-none absolute -inset-1 h-11 w-11 -rotate-90 fill-none ${tone.progress}`}
              aria-hidden="true"
            >
              <circle
                cx="22"
                cy="22"
                r="20"
                className="stroke-current opacity-15"
                strokeWidth="1.75"
              />
              <circle
                key={`${toast.type}-${toast.createdAt}-${duration}`}
                cx="22"
                cy="22"
                r="20"
                pathLength="100"
                strokeWidth="1.75"
                strokeLinecap="round"
                strokeDasharray="100"
                className="stroke-current animate-app-toast-progress motion-reduce:animate-none"
                style={{
                  animationDuration: `${duration}ms`,
                  animationPlayState:
                    paused || !toast.visible ? "paused" : "running",
                }}
              />
            </svg>
          )}
          <ToastGlyph toast={toast} />
        </div>

        <div className="min-w-0 flex-1 flex flex-col justify-center min-h-9">
          <div className="text-[14px] leading-snug">
            {typeof content.title === "string" ? (
              <p className="break-words font-medium">{content.title}</p>
            ) : (
              content.title
            )}
          </div>

          {content.hasDetails && (
            <div className="mt-2.5">
              <button
                type="button"
                className="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-medium text-primary-600 transition-colors hover:bg-brand-500/10 focus-visible:outline-2 focus-visible:outline-primary-500 dark:text-primary-400 dark:hover:bg-brand-400/10"
                aria-expanded={expanded}
                onClick={() => setExpanded(value => !value)}
              >
                <span
                  className={
                    expanded
                      ? "i-mdi-chevron-up h-3.5 w-3.5"
                      : "i-mdi-chevron-down h-3.5 w-3.5"
                  }
                  aria-hidden="true"
                />
                {expanded
                  ? t("common.toast.hideDetails")
                  : t("common.toast.showDetails")}
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
        className="absolute right-3 top-3 flex h-7 w-7 items-center justify-center rounded-full text-brand-400 transition-colors hover:bg-brand-500/10 hover:text-brand-700 focus-visible:outline-2 focus-visible:outline-primary-500 dark:text-brand-500 dark:hover:bg-brand-400/10 dark:hover:text-brand-200"
        onClick={() => toastApi.dismiss(toast.id)}
      >
        <span className="i-mdi-close h-4 w-4" aria-hidden="true" />
      </button>
    </div>
  );
}

function ToastStackItem({
  toast,
  index,
  offset,
  frontHeight,
  expanded,
  paused,
  bottom,
  updateHeight,
}: {
  toast: Toast;
  index: number;
  offset: number;
  frontHeight: number;
  expanded: boolean;
  paused: boolean;
  bottom: boolean;
  updateHeight: (id: string, height: number) => void;
}) {
  const contentRef = useRef<HTMLDivElement>(null);
  const lastLayout = useRef({ index, offset, frontHeight, expanded });

  useLayoutEffect(() => {
    if (toast.visible)
      lastLayout.current = { index, offset, frontHeight, expanded };
  }, [toast.visible, index, offset, frontHeight, expanded]);

  useLayoutEffect(() => {
    const element = contentRef.current;
    if (!element)
      return;

    const measure = () => updateHeight(toast.id, element.offsetHeight + 2);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, [toast.id, updateHeight]);

  // Preserve the dismissed card's position while the remaining cards move forward.
  const layout = toast.visible
    ? { index, offset, frontHeight, expanded }
    : lastLayout.current;
  const behind = layout.index > 0 && !layout.expanded;
  const height = layout.expanded ? toast.height : layout.frontHeight;
  const scale = layout.expanded ? 1 : 1 - layout.index * 0.045;
  const translateY
    = (bottom ? -1 : 1)
      * (layout.expanded ? layout.offset : layout.index * TOAST_PEEK);

  return (
    <div
      className="app-toast-stack-item motion-reduce:transition-none"
      {...(behind || !toast.visible ? { inert: "" } : {})}
      style={
        {
          "top": bottom ? undefined : 0,
          "bottom": bottom ? 0 : undefined,
          "height": height || undefined,
          "zIndex": MAX_VISIBLE_TOASTS - layout.index,
          "transform": `translate3d(0, ${translateY}px, 0) scale(${scale})`,
          "transformOrigin": bottom ? "bottom center" : "top center",
          "pointerEvents": toast.visible ? "auto" : "none",
          "opacity": toast.visible ? 1 : 0,
          // These variables also make entrance and exit follow the viewport edge.
          "--app-toast-enter-y": bottom
            ? "calc(100% + 24px)"
            : "calc(-100% - 24px)",
          "--app-toast-leave-y": bottom ? "16px" : "-16px",
        } as CSSProperties
      }
    >
      <div
        className={`h-full ${toast.visible ? "animate-app-toast-enter" : "animate-app-toast-leave"} motion-reduce:animate-none`}
      >
        <div className="app-toast-stack-surface">
          <div
            ref={contentRef}
            className="transition-opacity duration-200 motion-reduce:transition-none"
            style={{ opacity: behind ? 0 : 1 }}
          >
            {toast.type === "custom" ? (
              resolveValue(toast.message, toast)
            ) : (
              <ToastCard toast={toast} paused={paused} />
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

export function AppToaster({ topOffset = 16 }: { topOffset?: number }) {
  const { toasts, handlers } = useToaster(TOAST_OPTIONS);
  const { startPause, endPause, calculateOffset, updateHeight } = handlers;
  const [hoveredPosition, setHoveredPosition] = useState<ToastPosition | null>(
    null,
  );
  const [focusedPosition, setFocusedPosition] = useState<ToastPosition | null>(
    null,
  );
  const paused = hoveredPosition !== null || focusedPosition !== null;

  useEffect(() => {
    if (paused) {
      startPause();
      return endPause;
    }
  }, [paused, startPause, endPause]);

  useEffect(() => {
    const hasVisibleToasts = (position: ToastPosition | null) =>
      toasts.some(
        toast =>
          toast.visible && (toast.position ?? "top-right") === position,
      );
    if (hoveredPosition && !hasVisibleToasts(hoveredPosition))
      setHoveredPosition(null);
    if (focusedPosition && !hasVisibleToasts(focusedPosition))
      setFocusedPosition(null);
  }, [toasts, hoveredPosition, focusedPosition]);

  useEffect(() => {
    const visibleToasts = toasts
      .filter(toast => toast.visible)
      .sort((first, second) => first.createdAt - second.createdAt);

    if (visibleToasts.length <= MAX_VISIBLE_TOASTS)
      return;

    for (const toast of visibleToasts.slice(
      0,
      visibleToasts.length - MAX_VISIBLE_TOASTS,
    )) {
      toastApi.dismiss(toast.id);
    }
  }, [toasts]);

  return (
    <>
      {TOAST_POSITIONS.map((position) => {
        const positionToasts = toasts.filter(
          toast => (toast.position ?? "top-right") === position,
        );
        if (!positionToasts.length)
          return null;

        const visibleToasts = positionToasts.filter(toast => toast.visible);
        const expanded
          = hoveredPosition === position || focusedPosition === position;
        const bottom = position.startsWith("bottom");
        const frontHeight
          = visibleToasts[0]?.height ?? positionToasts[0]?.height ?? 0;
        const totalHeight = visibleToasts.reduce(
          (height, toast) => height + (toast.height ?? 0) + TOAST_GAP,
          -TOAST_GAP,
        );

        return (
          <div
            key={position}
            className="fixed z-[9999] w-[min(380px,calc(100vw-32px))]"
            style={{
              top: bottom ? undefined : topOffset,
              bottom: bottom ? 16 : undefined,
              right: position.endsWith("right") ? 16 : undefined,
              left: position.endsWith("left")
                ? 16
                : position.endsWith("center")
                  ? "50%"
                  : undefined,
              transform: position.endsWith("center")
                ? "translateX(-50%)"
                : undefined,
              height: Math.max(
                0,
                expanded
                  ? totalHeight
                  : frontHeight + (visibleToasts.length - 1) * TOAST_PEEK,
              ),
              pointerEvents: visibleToasts.length ? "auto" : "none",
            }}
            onMouseEnter={() => setHoveredPosition(position)}
            onMouseLeave={() => setHoveredPosition(null)}
            onFocusCapture={() => setFocusedPosition(position)}
            onBlurCapture={(event) => {
              if (!event.currentTarget.contains(event.relatedTarget))
                setFocusedPosition(null);
            }}
          >
            {positionToasts.map(toast => (
              <ToastStackItem
                key={toast.id}
                toast={toast}
                index={Math.max(
                  0,
                  visibleToasts.findIndex(item => item.id === toast.id),
                )}
                offset={calculateOffset(toast, {
                  gutter: TOAST_GAP,
                  defaultPosition: "top-right",
                })}
                frontHeight={frontHeight}
                expanded={expanded}
                paused={paused}
                bottom={bottom}
                updateHeight={updateHeight}
              />
            ))}
          </div>
        );
      })}
    </>
  );
}
