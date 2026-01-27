import { Listbox, ListboxButton, ListboxOption, ListboxOptions } from "@headlessui/react";

export interface BetterSelectOption {
  value: string;
  label: string;
}

interface BetterSelectProps {
  value: string;
  onChange: (value: string) => void;
  options: BetterSelectOption[];
  placeholder?: string;
  disabled?: boolean;
  className?: string;
  name?: string;
}

export function BetterSelect({
  value,
  onChange,
  options,
  placeholder = "请选择",
  disabled = false,
  className = "",
}: BetterSelectProps) {
  const selectedOption = options.find(opt => opt.value === value);
  const displayValue = selectedOption?.label || placeholder;

  return (
    <Listbox value={value} onChange={onChange} disabled={disabled}>
      <div className={`relative ${className}`}>
        {/* Select Button */}
        <ListboxButton
          className="glass-input relative w-full px-3 py-2 pr-10
                     text-left cursor-pointer
                     border border-brand-300 dark:border-brand-600
                     rounded-md shadow-sm
                     bg-white dark:bg-brand-700
                     text-brand-900 dark:text-white
                     focus:outline-none focus:ring-2 focus:ring-neutral-500
                     disabled:opacity-50 disabled:cursor-not-allowed
                     transition-colors"
        >
          <span className={`block truncate ${!selectedOption ? "text-brand-400 dark:text-brand-500" : ""}`}>
            {displayValue}
          </span>
          <span className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
            <div className="i-mdi-chevron-down h-5 w-5 text-brand-400 dark:text-brand-500" />
          </span>
        </ListboxButton>

        {/* Options Dropdown */}
        <ListboxOptions
          anchor="bottom start"
          className="absolute z-50 mt-1 max-h-60 w-[var(--button-width)] overflow-auto
                     glass-card
                     bg-white dark:bg-brand-800
                     border border-brand-300 dark:border-brand-600
                     rounded-md shadow-lg
                     py-1
                     focus:outline-none
                     [--anchor-gap:4px]"
        >
          {options.map(option => (
            <ListboxOption
              key={option.value}
              value={option.value}
              className={
                ({ focus, selected }: { focus: boolean; selected: boolean }) =>
                  `relative cursor-pointer select-none py-2 pl-10 pr-4
                   ${focus ? "bg-neutral-100 dark:bg-brand-700" : ""}
                   ${selected ? "bg-neutral-50 dark:bg-brand-750" : ""}
                   text-brand-900 dark:text-white
                   transition-colors`
              }
            >
              {({ selected }: { selected: boolean }) => (
                <>
                  {selected && (
                    <span className="absolute inset-y-0 left-0 flex items-center pl-3 text-neutral-600 dark:text-neutral-400">
                      <div className="i-mdi-check h-5 w-5" />
                    </span>
                  )}
                  <span className={`block truncate ${selected ? "font-medium" : "font-normal"}`}>
                    {option.label}
                  </span>
                </>
              )}
            </ListboxOption>
          ))}
        </ListboxOptions>
      </div>
    </Listbox>
  );
}
