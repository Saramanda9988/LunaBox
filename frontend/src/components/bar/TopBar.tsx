import { useEffect, useRef, useState } from "react";
import {
  Environment,
  Quit,
  WindowIsMaximised,
  WindowMaximise,
  WindowMinimise,
  WindowUnmaximise,
} from "../../../wailsjs/runtime/runtime";

export const TOPBAR_HEIGHT = 28;
const WINDOW_STATE_SYNC_INTERVAL_MS = 500;
const WINDOW_STATE_SETTLE_MS = 800;
const WINDOW_STATE_DRAG_SYNC_DELAYS_MS = [80, 300] as const;

export function TopBar() {
  const [platform, setPlatform] = useState<string | null>(null);
  const [isMaximised, setIsMaximised] = useState(false);
  const isMaximisedRef = useRef(false);
  const pendingWindowStateRef = useRef<{
    until: number;
    value: boolean;
  } | null>(null);
  const isMac = platform === "darwin";
  const showWindowControls = platform !== null && !isMac;

  function setMaximisedState(maximised: boolean) {
    isMaximisedRef.current = maximised;
    setIsMaximised(maximised);
  }

  async function syncMaximisedState(force = false) {
    const maximised = await WindowIsMaximised();
    const pending = pendingWindowStateRef.current;

    if (
      !force
      && pending
      && Date.now() < pending.until
      && maximised !== pending.value
    ) {
      return;
    }

    pendingWindowStateRef.current = null;
    setMaximisedState(maximised);
  }

  function setOptimisticMaximisedState(maximised: boolean) {
    pendingWindowStateRef.current = {
      until: Date.now() + WINDOW_STATE_SETTLE_MS,
      value: maximised,
    };
    setMaximisedState(maximised);
    window.setTimeout(() => {
      void syncMaximisedState(true);
    }, WINDOW_STATE_SETTLE_MS);
  }

  async function getMaximisedStateForCommand() {
    const pending = pendingWindowStateRef.current;

    if (pending && Date.now() < pending.until) {
      return pending.value;
    }

    const maximised = await WindowIsMaximised();
    setMaximisedState(maximised);
    return maximised;
  }

  function scheduleForcedMaximisedStateSync() {
    pendingWindowStateRef.current = null;

    for (const delay of WINDOW_STATE_DRAG_SYNC_DELAYS_MS) {
      window.setTimeout(() => {
        void syncMaximisedState(true);
      }, delay);
    }
  }

  useEffect(() => {
    let mounted = true;

    Environment()
      .then((environment) => {
        if (mounted) {
          setPlatform(environment.platform);
        }
      })
      .catch(() => {
        if (mounted) {
          setPlatform("unknown");
        }
      });

    return () => {
      mounted = false;
    };
  }, []);

  // 检查窗口最大化状态
  useEffect(() => {
    if (isMac) {
      return;
    }

    const sync = () => {
      void syncMaximisedState();
    };

    sync();
    window.addEventListener("resize", sync);

    // 定期检查（因为用户可能通过拖拽边缘等方式改变窗口状态）
    const interval = window.setInterval(sync, WINDOW_STATE_SYNC_INTERVAL_MS);
    return () => {
      window.removeEventListener("resize", sync);
      window.clearInterval(interval);
    };
  }, [isMac]);

  const handleMinimise = () => {
    WindowMinimise();
  };

  const toggleWindowMaximised = async () => {
    const maximised = await getMaximisedStateForCommand();

    if (maximised) {
      await WindowUnmaximise();
      setOptimisticMaximisedState(false);
    }
    else {
      await WindowMaximise();
      setOptimisticMaximisedState(true);
    }
  };

  const handleMaximise = async () => {
    await toggleWindowMaximised();
  };

  const handleClose = () => {
    Quit();
  };

  const handleTopBarMouseDown = async (e: React.MouseEvent<HTMLDivElement>) => {
    if (e.button !== 0) {
      return;
    }

    if (e.detail === 1 && isMaximisedRef.current) {
      window.addEventListener("mouseup", scheduleForcedMaximisedStateSync, {
        once: true,
      });
      return;
    }

    if (e.detail !== 2) {
      return;
    }

    e.preventDefault();
    await toggleWindowMaximised();
  };

  return (
    <div
      onMouseDown={handleTopBarMouseDown}
      className="flex h-[28px] select-none items-center justify-center border-b border-brand-200/50 bg-brand-50 dark:border-brand-700/50 dark:bg-brand-800"
      style={
        {
          "--wails-draggable": "drag",
        } as React.CSSProperties
      }
    >
      {/* 中央标题 */}
      <img
        src="/topbar-title-dark.png"
        className="h-[20px] absolute dark:hidden left-1/2 -translate-x-1/2 pointer-events-none"
        draggable="false"
        onDragStart={e => e.preventDefault()}
      />
      <img
        src="/topbar-title.png"
        className="h-[20px] absolute hidden dark:block left-1/2 -translate-x-1/2 pointer-events-none"
        draggable="false"
        onDragStart={e => e.preventDefault()}
      />

      {showWindowControls && (
        <div
          onDoubleClick={e => e.stopPropagation()}
          onMouseDown={e => e.stopPropagation()}
          className="ml-auto flex items-center"
          style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}
        >
          {/* 最小化 */}
          <button
            onClick={handleMinimise}
            className="flex h-[28px] w-[44px] items-center justify-center transition-colors hover:bg-brand-200 active:scale-98 dark:hover:bg-brand-700"
            title="最小化"
          >
            <svg
              className="h-[10px] w-[10px] text-brand-600 dark:text-brand-400"
              viewBox="0 0 12 12"
              fill="none"
            >
              <path d="M0 6h12" stroke="currentColor" strokeWidth="1.5" />
            </svg>
          </button>

          {/* 最大化/还原 */}
          <button
            onClick={handleMaximise}
            className="flex h-[28px] w-[44px] items-center justify-center transition-colors hover:bg-brand-200 active:scale-98 dark:hover:bg-brand-700"
            title={isMaximised ? "还原" : "最大化"}
          >
            {isMaximised ? (
              <svg
                className="h-[10px] w-[10px] text-brand-600 dark:text-brand-400"
                viewBox="0 0 12 12"
                fill="none"
              >
                <path
                  d="M3 3h6v6H3V3z"
                  stroke="currentColor"
                  strokeWidth="1.5"
                />
                <path d="M5 1h6v6" stroke="currentColor" strokeWidth="1.5" />
              </svg>
            ) : (
              <svg
                className="h-[10px] w-[10px] text-brand-600 dark:text-brand-400"
                viewBox="0 0 12 12"
                fill="none"
              >
                <path
                  d="M1 1h10v10H1V1z"
                  stroke="currentColor"
                  strokeWidth="1.5"
                />
              </svg>
            )}
          </button>

          {/* 关闭 */}
          <button
            onClick={handleClose}
            className="flex h-[28px] w-[44px] items-center justify-center transition-colors hover:bg-red-500 active:scale-98"
            title="关闭"
          >
            <svg
              className="h-[10px] w-[10px] text-brand-600 transition-colors group-hover:text-white dark:text-brand-400"
              viewBox="0 0 12 12"
              fill="none"
            >
              <path
                d="M1 1l10 10M11 1L1 11"
                stroke="currentColor"
                strokeWidth="1.5"
              />
            </svg>
          </button>
        </div>
      )}
    </div>
  );
}
