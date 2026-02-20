import { Menu, MenuButton, MenuItem, MenuItems } from "@headlessui/react";

// ──────────────────────────────────────────────────────────────────────────────
// Types
// ──────────────────────────────────────────────────────────────────────────────

export interface DropdownMenuItem {
  /** 唯一 key */
  key: string;
  /** 菜单项标签 */
  label: string;
  /** 可选副标题描述 */
  description?: string;
  /** UnoCSS / MDI 图标类名，例如 "i-mdi-gamepad-variant" */
  icon?: string;
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
  trigger: React.ReactNode;
  items: DropdownMenuItem[];
  align?: "start" | "end";
  menuWidth?: string;
  title?: string;
  disabled?: boolean;
}

export function BetterDropdownMenu({
  trigger,
  items,
  align = "end",
  menuWidth = "min-w-[180px]",
  title,
  disabled = false,
}: BetterDropdownMenuProps) {
  return (
    <Menu as="div" className="relative inline-block">
      <MenuButton disabled={disabled} as="div" className="cursor-pointer">
        {trigger}
      </MenuButton>

      <MenuItems
        anchor={align === "end" ? "bottom end" : "bottom start"}
        className={`z-50 mt-1.5 ${menuWidth} origin-top-right rounded-xl bg-white dark:bg-brand-800 border border-brand-200 dark:border-brand-700 shadow-xl focus:outline-none p-1.5 [--anchor-gap:6px]`}
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
                item.pill
                  ? (
                      <button
                        type="button"
                        onClick={item.onClick}
                        disabled={item.disabled}
                        className={`flex w-full items-center gap-1.5 rounded-full px-2.5 py-1.5 text-xs font-medium transition-all
                          ${item.pillColor ?? "bg-brand-100 text-brand-700 dark:bg-brand-700 dark:text-brand-300"}
                          ${focus ? "ring-2 ring-brand-400 ring-offset-1 dark:ring-offset-brand-900" : ""}
                          ${item.disabled ? "cursor-not-allowed opacity-50" : ""}`}
                      >
                        {item.icon && <div className={`${item.icon} text-sm shrink-0`} />}
                        {item.label}
                      </button>
                    )
                  : (
                      <button
                        type="button"
                        onClick={item.onClick}
                        disabled={item.disabled}
                        className={`flex w-full items-center rounded-lg px-3 py-2.5 text-sm text-brand-700 dark:text-brand-200 transition-colors
                          ${focus ? "bg-brand-100 dark:bg-brand-700" : ""}
                          ${item.disabled ? "cursor-not-allowed opacity-50" : ""}`}
                      >
                        {item.icon && (
                          <div className={`mr-3 text-xl shrink-0 ${item.iconColor ?? "text-brand-400 dark:text-brand-500"}`}>
                            <div className={item.icon} />
                          </div>
                        )}
                        <div className="text-left">
                          <div className="font-medium leading-tight">{item.label}</div>
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
      </MenuItems>
    </Menu>
  );
}
