import type { CSSProperties } from "react";
import { useId, useRef, useState } from "react";
import { ModalPortal } from "../ModalPortal";
import { ProxyImage } from "../ProxyImage";

const MIN_SCALE = 0.5;
const MAX_SCALE = 5;
const SCALE_STEP = 0.25;

export interface BetterImageViewerLabels {
  close: string;
  reset: string;
  save: string;
  zoomIn: string;
  zoomOut: string;
}

interface BetterImageViewerProps {
  alt?: string;
  fallbackSrc?: string;
  isOpen: boolean;
  labels?: BetterImageViewerLabels;
  onClose: () => void;
  onSave?: () => void | Promise<void>;
  src: string;
  title?: string;
}

const DEFAULT_LABELS: BetterImageViewerLabels = {
  close: "Close image viewer",
  reset: "Reset image position and scale",
  save: "Save image as",
  zoomIn: "Zoom in",
  zoomOut: "Zoom out",
};

function clampScale(value: number): number {
  return Math.min(MAX_SCALE, Math.max(MIN_SCALE, Number(value.toFixed(2))));
}

/**
 * 带缩放、拖拽及可选保存操作的图片查看器。
 */
export function BetterImageViewer({
  alt,
  fallbackSrc,
  isOpen,
  labels = DEFAULT_LABELS,
  onClose,
  onSave,
  src,
  title,
}: BetterImageViewerProps) {
  const [scale, setScale] = useState(1);
  const [translate, setTranslate] = useState({ x: 0, y: 0 });
  const [isDragging, setIsDragging] = useState(false);
  const titleId = useId();
  const dragStartRef = useRef({
    pointerX: 0,
    pointerY: 0,
    translateX: 0,
    translateY: 0,
  });
  const titleText = title || alt || "Image preview";

  const resetImage = () => {
    setScale(1);
    setTranslate({ x: 0, y: 0 });
    setIsDragging(false);
  };

  const closeViewer = () => {
    resetImage();
    onClose();
  };

  const zoomIn = () => setScale(current => clampScale(current + SCALE_STEP));

  const zoomOut = () => setScale(current => clampScale(current - SCALE_STEP));

  const handleWheel = (event: React.WheelEvent<HTMLDivElement>) => {
    event.preventDefault();
    setScale(current =>
      clampScale(current + (event.deltaY > 0 ? -SCALE_STEP : SCALE_STEP)),
    );
  };

  const handlePointerDown = (event: React.PointerEvent<HTMLImageElement>) => {
    if (event.button !== 0) {
      return;
    }

    event.preventDefault();
    event.stopPropagation();
    dragStartRef.current = {
      pointerX: event.clientX,
      pointerY: event.clientY,
      translateX: translate.x,
      translateY: translate.y,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
    setIsDragging(true);
  };

  const handlePointerMove = (event: React.PointerEvent<HTMLImageElement>) => {
    if (!isDragging) {
      return;
    }

    event.stopPropagation();
    const start = dragStartRef.current;
    setTranslate({
      x: start.translateX + event.clientX - start.pointerX,
      y: start.translateY + event.clientY - start.pointerY,
    });
  };

  const stopDragging = (event: React.PointerEvent<HTMLImageElement>) => {
    if (!isDragging) {
      return;
    }

    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    setIsDragging(false);
  };

  const imageStyle: CSSProperties = {
    transform: `translate3d(${translate.x}px, ${translate.y}px, 0) scale(${scale})`,
  };

  if (!isOpen) {
    return null;
  }

  return (
    <ModalPortal>
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        className="absolute inset-0 z-50 flex min-h-0 flex-col overflow-hidden bg-transparent p-0 outline-none"
        onKeyDownCapture={(event) => {
          if (event.key === "Escape") {
            event.preventDefault();
            closeViewer();
          }
        }}
      >
        <div
          aria-hidden="true"
          className="absolute inset-0 bg-transparent backdrop-blur-md"
        />

        <h2 id={titleId} className="sr-only">
          {titleText}
        </h2>

        <header className="pointer-events-none absolute inset-x-0 top-0 z-10 flex items-center justify-between gap-4 px-4 py-3 sm:px-6">
          <p className="pointer-events-auto min-w-0 truncate text-sm font-medium text-brand-900 dark:text-white">
            {titleText}
          </p>
          <button
            type="button"
            onClick={closeViewer}
            aria-label={labels.close}
            autoFocus
            className="pointer-events-auto inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-brand-200/80 bg-white/85 text-brand-600 shadow-sm shadow-black/10 backdrop-blur-xl transition-colors hover:bg-white hover:text-brand-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-500 dark:border-brand-600/80 dark:bg-brand-800/85 dark:text-brand-300 dark:hover:bg-brand-800 dark:hover:text-white"
          >
            <span className="i-mdi-close text-xl" aria-hidden="true" />
          </button>
        </header>

        <div
          className="flex min-h-0 flex-1 items-center justify-center overflow-hidden px-4 pb-24 pt-16 sm:px-8 sm:pb-28"
          onClick={(event) => {
            if (event.target === event.currentTarget) {
              closeViewer();
            }
          }}
          onWheel={handleWheel}
        >
          <ProxyImage
            src={src}
            fallbackSrc={fallbackSrc}
            alt={alt || titleText}
            draggable={false}
            style={imageStyle}
            className={`max-h-[calc(100dvh-8.5rem)] max-w-full select-none object-contain transition-transform duration-100 ease-out ${
              isDragging ? "cursor-grabbing transition-none" : "cursor-grab"
            }`}
            onPointerDown={handlePointerDown}
            onPointerMove={handlePointerMove}
            onPointerUp={stopDragging}
            onPointerCancel={stopDragging}
            onLostPointerCapture={() => setIsDragging(false)}
          />
        </div>

        <div className="pointer-events-none absolute inset-x-0 bottom-5 z-10 flex justify-center px-4 sm:bottom-7">
          <div className="pointer-events-auto flex items-center gap-1 rounded-full border border-brand-200/80 bg-white/85 p-1 shadow-lg shadow-black/10 backdrop-blur-xl dark:border-brand-600/80 dark:bg-brand-800/85">
            <button
              type="button"
              onClick={zoomIn}
              disabled={scale >= MAX_SCALE}
              aria-label={labels.zoomIn}
              className="inline-flex h-9 w-9 items-center justify-center rounded-full text-brand-600 transition-colors hover:bg-brand-100 hover:text-brand-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-500 disabled:cursor-not-allowed disabled:opacity-45 dark:text-brand-300 dark:hover:bg-brand-700 dark:hover:text-white"
            >
              <span
                className="i-mdi-magnify-plus-outline text-xl"
                aria-hidden="true"
              />
            </button>
            <button
              type="button"
              onClick={zoomOut}
              disabled={scale <= MIN_SCALE}
              aria-label={labels.zoomOut}
              className="inline-flex h-9 w-9 items-center justify-center rounded-full text-brand-600 transition-colors hover:bg-brand-100 hover:text-brand-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-500 disabled:cursor-not-allowed disabled:opacity-45 dark:text-brand-300 dark:hover:bg-brand-700 dark:hover:text-white"
            >
              <span
                className="i-mdi-magnify-minus-outline text-xl"
                aria-hidden="true"
              />
            </button>
            <button
              type="button"
              onClick={resetImage}
              aria-label={labels.reset}
              className="inline-flex h-9 w-9 items-center justify-center rounded-full text-brand-600 transition-colors hover:bg-brand-100 hover:text-brand-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-500 dark:text-brand-300 dark:hover:bg-brand-700 dark:hover:text-white"
            >
              <span className="i-mdi-restore text-xl" aria-hidden="true" />
            </button>
            {onSave && (
              <button
                type="button"
                onClick={() => void onSave()}
                aria-label={labels.save}
                className="inline-flex h-9 w-9 items-center justify-center rounded-full text-brand-600 transition-colors hover:bg-brand-100 hover:text-brand-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-500 dark:text-brand-300 dark:hover:bg-brand-700 dark:hover:text-white"
              >
                <span
                  className="i-mdi-content-save-outline text-xl"
                  aria-hidden="true"
                />
              </button>
            )}
          </div>
        </div>
      </div>
    </ModalPortal>
  );
}
