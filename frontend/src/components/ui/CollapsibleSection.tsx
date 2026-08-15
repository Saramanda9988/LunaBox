import type { ReactNode } from "react";
import { useId, useState } from "react";

interface CollapsibleSectionProps {
  title: string;
  icon: string;
  children: ReactNode;
  defaultOpen?: boolean;
}

export function CollapsibleSection({
  title,
  icon,
  children,
  defaultOpen = true,
}: CollapsibleSectionProps) {
  const [isOpen, setIsOpen] = useState(defaultOpen);
  const [hasOpened, setHasOpened] = useState(defaultOpen);
  const contentId = useId();

  const handleToggle = () => {
    if (!isOpen) {
      setHasOpened(true);
    }
    setIsOpen(current => !current);
  };

  return (
    <section className="glass-settings-section settings-section-render overflow-hidden rounded-xl border border-brand-200 bg-brand-50 dark:border-brand-700 dark:bg-brand-800">
      <button
        type="button"
        aria-controls={contentId}
        aria-expanded={isOpen}
        onClick={handleToggle}
        className="flex w-full items-center justify-between p-4 transition-colors data-glass:bg-white/20 data-glass:dark:bg-black/20"
      >
        <h2 className="flex items-center gap-2 text-lg font-semibold text-brand-900 dark:text-white">
          <span
            className={`${icon} text-xl text-neutral-500 dark:text-neutral-400`}
          />
          {title}
        </h2>
        <span
          className={`i-mdi-chevron-down text-xl text-brand-500 transition-transform duration-200 ${isOpen ? "rotate-180" : ""}`}
        />
      </button>
      <div
        id={contentId}
        aria-hidden={!isOpen}
        className={`settings-section-transition grid duration-200 ease-out motion-reduce:transition-none ${
          isOpen
            ? "visible grid-rows-[1fr] opacity-100"
            : "invisible pointer-events-none grid-rows-[0fr] opacity-0"
        }`}
      >
        <div className="min-h-0 overflow-hidden">
          {hasOpened && <div className="space-y-4 p-5">{children}</div>}
        </div>
      </div>
    </section>
  );
}
