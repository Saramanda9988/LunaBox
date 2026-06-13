import type { ButtonHTMLAttributes, ReactNode } from "react";
import { forwardRef } from "react";

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

const variantClasses: Record<ButtonVariant, string> = {
  secondary:
    "border border-brand-200 bg-brand-100 text-brand-700 hover:bg-brand-200 "
    + "dark:border-brand-700 dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600",
  primary:
    "border border-neutral-600 bg-neutral-600 text-white shadow-sm hover:bg-neutral-700 "
    + "dark:border-neutral-600 dark:bg-neutral-600 dark:hover:bg-neutral-700",
  danger:
    "border border-error-500 bg-error-500 text-white hover:bg-error-600 "
    + "dark:border-error-600 dark:bg-error-600 dark:hover:bg-error-700",
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

const sizeClasses: Record<ButtonSize, string> = {
  sm: "h-8 px-3 text-xs gap-1",
  md: "h-10 px-4 text-sm gap-1.5",
  lg: "h-11 px-5 text-sm gap-2",
};

const iconSizeClasses: Record<ButtonSize, string> = {
  sm: "text-base",
  md: "text-lg",
  lg: "text-xl",
};

export const BetterButton = forwardRef<HTMLButtonElement, BetterButtonProps>(
  (
    {
      variant = "secondary",
      size = "md",
      icon,
      isLoading = false,
      disabled,
      className = "",
      children,
      ...rest
    },
    ref,
  ) => {
    const isDisabled = disabled || isLoading;

    return (
      <button
        ref={ref}
        type="button"
        disabled={isDisabled}
        className={[
          "inline-flex shrink-0 items-center justify-center rounded-lg font-medium leading-none",
          "transition-all duration-200 active:scale-95",
          "disabled:opacity-50 disabled:cursor-not-allowed disabled:active:scale-100",
          variantClasses[variant],
          glassClasses[variant],
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
  },
);
