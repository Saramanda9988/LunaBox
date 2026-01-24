import { useEffect, useState } from "react";
import { Quit, WindowIsMaximised, WindowMaximise, WindowMinimise, WindowUnmaximise } from "../../../wailsjs/runtime/runtime";

interface TopBarProps {
  bgEnabled: boolean;
  bgOpacity: number;
}

export function TopBar({ bgEnabled, bgOpacity }: TopBarProps) {
  const [isMaximised, setIsMaximised] = useState(false);

  // 检查窗口最大化状态
  useEffect(() => {
    const checkMaximised = async () => {
      const maximised = await WindowIsMaximised();
      setIsMaximised(maximised);
    };

    checkMaximised();

    // 定期检查（因为用户可能通过拖拽边缘等方式改变窗口状态）
    const interval = setInterval(checkMaximised, 500);
    return () => clearInterval(interval);
  }, []);

  const handleMinimise = () => {
    WindowMinimise();
  };

  const handleMaximise = async () => {
    if (isMaximised) {
      await WindowUnmaximise();
      setIsMaximised(false);
    }
    else {
      await WindowMaximise();
      setIsMaximised(true);
    }
  };

  const handleClose = () => {
    Quit();
  };

  return (
    <div
      className="flex h-7 select-none items-center justify-center border-b border-brand-200/50 bg-brand-50 dark:border-brand-700/50 dark:bg-brand-800"
      style={{
        "--wails-draggable": "drag",
      } as React.CSSProperties}
    >
      {/* 中央标题 */}
      <img 
        src="/topbar-title-dark.png" 
        className="h-5 absolute dark:hidden left-1/2 -translate-x-1/2" 
        style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}
      />
      <img 
        src="/topbar-title.png" 
        className="h-5 absolute hidden dark:block left-1/2 -translate-x-1/2" 
        style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}
      />

      {/* 右侧：窗口控制按钮 */}
      <div className="ml-auto flex items-center" style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}>
        {/* 最小化 */}
        <button
          onClick={handleMinimise}
          className="flex h-7 w-11 items-center justify-center transition-colors hover:bg-brand-200 active:scale-98 dark:hover:bg-brand-700"
          title="最小化"
        >
          <svg className="h-2.5 w-2.5 text-brand-600 dark:text-brand-400" viewBox="0 0 12 12" fill="none">
            <path d="M0 6h12" stroke="currentColor" strokeWidth="1.5" />
          </svg>
        </button>

        {/* 最大化/还原 */}
        <button
          onClick={handleMaximise}
          className="flex h-7 w-11 items-center justify-center transition-colors hover:bg-brand-200 active:scale-98 dark:hover:bg-brand-700"
          title={isMaximised ? "还原" : "最大化"}
        >
          {isMaximised ? (
            <svg className="h-2.5 w-2.5 text-brand-600 dark:text-brand-400" viewBox="0 0 12 12" fill="none">
              <path d="M3 3h6v6H3V3z" stroke="currentColor" strokeWidth="1.5" />
              <path d="M5 1h6v6" stroke="currentColor" strokeWidth="1.5" />
            </svg>
          ) : (
            <svg className="h-2.5 w-2.5 text-brand-600 dark:text-brand-400" viewBox="0 0 12 12" fill="none">
              <path d="M1 1h10v10H1V1z" stroke="currentColor" strokeWidth="1.5" />
            </svg>
          )}
        </button>

        {/* 关闭 */}
        <button
          onClick={handleClose}
          className="flex h-7 w-11 items-center justify-center transition-colors hover:bg-red-500 active:scale-98"
          title="关闭"
        >
          <svg className="h-2.5 w-2.5 text-brand-600 transition-colors group-hover:text-white dark:text-brand-400" viewBox="0 0 12 12" fill="none">
            <path d="M1 1l10 10M11 1L1 11" stroke="currentColor" strokeWidth="1.5" />
          </svg>
        </button>
      </div>
    </div>
  );
}
