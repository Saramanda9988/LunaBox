import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import "emoji-picker-element";

interface EmojiPickerPopoverProps {
  value?: string;
  canEdit: boolean;
  fallbackIconClass: string;
  variant?: "normal" | "system";
  onChange?: (emoji: string) => void | Promise<void>;
}

export function EmojiPickerPopover({
  value = "",
  canEdit,
  fallbackIconClass,
  variant = "normal",
  onChange,
}: EmojiPickerPopoverProps) {
  const { t, i18n } = useTranslation();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement | null>(null);
  const pickerMountRef = useRef<HTMLDivElement | null>(null);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const displayEmoji = value.trim();

  useEffect(() => {
    if (!open)
      return;
    const handleClickOutside = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [open]);

  useEffect(() => {
    if (!open || !pickerMountRef.current)
      return;

    const mountElement = pickerMountRef.current;

    const pickerElement = document.createElement("emoji-picker");
    const isDarkMode = document.documentElement.classList.contains("dark");
    const pickerLocale = i18n.language.startsWith("zh")
      ? "zh"
      : i18n.language.startsWith("ja")
        ? "ja"
        : "en";

    pickerElement.setAttribute("locale", pickerLocale);
    pickerElement.setAttribute("theme", isDarkMode ? "dark" : "light");
    pickerElement.setAttribute("skin-tone-emoji", "👍");
    pickerElement.setAttribute(
      "style",
      [
        "height: 320px",
        "border: none",
        "box-shadow: none",
        "--border-radius: 10px",
        "--border-color: transparent",
        "--outline-size: 0",
        `--background: ${isDarkMode ? "#1c1e1f" : "#ffffff"}`,
        `--text-color: ${isDarkMode ? "#f3f4f6" : "#121416"}`,
        `--input-font-color: ${isDarkMode ? "#f3f4f6" : "#121416"}`,
        `--input-placeholder-color: ${isDarkMode ? "#9ca3af" : "#6b7280"}`,
        `--input-background-color: ${isDarkMode ? "#303235" : "#ffffff"}`,
        `--input-border-color: ${isDarkMode ? "#4b5563" : "#d1d5db"}`,
        `--outline-color: ${isDarkMode ? "#64748B" : "#475569"}`,
        `--button-hover-background: ${isDarkMode ? "#44484e" : "#f3f4f6"}`,
        `--indicator-color: ${isDarkMode ? "#94A3B8" : "#64748B"}`,
      ].join("; "),
    );

    const handleEmojiClick = async (event: Event) => {
      const detail = (event as CustomEvent<{ unicode: string }>).detail;
      if (!detail?.unicode)
        return;
      await onChangeRef.current?.(detail.unicode);
      setOpen(false);
    };

    pickerElement.addEventListener("emoji-click", handleEmojiClick as EventListener);
    mountElement.innerHTML = "";
    mountElement.appendChild(pickerElement);

    return () => {
      pickerElement.removeEventListener("emoji-click", handleEmojiClick as EventListener);
      mountElement.innerHTML = "";
    };
  }, [open, i18n.language]);

  const handleTriggerClick = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (!canEdit)
      return;
    setOpen(prev => !prev);
  };

  const handleClear = async (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    await onChange?.("");
    setOpen(false);
  };

  return (
    <div className="relative mr-4" ref={ref}>
      <button
        type="button"
        onClick={handleTriggerClick}
        className={`p-3 rounded-lg ${variant === "system"
          ? "bg-error-100 text-error-600 dark:bg-error-900/30 dark:text-error-400"
          : "bg-neutral-100 text-neutral-600 dark:bg-neutral-900/30 dark:text-neutral-400"
        } ${canEdit ? "cursor-pointer hover:bg-neutral-200 dark:hover:bg-neutral-800/50" : "cursor-default"}`}
        title={canEdit ? t("categories.emojiPicker.change") : t("categories.systemLocked")}
      >
        {displayEmoji
          ? (
              <span className="text-2xl leading-none">{displayEmoji}</span>
            )
          : (
              <div className={`text-2xl ${fallbackIconClass}`} />
            )}
      </button>

      {open && canEdit && (
        <div
          className="absolute left-0 top-full z-20 mt-2 w-[24rem] rounded-xl border border-brand-200 bg-white p-3 shadow-lg dark:border-brand-700 dark:bg-brand-800"
          onClick={e => e.stopPropagation()}
        >
          <div className="mb-2 flex items-center justify-between">
            <span className="text-sm font-medium text-brand-700 dark:text-brand-300">{t("categories.emojiPicker.title")}</span>
            <button
              type="button"
              onClick={handleClear}
              className="text-xs text-brand-500 hover:text-neutral-600 dark:text-brand-400 dark:hover:text-neutral-400"
            >
              {t("categories.emojiPicker.clear")}
            </button>
          </div>
          <div
            ref={pickerMountRef}
            className="overflow-hidden rounded-lg bg-white dark:bg-brand-800"
          />
        </div>
      )}
    </div>
  );
}
