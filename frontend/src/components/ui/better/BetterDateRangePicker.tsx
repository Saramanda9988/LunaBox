import type { ReactNode } from "react";
import { Popover, PopoverButton, PopoverPanel } from "@headlessui/react";
import { useMemo, useState } from "react";
import { formatDateToYYYYMMDD } from "../../../utils/time";
import { BetterButton } from "./BetterButton";

interface BetterDateRangePickerProps {
  startDate: string;
  endDate: string;
  triggerLabel: string;
  applyLabel: string;
  resetLabel: string;
  onStartDateChange: (value: string) => void;
  onEndDateChange: (value: string) => void;
  onApply: () => void;
  onReset: () => void;
  active?: boolean;
  disabled?: boolean;
  className?: string;
}

const dayFormatter = new Intl.DateTimeFormat(undefined, { weekday: "short" });
const monthFormatter = new Intl.DateTimeFormat(undefined, {
  year: "numeric",
  month: "long",
});
const todayValue = formatDateToYYYYMMDD(new Date());

function parseDateValue(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match)
    return null;

  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(year, month - 1, day);

  if (
    date.getFullYear() !== year
    || date.getMonth() !== month - 1
    || date.getDate() !== day
  ) {
    return null;
  }

  return date;
}

function startOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function addMonths(date: Date, amount: number): Date {
  return new Date(date.getFullYear(), date.getMonth() + amount, 1);
}

function getLatestVisibleMonth(): Date {
  return addMonths(startOfMonth(new Date()), -1);
}

function getInitialVisibleMonth(startDate: string): Date {
  const selectedMonth = startOfMonth(parseDateValue(startDate) ?? new Date());
  const latestVisibleMonth = getLatestVisibleMonth();

  if (selectedMonth > latestVisibleMonth) {
    return latestVisibleMonth;
  }

  return selectedMonth;
}

function getCalendarDays(monthDate: Date) {
  const firstDay = new Date(monthDate.getFullYear(), monthDate.getMonth(), 1);
  const start = new Date(firstDay);
  start.setDate(firstDay.getDate() - firstDay.getDay());

  return Array.from({ length: 42 }, (_, index) => {
    const date = new Date(start);
    date.setDate(start.getDate() + index);
    return date;
  });
}

function isInRange(day: string, startDate: string, endDate: string) {
  return Boolean(startDate && endDate && day > startDate && day < endDate);
}

function CalendarMonth({
  monthDate,
  startDate,
  endDate,
  onSelectDate,
  headerStart,
  headerEnd,
}: {
  monthDate: Date;
  startDate: string;
  endDate: string;
  onSelectDate: (value: string) => void;
  headerStart?: ReactNode;
  headerEnd?: ReactNode;
}) {
  const days = useMemo(() => getCalendarDays(monthDate), [monthDate]);

  return (
    <div className="w-[18.5rem]">
      <div className="mb-3 grid h-9 grid-cols-[4.5rem_1fr_4.5rem] items-center">
        <div className="flex items-center gap-1">{headerStart}</div>
        <div className="text-center text-sm font-semibold text-brand-900 dark:text-white">
          {monthFormatter.format(monthDate)}
        </div>
        <div className="flex items-center justify-end gap-1">{headerEnd}</div>
      </div>
      <div className="grid grid-cols-7 gap-1 text-center">
        {Array.from({ length: 7 }, (_, index) => (
          <div
            key={`weekday-${index}`}
            className="h-8 text-[11px] font-medium leading-8 text-brand-400 dark:text-brand-500"
          >
            {dayFormatter.format(new Date(2026, 1, index + 1))}
          </div>
        ))}
        {days.map((date) => {
          const value = formatDateToYYYYMMDD(date);
          const isOutside = date.getMonth() !== monthDate.getMonth();
          const isStart = value === startDate;
          const isEnd = value === endDate;
          const selected = isStart || isEnd;
          const ranged = isInRange(value, startDate, endDate);
          const isFuture = value > todayValue;

          return (
            <button
              key={value}
              type="button"
              disabled={isFuture}
              onClick={() => {
                if (!isFuture) {
                  onSelectDate(value);
                }
              }}
              className={[
                "flex h-9 items-center justify-center rounded-lg text-sm transition-colors",
                "focus:outline-none focus:ring-2 focus:ring-neutral-500/30",
                isFuture
                  ? "cursor-not-allowed text-brand-200 dark:text-brand-700"
                  : "",
                isOutside
                  ? "text-brand-300 hover:text-brand-600 dark:text-brand-600 dark:hover:text-brand-300"
                  : "text-brand-700 hover:bg-brand-100 dark:text-brand-200 dark:hover:bg-brand-700",
                ranged
                  ? "bg-brand-100 text-brand-800 dark:bg-brand-700/70 dark:text-white"
                  : "",
                selected
                  ? "bg-neutral-800 text-white hover:bg-neutral-800 dark:bg-white dark:text-neutral-950 dark:hover:bg-white"
                  : "",
                isFuture
                  ? "cursor-not-allowed text-brand-200 hover:bg-transparent hover:text-brand-200 dark:text-brand-700 dark:hover:bg-transparent dark:hover:text-brand-700"
                  : "",
              ].join(" ")}
            >
              {date.getDate()}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function CalendarNavButton({
  icon,
  label,
  disabled,
  onClick,
}: {
  icon: string;
  label: string;
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      disabled={disabled}
      onClick={onClick}
      className="flex h-8 w-8 items-center justify-center rounded-lg border border-transparent text-brand-500 transition-colors hover:border-brand-200 hover:bg-brand-100 hover:text-brand-900 disabled:cursor-not-allowed disabled:opacity-35 dark:text-brand-400 dark:hover:border-brand-700 dark:hover:bg-brand-700 dark:hover:text-white"
    >
      <span className={`${icon} text-lg`} aria-hidden="true" />
    </button>
  );
}

export function BetterDateRangePicker({
  startDate,
  endDate,
  triggerLabel,
  applyLabel,
  resetLabel,
  onStartDateChange,
  onEndDateChange,
  onApply,
  onReset,
  active = false,
  disabled = false,
  className = "",
}: BetterDateRangePickerProps) {
  const [visibleMonth, setVisibleMonth] = useState(() =>
    getInitialVisibleMonth(startDate),
  );
  const latestVisibleMonth = getLatestVisibleMonth();
  const nextMonth = addMonths(visibleMonth, 1);
  const canGoNextMonth = addMonths(visibleMonth, 1) <= latestVisibleMonth;
  const canGoNextYear = addMonths(visibleMonth, 12) <= latestVisibleMonth;
  const rangeText
    = startDate && endDate ? `${startDate} - ${endDate}` : triggerLabel;
  const selectedRangeText
    = startDate && endDate ? `${startDate} - ${endDate}` : startDate || "";

  const handleSelectDate = (value: string) => {
    if (!startDate || (startDate && endDate)) {
      onStartDateChange(value);
      onEndDateChange("");
      const selectedMonth = startOfMonth(parseDateValue(value) ?? visibleMonth);
      if (selectedMonth <= latestVisibleMonth) {
        setVisibleMonth(selectedMonth);
      }
      return;
    }

    if (value < startDate) {
      onStartDateChange(value);
      const selectedMonth = startOfMonth(parseDateValue(value) ?? visibleMonth);
      if (selectedMonth <= latestVisibleMonth) {
        setVisibleMonth(selectedMonth);
      }
      return;
    }

    onEndDateChange(value);
  };

  return (
    <Popover className={`relative inline-block ${className}`}>
      <PopoverButton
        as={BetterButton}
        size="md"
        variant={active ? "primary" : "secondary"}
        icon="i-mdi-calendar-range"
        disabled={disabled}
        className="h-10"
        aria-pressed={active}
      >
        <span className="max-w-[14rem] truncate">{rangeText}</span>
      </PopoverButton>

      <PopoverPanel
        anchor="bottom start"
        className="z-[9999] mt-2 w-auto max-w-[calc(100vw-2rem)] overflow-auto rounded-xl border border-brand-200 bg-white p-3 shadow-xl focus:outline-none dark:border-brand-700 dark:bg-brand-800 data-glass:bg-white/90 data-glass:backdrop-blur-20 data-glass:dark:bg-brand-900/90 [--anchor-gap:8px]"
      >
        {({ close }: { close: () => void }) => (
          <div className="space-y-3">
            <div className="flex flex-col gap-4 xl:flex-row">
              <CalendarMonth
                monthDate={visibleMonth}
                startDate={startDate}
                endDate={endDate}
                onSelectDate={handleSelectDate}
                headerStart={(
                  <>
                    <CalendarNavButton
                      icon="i-mdi-chevron-double-left"
                      label="Previous year"
                      onClick={() =>
                        setVisibleMonth(month => addMonths(month, -12))}
                    />
                    <CalendarNavButton
                      icon="i-mdi-chevron-left"
                      label="Previous month"
                      onClick={() =>
                        setVisibleMonth(month => addMonths(month, -1))}
                    />
                  </>
                )}
              />
              <CalendarMonth
                monthDate={nextMonth}
                startDate={startDate}
                endDate={endDate}
                onSelectDate={handleSelectDate}
                headerEnd={(
                  <>
                    <CalendarNavButton
                      icon="i-mdi-chevron-right"
                      label="Next month"
                      disabled={!canGoNextMonth}
                      onClick={() =>
                        setVisibleMonth(month => addMonths(month, 1))}
                    />
                    <CalendarNavButton
                      icon="i-mdi-chevron-double-right"
                      label="Next year"
                      disabled={!canGoNextYear}
                      onClick={() =>
                        setVisibleMonth(month => addMonths(month, 12))}
                    />
                  </>
                )}
              />
            </div>

            <div className="flex items-center justify-between gap-3 border-t border-brand-200 pt-3 dark:border-brand-700">
              <div className="min-w-0 truncate text-xs text-brand-500 dark:text-brand-400">
                {selectedRangeText}
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <BetterButton
                  size="sm"
                  variant="secondary"
                  icon="i-mdi-restore"
                  disabled={disabled}
                  onClick={() => {
                    onReset();
                    close();
                  }}
                >
                  {resetLabel}
                </BetterButton>
                <BetterButton
                  size="sm"
                  variant="primary"
                  icon="i-mdi-check"
                  disabled={disabled}
                  onClick={() => {
                    onApply();
                    if (startDate && endDate && startDate < endDate) {
                      close();
                    }
                  }}
                >
                  {applyLabel}
                </BetterButton>
              </div>
            </div>
          </div>
        )}
      </PopoverPanel>
    </Popover>
  );
}
