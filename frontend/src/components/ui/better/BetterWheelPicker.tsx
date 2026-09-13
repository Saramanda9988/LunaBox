import type { WheelPickerOption } from "@ncdai/react-wheel-picker";
import { WheelPicker, WheelPickerWrapper } from "@ncdai/react-wheel-picker";
import { useMemo } from "react";
import "@ncdai/react-wheel-picker/style.css";

interface BetterNumberWheelPickerProps {
  firstValue: number;
  secondValue: number;
  thirdValue?: number;
  firstOptions: WheelNumberOption[];
  secondOptions: WheelNumberOption[];
  thirdOptions?: WheelNumberOption[];
  onFirstChange: (value: number) => void;
  onSecondChange: (value: number) => void;
  onThirdChange?: (value: number) => void;
  firstLabel?: string;
  secondLabel?: string;
  thirdLabel?: string;
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
  valueSeconds: number;
  onChange: (value: number) => void;
  minSeconds?: number;
  maxSeconds?: number;
  hourLabel?: string;
  minuteLabel?: string;
  secondLabel?: string;
  disabled?: boolean;
  className?: string;
}

type WheelNumberOption = WheelPickerOption<number>;

const optionItemHeight = 32;
const visibleCount = 20;
const defaultMaxSeconds = 99 * 60 * 60 + 59 * 60 + 59;

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

function getAllowedHours(minSeconds: number, maxSeconds: number) {
  const minHour = Math.floor(minSeconds / 3600);
  const maxHour = Math.floor(maxSeconds / 3600);
  return createRange(minHour, maxHour);
}

function getAllowedMinutesForHour(
  hour: number,
  minSeconds: number,
  maxSeconds: number,
) {
  const minHour = Math.floor(minSeconds / 3600);
  const minMinute = Math.floor((minSeconds % 3600) / 60);
  const maxHour = Math.floor(maxSeconds / 3600);
  const maxMinute = Math.floor((maxSeconds % 3600) / 60);

  const lowerBound = hour === minHour ? minMinute : 0;
  const upperBound = hour === maxHour ? maxMinute : 59;

  return createRange(lowerBound, upperBound);
}

function getAllowedSecondsForHourAndMinute(
  hour: number,
  minute: number,
  minSeconds: number,
  maxSeconds: number,
) {
  const minHour = Math.floor(minSeconds / 3600);
  const minMinute = Math.floor((minSeconds % 3600) / 60);
  const maxHour = Math.floor(maxSeconds / 3600);
  const maxMinute = Math.floor((maxSeconds % 3600) / 60);

  const lowerBound
    = hour === minHour && minute === minMinute ? minSeconds % 60 : 0;
  const upperBound
    = hour === maxHour && minute === maxMinute ? maxSeconds % 60 : 59;

  return createRange(lowerBound, upperBound);
}

function coerceValue(value: number, allowedValues: number[]) {
  if (allowedValues.includes(value)) {
    return value;
  }

  const firstValue = allowedValues[0] ?? 0;
  const lastValue = allowedValues.at(-1) ?? firstValue;
  return clamp(value, firstValue, lastValue);
}

function coerceMinuteForHour(
  minute: number,
  hour: number,
  minSeconds: number,
  maxSeconds: number,
) {
  return coerceValue(
    minute,
    getAllowedMinutesForHour(hour, minSeconds, maxSeconds),
  );
}

export function BetterNumberWheelPicker({
  firstValue,
  secondValue,
  thirdValue,
  firstOptions,
  secondOptions,
  thirdOptions,
  onFirstChange,
  onSecondChange,
  onThirdChange,
  firstLabel,
  secondLabel,
  thirdLabel,
  separator = ":",
  disabled = false,
  className = "",
}: BetterNumberWheelPickerProps) {
  const hasThirdColumn = Boolean(
    thirdOptions && onThirdChange && thirdValue !== undefined,
  );

  return (
    <div
      aria-disabled={disabled}
      className={[
        "glass-input overflow-hidden rounded-xl border border-brand-200 bg-brand-50/75 p-3",
        "dark:border-brand-700 dark:bg-brand-900/35",
        "[&_li[data-rwp-option]]:text-xl [&_li[data-rwp-option]]:font-semibold [&_li[data-rwp-option]]:tabular-nums [&_li[data-rwp-option]]:text-brand-500/45",
        "dark:[&_li[data-rwp-option]]:text-brand-400/35",
        "[&_li[data-rwp-highlight-item]]:text-xl [&_li[data-rwp-highlight-item]]:font-semibold [&_li[data-rwp-highlight-item]]:tabular-nums [&_li[data-rwp-highlight-item]]:text-brand-900",
        "dark:[&_li[data-rwp-highlight-item]]:text-white",
        disabled ? "pointer-events-none opacity-60" : "",
        className,
      ].join(" ")}
    >
      <div className="relative overflow-hidden">
        <div className="pointer-events-none absolute inset-x-0 top-1/2 h-8 -translate-y-1/2 rounded-md bg-brand-200/70 dark:bg-white/10" />
        <WheelPickerWrapper className="relative z-10 items-stretch gap-1">
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
                "text-xl font-semibold tabular-nums text-brand-900 dark:text-white",
              highlightWrapper: "rounded-md",
              optionItem:
                "text-xl font-semibold tabular-nums text-brand-500/45 dark:text-brand-400/35",
            }}
          />
          <div className="flex h-8 w-7 shrink-0 self-center items-center justify-center text-xl font-semibold leading-none text-brand-800 dark:text-brand-100">
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
                "text-xl font-semibold tabular-nums text-brand-900 dark:text-white",
              highlightWrapper: "rounded-md",
              optionItem:
                "text-xl font-semibold tabular-nums text-brand-500/45 dark:text-brand-400/35",
            }}
          />
          {hasThirdColumn && (
            <>
              <div className="flex h-8 w-7 shrink-0 self-center items-center justify-center text-xl font-semibold leading-none text-brand-800 dark:text-brand-100">
                {separator}
              </div>
              <WheelPicker
                value={thirdValue}
                options={thirdOptions!}
                infinite
                visibleCount={visibleCount}
                optionItemHeight={optionItemHeight}
                dragSensitivity={2.6}
                scrollSensitivity={5}
                onValueChange={onThirdChange}
                classNames={{
                  highlightItem:
                    "text-xl font-semibold tabular-nums text-brand-900 dark:text-white",
                  highlightWrapper: "rounded-md",
                  optionItem:
                    "text-xl font-semibold tabular-nums text-brand-500/45 dark:text-brand-400/35",
                }}
              />
            </>
          )}
        </WheelPickerWrapper>
      </div>
      {(firstLabel || secondLabel || thirdLabel) && (
        <div
          className={[
            "grid pt-1 text-center text-xs font-medium leading-4 text-brand-500 dark:text-brand-400",
            hasThirdColumn
              ? "grid-cols-[minmax(0,1fr)_1.75rem_minmax(0,1fr)_1.75rem_minmax(0,1fr)]"
              : "grid-cols-[minmax(0,1fr)_1.75rem_minmax(0,1fr)]",
          ].join(" ")}
        >
          <span>{firstLabel}</span>
          <span />
          <span>{secondLabel}</span>
          {hasThirdColumn && (
            <>
              <span />
              <span>{thirdLabel}</span>
            </>
          )}
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
  valueSeconds,
  onChange,
  minSeconds = 1,
  maxSeconds = defaultMaxSeconds,
  hourLabel = "h",
  minuteLabel = "m",
  secondLabel = "s",
  disabled = false,
  className = "",
}: BetterDurationWheelPickerProps) {
  const effectiveMinSeconds = Math.max(0, Math.floor(minSeconds));
  const effectiveMaxSeconds = Math.max(
    effectiveMinSeconds,
    Math.floor(maxSeconds),
  );
  const normalizedValue = clamp(
    Math.floor(valueSeconds),
    effectiveMinSeconds,
    effectiveMaxSeconds,
  );
  const selectedHour = Math.floor(normalizedValue / 3600);
  const selectedMinute = Math.floor((normalizedValue % 3600) / 60);
  const selectedSecond = normalizedValue % 60;
  const hourValues = useMemo(
    () => getAllowedHours(effectiveMinSeconds, effectiveMaxSeconds),
    [effectiveMaxSeconds, effectiveMinSeconds],
  );
  const minuteValues = useMemo(
    () =>
      getAllowedMinutesForHour(
        selectedHour,
        effectiveMinSeconds,
        effectiveMaxSeconds,
      ),
    [effectiveMaxSeconds, effectiveMinSeconds, selectedHour],
  );
  const secondValues = useMemo(
    () =>
      getAllowedSecondsForHourAndMinute(
        selectedHour,
        selectedMinute,
        effectiveMinSeconds,
        effectiveMaxSeconds,
      ),
    [effectiveMaxSeconds, effectiveMinSeconds, selectedHour, selectedMinute],
  );
  const hourOptions = useMemo(
    () => createWheelOptions(hourValues),
    [hourValues],
  );
  const minuteOptions = useMemo(
    () => createWheelOptions(minuteValues),
    [minuteValues],
  );
  const secondOptions = useMemo(
    () => createWheelOptions(secondValues),
    [secondValues],
  );

  const updateDuration = (hour: number, minute: number, second: number) => {
    onChange(
      clamp(
        hour * 3600 + minute * 60 + second,
        effectiveMinSeconds,
        effectiveMaxSeconds,
      ),
    );
  };

  const handleHourChange = (hour: number) => {
    const minute = coerceMinuteForHour(
      selectedMinute,
      hour,
      effectiveMinSeconds,
      effectiveMaxSeconds,
    );
    updateDuration(
      hour,
      minute,
      coerceValue(
        selectedSecond,
        getAllowedSecondsForHourAndMinute(
          hour,
          minute,
          effectiveMinSeconds,
          effectiveMaxSeconds,
        ),
      ),
    );
  };

  const handleMinuteChange = (minute: number) => {
    updateDuration(
      selectedHour,
      minute,
      coerceValue(
        selectedSecond,
        getAllowedSecondsForHourAndMinute(
          selectedHour,
          minute,
          effectiveMinSeconds,
          effectiveMaxSeconds,
        ),
      ),
    );
  };

  const handleSecondChange = (second: number) => {
    updateDuration(selectedHour, selectedMinute, second);
  };

  return (
    <BetterNumberWheelPicker
      firstValue={selectedHour}
      secondValue={selectedMinute}
      thirdValue={selectedSecond}
      firstOptions={hourOptions}
      secondOptions={minuteOptions}
      thirdOptions={secondOptions}
      firstLabel={hourLabel}
      secondLabel={minuteLabel}
      thirdLabel={secondLabel}
      disabled={disabled}
      className={className}
      onFirstChange={handleHourChange}
      onSecondChange={handleMinuteChange}
      onThirdChange={handleSecondChange}
    />
  );
}
