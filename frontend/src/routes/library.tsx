import type { vo } from "../../wailsjs/go/models";
import type { ImportSource } from "../components/modal/GameImportModal";
import { createRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { enums } from "../../wailsjs/go/models";
import { AddGamesToCategories, GetCategories } from "../../wailsjs/go/service/CategoryService";
import { BatchUpdateStatus, DeleteGames } from "../../wailsjs/go/service/GameService";
import { FilterBar } from "../components/bar/FilterBar";
import { GameCard } from "../components/card/GameCard";
import { AddGameModal } from "../components/modal/AddGameModal";
import { AddToCategoryModal } from "../components/modal/AddToCategoryModal";
import { BatchImportModal } from "../components/modal/BatchImportModal";
import { ConfirmModal } from "../components/modal/ConfirmModal";
import { GameImportModal } from "../components/modal/GameImportModal";
import { LibrarySkeleton } from "../components/skeleton/LibrarySkeleton";
import { BetterDropdownMenu } from "../components/ui/BetterDropdownMenu";
import { sortOptions, statusOptions } from "../consts/options";
import { useAppStore } from "../store";
import { Route as rootRoute } from "./__root";

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/library",
  component: LibraryPage,
});

function LibraryPage() {
  const { games, gamesLoading, fetchGames } = useAppStore();
  const { t } = useTranslation();
  const [showSkeleton, setShowSkeleton] = useState(false);
  const [isAddGameModalOpen, setIsAddGameModalOpen] = useState(false);
  const [isBatchImportOpen, setIsBatchImportOpen] = useState(false);
  const [importSource, setImportSource] = useState<ImportSource | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [sortBy, setSortBy] = useState<"name" | "created_at">("created_at");
  const [sortOrder, setSortOrder] = useState<"asc" | "desc">("desc");
  const [statusFilter, setStatusFilter] = useState<string>("");
  const [batchMode, setBatchMode] = useState(false);
  const [selectedGameIds, setSelectedGameIds] = useState<string[]>([]);
  const [allCategories, setAllCategories] = useState<vo.CategoryVO[]>([]);
  const [isBatchCategoryModalOpen, setIsBatchCategoryModalOpen] = useState(false);
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

  // 延迟显示骨架屏
  useEffect(() => {
    let timer: number;
    if (gamesLoading) {
      timer = window.setTimeout(() => {
        setShowSkeleton(true);
      }, 300);
    }
    else {
      setShowSkeleton(false);
    }
    return () => clearTimeout(timer);
  }, [gamesLoading]);

  const filteredGames = games
    .filter((game) => {
      // 搜索过滤：同时匹配游戏名和开发商/公司
      if (searchQuery) {
        const q = searchQuery.toLowerCase();
        const matchName = game.name.toLowerCase().includes(q);
        const matchCompany = (game.company || "").toLowerCase().includes(q);
        if (!matchName && !matchCompany)
          return false;
      }
      // 状态过滤
      return !(statusFilter && game.status !== statusFilter);
    })
    .sort((a, b) => {
      let comparison = 0;
      switch (sortBy) {
        case "name":
          comparison = a.name.localeCompare(b.name);
          break;
        case "created_at":
          comparison = String(a.created_at || "").localeCompare(String(b.created_at || ""));
          break;
      }
      return sortOrder === "asc" ? comparison : -comparison;
    });

  const handleBatchModeChange = (enabled: boolean) => {
    setBatchMode(enabled);
    if (!enabled) {
      setSelectedGameIds([]);
    }
  };

  const setGameSelection = (gameId: string, selected: boolean) => {
    setSelectedGameIds((prev) => {
      if (selected) {
        return prev.includes(gameId) ? prev : [...prev, gameId];
      }
      return prev.filter(id => id !== gameId);
    });
  };

  const handleSelectAll = () => {
    setSelectedGameIds((prev) => {
      const next = new Set(prev);
      filteredGames.forEach((game) => {
        if (game.id) {
          next.add(game.id);
        }
      });
      return Array.from(next);
    });
  };

  const handleClearSelection = () => {
    setSelectedGameIds([]);
  };

  const statusConfig = {
    [enums.GameStatus.NOT_STARTED]: { label: t("common.notStarted"), icon: "i-mdi-clock-outline", color: "bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300" },
    [enums.GameStatus.PLAYING]: { label: t("common.playing"), icon: "i-mdi-gamepad-variant", color: "bg-neutral-100 text-neutral-700 dark:bg-neutral-900 dark:text-neutral-300" },
    [enums.GameStatus.COMPLETED]: { label: t("common.completed"), icon: "i-mdi-trophy", color: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300" },
    [enums.GameStatus.ON_HOLD]: { label: t("common.onHold"), icon: "i-mdi-pause-circle-outline", color: "bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300" },
  };

  const handleBatchStatusUpdate = async (newStatus: string) => {
    if (selectedGameIds.length === 0)
      return;
    try {
      await BatchUpdateStatus(selectedGameIds, newStatus);
      await fetchGames();
      const label = statusConfig[newStatus as keyof typeof statusConfig]?.label ?? newStatus;
      toast.success(t("library.toast.batchStatusUpdated", { count: selectedGameIds.length, status: label }));
    }
    catch (error) {
      console.error("Failed to batch update status:", error);
      toast.error(t("library.toast.batchStatusFailed"));
    }
  };

  const openBatchAddModal = async () => {
    if (selectedGameIds.length === 0)
      return;
    try {
      const result = await GetCategories();
      setAllCategories(result || []);
      setIsBatchCategoryModalOpen(true);
    }
    catch (error) {
      console.error("Failed to load categories:", error);
      toast.error(t("library.toast.loadFavFailed"));
    }
  };

  const handleBatchAddToCategory = async (categoryIds: string[]) => {
    if (selectedGameIds.length === 0 || categoryIds.length === 0)
      return;
    try {
      await AddGamesToCategories(selectedGameIds, categoryIds);
      toast.success(t("library.toast.batchAddFavSuccess", { count: selectedGameIds.length }));
      setSelectedGameIds([]);
      setBatchMode(false);
    }
    catch (error) {
      console.error("Failed to batch add games to category:", error);
      toast.error(t("library.toast.batchAddFavFailed"));
    }
  };

  const handleBatchDelete = () => {
    if (selectedGameIds.length === 0)
      return;
    setConfirmConfig({
      isOpen: true,
      title: t("library.toast.batchDeleteTitle"),
      message: t("library.toast.batchDeleteConfirmMsg", { count: selectedGameIds.length }),
      type: "danger",
      onConfirm: async () => {
        try {
          await DeleteGames(selectedGameIds);
          await fetchGames();
          setSelectedGameIds([]);
          setBatchMode(false);
          toast.success(t("library.toast.batchDeleteSuccess"));
        }
        catch (error) {
          console.error("Failed to batch delete games:", error);
          toast.error(t("library.toast.batchDeleteFailed"));
        }
      },
    });
  };

  useEffect(() => {
    fetchGames();
  }, [fetchGames]);

  if (gamesLoading && games.length === 0) {
    if (!showSkeleton) {
      return null;
    }
    return <LibrarySkeleton />;
  }

  return (
    <div className={`space-y-6 max-w-8xl mx-auto p-8 transition-opacity duration-300 ${gamesLoading ? "opacity-50 pointer-events-none" : "opacity-100"}`}>
      <div className="flex items-center justify-between">
        <h1 className="text-4xl font-bold text-brand-900 dark:text-white">{t("library.title")}</h1>
      </div>

      <FilterBar
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        searchPlaceholder={t("library.searchPlaceholder")}
        sortBy={sortBy}
        onSortByChange={val => setSortBy(val as "name" | "created_at")}
        sortOptions={sortOptions.map(opt => ({ ...opt, label: t(opt.label) }))}
        sortOrder={sortOrder}
        onSortOrderChange={setSortOrder}
        statusFilter={statusFilter}
        onStatusFilterChange={setStatusFilter}
        statusOptions={statusOptions.map(opt => ({ ...opt, label: t(opt.label) }))}
        storageKey="library"
        batchMode={batchMode}
        onBatchModeChange={handleBatchModeChange}
        selectedCount={selectedGameIds.length}
        onSelectAll={handleSelectAll}
        onClearSelection={handleClearSelection}
        batchActions={(
          <>
            {/* 批量更新状态 */}
            <BetterDropdownMenu
              title={t("library.setStatus")}
              align="end"
              menuWidth="min-w-[130px]"
              disabled={selectedGameIds.length === 0}
              trigger={(
                <div
                  title={t("library.batchUpdateStatus")}
                  className={`glass-panel flex items-center gap-2 px-3 py-2 text-sm
                              bg-white dark:bg-brand-800 border border-brand-200 dark:border-brand-700
                              rounded-lg hover:bg-brand-100 dark:hover:bg-brand-700 text-brand-700 dark:text-brand-300
                              ${selectedGameIds.length === 0 ? "opacity-50 cursor-not-allowed" : ""}`}
                >
                  <div className="i-mdi-tag-edit-outline text-lg" />
                </div>
              )}
              items={Object.entries(statusConfig).map(([key, cfg]) => ({
                key,
                label: cfg.label,
                icon: cfg.icon,
                pill: true,
                pillColor: cfg.color,
                onClick: () => handleBatchStatusUpdate(key),
              }))}
            />
            {/* 批量添加到收藏 */}
            <button
              type="button"
              onClick={openBatchAddModal}
              disabled={selectedGameIds.length === 0}
              title={t("library.batchAddToFilter")}
              className={`glass-panel flex items-center gap-2 px-3 py-2 text-sm
                          bg-white dark:bg-brand-800 border border-brand-200 dark:border-brand-700
                          rounded-lg hover:bg-brand-100 dark:hover:bg-brand-700 text-brand-700 dark:text-brand-300
                          ${selectedGameIds.length === 0 ? "opacity-50 cursor-not-allowed" : ""}`}
            >
              <div className="i-mdi-folder-plus-outline text-lg" />
            </button>
            {/* 批量删除 */}
            <button
              type="button"
              onClick={handleBatchDelete}
              disabled={selectedGameIds.length === 0}
              title={t("library.batchDelete")}
              className={`glass-panel flex items-center gap-2 px-3 py-2 text-sm
                          bg-white dark:bg-brand-800 border border-brand-200 dark:border-brand-700
                          rounded-lg hover:bg-brand-100 dark:hover:bg-brand-700 text-error-600 dark:text-error-400
                          ${selectedGameIds.length === 0 ? "opacity-50 cursor-not-allowed" : ""}`}
            >
              <div className="i-mdi-delete text-lg" />
            </button>
          </>
        )}
        actionButton={(
          <BetterDropdownMenu
            align="end"
            menuWidth="min-w-[220px]"
            trigger={(
              <div className="glass-btn-neutral flex items-center rounded-lg bg-neutral-600 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-700 focus:outline-none focus:ring-4 focus:ring-neutral-300 dark:bg-neutral-600 dark:hover:bg-neutral-700 dark:focus:ring-neutral-800">
                <div className="i-mdi-plus mr-2 text-lg" />
                {t("library.addGame")}
                <div className="i-mdi-chevron-down ml-2 text-lg" />
              </div>
            )}
            items={[
              {
                key: "manual",
                label: t("common.manualAdd"),
                description: t("library.addGameDesc1"),
                icon: "i-mdi-gamepad-variant",
                iconColor: "text-neutral-500",
                onClick: () => setIsAddGameModalOpen(true),
              },
              {
                key: "batch",
                label: t("library.batchImport"),
                description: t("library.batchImportDesc"),
                icon: "i-mdi-folder-multiple",
                iconColor: "text-success-500",
                onClick: () => setIsBatchImportOpen(true),
              },
              {
                key: "potatovn",
                label: t("library.importPotatoVN"),
                description: t("library.importPotatoVNDesc"),
                icon: "i-mdi-database-import",
                iconColor: "text-orange-500",
                dividerBefore: true,
                onClick: () => setImportSource("potatovn"),
              },
              {
                key: "playnite",
                label: t("library.importPlaynite"),
                description: t("library.importPlayniteDesc"),
                icon: "i-mdi-application-import",
                iconColor: "text-purple-500",
                onClick: () => setImportSource("playnite"),
              },
              {
                key: "vnite",
                label: t("library.importVnite"),
                description: t("library.importVniteDesc"),
                icon: "i-mdi-folder-cog-outline",
                iconColor: "text-sky-500",
                onClick: () => setImportSource("vnite"),
              },
            ]}
          />
        )}
      />

      {games.length === 0
        ? (
            <div className="flex-1 flex items-center justify-center w-full">
              <div className="flex flex-col items-center justify-center py-20 text-brand-500 dark:text-brand-400">
                <div className="i-mdi-gamepad-variant-outline text-6xl mb-4" />
                <p className="text-xl">{t("library.emptyState")}</p>
                <p className="text-sm mt-2">{t("library.emptyStateAction")}</p>
                <div className="flex flex-col gap-3 mt-4">
                  <button
                    type="button"
                    onClick={() => setImportSource("potatovn")}
                    className="rounded-lg border border-success-600 px-5 py-2.5 text-sm font-medium text-success-600 hover:bg-success-50 focus:outline-none focus:ring-4 focus:ring-success-300 dark:border-success-500 dark:text-success-500 dark:hover:bg-success-900/20"
                  >
                    {t("library.importPotatoVN")}
                  </button>
                  <button
                    type="button"
                    onClick={() => setImportSource("playnite")}
                    className="rounded-lg border border-purple-600 px-5 py-2.5 text-sm font-medium text-purple-600 hover:bg-purple-50 focus:outline-none focus:ring-4 focus:ring-purple-300 dark:border-purple-500 dark:text-purple-500 dark:hover:bg-purple-900/20"
                  >
                    {t("library.importPlaynite")}
                  </button>
                  <button
                    type="button"
                    onClick={() => setImportSource("vnite")}
                    className="rounded-lg border border-sky-600 px-5 py-2.5 text-sm font-medium text-sky-600 hover:bg-sky-50 focus:outline-none focus:ring-4 focus:ring-sky-300 dark:border-sky-500 dark:text-sky-500 dark:hover:bg-sky-900/20"
                  >
                    {t("library.importVnite")}
                  </button>
                </div>
              </div>
            </div>
          )
        : filteredGames.length === 0
          ? (
              <div className="flex-1 flex items-center justify-center w-full text-brand-500 dark:text-brand-400">
                <div className="flex flex-col items-center">
                  <div className="i-mdi-magnify text-4xl mb-2" />
                  <p>{t("library.notFound")}</p>
                </div>
              </div>
            )
          : (
              <div className="grid grid-cols-[repeat(auto-fill,minmax(8.75rem,1fr))] gap-3">
                {filteredGames.map(game => (
                  <GameCard
                    key={game.id}
                    game={game}
                    searchQuery={searchQuery}
                    selectionMode={batchMode}
                    selected={selectedGameIds.includes(game.id)}
                    onSelectChange={selected => setGameSelection(game.id, selected)}
                  />
                ))}
              </div>
            )}

      <AddGameModal
        isOpen={isAddGameModalOpen}
        onClose={() => setIsAddGameModalOpen(false)}
        onGameAdded={fetchGames}
      />

      <GameImportModal
        isOpen={importSource !== null}
        source={importSource || "potatovn"}
        onClose={() => setImportSource(null)}
        onImportComplete={fetchGames}
      />

      <BatchImportModal
        isOpen={isBatchImportOpen}
        onClose={() => setIsBatchImportOpen(false)}
        onImportComplete={fetchGames}
      />

      <AddToCategoryModal
        isOpen={isBatchCategoryModalOpen}
        allCategories={allCategories}
        initialSelectedIds={[]}
        onClose={() => setIsBatchCategoryModalOpen(false)}
        onSave={handleBatchAddToCategory}
        title={t("library.batchAddToFilter")}
        confirmText={t("common.add")}
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
