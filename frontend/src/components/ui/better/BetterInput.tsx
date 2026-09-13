import type { InputHTMLAttributes } from "react";
import { forwardRef } from "react";

interface BetterInputProps extends InputHTMLAttributes<HTMLInputElement> {
  fullWidth?: boolean;
  variant?: "default" | "unstyled";
}

export const BetterInput = forwardRef<HTMLInputElement, BetterInputProps>(
  (
    { className = "", fullWidth = true, variant = "default", ...props },
    ref,
  ) => {
    const defaultClasses = [
      "glass-input rounded-md border border-brand-300 bg-white px-3 py-2 text-base text-brand-900",
      "outline-none transition-colors placeholder:text-brand-400",
      "focus:border-neutral-500 focus:ring-2 focus:ring-neutral-500/30",
      "disabled:cursor-not-allowed disabled:opacity-60",
      "dark:border-brand-600 dark:bg-brand-700 dark:text-white dark:placeholder:text-brand-500",
      fullWidth ? "w-full" : "",
    ].join(" ");

    return (
      <input
        ref={ref}
        className={`${variant === "default" ? defaultClasses : ""} ${className}`}
        {...props}
      />
    );
  },
);

BetterInput.displayName = "BetterInput";
