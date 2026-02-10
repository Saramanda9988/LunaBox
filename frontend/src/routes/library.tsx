import type { vo } from "../../wailsjs/go/models";
import type { ImportSource } from "../components/modal/GameImportModal";
import { createRoute } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { AddGamesToCategories, GetCategories } from "../../wailsjs/go/service/CategoryService";
import { DeleteGames } from "../../wailsjs/go/service/GameService";
import { FilterBar } from "../components/bar/FilterBar";
import { GameCard } from "../components/card/GameCard";
import { AddGameModal } from "../components/modal/AddGameModal";
import { AddToCategoryModal } from "../components/modal/AddToCategoryModal";
import { BatchImportModal } from "../components/modal/BatchImportModal";
import { ConfirmModal } from "../components/modal/ConfirmModal";
import { GameImportModal } from "../components/modal/GameImportModal";
import { LibrarySkeleton } from "../components/skeleton/LibrarySkeleton";
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
  const [showSkeleton, setShowSkeleton] = useState(false);
  const [isAddGameModalOpen, setIsAddGameModalOpen] = useState(false);
  const [isBatchImportOpen, setIsBatchImportOpen] = useState(false);
  const [importSource, setImportSource] = useState<ImportSource | null>(null);
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
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
    onConfirm: () => {},
  });
  const dropdownRef = useRef<HTMLDivElement>(null);

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

  // 点击外部关闭下拉菜单
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsDropdownOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const filteredGames = games
    .filter((game) => {
      // 搜索过滤
      if (searchQuery && !game.name.toLowerCase().includes(searchQuery.toLowerCase())) {
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
      toast.error("加载收藏夹失败");
    }
  };

  const handleBatchAddToCategory = async (categoryIds: string[]) => {
    if (selectedGameIds.length === 0 || categoryIds.length === 0)
      return;
    try {
      await AddGamesToCategories(selectedGameIds, categoryIds);
      toast.success(`已添加 ${selectedGameIds.length} 个游戏到收藏`);
      setSelectedGameIds([]);
      setBatchMode(false);
    }
    catch (error) {
      console.error("Failed to batch add games to category:", error);
      toast.error("批量添加失败");
    }
  };

  const handleBatchDelete = () => {
    if (selectedGameIds.length === 0)
      return;
    setConfirmConfig({
      isOpen: true,
      title: "批量删除游戏",
      message: `确定要删除选中的 ${selectedGameIds.length} 个游戏吗？此操作将从库中移除这些游戏，但不会删除本地游戏文件。`,
      type: "danger",
      onConfirm: async () => {
        try {
          await DeleteGames(selectedGameIds);
          await fetchGames();
          setSelectedGameIds([]);
          setBatchMode(false);
          toast.success("批量删除成功");
        }
        catch (error) {
          console.error("Failed to batch delete games:", error);
          toast.error("批量删除失败");
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
        <h1 className="text-4xl font-bold text-brand-900 dark:text-white">游戏库</h1>
      </div>

      <FilterBar
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        searchPlaceholder="搜索游戏..."
        sortBy={sortBy}
        onSortByChange={val => setSortBy(val as "name" | "created_at")}
        sortOptions={sortOptions}
        sortOrder={sortOrder}
        onSortOrderChange={setSortOrder}
        statusFilter={statusFilter}
        onStatusFilterChange={setStatusFilter}
        statusOptions={statusOptions}
        storageKey="library"
        batchMode={batchMode}
        onBatchModeChange={handleBatchModeChange}
        selectedCount={selectedGameIds.length}
        onSelectAll={handleSelectAll}
        onClearSelection={handleClearSelection}
        batchActions={(
          <>
            <button
              type="button"
              onClick={openBatchAddModal}
              disabled={selectedGameIds.length === 0}
              className={`glass-panel flex items-center gap-2 px-3 py-2 text-sm
                          bg-white dark:bg-brand-800 border border-brand-200 dark:border-brand-700
                          rounded-lg hover:bg-brand-100 dark:hover:bg-brand-700 text-brand-700 dark:text-brand-300
                          ${selectedGameIds.length === 0 ? "opacity-50 cursor-not-allowed" : ""}`}
            >
              <div className="i-mdi-folder-plus-outline text-lg" />
            </button>
            <button
              type="button"
              onClick={handleBatchDelete}
              disabled={selectedGameIds.length === 0}
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
          <div className="relative" ref={dropdownRef}>
            <button
              onClick={() => setIsDropdownOpen(!isDropdownOpen)}
              className="glass-btn-neutral flex items-center rounded-lg bg-neutral-600 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-700 focus:outline-none focus:ring-4 focus:ring-neutral-300 dark:bg-neutral-600 dark:hover:bg-neutral-700 dark:focus:ring-neutral-800"
            >
              <div className="i-mdi-plus mr-2 text-lg" />
              添加游戏
              <div className="i-mdi-chevron-down ml-2 text-lg" />
            </button>

            {/* Dropdown Menu */}
            {isDropdownOpen && (
              <div className="absolute right-0 mt-2 w-56 origin-top-right rounded-lg bg-white shadow-lg ring-1 ring-black/5 dark:bg-brand-700 dark:ring-white/10 z-50">
                <div className="py-1">
                  <button
                    onClick={() => {
                      setIsAddGameModalOpen(true);
                      setIsDropdownOpen(false);
                    }}
                    className="flex w-full items-center px-4 py-3 text-sm text-brand-700 hover:bg-brand-100 dark:text-brand-200 dark:hover:bg-brand-600"
                  >
                    <div className="i-mdi-gamepad-variant mr-3 text-xl text-neutral-500" />
                    <div className="text-left">
                      <div className="font-medium">手动添加</div>
                      <div className="text-xs text-brand-400 dark:text-brand-400">
                        选择可执行文件并搜索元数据
                      </div>
                    </div>
                  </button>
                  <button
                    onClick={() => {
                      setIsBatchImportOpen(true);
                      setIsDropdownOpen(false);
                    }}
                    className="flex w-full items-center px-4 py-3 text-sm text-brand-700 hover:bg-brand-100 dark:text-brand-200 dark:hover:bg-brand-600"
                  >
                    <div className="i-mdi-folder-multiple mr-3 text-xl text-success-500" />
                    <div className="text-left">
                      <div className="font-medium">批量导入</div>
                      <div className="text-xs text-brand-400 dark:text-brand-400">
                        扫描游戏库目录批量添加
                      </div>
                    </div>
                  </button>
                  <div className="border-t border-brand-200 dark:border-brand-600 my-1" />
                  <button
                    onClick={() => {
                      setImportSource("potatovn");
                      setIsDropdownOpen(false);
                    }}
                    className="flex w-full items-center px-4 py-3 text-sm text-brand-700 hover:bg-brand-100 dark:text-brand-200 dark:hover:bg-brand-600"
                  >
                    <div className="i-mdi-database-import mr-3 text-xl text-orange-500" />
                    <div className="text-left">
                      <div className="font-medium">从 PotatoVN 导入</div>
                      <div className="text-xs text-brand-400 dark:text-brand-400">
                        导入 PotatoVN 导出的 ZIP 文件
                      </div>
                    </div>
                  </button>
                  <button
                    onClick={() => {
                      setImportSource("playnite");
                      setIsDropdownOpen(false);
                    }}
                    className="flex w-full items-center px-4 py-3 text-sm text-brand-700 hover:bg-brand-100 dark:text-brand-200 dark:hover:bg-brand-600"
                  >
                    <div className="i-mdi-application-import mr-3 text-xl text-purple-500" />
                    <div className="text-left">
                      <div className="font-medium">从 Playnite 导入</div>
                      <div className="text-xs text-brand-400 dark:text-brand-400">
                        导入 Playnite 导出的 JSON 文件
                      </div>
                    </div>
                  </button>
                </div>
              </div>
            )}
          </div>
        )}
      />

      {games.length === 0
        ? (
            <div className="flex-1 flex items-center justify-center w-full">
              <div className="flex flex-col items-center justify-center py-20 text-brand-500 dark:text-brand-400">
                <div className="i-mdi-gamepad-variant-outline text-6xl mb-4" />
                <p className="text-xl">暂无游戏</p>
                <p className="text-sm mt-2">添加一些游戏开始吧</p>
                <div className="flex flex-col gap-3 mt-4">
                  <button
                    onClick={() => setImportSource("potatovn")}
                    className="rounded-lg border border-success-600 px-5 py-2.5 text-sm font-medium text-success-600 hover:bg-success-50 focus:outline-none focus:ring-4 focus:ring-success-300 dark:border-success-500 dark:text-success-500 dark:hover:bg-success-900/20"
                  >
                    从 PotatoVN 导入
                  </button>
                  <button
                    onClick={() => setImportSource("playnite")}
                    className="rounded-lg border border-purple-600 px-5 py-2.5 text-sm font-medium text-purple-600 hover:bg-purple-50 focus:outline-none focus:ring-4 focus:ring-purple-300 dark:border-purple-500 dark:text-purple-500 dark:hover:bg-purple-900/20"
                  >
                    从 Playnite 导入
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
                  <p>未找到匹配的游戏</p>
                </div>
              </div>
            )
          : (
              <div className="grid grid-cols-[repeat(auto-fill,minmax(max(8rem,11%),1fr))] gap-3">
                {filteredGames.map(game => (
                  <GameCard
                    key={game.id}
                    game={game}
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
        title="批量添加到收藏"
        confirmText="添加"
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
