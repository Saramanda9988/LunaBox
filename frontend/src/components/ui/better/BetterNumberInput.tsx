import type {
  ChangeEvent,
  InputHTMLAttributes,
  KeyboardEvent,
  WheelEvent,
} from "react";
import { useCallback, useEffect, useState } from "react";

interface BetterNumberInputProps extends Omit<
  InputHTMLAttributes<HTMLInputElement>,
  "max" | "min" | "onChange" | "size" | "step" | "type" | "value"
> {
  value: number;
  onValueChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  unit?: string;
  size?: "sm" | "md";
}

function clampValue(value: number, min?: number, max?: number) {
  if (typeof min === "number" && value < min) {
    return min;
  }
  if (typeof max === "number" && value > max) {
    return max;
  }
  return value;
}

function formatValue(value: number) {
  return Number.isFinite(value) ? String(value) : "";
}

export function BetterNumberInput({
  value,
  onValueChange,
  min,
  max,
  step = 1,
  unit,
  size = "md",
  disabled,
  className = "",
  onBlur,
  onFocus,
  onKeyDown,
  onWheel,
  ...rest
}: BetterNumberInputProps) {
  const [draftValue, setDraftValue] = useState(formatValue(value));
  const [isFocused, setIsFocused] = useState(false);

  useEffect(() => {
    if (!isFocused) {
      setDraftValue(formatValue(value));
    }
  }, [isFocused, value]);

  const commitValue = useCallback(
    (nextDraft: string) => {
      const parsed = Number.parseInt(nextDraft, 10);
      const nextValue = Number.isNaN(parsed)
        ? clampValue(value, min, max)
        : clampValue(parsed, min, max);
      setDraftValue(formatValue(nextValue));
      if (nextValue !== value) {
        onValueChange(nextValue);
      }
    },
    [max, min, onValueChange, value],
  );

  const nudgeValue = useCallback(
    (direction: 1 | -1) => {
      if (disabled) {
        return;
      }

      const parsed = Number.parseInt(draftValue, 10);
      const baseValue = Number.isNaN(parsed) ? value : parsed;
      const nextValue = clampValue(baseValue + step * direction, min, max);
      setDraftValue(formatValue(nextValue));
      if (nextValue !== value) {
        onValueChange(nextValue);
      }
    },
    [disabled, draftValue, max, min, onValueChange, step, value],
  );

  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    const nextDraft = event.target.value.replace(/[^\d-]/g, "");
    setDraftValue(nextDraft);

    const parsed = Number.parseInt(nextDraft, 10);
    if (
      Number.isNaN(parsed)
      || (typeof min === "number" && parsed < min)
      || (typeof max === "number" && parsed > max)
    ) {
      return;
    }

    if (parsed !== value) {
      onValueChange(parsed);
    }
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "ArrowUp") {
      event.preventDefault();
      nudgeValue(1);
    }
    else if (event.key === "ArrowDown") {
      event.preventDefault();
      nudgeValue(-1);
    }
    else if (event.key === "Enter") {
      event.currentTarget.blur();
    }

    onKeyDown?.(event);
  };

  const handleWheel = (event: WheelEvent<HTMLInputElement>) => {
    event.currentTarget.blur();
    onWheel?.(event);
  };

  const controlSizeClass
    = size === "sm" ? "h-9 w-25 rounded-lg" : "h-11 w-36 rounded-xl";
  const inputSizeClass = size === "sm" ? "px-3 text-sm" : "px-4 text-sm";
  const stepperSizeClass = size === "sm" ? "w-7" : "w-9";
  const iconSizeClass = size === "sm" ? "text-sm" : "text-base";

  return (
    <div className={`inline-flex items-center gap-2 ${className}`}>
      <div
        className={[
          "glass-input group inline-flex items-stretch overflow-hidden",
          controlSizeClass,
          "border border-brand-300 bg-white shadow-sm transition-all duration-200",
          "focus-within:border-neutral-500 focus-within:ring-2 focus-within:ring-neutral-500/30",
          "dark:border-brand-600 dark:bg-brand-700",
          disabled ? "cursor-not-allowed opacity-60" : "",
        ].join(" ")}
      >
        <input
          {...rest}
          type="text"
          inputMode="numeric"
          value={draftValue}
          disabled={disabled}
          onChange={handleChange}
          onFocus={(event) => {
            setIsFocused(true);
            onFocus?.(event);
          }}
          onBlur={(event) => {
            setIsFocused(false);
            commitValue(event.target.value);
            onBlur?.(event);
          }}
          onKeyDown={handleKeyDown}
          onWheel={handleWheel}
          className={`min-w-0 flex-1 bg-transparent font-medium tabular-nums text-brand-900 outline-none placeholder:text-brand-400 disabled:cursor-not-allowed dark:text-white dark:placeholder:text-brand-500 ${inputSizeClass}`}
        />
        <div
          className={`flex shrink-0 flex-col border-l border-brand-200 bg-brand-50/80 dark:border-brand-600 dark:bg-brand-700 ${stepperSizeClass}`}
        >
          <button
            type="button"
            tabIndex={-1}
            disabled={disabled}
            onClick={() => nudgeValue(1)}
            className="flex flex-1 items-center justify-center text-brand-500 transition-colors hover:bg-brand-200 hover:text-brand-800 disabled:cursor-not-allowed dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white"
            aria-label="Increase value"
          >
            <span className={`i-mdi-chevron-up ${iconSizeClass}`} />
          </button>
          <div className="h-px bg-brand-200 dark:bg-brand-700" />
          <button
            type="button"
            tabIndex={-1}
            disabled={disabled}
            onClick={() => nudgeValue(-1)}
            className="flex flex-1 items-center justify-center text-brand-500 transition-colors hover:bg-brand-200 hover:text-brand-800 disabled:cursor-not-allowed dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white"
            aria-label="Decrease value"
          >
            <span className={`i-mdi-chevron-down ${iconSizeClass}`} />
          </button>
        </div>
      </div>
      {unit && (
        <span className="text-sm text-brand-500 dark:text-brand-400">
          {unit}
        </span>
      )}
    </div>
  );
}
