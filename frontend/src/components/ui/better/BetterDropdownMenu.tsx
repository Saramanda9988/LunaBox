import type { MouseEvent, PointerEvent, ReactNode } from "react";
import { Menu, MenuButton, MenuItem, MenuItems } from "@headlessui/react";
import { useEffect, useRef } from "react";

export interface DropdownMenuItem {
  /** 唯一 key */
  key: string;
  /** 菜单项标签 */
  label: string;
  /** 可选副标题描述 */
  description?: string;
  /** UnoCSS / MDI 图标类名，例如 "i-mdi-gamepad-variant" */
  icon?: string;
  /** 图片资源 URL，例如通过静态 import 获取的图片地址 */
  iconSrc?: string;
  /** 图标颜色类名，例如 "text-success-500" */
  iconColor?: string;
  /** 点击回调 */
  onClick: () => void;
  /** 使用彩色胶囊样式（适合状态标签） */
  pill?: boolean;
  /** 胶囊的颜色类名，例如 "bg-yellow-100 text-yellow-700 ..." */
  pillColor?: string;
  /** 是否在此项前插入分隔线 */
  dividerBefore?: boolean;
  /** 是否禁用 */
  disabled?: boolean;
}

interface BetterDropdownMenuProps {
  trigger: ReactNode;
  items: DropdownMenuItem[];
  ariaLabel?: string;
  align?: "start" | "end";
  menuWidth?: string;
  title?: string;
  disabled?: boolean;
  footer?: ReactNode;
  openOnHover?: boolean;
  selectedKey?: string;
}

export function BetterDropdownMenu({
  trigger,
  items,
  ariaLabel,
  align = "end",
  menuWidth = "min-w-[180px]",
  title,
  disabled = false,
  footer,
  openOnHover = false,
  selectedKey,
}: BetterDropdownMenuProps) {
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const hoverTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const openedByHover = useRef(false);

  const clearHoverTimer = () => {
    if (hoverTimer.current !== null) {
      clearTimeout(hoverTimer.current);
      hoverTimer.current = null;
    }
  };

  useEffect(() => clearHoverTimer, []);

  return (
    <Menu as="div" className="relative inline-block">
      {({ open, close }) => {
        const handlePointerLeave = () => {
          clearHoverTimer();
          if (openedByHover.current) {
            hoverTimer.current = setTimeout(() => {
              openedByHover.current = false;
              close();
            }, 200);
          }
        };

        return (
          <>
            <MenuButton
              ref={openOnHover ? triggerRef : undefined}
              disabled={disabled}
              as={openOnHover ? "button" : "div"}
              className={
                openOnHover
                  ? "cursor-pointer rounded-full focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400/70"
                  : "cursor-pointer"
              }
              aria-label={ariaLabel}
              onPointerEnter={(event: PointerEvent<HTMLElement>) => {
                clearHoverTimer();
                if (
                  !openOnHover
                  || disabled
                  || event.pointerType !== "mouse"
                  || open
                ) {
                  return;
                }
                hoverTimer.current = setTimeout(() => {
                  triggerRef.current?.click();
                  openedByHover.current = true;
                }, 150);
              }}
              onPointerLeave={handlePointerLeave}
              onPointerDown={() => {
                clearHoverTimer();
              }}
              onClick={(event: MouseEvent<HTMLElement>) => {
                // Clicking a hover-opened menu pins it for deliberate selection.
                if (open && openedByHover.current)
                  event.preventDefault();
                openedByHover.current = false;
              }}
              onKeyDown={() => {
                clearHoverTimer();
                openedByHover.current = false;
              }}
            >
              {trigger}
            </MenuButton>

            <MenuItems
              onPointerEnter={clearHoverTimer}
              onPointerLeave={handlePointerLeave}
              onKeyDown={() => {
                clearHoverTimer();
                openedByHover.current = false;
              }}
              modal={false}
              portal
              anchor={align === "end" ? "bottom end" : "bottom start"}
              className={`z-[9000] mt-1.5 ${menuWidth} origin-top-right rounded-xl bg-white dark:bg-brand-800 border border-brand-200 dark:border-brand-700 focus:outline-none p-1.5 [--anchor-gap:6px]`}
            >
              {title && (
                <div className="px-2 pb-1 pt-0.5 text-xs font-medium text-brand-400 dark:text-brand-500">
                  {title}
                </div>
              )}

              {items.map(item => (
                <div key={item.key}>
                  {item.dividerBefore && (
                    <div className="my-1 border-t border-brand-200 dark:border-brand-700" />
                  )}
                  <MenuItem disabled={item.disabled}>
                    {({ focus }: { focus: boolean }) =>
                      item.pill ? (
                        <button
                          type="button"
                          onClick={item.onClick}
                          disabled={item.disabled}
                          aria-current={
                            item.key === selectedKey ? "true" : undefined
                          }
                          className={`flex w-full items-center gap-1.5 rounded-full px-2.5 py-1.5 text-xs font-medium transition-all
                          ${item.pillColor ?? "bg-brand-100 text-brand-700 dark:bg-brand-700 dark:text-brand-300"}
                          ${focus ? "ring-2 ring-brand-400 ring-offset-1 dark:ring-offset-brand-900" : ""}
                          ${item.disabled ? "cursor-not-allowed opacity-50" : ""}`}
                        >
                          {item.icon && (
                            <div className={`${item.icon} text-sm shrink-0`} />
                          )}
                          {item.label}
                          {item.key === selectedKey && (
                            <span
                              className="i-mdi-check ml-auto text-base shrink-0"
                              aria-hidden="true"
                            />
                          )}
                        </button>
                      ) : (
                        <button
                          type="button"
                          onClick={item.onClick}
                          disabled={item.disabled}
                          className={`flex w-full items-center rounded-lg px-3 py-2.5 text-sm text-brand-700 dark:text-brand-200 transition-colors
                          ${focus ? "bg-brand-100 dark:bg-brand-700" : ""}
                          ${item.disabled ? "cursor-not-allowed opacity-50" : ""}`}
                        >
                          {item.iconSrc ? (
                            <img
                              src={item.iconSrc}
                              alt=""
                              className="mr-3 h-6 w-6 shrink-0 object-contain"
                            />
                          ) : item.icon ? (
                            <div
                              className={`mr-3 text-xl shrink-0 ${item.iconColor ?? "text-brand-400 dark:text-brand-500"}`}
                            >
                              <div className={item.icon} />
                            </div>
                          ) : null}
                          <div className="text-left">
                            <div className="font-medium leading-tight">
                              {item.label}
                            </div>
                            {item.description && (
                              <div className="mt-0.5 text-xs leading-tight text-brand-400 dark:text-brand-500">
                                {item.description}
                              </div>
                            )}
                          </div>
                        </button>
                      )}
                  </MenuItem>
                </div>
              ))}

              {footer}
            </MenuItems>
          </>
        );
      }}
    </Menu>
  );
}
