import type { FormEvent } from "react";
import type { vo } from "../../../src/bindings/models";
import { useMemo, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  AddCategory,
  GetCategories,
} from "../../../bindings/lunabox/internal/service/categoryservice";
import { CATEGORY_NAME_MAX_LENGTH } from "../../consts/category";
import { EmojiPickerPopover } from "../ui/EmojiPickerPopover";
import { ModalPortal } from "../ui/ModalPortal";

interface AddToCategoryModalProps {
  isOpen: boolean;
  allCategories: vo.CategoryVO[];
  initialSelectedIds: string[];
  onClose: () => void;
  onSave: (selectedIds: string[]) => void;
  selectionMode?: "single" | "multiple";
  title?: string;
  confirmText?: string;
}

type AddToCategoryModalContentProps = Omit<AddToCategoryModalProps, "isOpen">;

function AddToCategoryModalContent({
  allCategories,
  initialSelectedIds,
  onClose,
  onSave,
  selectionMode = "multiple",
  title,
  confirmText,
}: AddToCategoryModalContentProps) {
  const { t } = useTranslation();
  const [categories, setCategories] = useState(allCategories);
  const [selectedIds, setSelectedIds] = useState<string[]>(() =>
    selectionMode === "single"
      ? initialSelectedIds.slice(0, 1)
      : initialSelectedIds,
  );
  const [searchQuery, setSearchQuery] = useState("");
  const [isCreateFormOpen, setIsCreateFormOpen] = useState(false);
  const [newCategoryName, setNewCategoryName] = useState("");
  const [newCategoryEmoji, setNewCategoryEmoji] = useState("");
  const [isCreating, setIsCreating] = useState(false);

  const modalTitle = title || t("addToCategory.title");
  const modalConfirmText = confirmText || t("common.confirm");
  const newCategoryNameCountText = `${newCategoryName.length}/${CATEGORY_NAME_MAX_LENGTH}`;
  const normalizedQuery = searchQuery.trim().toLocaleLowerCase();
  const filteredCategories = useMemo(() => {
    if (!normalizedQuery) {
      return categories;
    }

    return categories.filter((category) => {
      const displayName = category.is_system
        ? t("categories.favorites")
        : category.name;
      return `${displayName} ${category.name}`
        .toLocaleLowerCase()
        .includes(normalizedQuery);
    });
  }, [categories, normalizedQuery, t]);

  const toggleCategory = (categoryId: string) => {
    setSelectedIds(prev =>
      selectionMode === "single"
        ? prev[0] === categoryId
          ? []
          : [categoryId]
        : prev.includes(categoryId)
          ? prev.filter(id => id !== categoryId)
          : [...prev, categoryId],
    );
  };

  const openCreateForm = () => {
    setNewCategoryName(searchQuery.trim().slice(0, CATEGORY_NAME_MAX_LENGTH));
    setSearchQuery("");
    setIsCreateFormOpen(true);
  };

  const closeCreateForm = () => {
    setIsCreateFormOpen(false);
    setNewCategoryName("");
    setNewCategoryEmoji("");
  };

  const handleCreateCategory = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const name = newCategoryName.trim();
    if (!name || isCreating) {
      return;
    }

    const existingIds = new Set(categories.map(category => category.id));
    setIsCreating(true);
    try {
      await AddCategory(name, newCategoryEmoji.trim());
      const refreshedCategories = (await GetCategories()) || [];
      const createdCategory = refreshedCategories.find(
        category => !existingIds.has(category.id),
      );

      setCategories(refreshedCategories);
      if (createdCategory) {
        setSelectedIds(prev =>
          selectionMode === "single"
            ? [createdCategory.id]
            : prev.includes(createdCategory.id)
              ? prev
              : [...prev, createdCategory.id],
        );
      }
      setSearchQuery("");
      closeCreateForm();
      toast.success(t("categories.toast.createSuccess"));
    }
    catch (error) {
      console.error("Failed to create category:", error);
      toast.error(t("categories.toast.createFailed"));
    }
    finally {
      setIsCreating(false);
    }
  };

  const handleSave = () => {
    onSave(selectedIds);
    onClose();
  };

  return (
    <ModalPortal>
      <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
        <div
          className="w-full max-w-lg max-h-[78vh] rounded-xl border border-brand-200 bg-white flex flex-col shadow-xl dark:border-brand-700 dark:bg-brand-800"
          role="dialog"
          aria-modal="true"
          aria-labelledby="add-to-category-modal-title"
        >
          <div className="p-6 border-b border-brand-200 dark:border-brand-700 flex justify-between items-center gap-4">
            <div className="min-w-0">
              <h3
                id="add-to-category-modal-title"
                className="truncate text-xl font-bold text-brand-900 dark:text-white"
              >
                {modalTitle}
              </h3>
              <p className="mt-1 text-xs text-brand-500 dark:text-brand-400">
                {t("addToCategory.selectedCount", {
                  count: selectedIds.length,
                })}
              </p>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="shrink-0 rounded-lg p-2 text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-500 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white"
              aria-label={t("common.cancel")}
            >
              <div className="i-mdi-close text-xl" aria-hidden="true" />
            </button>
          </div>

          <form
            className="px-6 pt-4 flex items-center gap-3"
            onSubmit={handleCreateCategory}
          >
            {isCreateFormOpen ? (
              <>
                <EmojiPickerPopover
                  value={newCategoryEmoji}
                  canEdit
                  compact
                  fallbackIconClass="i-mdi-folder-outline"
                  onChange={setNewCategoryEmoji}
                />
                <div className="relative min-w-0 flex-1">
                  <input
                    type="text"
                    value={newCategoryName}
                    onChange={event => setNewCategoryName(event.target.value)}
                    maxLength={CATEGORY_NAME_MAX_LENGTH}
                    placeholder={t("categories.modal.namePlaceholder")}
                    className="glass-input h-11 w-full rounded-lg border border-brand-200 bg-white py-0 pl-3 pr-24 text-sm text-brand-900 outline-none dark:border-brand-700 dark:bg-brand-900 dark:text-white"
                    autoFocus
                  />
                  <span className="pointer-events-none absolute right-10 top-1/2 -translate-y-1/2 text-xs tabular-nums text-brand-400 dark:text-brand-500">
                    {newCategoryNameCountText}
                  </span>
                  <button
                    type="button"
                    onClick={closeCreateForm}
                    className="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-1.5 text-brand-400 transition-colors hover:bg-brand-100 hover:text-brand-700 dark:hover:bg-brand-700 dark:hover:text-brand-200"
                    aria-label={t("common.cancel")}
                  >
                    <div className="i-mdi-close text-base" aria-hidden="true" />
                  </button>
                </div>
              </>
            ) : (
              <div className="relative min-w-0 flex-1">
                <div
                  className="i-mdi-magnify pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-xl text-brand-400"
                  aria-hidden="true"
                />
                <input
                  type="search"
                  value={searchQuery}
                  onChange={event => setSearchQuery(event.target.value)}
                  placeholder={t("addToCategory.searchPlaceholder")}
                  className="glass-input h-11 w-full rounded-lg border border-brand-200 bg-white py-0 pl-10 pr-10 text-sm text-brand-900 outline-none dark:border-brand-700 dark:bg-brand-900 dark:text-white"
                  autoFocus
                />
                {searchQuery && (
                  <button
                    type="button"
                    onClick={() => setSearchQuery("")}
                    className="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-1.5 text-brand-400 transition-colors hover:bg-brand-100 hover:text-brand-700 dark:hover:bg-brand-700 dark:hover:text-brand-200"
                    aria-label={t("addToCategory.clearSearch")}
                  >
                    <div className="i-mdi-close text-base" aria-hidden="true" />
                  </button>
                )}
              </div>
            )}
            <button
              type={isCreateFormOpen ? "submit" : "button"}
              onClick={isCreateFormOpen ? undefined : openCreateForm}
              disabled={
                isCreateFormOpen && (!newCategoryName.trim() || isCreating)
              }
              className="h-11 w-11 shrink-0 rounded-full bg-neutral-600 flex items-center justify-center text-white shadow-sm transition-colors hover:bg-neutral-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-400 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 dark:ring-offset-brand-800"
              aria-label={
                isCreateFormOpen
                  ? isCreating
                    ? t("addToCategory.creating")
                    : t("categories.modal.create")
                  : t("categories.newCategory")
              }
            >
              <div
                className={`${
                  isCreating
                    ? "i-mdi-loading animate-spin"
                    : isCreateFormOpen
                      ? "i-mdi-check"
                      : "i-mdi-plus"
                } text-2xl`}
                aria-hidden="true"
              />
            </button>
          </form>

          <div className="min-h-0 flex-1 overflow-y-auto p-6">
            {filteredCategories.length > 0 ? (
              <div className="space-y-2">
                {filteredCategories.map((category) => {
                  const isSelected = selectedIds.includes(category.id);
                  const displayName = category.is_system
                    ? t("categories.favorites")
                    : category.name;
                  const displayEmoji = (category.emoji || "").trim();
                  return (
                    <button
                      type="button"
                      key={category.id}
                      onClick={() => toggleCategory(category.id)}
                      className="w-full rounded-lg bg-brand-50 p-3 flex items-center gap-3 text-left transition-colors hover:bg-brand-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-500/60 dark:bg-brand-900 dark:hover:bg-brand-700"
                    >
                      <span className="w-8 shrink-0 flex items-center justify-center text-brand-500 dark:text-brand-400">
                        {displayEmoji ? (
                          <span className="text-2xl leading-none">
                            {displayEmoji}
                          </span>
                        ) : (
                          <div
                            className={`${category.is_system ? "i-mdi-heart" : "i-mdi-folder-outline"} text-xl`}
                            aria-hidden="true"
                          />
                        )}
                      </span>
                      <span className="min-w-0 flex-1 truncate font-medium text-brand-900 dark:text-white">
                        {displayName}
                      </span>
                      <span className="shrink-0 text-sm text-brand-500 dark:text-brand-400">
                        {t("addToCategory.gameCount", {
                          count: category.game_count || 0,
                        })}
                      </span>
                      <span
                        className={`shrink-0 text-xl ${
                          isSelected
                            ? "text-neutral-600 dark:text-neutral-300"
                            : "text-brand-300 dark:text-brand-600"
                        } ${
                          selectionMode === "single"
                            ? isSelected
                              ? "i-mdi-radiobox-marked"
                              : "i-mdi-radiobox-blank"
                            : isSelected
                              ? "i-mdi-check-circle"
                              : "i-mdi-circle-outline"
                        }`}
                        aria-hidden="true"
                      />
                    </button>
                  );
                })}
              </div>
            ) : (
              <div className="min-h-44 flex flex-col items-center justify-center px-6 text-center text-brand-500 dark:text-brand-400">
                <div
                  className={`${categories.length > 0 ? "i-mdi-folder-search-outline" : "i-mdi-folder-plus-outline"} mb-3 text-4xl`}
                  aria-hidden="true"
                />
                <p className="text-sm font-medium">
                  {categories.length > 0
                    ? t("addToCategory.noResults")
                    : t("addToCategory.empty")}
                </p>
              </div>
            )}
          </div>

          <div className="border-t border-brand-200 p-4 dark:border-brand-700 flex gap-3">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 rounded-xl border border-brand-300 py-2.5 font-medium text-brand-600 transition-colors hover:bg-brand-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-500 dark:border-brand-600 dark:text-brand-400 dark:hover:bg-brand-700"
            >
              {t("common.cancel")}
            </button>
            <button
              type="button"
              onClick={handleSave}
              className="flex-1 rounded-xl bg-neutral-600 py-2.5 font-medium text-white transition-colors hover:bg-neutral-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-400 focus-visible:ring-offset-2 dark:ring-offset-brand-800"
            >
              {modalConfirmText}
            </button>
          </div>
        </div>
      </div>
    </ModalPortal>
  );
}

export function AddToCategoryModal({
  isOpen,
  ...props
}: AddToCategoryModalProps) {
  if (!isOpen) {
    return null;
  }

  return <AddToCategoryModalContent {...props} />;
}
