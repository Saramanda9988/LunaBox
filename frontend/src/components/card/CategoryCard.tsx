import type { vo } from "../../../src/bindings/models";
import { useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { EmojiPickerPopover } from "../ui/EmojiPickerPopover";

interface CategoryCardProps {
  category: vo.CategoryVO;
  selectionMode?: boolean;
  selected?: boolean;
  selectionDisabled?: boolean;
  onSelectChange?: (selected: boolean) => void;
  onEmojiChange?: (emoji: string) => void | Promise<void>;
}

export function CategoryCard({
  category,
  selectionMode = false,
  selected = false,
  selectionDisabled = false,
  onSelectChange,
  onEmojiChange,
}: CategoryCardProps) {
  const navigate = useNavigate();
  const { t } = useTranslation();

  const handleViewDetails = () => {
    navigate({ to: `/categories/${category.id}` });
  };

  const handleToggleSelect = (e?: React.MouseEvent) => {
    if (e) {
      e.preventDefault();
      e.stopPropagation();
    }
    if (selectionDisabled)
      return;
    onSelectChange?.(!selected);
  };

  const handleCardClick = () => {
    if (selectionMode) {
      handleToggleSelect();
      return;
    }
    handleViewDetails();
  };

  const canEditEmoji = !category.is_system && !selectionMode;

  return (
    <div
      data-drag-selection-id={
        selectionMode && !selectionDisabled ? category.id : undefined
      }
      className={`glass-card flex items-center p-4 bg-white dark:bg-brand-800 border border-brand-200 dark:border-brand-700 rounded-xl shadow-sm hover:shadow-md transition-all text-left group relative ${selectionMode ? "cursor-pointer" : ""} ${selectionMode && !selectionDisabled ? "[touch-action:none]" : ""} ${selectionMode && selected ? "ring-2 ring-neutral-500 dark:ring-neutral-400" : ""}`}
      onClick={handleCardClick}
    >
      <EmojiPickerPopover
        value={category.emoji || ""}
        canEdit={canEditEmoji}
        variant={category.is_system ? "system" : "normal"}
        fallbackIconClass={category.is_system ? "i-mdi-heart" : "i-mdi-folder"}
        onChange={onEmojiChange}
      />
      <div className="min-w-0 flex-1">
        <h3 className="truncate font-semibold text-brand-900 transition-colors group-hover:text-neutral-600 dark:text-white dark:group-hover:text-neutral-400">
          {category.is_system ? t("categories.favorites") : category.name}
        </h3>
        <p className="text-sm text-brand-500 dark:text-brand-400">
          {t("categories.gameCount", { count: category.game_count })}
        </p>
      </div>

      {selectionMode && (
        <button
          type="button"
          onClick={handleToggleSelect}
          className="absolute right-3 top-1/2 -translate-y-1/2"
        >
          {selectionDisabled ? (
            <div className="i-mdi-lock text-brand-300 dark:text-brand-600 text-lg" />
          ) : (
            <div
              className={`flex h-6 w-6 items-center justify-center rounded-full border shadow-sm ${
                selected
                  ? "bg-neutral-600 text-white border-neutral-600"
                  : "bg-white/90 text-transparent border-brand-300 dark:bg-brand-800/90 dark:border-brand-600"
              }`}
            >
              <div className="i-mdi-check text-sm" />
            </div>
          )}
        </button>
      )}
    </div>
  );
}
