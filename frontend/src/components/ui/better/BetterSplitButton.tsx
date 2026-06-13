import type { ReactNode } from "react";
import type { ButtonSize, ButtonVariant } from "./BetterButton";
import { Menu, MenuButton, MenuItem, MenuItems } from "@headlessui/react";

interface SplitButtonOption<Key extends string = string> {
  key: Key;
  label: string;
  description?: string;
  icon?: string;
}

interface BetterSplitButtonProps<Key extends string = string> {
  label: ReactNode;
  icon?: string;
  options: Array<SplitButtonOption<Key>>;
  selectedKey: Key;
  onClick: () => void;
  onSelect: (key: Key) => void;
  variant?: ButtonVariant;
  size?: ButtonSize;
  disabled?: boolean;
  isLoading?: boolean;
  className?: string;
  menuTitle?: string;
}

const variantClasses: Record<ButtonVariant, string> = {
  secondary:
    "bg-brand-100 text-brand-700 hover:bg-brand-200 "
    + "dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600 "
    + "border border-brand-200 dark:border-brand-700",
  primary:
    "bg-neutral-600 text-white hover:bg-neutral-700 "
    + "dark:bg-neutral-600 dark:hover:bg-neutral-700 "
    + "border border-neutral-600 shadow-sm",
  danger:
    "bg-error-500 text-white hover:bg-error-600 "
    + "dark:bg-error-600 dark:hover:bg-error-700 "
    + "border border-transparent",
  ghost:
    "bg-transparent text-brand-600 hover:text-brand-900 hover:bg-brand-100 "
    + "dark:text-brand-400 dark:hover:text-brand-200 dark:hover:bg-brand-800 "
    + "border border-transparent",
};

const glassClasses: Record<ButtonVariant, string> = {
  secondary: "glass-btn-neutral",
  primary: "glass-btn-neutral",
  danger: "glass-btn-error",
  ghost: "glass-btn-none",
};

const sizeClasses: Record<
  ButtonSize,
  {
    icon: string;
    main: string;
    menu: string;
    height: string;
  }
> = {
  sm: {
    icon: "text-base",
    main: "px-2.5 text-xs gap-1",
    menu: "px-1.5",
    height: "h-8",
  },
  md: {
    icon: "text-lg",
    main: "px-4 text-sm gap-1.5",
    menu: "px-2.5",
    height: "h-10",
  },
  lg: {
    icon: "text-xl",
    main: "px-6 text-sm gap-2",
    menu: "px-3",
    height: "h-11",
  },
};

export function BetterSplitButton<Key extends string = string>({
  label,
  icon,
  options,
  selectedKey,
  onClick,
  onSelect,
  variant = "primary",
  size = "md",
  disabled = false,
  isLoading = false,
  className = "",
  menuTitle,
}: BetterSplitButtonProps<Key>) {
  const isDisabled = disabled || isLoading;

  return (
    <Menu
      as="div"
      className={`relative inline-flex items-stretch ${className}`}
    >
      <button
        type="button"
        disabled={isDisabled}
        onClick={onClick}
        className={[
          "inline-flex items-center justify-center rounded-l-lg font-medium",
          "transition-all duration-200 active:scale-95",
          "disabled:opacity-50 disabled:cursor-not-allowed disabled:active:scale-100",
          variantClasses[variant],
          glassClasses[variant],
          sizeClasses[size].main,
          sizeClasses[size].height,
        ].join(" ")}
      >
        {isLoading ? (
          <span
            className={`i-mdi-loading animate-spin ${sizeClasses[size].icon}`}
          />
        ) : (
          icon && <span className={`${icon} ${sizeClasses[size].icon}`} />
        )}
        <span>{label}</span>
      </button>

      <MenuButton
        type="button"
        disabled={isDisabled}
        className={[
          "inline-flex items-center justify-center rounded-r-lg font-medium",
          "transition-all duration-200 active:scale-95",
          "disabled:opacity-50 disabled:cursor-not-allowed disabled:active:scale-100",
          variantClasses[variant],
          glassClasses[variant],
          "border-l border-white/20 dark:border-neutral-900/15",
          sizeClasses[size].menu,
          sizeClasses[size].height,
        ].join(" ")}
        aria-label={menuTitle}
      >
        <span className={`i-mdi-chevron-down ${sizeClasses[size].icon}`} />
      </MenuButton>

      <MenuItems className="absolute left-0 top-full z-[9999] mt-1.5 min-w-full w-max max-w-[min(22rem,calc(100vw-2rem))] origin-top-left rounded-xl border border-brand-200 bg-white p-1.5 shadow-xl focus:outline-none dark:border-brand-700 dark:bg-brand-800">
        {menuTitle && (
          <div className="px-2 pb-1 pt-0.5 text-xs font-medium text-brand-400 dark:text-brand-500">
            {menuTitle}
          </div>
        )}

        {options.map((option) => {
          const selected = option.key === selectedKey;
          return (
            <MenuItem key={option.key}>
              {({ focus }: { focus: boolean }) => (
                <button
                  type="button"
                  onClick={() => onSelect(option.key)}
                  className={[
                    "flex w-full items-center rounded-lg px-3 py-2.5 text-sm transition-colors",
                    "text-brand-700 dark:text-brand-200",
                    focus || selected ? "bg-brand-100 dark:bg-brand-700" : "",
                  ].join(" ")}
                >
                  <div className="mr-3 text-xl text-brand-400 dark:text-brand-500">
                    <div className={option.icon ?? "i-mdi-play"} />
                  </div>
                  <div className="min-w-0 flex-1 text-left">
                    <div className="font-medium leading-tight">
                      {option.label}
                    </div>
                    {option.description && (
                      <div className="mt-0.5 text-xs leading-tight text-brand-400 dark:text-brand-500">
                        {option.description}
                      </div>
                    )}
                  </div>
                  {selected && (
                    <div className="i-mdi-check ml-3 text-lg text-neutral-600 dark:text-neutral-300" />
                  )}
                </button>
              )}
            </MenuItem>
          );
        })}
      </MenuItems>
    </Menu>
  );
}
