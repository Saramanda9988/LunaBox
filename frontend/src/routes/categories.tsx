import type { CategoryVO } from "../../bindings/lunabox/internal/vo";
import { createRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import toast from "react-hot-toast";
import {
  AddCategory,
  DeleteCategory,
  GetCategories,
} from "../../bindings/lunabox/internal/service/categoryservice";
import { FilterBar } from "../components/bar/FilterBar";
import { CategoryCard } from "../components/card/CategoryCard";
import { AddCategoryModal } from "../components/modal/AddCategoryModal";
import { ConfirmModal } from "../components/modal/ConfirmModal";
import { CategoriesSkeleton } from "../components/skeleton/CategoriesSkeleton";
import { Route as rootRoute } from "./__root";

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/categories",
  component: CategoriesPage,
});

function CategoriesPage() {
  const [categories, setCategories] = useState<CategoryVO[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [showSkeleton, setShowSkeleton] = useState(false);
  const [isAddCategoryModalOpen, setIsAddCategoryModalOpen] = useState(false);
  const [newCategoryName, setNewCategoryName] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [sortBy, setSortBy] = useState<"name" | "game_count" | "created_at" | "updated_at">("name");
  const [sortOrder, setSortOrder] = useState<"asc" | "desc">("asc");

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
    onConfirm: () => {},
  });

  const loadCategories = async () => {
    try {
      const result = await GetCategories();
      setCategories(result || []);
    }
    catch (error) {
      console.error("Failed to load categories:", error);
      toast.error("加载收藏夹失败");
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
      toast.success("收藏夹创建成功");
    }
    catch (error) {
      console.error("Failed to add category:", error);
      toast.error("创建收藏夹失败");
    }
  };

  const handleDeleteCategory = async (e: React.MouseEvent, category: CategoryVO) => {
    e.stopPropagation();
    setConfirmConfig({
      isOpen: true,
      title: "删除收藏夹",
      message: `确定要删除收藏夹 "${category.name}" 吗？此操作无法撤销。`,
      type: "danger",
      onConfirm: async () => {
        try {
          await DeleteCategory(category.id);
          await loadCategories();
          toast.success("收藏夹已删除");
        }
        catch (error) {
          console.error("Failed to delete category:", error);
          toast.error("删除收藏夹失败");
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
      return <div className="min-h-screen bg-brand-100 dark:bg-brand-900" />;
    }
    return <CategoriesSkeleton />;
  }

  return (
    <div className={`h-full w-full overflow-y-auto p-8 transition-opacity duration-300 ${isLoading ? "opacity-50 pointer-events-none" : "opacity-100"}`}>
      <div className="flex items-center justify-between">
        <h1 className="text-4xl font-bold text-brand-900 dark:text-white">收藏</h1>
      </div>

      <FilterBar
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        searchPlaceholder="搜索收藏夹..."
        sortBy={sortBy}
        onSortByChange={val => setSortBy(val as any)}
        sortOptions={[
          { label: "名称", value: "name" },
          { label: "游戏数量", value: "game_count" },
          { label: "创建时间", value: "created_at" },
          { label: "更新时间", value: "updated_at" },
        ]}
        sortOrder={sortOrder}
        onSortOrderChange={setSortOrder}
        actionButton={(
          <button
            onClick={() => setIsAddCategoryModalOpen(true)}
            className="flex items-center rounded-lg bg-neutral-600 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-700 focus:outline-none focus:ring-4 focus:ring-neutral-300 dark:bg-neutral-600 dark:hover:bg-neutral-700 dark:focus:ring-neutral-800"
          >
            <div className="i-mdi-plus mr-2 text-lg" />
            新建收藏夹
          </button>
        )}
      />

      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
        {filteredCategories.map(category => (
          <CategoryCard
            key={category.id}
            category={category}
            onDelete={e => handleDeleteCategory(e, category)}
          />
        ))}
      </div>

      <AddCategoryModal
        isOpen={isAddCategoryModalOpen}
        value={newCategoryName}
        onChange={setNewCategoryName}
        onClose={() => setIsAddCategoryModalOpen(false)}
        onSubmit={handleAddCategory}
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
