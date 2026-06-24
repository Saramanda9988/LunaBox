import type {
  CSSProperties,
  KeyboardEvent as ReactKeyboardEvent,
  MouseEvent as ReactMouseEvent,
  PointerEvent as ReactPointerEvent,
} from "react";
import type { GameRuntimeInfo } from "../../store";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { EndCurrentPlaySession } from "../../../wailsjs/go/service/StartService";
import { useElapsedSeconds } from "../../hooks/useElapsedSeconds";
import { isGameRuntimeVisible, useAppStore } from "../../store";
import { formatDurationCompact } from "../../utils/time";
import { ProxyImage } from "../ui/ProxyImage";

const COLLAPSE_DRAG_THRESHOLD = 24;
const ISLAND_HEIGHT = 56;
const EXPANDED_TOP_GAP = 12;
const COLLAPSED_PEEK_HEIGHT = 14;
const COLLAPSE_TRANSLATE_Y
  = COLLAPSED_PEEK_HEIGHT - ISLAND_HEIGHT - EXPANDED_TOP_GAP;
const HIDE_DISTANCE = Math.abs(COLLAPSE_TRANSLATE_Y);
const EXPANDED_ISLAND_WIDTH = "min(19rem, calc(100vw - 9rem))";
const COLLAPSED_ISLAND_WIDTH = "15rem";
const AUTO_COLLAPSE_AFTER_PLAYING_SECONDS = 5;
const INTERACTIVE_SELECTOR
  = "[data-playing-island-end],[data-playing-island-switch]";
const ENTER_ANIMATION_MS = 360;
const EXIT_ANIMATION_MS = 220;
interface IslandDragState {
  pointerId: number;
  startCollapsed: boolean;
  startY: number;
}

export function PlayingIsland() {
  const gameRuntime = useAppStore(
    state => state.gameRuntimes[state.activeGameRuntimeId] ?? null,
  );
  const runtimeCount = useAppStore(
    state => Object.keys(state.gameRuntimes).length,
  );
  const selectNextGameRuntime = useAppStore(
    state => state.selectNextGameRuntime,
  );
  const visible = isGameRuntimeVisible(gameRuntime);
  const [renderRuntime, setRenderRuntime] = useState<GameRuntimeInfo | null>(
    () => (visible && gameRuntime?.game ? gameRuntime : null),
  );
  const [isExiting, setIsExiting] = useState(false);
  const exitTimerRef = useRef<number | null>(null);

  useEffect(() => {
    if (visible && gameRuntime?.game) {
      if (exitTimerRef.current !== null) {
        window.clearTimeout(exitTimerRef.current);
        exitTimerRef.current = null;
      }
      setRenderRuntime(gameRuntime);
      setIsExiting(false);
      return;
    }

    if (!renderRuntime || isExiting) {
      return;
    }

    setIsExiting(true);
    exitTimerRef.current = window.setTimeout(() => {
      setRenderRuntime(null);
      setIsExiting(false);
      exitTimerRef.current = null;
    }, EXIT_ANIMATION_MS);
  }, [gameRuntime, isExiting, renderRuntime, visible]);

  useEffect(() => {
    return () => {
      if (exitTimerRef.current !== null) {
        window.clearTimeout(exitTimerRef.current);
      }
    };
  }, []);

  const game = renderRuntime?.game;

  if (!renderRuntime || !game) {
    return null;
  }

  return (
    <PlayingIslandBody
      key={`${renderRuntime.gameId}:${renderRuntime.sessionId}`}
      game={game}
      gameRuntime={renderRuntime}
      isExiting={isExiting}
      onSwitchRuntime={selectNextGameRuntime}
      runtimeCount={runtimeCount}
    />
  );
}

function PlayingIslandBody({
  game,
  gameRuntime,
  isExiting,
  onSwitchRuntime,
  runtimeCount,
}: {
  game: NonNullable<GameRuntimeInfo["game"]>;
  gameRuntime: GameRuntimeInfo;
  isExiting: boolean;
  onSwitchRuntime: () => void;
  runtimeCount: number;
}) {
  const { t } = useTranslation();
  const [isCollapsed, setIsCollapsed] = useState(false);
  const [isEnding, setIsEnding] = useState(false);
  const [dragOffset, setDragOffset] = useState(0);
  const [shouldScrollTitle, setShouldScrollTitle] = useState(false);
  const [hasPlayedEnterAnimation, setHasPlayedEnterAnimation] = useState(false);
  const dragRef = useRef<IslandDragState | null>(null);
  const suppressNextClickRef = useRef(false);
  const autoCollapsedRuntimeRef = useRef("");
  const titleMeasureRef = useRef<HTMLSpanElement | null>(null);
  const titleViewportRef = useRef<HTMLDivElement | null>(null);
  const visible = isGameRuntimeVisible(gameRuntime);
  const hasMultipleRuntimes = runtimeCount > 1;
  const elapsedSeconds = useElapsedSeconds(
    gameRuntime.startTime,
    visible && Boolean(gameRuntime.startTime),
  );

  useEffect(() => {
    if (hasPlayedEnterAnimation) {
      return;
    }

    const timer = window.setTimeout(() => {
      setHasPlayedEnterAnimation(true);
    }, ENTER_ANIMATION_MS);

    return () => window.clearTimeout(timer);
  }, [hasPlayedEnterAnimation]);

  const statusText = useMemo(() => {
    if (gameRuntime.state === "launching") {
      return t("playingIsland.launching");
    }
    if (gameRuntime.state === "ending" || isEnding) {
      return t("playingIsland.ending");
    }
    return t("playingIsland.elapsed", {
      duration: formatDurationCompact(elapsedSeconds, t),
    });
  }, [elapsedSeconds, gameRuntime.state, isEnding, t]);

  const runtimeKey = `${gameRuntime.gameId}:${gameRuntime.sessionId}:${String(gameRuntime.startTime ?? "")}`;

  useEffect(() => {
    if (
      isExiting
      || isCollapsed
      || gameRuntime.state !== "playing"
      || autoCollapsedRuntimeRef.current === runtimeKey
    ) {
      return;
    }

    const timer = window.setTimeout(() => {
      autoCollapsedRuntimeRef.current = runtimeKey;
      setIsCollapsed(true);
    }, AUTO_COLLAPSE_AFTER_PLAYING_SECONDS * 1000);

    return () => window.clearTimeout(timer);
  }, [gameRuntime.state, isCollapsed, isExiting, runtimeKey]);

  const canMeasureTitle = !isCollapsed || dragOffset > 0;

  useEffect(() => {
    const measureElement = titleMeasureRef.current;
    const viewportElement = titleViewportRef.current;

    if (!measureElement || !viewportElement || !canMeasureTitle) {
      setShouldScrollTitle(false);
      return;
    }

    let animationFrame = 0;
    let settleTimer = 0;

    const updateTitleOverflow = () => {
      if (animationFrame) {
        window.cancelAnimationFrame(animationFrame);
      }

      animationFrame = window.requestAnimationFrame(() => {
        animationFrame = 0;
        const nextShouldScroll
          = measureElement.offsetWidth > viewportElement.clientWidth + 1;
        setShouldScrollTitle(current =>
          current === nextShouldScroll ? current : nextShouldScroll,
        );
      });
    };

    updateTitleOverflow();
    settleTimer = window.setTimeout(updateTitleOverflow, 430);

    const resizeObserver = new ResizeObserver(updateTitleOverflow);
    resizeObserver.observe(viewportElement);
    resizeObserver.observe(measureElement);

    return () => {
      if (animationFrame) {
        window.cancelAnimationFrame(animationFrame);
      }
      if (settleTimer) {
        window.clearTimeout(settleTimer);
      }
      resizeObserver.disconnect();
    };
  }, [canMeasureTitle, game.name]);

  const handleEndPlay = async () => {
    if (!gameRuntime.gameId || isEnding) {
      return;
    }

    setIsEnding(true);
    try {
      await EndCurrentPlaySession(gameRuntime.gameId);
      toast.success(t("playingIsland.toast.endSuccess"));
    }
    catch (error) {
      console.error("Failed to end current play session:", error);
      toast.error(t("playingIsland.toast.endFailed"));
      setIsEnding(false);
    }
  };

  const handleSwitchRuntime = useCallback(
    (event: ReactMouseEvent<HTMLButtonElement>) => {
      event.stopPropagation();
      if (!hasMultipleRuntimes) {
        return;
      }

      onSwitchRuntime();
    },
    [hasMultipleRuntimes, onSwitchRuntime],
  );

  const resetDrag = useCallback(() => {
    dragRef.current = null;
    setDragOffset(0);
  }, []);

  const handlePointerDown = useCallback(
    (event: ReactPointerEvent<HTMLDivElement>) => {
      if (event.pointerType === "mouse" && event.button !== 0) {
        return;
      }

      const target = event.target;
      if (target instanceof Element && target.closest(INTERACTIVE_SELECTOR)) {
        return;
      }

      dragRef.current = {
        pointerId: event.pointerId,
        startCollapsed: isCollapsed,
        startY: event.clientY,
      };
      event.currentTarget.setPointerCapture(event.pointerId);
    },
    [isCollapsed],
  );

  const handlePointerMove = useCallback(
    (event: ReactPointerEvent<HTMLDivElement>) => {
      const drag = dragRef.current;
      if (!drag || drag.pointerId !== event.pointerId) {
        return;
      }

      const deltaY = event.clientY - drag.startY;

      if (drag.startCollapsed) {
        if (deltaY <= 0) {
          setDragOffset(0);
          return;
        }

        event.preventDefault();
        setDragOffset(Math.min(deltaY, HIDE_DISTANCE));
      }
      else {
        if (deltaY >= 0) {
          setDragOffset(0);
          return;
        }

        event.preventDefault();
        setDragOffset(Math.max(deltaY, COLLAPSE_TRANSLATE_Y));
      }
    },
    [],
  );

  const handlePointerUp = useCallback(
    (event: ReactPointerEvent<HTMLDivElement>) => {
      const drag = dragRef.current;
      if (!drag || drag.pointerId !== event.pointerId) {
        return;
      }

      if (event.currentTarget.hasPointerCapture(event.pointerId)) {
        event.currentTarget.releasePointerCapture(event.pointerId);
      }

      const deltaY = event.clientY - drag.startY;
      dragRef.current = null;
      setDragOffset(0);

      if (drag.startCollapsed && deltaY >= COLLAPSE_DRAG_THRESHOLD) {
        suppressNextClickRef.current = true;
        setIsCollapsed(false);
        window.setTimeout(() => {
          suppressNextClickRef.current = false;
        }, 0);
      }
      else if (!drag.startCollapsed && deltaY <= -COLLAPSE_DRAG_THRESHOLD) {
        suppressNextClickRef.current = true;
        setIsCollapsed(true);
        window.setTimeout(() => {
          suppressNextClickRef.current = false;
        }, 0);
      }
    },
    [],
  );

  const handleCollapsedClick = useCallback(() => {
    if (!isCollapsed) {
      return;
    }

    if (suppressNextClickRef.current) {
      suppressNextClickRef.current = false;
      return;
    }

    setIsCollapsed(false);
  }, [isCollapsed]);

  const handleCollapsedKeyDown = useCallback(
    (event: ReactKeyboardEvent<HTMLDivElement>) => {
      if (!isCollapsed) {
        return;
      }

      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        setIsCollapsed(false);
      }
    },
    [isCollapsed],
  );

  const translateY = isCollapsed
    ? COLLAPSE_TRANSLATE_Y + dragOffset
    : dragOffset;
  const isDragging = dragOffset !== 0;
  const hideProgress = isCollapsed
    ? 1 - Math.min(Math.max(dragOffset, 0) / HIDE_DISTANCE, 1)
    : Math.min(Math.abs(dragOffset) / HIDE_DISTANCE, 1);
  const contentOpacity = 1 - hideProgress;
  const handleOpacity = Math.max(hideProgress, isCollapsed ? 1 : 0);
  const isExpandedWidth = !isCollapsed || dragOffset > 0;
  const islandFrameStyle = {
    transform: `translate(-50%, ${translateY}px)`,
    width: isExpandedWidth ? EXPANDED_ISLAND_WIDTH : COLLAPSED_ISLAND_WIDTH,
  } as CSSProperties;
  const islandStyle = {
    "--wails-draggable": "no-drag",
    "touchAction": "none",
    "cursor": isDragging ? "grabbing" : "grab",
  } as CSSProperties;

  return (
    <div
      className={[
        "pointer-events-none absolute left-1/2 top-[calc(28px+0.75rem)] z-45",
        isDragging
          ? "transition-none"
          : "transition-[width,transform,opacity] duration-[420ms] ease-[cubic-bezier(.2,.9,.18,1)]",
      ].join(" ")}
      style={islandFrameStyle}
    >
      <div
        className={[
          "pointer-events-auto relative w-full overflow-hidden rounded-full bg-white/96 text-brand-900 backdrop-blur-xl dark:bg-black dark:text-white data-glass:bg-white/82 data-glass:dark:bg-black/62",
          "shadow-none ring-1 ring-brand-900/10 dark:ring-white/12",
          "h-14 origin-center transition-[box-shadow,opacity,transform] duration-[420ms] ease-[cubic-bezier(.2,.9,.18,1)]",
          isExiting
            ? "animate-playing-island-leave"
            : hasPlayedEnterAnimation
              ? ""
              : "animate-playing-island-enter",
          isCollapsed ? "opacity-100 shadow-none" : "opacity-100",
        ].join(" ")}
        style={islandStyle}
        role={isCollapsed ? "button" : undefined}
        tabIndex={isCollapsed ? 0 : undefined}
        aria-label={isCollapsed ? t("playingIsland.expand") : undefined}
        onClick={handleCollapsedClick}
        onKeyDown={handleCollapsedKeyDown}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerCancel={resetDrag}
      >
        <div
          className={[
            "flex h-full min-w-0 items-center gap-3 px-3",
            isCollapsed && !isDragging ? "pointer-events-none" : "",
            isDragging
              ? "transition-none"
              : "transition-opacity duration-[260ms] ease-out",
          ].join(" ")}
          style={{ opacity: contentOpacity }}
        >
          <button
            type="button"
            data-playing-island-switch
            aria-label={t("playingIsland.switchGame", {
              count: runtimeCount,
            })}
            disabled={!hasMultipleRuntimes}
            onClick={handleSwitchRuntime}
            className={[
              "relative h-9 w-9 shrink-0 overflow-hidden rounded-full bg-brand-100 ring-1 ring-brand-900/10 transition-transform dark:bg-brand-800 dark:ring-white/16",
              hasMultipleRuntimes
                ? "cursor-pointer hover:scale-105 active:scale-95"
                : "cursor-default",
            ].join(" ")}
          >
            {game.cover_url ? (
              <ProxyImage
                src={game.cover_url}
                alt={game.name}
                className="h-full w-full object-cover"
                decoding="async"
              />
            ) : (
              <div className="flex h-full w-full items-center justify-center">
                <span className="i-mdi-gamepad-variant text-lg text-brand-500 dark:text-white/65" />
              </div>
            )}
            {hasMultipleRuntimes && (
              <span className="absolute right-0 top-0 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-brand-900 px-1 text-[9px] font-semibold leading-none text-white ring-1 ring-white/90 dark:bg-white dark:text-brand-900 dark:ring-black/80">
                {runtimeCount}
              </span>
            )}
          </button>
          <div className="min-w-0 flex-1 overflow-hidden">
            <div
              ref={titleViewportRef}
              className="relative overflow-hidden whitespace-nowrap"
            >
              {shouldScrollTitle ? (
                <div className="inline-block min-w-max animate-playing-island-marquee text-sm font-semibold leading-5">
                  <span>{game.name}</span>
                  <span className="px-8">{game.name}</span>
                </div>
              ) : (
                <div className="truncate text-sm font-semibold leading-5">
                  {game.name}
                </div>
              )}
              <span
                ref={titleMeasureRef}
                aria-hidden="true"
                className="pointer-events-none invisible absolute left-0 top-0 inline-block whitespace-nowrap text-sm font-semibold leading-5"
              >
                {game.name}
              </span>
            </div>
            <div className="text-xs leading-4 text-brand-600 dark:text-white/68">
              {statusText}
            </div>
          </div>
          <button
            type="button"
            data-playing-island-end
            aria-label={t("playingIsland.end")}
            disabled={gameRuntime.state === "ending" || isEnding}
            onClick={handleEndPlay}
            className="flex h-9 w-9 shrink-0 cursor-pointer items-center justify-center rounded-full text-brand-700 transition-colors hover:bg-brand-900/8 active:scale-95 disabled:cursor-not-allowed disabled:opacity-55 dark:text-white dark:hover:bg-white/12"
          >
            <span
              className={
                gameRuntime.state === "ending" || isEnding
                  ? "i-mdi-loading animate-spin text-xl"
                  : "i-mdi-stop text-xl"
              }
            />
          </button>
        </div>
        <div
          className={[
            "pointer-events-none absolute inset-x-0 bottom-1.5 flex justify-center",
            isDragging
              ? "transition-none"
              : "transition-opacity duration-[260ms] ease-out",
          ].join(" ")}
          style={{ opacity: handleOpacity }}
          aria-hidden="true"
        >
          <span className="h-1 w-16 rounded-full bg-brand-500/55 shadow-[0_0_16px_rgba(15,23,42,0.10)] dark:bg-white/68 dark:shadow-[0_0_16px_rgba(255,255,255,0.10)]" />
        </div>
      </div>
    </div>
  );
}
