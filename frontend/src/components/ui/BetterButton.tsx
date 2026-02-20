import type { ButtonHTMLAttributes, ReactNode } from "react";

// ──────────────────────────────────────────────────────────────────────────────
// Types

export type ButtonVariant = "secondary" | "primary" | "danger" | "ghost";
export type ButtonSize = "sm" | "md" | "lg";

interface BetterButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** 按钮风格：secondary（默认中性）、primary（深色强调）、danger（危险红色）、ghost（透明/文字） */
  variant?: ButtonVariant;
  /** 按钮大小：sm（紧凑）、md（标准，默认）、lg（宽松） */
  size?: ButtonSize;
  /** 在文字左侧渲染的 UnoCSS 图标类名，例如 "i-mdi-play" */
  icon?: string;
  /** 加载中：禁用按钮并展示旋转图标 */
  isLoading?: boolean;
  children?: ReactNode;
}

// ──────────────────────────────────────────────────────────────────────────────
// Style maps

const variantClasses: Record<ButtonVariant, string> = {
  secondary:
    "bg-brand-100 text-brand-700 hover:bg-brand-200 "
    + "dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600 "
    + "border border-transparent",
  primary:
    "bg-neutral-700 text-white hover:bg-neutral-800 "
    + "dark:bg-white dark:text-neutral-900 dark:hover:bg-neutral-100 "
    + "border border-transparent shadow-sm",
  danger:
    "bg-error-500 text-white hover:bg-error-600 "
    + "dark:bg-error-600 dark:hover:bg-error-700 "
    + "border border-transparent",
  ghost:
    "bg-transparent text-brand-600 hover:text-brand-900 hover:bg-brand-100 "
    + "dark:text-brand-400 dark:hover:text-brand-200 dark:hover:bg-brand-800 "
    + "border border-transparent",
};

const sizeClasses: Record<ButtonSize, string> = {
  sm: "px-2.5 py-1 text-xs gap-1",
  md: "px-4 py-2 text-sm gap-1.5",
  lg: "px-6 py-2.5 text-sm gap-2",
};

const iconSizeClasses: Record<ButtonSize, string> = {
  sm: "text-base",
  md: "text-lg",
  lg: "text-xl",
};

// ──────────────────────────────────────────────────────────────────────────────
// Component

export function BetterButton({
  variant = "secondary",
  size = "md",
  icon,
  isLoading = false,
  disabled,
  className = "",
  children,
  ...rest
}: BetterButtonProps) {
  const isDisabled = disabled || isLoading;

  return (
    <button
      type="button"
      disabled={isDisabled}
      className={[
        "inline-flex items-center justify-center rounded-lg font-medium",
        "transition-all duration-200 active:scale-95",
        "disabled:opacity-50 disabled:cursor-not-allowed disabled:active:scale-100",
        variantClasses[variant],
        sizeClasses[size],
        className,
      ].join(" ")}
      {...rest}
    >
      {isLoading ? (
        <span
          className={`i-mdi-loading animate-spin ${iconSizeClasses[size]}`}
        />
      ) : (
        icon && <span className={`${icon} ${iconSizeClasses[size]}`} />
      )}
      {children}
    </button>
  );
}
