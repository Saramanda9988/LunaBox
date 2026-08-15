import type { ReactNode } from "react";
import {
  Dialog,
  DialogBackdrop,
  DialogDescription,
  DialogPanel,
  DialogTitle,
} from "@headlessui/react";

export type BetterDrawerPlacement = "bottom" | "right";

interface BetterDrawerProps {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  title: ReactNode;
  children: ReactNode;
  placement?: BetterDrawerPlacement;
  description?: ReactNode;
  footer?: ReactNode;
  closeLabel?: string;
  className?: string;
  bodyClassName?: string;
  topOffset?: number | string;
}

const PLACEMENT_CLASSES: Record<BetterDrawerPlacement, string> = {
  bottom:
    "max-h-[85dvh] w-full rounded-t-2xl border-t data-closed:translate-y-full",
  right: "h-full w-[min(92vw,26rem)] border-l data-closed:translate-x-full",
};

const WRAPPER_CLASSES: Record<BetterDrawerPlacement, string> = {
  bottom: "items-end",
  right: "justify-end",
};

/**
 * 带焦点约束与退出动效的边缘抽屉。
 *
 * 抽屉打开后，焦点会限制在面板内部；关闭后，焦点会回到触发控件。
 */
export function BetterDrawer({
  isOpen,
  onOpenChange,
  title,
  children,
  placement = "right",
  description,
  footer,
  closeLabel = "Close",
  className = "",
  bodyClassName = "",
  topOffset = 0,
}: BetterDrawerProps) {
  return (
    <Dialog
      open={isOpen}
      onClose={onOpenChange}
      transition
      className="relative z-50"
    >
      <DialogBackdrop
        transition
        className="fixed inset-0 bg-black/35 backdrop-blur-[2px] transition-opacity duration-300 ease-out data-closed:opacity-0 data-leave:duration-200 data-leave:ease-in motion-reduce:duration-0"
        style={{ top: topOffset }}
      />

      <div
        className={`fixed inset-0 flex overflow-hidden pointer-events-none ${WRAPPER_CLASSES[placement]}`}
        style={{ top: topOffset }}
      >
        <DialogPanel
          transition
          className={`pointer-events-auto flex flex-col overflow-hidden border-brand-200 bg-white/96 shadow-2xl shadow-black/20 backdrop-blur-20 transition-[transform,opacity] duration-300 ease-[cubic-bezier(.22,1,.36,1)] data-closed:opacity-95 data-leave:duration-200 data-leave:ease-in dark:border-brand-700 dark:bg-brand-800/96 motion-reduce:duration-0 ${PLACEMENT_CLASSES[placement]} ${className}`}
        >
          {placement === "bottom" && (
            <div
              className="flex h-5 shrink-0 items-center justify-center"
              aria-hidden="true"
            >
              <div className="h-1 w-10 rounded-full bg-brand-300 dark:bg-brand-600" />
            </div>
          )}

          <div className="flex shrink-0 items-start justify-between gap-4 border-b border-brand-200 px-5 py-4 dark:border-brand-700">
            <div className="min-w-0">
              <DialogTitle className="text-base font-semibold text-brand-900 dark:text-white">
                {title}
              </DialogTitle>
              {description && (
                <DialogDescription className="mt-1 text-xs leading-5 text-brand-500 dark:text-brand-400">
                  {description}
                </DialogDescription>
              )}
            </div>
            <button
              type="button"
              onClick={() => onOpenChange(false)}
              aria-label={closeLabel}
              className="-mr-1 inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-500 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white"
            >
              <div className="i-mdi-close text-xl" aria-hidden="true" />
            </button>
          </div>

          <div
            className={`min-h-0 flex-1 overflow-y-auto overscroll-contain p-4 ${bodyClassName}`}
          >
            {children}
          </div>

          {footer && (
            <div className="shrink-0 border-t border-brand-200 p-4 dark:border-brand-700">
              {footer}
            </div>
          )}
        </DialogPanel>
      </div>
    </Dialog>
  );
}
