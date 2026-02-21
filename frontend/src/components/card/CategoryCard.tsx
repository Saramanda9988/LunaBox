import type { vo } from "../../../wailsjs/go/models";
import { useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

interface CategoryCardProps {
  category: vo.CategoryVO;
  onDelete?: (e: React.MouseEvent) => void;
  onEdit?: (e: React.MouseEvent) => void;
  selectionMode?: boolean;
  selected?: boolean;
  selectionDisabled?: boolean;
  onSelectChange?: (selected: boolean) => void;
}

export function CategoryCard({
  category,
  onDelete,
  onEdit,
  selectionMode = false,
  selected = false,
  selectionDisabled = false,
  onSelectChange,
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

  return (
    <div
      className={`glass-card flex items-center p-4 bg-white dark:bg-brand-800 border border-brand-200 dark:border-brand-700 rounded-xl shadow-sm hover:shadow-md transition-all text-left group relative ${selectionMode ? "cursor-pointer" : ""} ${selectionMode && selected ? "ring-2 ring-neutral-500 dark:ring-neutral-400" : ""}`}
      onClick={handleCardClick}
    >
      <div className={`p-3 rounded-lg mr-4 ${category.is_system
        ? "bg-error-100 text-error-600 dark:bg-error-900/30 dark:text-error-400"
        : "bg-neutral-100 text-neutral-600 dark:bg-neutral-900/30 dark:text-neutral-400"
      }`}
      >
        <div className={`text-2xl ${category.is_system ? "i-mdi-heart" : "i-mdi-folder"}`} />
      </div>
      <div className="flex-1">
        <h3 className="font-semibold text-brand-900 dark:text-white group-hover:text-neutral-600 dark:group-hover:text-neutral-400 transition-colors">
          {category.name}
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
          title={selectionDisabled ? t("categories.systemLocked") : (selected ? t("common.select") : t("common.select"))}
        >
          {selectionDisabled
            ? (
                <div className="i-mdi-lock text-brand-300 dark:text-brand-600 text-lg" />
              )
            : (
                <div className={`flex h-6 w-6 items-center justify-center rounded-full border shadow-sm ${selected
                  ? "bg-neutral-600 text-white border-neutral-600"
                  : "bg-white/90 text-transparent border-brand-300 dark:bg-brand-800/90 dark:border-brand-600"}`}
                >
                  <div className="i-mdi-check text-sm" />
                </div>
              )}
        </button>
      )}

      {!selectionMode && !category.is_system && (
        <div className="absolute right-2 flex flex-col gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
          {onEdit && (
            <button
              type="button"
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                onEdit(e);
              }}
              className="p-2 text-brand-400 hover:text-neutral-500"
              title={t("categories.editBtnTitle")}
            >
              <div className="i-mdi-pencil text-lg" />
            </button>
          )}
          {onDelete && (
            <button
              type="button"
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                onDelete(e);
              }}
              className="p-2 text-brand-400 hover:text-error-500"
              title={t("categories.deleteBtnTitle")}
            >
              <div className="i-mdi-delete text-lg" />
            </button>
          )}
        </div>
      )}
    </div>
  );
}
