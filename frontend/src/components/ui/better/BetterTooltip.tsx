import type { CSSProperties, ReactNode } from "react";
import { useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

export type BetterTooltipSide = "top" | "right" | "bottom" | "left";

interface BetterTooltipProps {
  children: ReactNode;
  content: ReactNode;
  side?: BetterTooltipSide;
  delay?: number;
  className?: string;
}

interface TooltipPosition {
  left: number;
  side: BetterTooltipSide;
  top: number;
}

interface TooltipSize {
  height: number;
  width: number;
}

const viewportPadding = 8;
const triggerGap = 8;

function getTooltipPosition(
  triggerRect: DOMRect,
  tooltipSize: TooltipSize,
  preferredSide: BetterTooltipSide,
): TooltipPosition {
  let side = preferredSide;

  if (side === "top" && triggerRect.top - tooltipSize.height < triggerGap) {
    side = "bottom";
  }
  else if (
    side === "bottom"
    && triggerRect.bottom + tooltipSize.height + triggerGap > window.innerHeight
  ) {
    side = "top";
  }
  else if (
    side === "left"
    && triggerRect.left - tooltipSize.width < triggerGap
  ) {
    side = "right";
  }
  else if (
    side === "right"
    && triggerRect.right + tooltipSize.width + triggerGap > window.innerWidth
  ) {
    side = "left";
  }

  let left = triggerRect.left + (triggerRect.width - tooltipSize.width) / 2;
  let top = triggerRect.top - tooltipSize.height - triggerGap;

  if (side === "bottom") {
    top = triggerRect.bottom + triggerGap;
  }
  else if (side === "left") {
    left = triggerRect.left - tooltipSize.width - triggerGap;
    top = triggerRect.top + (triggerRect.height - tooltipSize.height) / 2;
  }
  else if (side === "right") {
    left = triggerRect.right + triggerGap;
    top = triggerRect.top + (triggerRect.height - tooltipSize.height) / 2;
  }

  return {
    left: Math.min(
      Math.max(left, viewportPadding),
      window.innerWidth - tooltipSize.width - viewportPadding,
    ),
    side,
    top: Math.min(
      Math.max(top, viewportPadding),
      window.innerHeight - tooltipSize.height - viewportPadding,
    ),
  };
}

const arrowPositionClasses: Record<BetterTooltipSide, string> = {
  top: "left-1/2 top-full -translate-x-1/2 -translate-y-1/2",
  right: "right-full top-1/2 translate-x-1/2 -translate-y-1/2",
  bottom: "bottom-full left-1/2 -translate-x-1/2 translate-y-1/2",
  left: "left-full top-1/2 -translate-x-1/2 -translate-y-1/2",
};

export function BetterTooltip({
  children,
  content,
  side = "top",
  delay = 300,
  className = "",
}: BetterTooltipProps) {
  const tooltipId = useId();
  const triggerRef = useRef<HTMLSpanElement>(null);
  const tooltipRef = useRef<HTMLDivElement>(null);
  const openTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const [position, setPosition] = useState<TooltipPosition | null>(null);

  const clearOpenTimer = () => {
    if (openTimerRef.current) {
      clearTimeout(openTimerRef.current);
      openTimerRef.current = null;
    }
  };

  const openWithDelay = () => {
    clearOpenTimer();
    openTimerRef.current = setTimeout(() => setIsOpen(true), delay);
  };

  const openImmediately = () => {
    clearOpenTimer();
    setIsOpen(true);
  };

  const close = () => {
    clearOpenTimer();
    setIsOpen(false);
    setPosition(null);
  };

  useEffect(() => clearOpenTimer, []);

  useLayoutEffect(() => {
    if (!isOpen) {
      return;
    }

    const updatePosition = () => {
      if (!triggerRef.current || !tooltipRef.current) {
        return;
      }
      // The floating element must be measured after the portal has mounted.
      // eslint-disable-next-line react-hooks-extra/no-direct-set-state-in-use-effect
      setPosition(
        getTooltipPosition(
          triggerRef.current.getBoundingClientRect(),
          {
            height: tooltipRef.current.offsetHeight,
            width: tooltipRef.current.offsetWidth,
          },
          side,
        ),
      );
    };

    updatePosition();
    window.addEventListener("resize", updatePosition);
    window.addEventListener("scroll", updatePosition, true);

    return () => {
      window.removeEventListener("resize", updatePosition);
      window.removeEventListener("scroll", updatePosition, true);
    };
  }, [isOpen, side]);

  const tooltipStyle: CSSProperties | undefined = position
    ? { left: position.left, top: position.top }
    : undefined;

  return (
    <span
      ref={triggerRef}
      className="inline-flex"
      aria-describedby={isOpen ? tooltipId : undefined}
      onMouseEnter={openWithDelay}
      onMouseLeave={close}
      onFocusCapture={openImmediately}
      onBlurCapture={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) {
          close();
        }
      }}
      onClick={close}
      onKeyDown={(event) => {
        if (event.key === "Escape") {
          close();
        }
      }}
    >
      {children}
      {isOpen
        && createPortal(
          <div
            ref={tooltipRef}
            id={tooltipId}
            role="tooltip"
            style={tooltipStyle}
            className={[
              "pointer-events-none fixed z-[9500] max-w-72 rounded-md px-2.5 py-1.5",
              "border border-white/10 bg-brand-900 text-xs font-medium leading-4 text-white",
              "animate-tooltip-enter motion-reduce:animate-none dark:border-brand-200 dark:bg-brand-100 dark:text-brand-900",
              "data-glass:backdrop-blur-8 data-glass:bg-brand-900/85 data-glass:dark:bg-brand-100/90",
              position ? "visible" : "invisible",
              className,
            ].join(" ")}
          >
            {content}
            {position && (
              <span
                aria-hidden="true"
                className={[
                  "absolute h-2 w-2 rotate-45 bg-brand-900",
                  "dark:bg-brand-100 data-glass:bg-brand-900/85 data-glass:dark:bg-brand-100/90",
                  arrowPositionClasses[position.side],
                ].join(" ")}
              />
            )}
          </div>,
          document.body,
        )}
    </span>
  );
}
