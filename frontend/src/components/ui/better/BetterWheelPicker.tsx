import type { WheelPickerOption } from "@ncdai/react-wheel-picker";
import { WheelPicker, WheelPickerWrapper } from "@ncdai/react-wheel-picker";
import { useMemo } from "react";
import "@ncdai/react-wheel-picker/style.css";

interface BetterNumberWheelPickerProps {
  firstValue: number;
  secondValue: number;
  firstOptions: WheelNumberOption[];
  secondOptions: WheelNumberOption[];
  onFirstChange: (value: number) => void;
  onSecondChange: (value: number) => void;
  firstLabel?: string;
  secondLabel?: string;
  separator?: string;
  disabled?: boolean;
  className?: string;
}

interface BetterTimeWheelPickerProps {
  valueMinutes: number;
  onChange: (value: number) => void;
  disabled?: boolean;
  className?: string;
}

interface BetterDurationWheelPickerProps {
  valueMinutes: number;
  onChange: (value: number) => void;
  minMinutes?: number;
  maxMinutes?: number;
  hourLabel?: string;
  minuteLabel?: string;
  disabled?: boolean;
  className?: string;
}

type WheelNumberOption = WheelPickerOption<number>;

const optionItemHeight = 44;
const visibleCount = 9;
const defaultMaxMinutes = 99 * 60 + 59;

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

function createRange(start: number, end: number) {
  if (end < start) {
    return [];
  }

  return Array.from({ length: end - start + 1 }, (_, index) => start + index);
}

function formatWheelValue(value: number) {
  return String(value).padStart(2, "0");
}

function createWheelOptions(values: number[]): WheelNumberOption[] {
  return values.map(value => ({
    label: formatWheelValue(value),
    textValue: formatWheelValue(value),
    value,
  }));
}

function getAllowedHours(minMinutes: number, maxMinutes: number) {
  const minHour = Math.floor(minMinutes / 60);
  const maxHour = Math.floor(maxMinutes / 60);
  return createRange(minHour, maxHour);
}

function getAllowedMinutesForHour(
  hour: number,
  minMinutes: number,
  maxMinutes: number,
) {
  const minHour = Math.floor(minMinutes / 60);
  const minMinute = minMinutes % 60;
  const maxHour = Math.floor(maxMinutes / 60);
  const maxMinute = maxMinutes % 60;

  const lowerBound = hour === minHour ? minMinute : 0;
  const upperBound = hour === maxHour ? maxMinute : 59;

  return createRange(lowerBound, upperBound);
}

function coerceMinuteForHour(
  minute: number,
  hour: number,
  minMinutes: number,
  maxMinutes: number,
) {
  const allowedMinutes = getAllowedMinutesForHour(hour, minMinutes, maxMinutes);

  if (allowedMinutes.includes(minute)) {
    return minute;
  }

  const firstMinute = allowedMinutes[0] ?? 0;
  const lastMinute = allowedMinutes.at(-1) ?? firstMinute;
  return clamp(minute, firstMinute, lastMinute);
}

export function BetterNumberWheelPicker({
  firstValue,
  secondValue,
  firstOptions,
  secondOptions,
  onFirstChange,
  onSecondChange,
  firstLabel,
  secondLabel,
  separator = ":",
  disabled = false,
  className = "",
}: BetterNumberWheelPickerProps) {
  return (
    <div
      aria-disabled={disabled}
      className={[
        "glass-input overflow-hidden rounded-xl border border-brand-200 bg-brand-50/75 p-2 shadow-sm",
        "dark:border-brand-700 dark:bg-brand-900/35",
        "[&_li[data-rwp-option]]:text-2xl [&_li[data-rwp-option]]:font-semibold [&_li[data-rwp-option]]:tabular-nums [&_li[data-rwp-option]]:leading-[44px] [&_li[data-rwp-option]]:text-brand-500/45",
        "dark:[&_li[data-rwp-option]]:text-brand-400/35",
        "[&_li[data-rwp-highlight-item]]:text-2xl [&_li[data-rwp-highlight-item]]:font-semibold [&_li[data-rwp-highlight-item]]:tabular-nums [&_li[data-rwp-highlight-item]]:leading-[44px] [&_li[data-rwp-highlight-item]]:text-brand-900",
        "dark:[&_li[data-rwp-highlight-item]]:text-white",
        disabled ? "pointer-events-none opacity-60" : "",
        className,
      ].join(" ")}
    >
      <div className="relative overflow-hidden">
        <div className="pointer-events-none absolute inset-x-3 top-1/2 h-11 -translate-y-1/2 rounded-md bg-brand-200/70 dark:bg-white/10" />
        <WheelPickerWrapper className="relative z-10 items-center">
          <WheelPicker
            value={firstValue}
            options={firstOptions}
            infinite
            visibleCount={visibleCount}
            optionItemHeight={optionItemHeight}
            dragSensitivity={2.6}
            scrollSensitivity={5}
            onValueChange={onFirstChange}
            classNames={{
              highlightItem:
                "text-2xl font-semibold tabular-nums leading-[44px] text-brand-900 dark:text-white",
              highlightWrapper: "rounded-md",
              optionItem:
                "text-2xl font-semibold tabular-nums leading-[44px] text-brand-500/45 dark:text-brand-400/35",
            }}
          />
          <div className="flex h-9 w-9 shrink-0 items-center justify-center text-2xl font-semibold leading-none text-brand-800 dark:text-brand-100">
            {separator}
          </div>
          <WheelPicker
            value={secondValue}
            options={secondOptions}
            infinite
            visibleCount={visibleCount}
            optionItemHeight={optionItemHeight}
            dragSensitivity={2.6}
            scrollSensitivity={5}
            onValueChange={onSecondChange}
            classNames={{
              highlightItem:
                "text-2xl font-semibold tabular-nums leading-[44px] text-brand-900 dark:text-white",
              highlightWrapper: "rounded-md",
              optionItem:
                "text-2xl font-semibold tabular-nums leading-[44px] text-brand-500/45 dark:text-brand-400/35",
            }}
          />
        </WheelPickerWrapper>
      </div>
      {(firstLabel || secondLabel) && (
        <div className="grid grid-cols-[minmax(0,1fr)_2.25rem_minmax(0,1fr)] pt-1 text-center text-xs font-medium leading-4 text-brand-500 dark:text-brand-400">
          <span>{firstLabel}</span>
          <span />
          <span>{secondLabel}</span>
        </div>
      )}
    </div>
  );
}

export function BetterTimeWheelPicker({
  valueMinutes,
  onChange,
  disabled = false,
  className = "",
}: BetterTimeWheelPickerProps) {
  const hourOptions = useMemo(() => createWheelOptions(createRange(0, 23)), []);
  const minuteOptions = useMemo(
    () => createWheelOptions(createRange(0, 59)),
    [],
  );
  const normalizedValue = clamp(Math.floor(valueMinutes), 0, 23 * 60 + 59);
  const selectedHour = Math.floor(normalizedValue / 60);
  const selectedMinute = normalizedValue % 60;

  return (
    <BetterNumberWheelPicker
      firstValue={selectedHour}
      secondValue={selectedMinute}
      firstOptions={hourOptions}
      secondOptions={minuteOptions}
      disabled={disabled}
      className={className}
      onFirstChange={hour => onChange(hour * 60 + selectedMinute)}
      onSecondChange={minute => onChange(selectedHour * 60 + minute)}
    />
  );
}

export function BetterDurationWheelPicker({
  valueMinutes,
  onChange,
  minMinutes = 1,
  maxMinutes = defaultMaxMinutes,
  hourLabel = "h",
  minuteLabel = "m",
  disabled = false,
  className = "",
}: BetterDurationWheelPickerProps) {
  const effectiveMinMinutes = Math.max(0, Math.floor(minMinutes));
  const effectiveMaxMinutes = Math.max(
    effectiveMinMinutes,
    Math.floor(maxMinutes),
  );
  const normalizedValue = clamp(
    Math.floor(valueMinutes),
    effectiveMinMinutes,
    effectiveMaxMinutes,
  );
  const selectedHour = Math.floor(normalizedValue / 60);
  const selectedMinute = normalizedValue % 60;
  const hourValues = useMemo(
    () => getAllowedHours(effectiveMinMinutes, effectiveMaxMinutes),
    [effectiveMaxMinutes, effectiveMinMinutes],
  );
  const minuteValues = useMemo(
    () =>
      getAllowedMinutesForHour(
        selectedHour,
        effectiveMinMinutes,
        effectiveMaxMinutes,
      ),
    [effectiveMaxMinutes, effectiveMinMinutes, selectedHour],
  );
  const hourOptions = useMemo(
    () => createWheelOptions(hourValues),
    [hourValues],
  );
  const minuteOptions = useMemo(
    () => createWheelOptions(minuteValues),
    [minuteValues],
  );

  const updateDuration = (hour: number, minute: number) => {
    onChange(
      clamp(hour * 60 + minute, effectiveMinMinutes, effectiveMaxMinutes),
    );
  };

  const handleHourChange = (hour: number) => {
    updateDuration(
      hour,
      coerceMinuteForHour(
        selectedMinute,
        hour,
        effectiveMinMinutes,
        effectiveMaxMinutes,
      ),
    );
  };

  const handleMinuteChange = (minute: number) => {
    updateDuration(selectedHour, minute);
  };

  return (
    <BetterNumberWheelPicker
      firstValue={selectedHour}
      secondValue={selectedMinute}
      firstOptions={hourOptions}
      secondOptions={minuteOptions}
      firstLabel={hourLabel}
      secondLabel={minuteLabel}
      disabled={disabled}
      className={className}
      onFirstChange={handleHourChange}
      onSecondChange={handleMinuteChange}
    />
  );
}
