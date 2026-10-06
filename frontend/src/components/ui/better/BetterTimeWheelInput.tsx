import { Popover, PopoverButton, PopoverPanel } from "@headlessui/react";
import { BetterActionInput } from "./BetterActionInput";
import { BetterTimeWheelPicker } from "./BetterWheelPicker";

interface BetterTimeWheelInputProps {
  /** 24 小时制时间字符串，格式为 "HH:mm" */
  value: string;
  onChange: (value: string) => void;
  /** 时钟按钮的无障碍名称，同时作为 hover 提示文案 */
  actionLabel: string;
  id?: string;
  disabled?: boolean;
  /** 作用于外层 Popover 容器，用于控制宽度与对齐 */
  className?: string;
}

const clockPattern = /^(\d{1,2}):(\d{1,2})$/;
const minutesPerDay = 24 * 60;

function parseClockMinutes(value: string): number {
  const matched = clockPattern.exec(value.trim());

  if (!matched) {
    return 0;
  }

  const hours = Number(matched[1]);
  const minutes = Number(matched[2]);

  if (hours > 23 || minutes > 59) {
    return 0;
  }

  return hours * 60 + minutes;
}

function formatClockMinutes(value: number): string {
  const normalized
    = ((Math.floor(value) % minutesPerDay) + minutesPerDay) % minutesPerDay;
  const hours = Math.floor(normalized / 60);
  const minutes = normalized % 60;

  return `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}`;
}

export function BetterTimeWheelInput({
  value,
  onChange,
  actionLabel,
  id,
  disabled = false,
  className = "",
}: BetterTimeWheelInputProps) {
  const valueMinutes = parseClockMinutes(value);

  return (
    <Popover className={`relative inline-block ${className}`}>
      <PopoverButton
        as="div"
        disabled={disabled}
        tabIndex={-1}
        className="flex w-full min-w-0 items-center rounded-md focus:outline-none focus:ring-2 focus:ring-neutral-500"
      >
        <BetterActionInput
          id={id}
          readOnly
          value={formatClockMinutes(valueMinutes)}
          disabled={disabled}
          containerClassName="cursor-pointer"
          className="cursor-pointer font-medium tabular-nums"
          actions={[
            {
              ariaLabel: actionLabel,
              icon: "i-mdi-clock-outline",
              tabIndex: -1,
            },
          ]}
        />
      </PopoverButton>

      <PopoverPanel
        anchor="bottom end"
        className="z-[9000] w-72 max-w-[calc(100vw-2rem)] rounded-xl border border-brand-200 bg-white p-3 shadow-xl focus:outline-none dark:border-brand-700 dark:bg-brand-800 data-glass:bg-white/90 data-glass:backdrop-blur-20 data-glass:dark:bg-brand-900/90 [--anchor-gap:8px]"
      >
        <BetterTimeWheelPicker
          valueMinutes={valueMinutes}
          onChange={minutes => onChange(formatClockMinutes(minutes))}
          disabled={disabled}
        />
      </PopoverPanel>
    </Popover>
  );
}
