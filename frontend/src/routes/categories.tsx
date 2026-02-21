import type { vo } from "../../wailsjs/go/models";
import { createRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  AddCategory,
  DeleteCategories,
  DeleteCategory,
  GetCategories,
  UpdateCategory,
} from "../../wailsjs/go/service/CategoryService";
import { FilterBar } from "../components/bar/FilterBar";
import { CategoryCard } from "../components/card/CategoryCard";
import { CategoryModal } from "../components/modal/CategoryModal";
import { ConfirmModal } from "../components/modal/ConfirmModal";
import { CategoriesSkeleton } from "../components/skeleton/CategoriesSkeleton";
import { Route as rootRoute } from "./__root";

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/categories",
  component: CategoriesPage,
});

function CategoriesPage() {
  const { t } = useTranslation();
  const [categories, setCategories] = useState<vo.CategoryVO[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [showSkeleton, setShowSkeleton] = useState(false);
  const [isAddCategoryModalOpen, setIsAddCategoryModalOpen] = useState(false);
  const [isEditCategoryModalOpen, setIsEditCategoryModalOpen] = useState(false);
  const [newCategoryName, setNewCategoryName] = useState("");
  const [editingCategory, setEditingCategory] = useState<vo.CategoryVO | null>(null);
  const [editCategoryName, setEditCategoryName] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [sortBy, setSortBy] = useState<"name" | "game_count" | "created_at" | "updated_at">("name");
  const [sortOrder, setSortOrder] = useState<"asc" | "desc">("asc");
  const [batchMode, setBatchMode] = useState(false);
  const [selectedCategoryIds, setSelectedCategoryIds] = useState<string[]>([]);

  // 确认弹窗状态
  const [confirmConfig, setConfirmConfig] = useState<{
    isOpen: boolean;
    title: string;
    message: string;
    type: "danger" | "info";
    onConfirm: () => void;
  }>({
    isOpen: false,
    title: "",
    message: "",
    type: "info",
    onConfirm: () => { },
  });

  const loadCategories = async () => {
    try {
      const result = await GetCategories();
      setCategories(result || []);
    }
    catch (error) {
      console.error("Failed to load categories:", error);
      toast.error(t("categories.toast.loadFailed"));
    }
    finally {
      setIsLoading(false);
    }
  };

  const handleAddCategory = async () => {
    if (!newCategoryName.trim())
      return;
    try {
      await AddCategory(newCategoryName);
      setNewCategoryName("");
      setIsAddCategoryModalOpen(false);
      await loadCategories();
      toast.success(t("categories.toast.createSuccess"));
    }
    catch (error) {
      console.error("Failed to add category:", error);
      toast.error(t("categories.toast.createFailed"));
    }
  };

  const handleEditCategory = (e: React.MouseEvent, category: vo.CategoryVO) => {
    e.stopPropagation();
    setEditingCategory(category);
    setEditCategoryName(category.name);
    setIsEditCategoryModalOpen(true);
  };

  const handleUpdateCategory = async () => {
    if (!editCategoryName.trim() || !editingCategory)
      return;
    try {
      await UpdateCategory(editingCategory.id, editCategoryName);
      setEditCategoryName("");
      setEditingCategory(null);
      setIsEditCategoryModalOpen(false);
      await loadCategories();
      toast.success(t("categories.toast.updateSuccess"));
    }
    catch (error) {
      console.error("Failed to update category:", error);
      toast.error(t("categories.toast.updateFailed"));
    }
  };

  const handleDeleteCategory = async (e: React.MouseEvent, category: vo.CategoryVO) => {
    e.stopPropagation();
    setConfirmConfig({
      isOpen: true,
      title: t("categories.toast.deleteTitle"),
      message: t("categories.toast.deleteMsg", { name: category.name }),
      type: "danger",
      onConfirm: async () => {
        try {
          await DeleteCategory(category.id);
          await loadCategories();
          toast.success(t("categories.toast.deleteSuccess"));
        }
        catch (error) {
          console.error("Failed to delete category:", error);
          toast.error(t("categories.toast.deleteFailed"));
        }
      },
    });
  };

  const filteredCategories = categories
    .filter((category) => {
      if (!searchQuery)
        return true;
      return category.name.toLowerCase().includes(searchQuery.toLowerCase());
    })
    .sort((a, b) => {
      let comparison = 0;
      switch (sortBy) {
        case "name":
          comparison = a.name.localeCompare(b.name);
          break;
        case "game_count":
          comparison = (a.game_count || 0) - (b.game_count || 0);
          break;
        case "created_at":
          comparison = (a.created_at || "").toString().localeCompare((b.created_at || "").toString());
          break;
        case "updated_at":
          comparison = (a.updated_at || "").toString().localeCompare((b.updated_at || "").toString());
          break;
      }
      return sortOrder === "asc" ? comparison : -comparison;
    });

  const handleBatchModeChange = (enabled: boolean) => {
    setBatchMode(enabled);
    if (!enabled) {
      setSelectedCategoryIds([]);
    }
  };

  const setCategorySelection = (category: vo.CategoryVO, selected: boolean) => {
    if (category.is_system)
      return;
    setSelectedCategoryIds((prev) => {
      if (selected) {
        return prev.includes(category.id) ? prev : [...prev, category.id];
      }
      return prev.filter(id => id !== category.id);
    });
  };

  const handleSelectAll = () => {
    setSelectedCategoryIds((prev) => {
      const next = new Set(prev);
      filteredCategories.forEach((category) => {
        if (!category.is_system) {
          next.add(category.id);
        }
      });
      return Array.from(next);
    });
  };

  const handleClearSelection = () => {
    setSelectedCategoryIds([]);
  };

  const handleBatchDelete = () => {
    if (selectedCategoryIds.length === 0)
      return;
    setConfirmConfig({
      isOpen: true,
      title: t("categories.toast.batchDeleteTitle"),
      message: t("categories.toast.batchDeleteMsg", { count: selectedCategoryIds.length }),
      type: "danger",
      onConfirm: async () => {
        try {
          await DeleteCategories(selectedCategoryIds);
          await loadCategories();
          setSelectedCategoryIds([]);
          setBatchMode(false);
          toast.success(t("categories.toast.batchDeleteSuccess"));
        }
        catch (error) {
          console.error("Failed to batch delete categories:", error);
          toast.error(t("categories.toast.batchDeleteFailed"));
        }
      },
    });
  };

  useEffect(() => {
    loadCategories();
  }, []);

  // 延迟显示骨架屏
  useEffect(() => {
    let timer: number;
    if (isLoading) {
      timer = window.setTimeout(() => {
        setShowSkeleton(true);
      }, 300);
    }
    else {
      setShowSkeleton(false);
    }
    return () => clearTimeout(timer);
  }, [isLoading]);

  if (isLoading && categories.length === 0) {
    if (!showSkeleton) {
      return null;
    }
    return <CategoriesSkeleton />;
  }

  return (
    <div className={`h-full w-full overflow-y-auto p-8 transition-opacity duration-300 ${isLoading ? "opacity-50 pointer-events-none" : "opacity-100"}`}>
      <div className="flex items-center justify-between">
        <h1 className="text-4xl font-bold text-brand-900 dark:text-white">{t("categories.title")}</h1>
      </div>

      <FilterBar
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        searchPlaceholder={t("categories.searchPlaceholder")}
        sortBy={sortBy}
        onSortByChange={val => setSortBy(val as any)}
        sortOptions={[
          { label: t("categories.sortOptions.name"), value: "name" },
          { label: t("categories.sortOptions.gameCount"), value: "game_count" },
          { label: t("categories.sortOptions.createdAt"), value: "created_at" },
          { label: t("categories.sortOptions.updatedAt"), value: "updated_at" },
        ]}
        sortOrder={sortOrder}
        onSortOrderChange={setSortOrder}
        batchMode={batchMode}
        onBatchModeChange={handleBatchModeChange}
        selectedCount={selectedCategoryIds.length}
        onSelectAll={handleSelectAll}
        onClearSelection={handleClearSelection}
        batchActions={(
          <button
            type="button"
            onClick={handleBatchDelete}
            disabled={selectedCategoryIds.length === 0}
            className={`glass-panel flex items-center gap-2 px-3 py-2 text-sm
                        bg-white dark:bg-brand-800 border border-brand-200 dark:border-brand-700
                        rounded-lg hover:bg-brand-100 dark:hover:bg-brand-700 text-error-600 dark:text-error-400
                        ${selectedCategoryIds.length === 0 ? "opacity-50 cursor-not-allowed" : ""}`}
          >
            <div className="i-mdi-delete text-lg" />
          </button>
        )}
        actionButton={(
          <button
            onClick={() => setIsAddCategoryModalOpen(true)}
            className="glass-btn-neutral flex items-center rounded-lg bg-neutral-600 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-700 focus:outline-none focus:ring-4 focus:ring-neutral-300 dark:bg-neutral-600 dark:hover:bg-neutral-700 dark:focus:ring-neutral-800"
          >
            <div className="i-mdi-plus mr-2 text-lg" />
            {t("categories.newCategory")}
          </button>
        )}
      />

      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
        {filteredCategories.map(category => (
          <CategoryCard
            key={category.id}
            category={category}
            onEdit={e => handleEditCategory(e, category)}
            onDelete={e => handleDeleteCategory(e, category)}
            selectionMode={batchMode}
            selected={selectedCategoryIds.includes(category.id)}
            selectionDisabled={category.is_system}
            onSelectChange={selected => setCategorySelection(category, selected)}
          />
        ))}
      </div>

      <CategoryModal
        isOpen={isAddCategoryModalOpen}
        value={newCategoryName}
        onChange={setNewCategoryName}
        onClose={() => setIsAddCategoryModalOpen(false)}
        onSubmit={handleAddCategory}
      />

      <CategoryModal
        isOpen={isEditCategoryModalOpen}
        value={editCategoryName}
        onChange={setEditCategoryName}
        onClose={() => {
          setIsEditCategoryModalOpen(false);
          setEditingCategory(null);
          setEditCategoryName("");
        }}
        onSubmit={handleUpdateCategory}
        mode="edit"
      />

      <ConfirmModal
        isOpen={confirmConfig.isOpen}
        title={confirmConfig.title}
        message={confirmConfig.message}
        type={confirmConfig.type}
        onClose={() => setConfirmConfig({ ...confirmConfig, isOpen: false })}
        onConfirm={confirmConfig.onConfirm}
      />
    </div>
  );
}
