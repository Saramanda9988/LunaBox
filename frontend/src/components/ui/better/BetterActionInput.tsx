import type { ButtonHTMLAttributes, InputHTMLAttributes } from "react";
import { forwardRef } from "react";
import { BetterTooltip } from "./BetterTooltip";

export interface BetterActionInputAction extends Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "aria-label" | "children" | "className" | "title" | "type"
> {
  ariaLabel: string;
  icon: string;
}

interface BetterActionInputProps extends InputHTMLAttributes<HTMLInputElement> {
  actions?: readonly BetterActionInputAction[];
  containerClassName?: string;
}

const emptyActions: readonly BetterActionInputAction[] = [];

export const BetterActionInput = forwardRef<
  HTMLInputElement,
  BetterActionInputProps
>(
  (
    {
      actions = emptyActions,
      className = "",
      containerClassName = "",
      disabled,
      type = "text",
      ...rest
    },
    ref,
  ) => (
    <div
      className={[
        "glass-input flex w-full min-w-0 items-center gap-1 rounded-md",
        "border border-brand-300 bg-white pr-1",
        "focus-within:ring-2 focus-within:ring-neutral-500",
        "dark:border-brand-600 dark:bg-brand-700",
        disabled ? "cursor-not-allowed opacity-60" : "",
        containerClassName,
      ].join(" ")}
    >
      <input
        ref={ref}
        type={type}
        disabled={disabled}
        className={[
          "min-w-0 flex-1 bg-transparent px-3 py-2 text-brand-900 outline-none",
          "placeholder:text-brand-400 disabled:cursor-not-allowed",
          "dark:text-white dark:placeholder:text-brand-500",
          className,
        ].join(" ")}
        {...rest}
      />
      {actions.map((action) => {
        const {
          ariaLabel,
          disabled: actionDisabled,
          icon,
          ...buttonProps
        } = action;

        return (
          <BetterTooltip key={ariaLabel} content={ariaLabel}>
            <button
              type="button"
              disabled={disabled || actionDisabled}
              aria-label={ariaLabel}
              className={[
                "inline-flex h-10 w-10 shrink-0 items-center justify-center",
                "bg-transparent text-brand-500 transition-colors duration-200",
                "hover:text-brand-900 focus-visible:text-brand-900 focus-visible:outline-none",
                "disabled:cursor-not-allowed disabled:opacity-50",
                "dark:text-brand-400 dark:hover:text-brand-100 dark:focus-visible:text-brand-100",
              ].join(" ")}
              {...buttonProps}
            >
              <span className={`${icon} text-xl`} aria-hidden="true" />
            </button>
          </BetterTooltip>
        );
      })}
    </div>
  ),
);

BetterActionInput.displayName = "BetterActionInput";
