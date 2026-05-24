import type { models, vo } from "../../wailsjs/go/models";
import { createRoute, useNavigate } from "@tanstack/react-router";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  AddGameToCategory,
  GetCategoryByID,
  GetCategoryGames,
  RemoveGameFromCategory,
  RemoveGamesFromCategory,
  SearchCategoryGameCandidates,
} from "../../wailsjs/go/service/CategoryService";
import { FilterBar } from "../components/bar/FilterBar";
import { TagFilterMenu } from "../components/bar/TagFilterMenu";
import { VirtualGameGrid } from "../components/grid/VirtualGameGrid";
import { AddGameToCategoryModal } from "../components/modal/AddGameToCategoryModal";
import { CategorySkeleton } from "../components/skeleton/CategorySkeleton";
import { sortOptions, statusOptions } from "../consts/options";
import { useDebouncedValue } from "../hooks/useDebouncedValue";
import { useTagGameFilter } from "../hooks/useTagGameFilter";
import { Route as rootRoute } from "./__root";

type CategorySortBy
  = | "name"
    | "last_played_at"
    | "created_at"
    | "rating"
    | "release_date";

const CATEGORY_STORAGE_KEY = "category";
const PAGE_SIZE = 120;
const CANDIDATE_PAGE_SIZE = 80;
const CATEGORY_SORT_BY_VALUES = new Set<CategorySortBy>([
  "name",
  "last_played_at",
  "created_at",
  "rating",
  "release_date",
]);
const CATEGORY_STATUS_VALUES = new Set(
  statusOptions.map(option => option.value),
);

function readStoredValue(key: string) {
  if (typeof window === "undefined") {
    return null;
  }
  return window.localStorage.getItem(key);
}

function readStoredCategorySortBy() {
  const savedSortBy = readStoredValue(`${CATEGORY_STORAGE_KEY}_sortBy`);
  if (
    savedSortBy
    && CATEGORY_SORT_BY_VALUES.has(savedSortBy as CategorySortBy)
  ) {
    return savedSortBy as CategorySortBy;
  }
  return "created_at";
}

function readStoredCategorySortOrder() {
  const savedSortOrder = readStoredValue(`${CATEGORY_STORAGE_KEY}_sortOrder`);
  return savedSortOrder === "asc" || savedSortOrder === "desc"
    ? savedSortOrder
    : "desc";
}

function readStoredCategorySearchQuery() {
  return readStoredValue(`${CATEGORY_STORAGE_KEY}_searchQuery`) || "";
}

function readStoredCategoryStatusFilter() {
  const savedStatusFilter = readStoredValue(
    `${CATEGORY_STORAGE_KEY}_statusFilter`,
  );
  return savedStatusFilter && CATEGORY_STATUS_VALUES.has(savedStatusFilter)
    ? savedStatusFilter
    : "";
}

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/categories/$categoryId",
  component: CategoryDetailPage,
});

function CategoryDetailPage() {
  const navigate = useNavigate();
  const { categoryId } = Route.useParams();
  const { t } = useTranslation();
  const [category, setCategory] = useState<vo.CategoryVO | null>(null);
  const [games, setGames] = useState<models.Game[]>([]);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const requestIdRef = useRef(0);
  const [loading, setLoading] = useState(true);
  const [showSkeleton, setShowSkeleton] = useState(false);
  const [isAddGameModalOpen, setIsAddGameModalOpen] = useState(false);
  const [allGames, setAllGames] = useState<models.Game[]>([]);
  const [candidateSearchQuery, setCandidateSearchQuery] = useState("");
  const [candidateHasMore, setCandidateHasMore] = useState(false);
  const [candidateLoading, setCandidateLoading] = useState(false);
  const candidateRequestIdRef = useRef(0);
  const [searchQuery, setSearchQuery] = useState(() =>
    readStoredCategorySearchQuery(),
  );
  const [sortBy, setSortBy] = useState<CategorySortBy>(() =>
    readStoredCategorySortBy(),
  );
  const [sortOrder, setSortOrder] = useState<"asc" | "desc">(() =>
    readStoredCategorySortOrder(),
  );
  const [statusFilter, setStatusFilter] = useState<string>(() =>
    readStoredCategoryStatusFilter(),
  );
  const debouncedSearchQuery = useDebouncedValue(searchQuery, 250);
  const debouncedCandidateSearchQuery = useDebouncedValue(
    candidateSearchQuery,
    250,
  );
  const [batchMode, setBatchMode] = useState(false);
  const [selectedGameIds, setSelectedGameIds] = useState<string[]>([]);
  const {
    selectedTags,
    tagInput,
    setTagInput,
    tagSuggestions,
    selectTag,
    removeTag,
    clearTagFilter,
  } = useTagGameFilter();

  useEffect(() => {
    let timer: number;
    if (loading) {
      timer = window.setTimeout(() => {
        setShowSkeleton(true);
      }, 300);
    }
    else {
      setShowSkeleton(false);
    }
    return () => clearTimeout(timer);
  }, [loading]);

  const loadCategory = async (id: string) => {
    try {
      const result = await GetCategoryByID(id);
      setCategory(result);
    }
    catch (error) {
      console.error("Failed to load category:", error);
      toast.error(t("category.toast.loadCategoryFailed"));
    }
  };

  const queryParams = useMemo(
    () => ({
      search_query: debouncedSearchQuery.trim(),
      status: statusFilter,
      tags: selectedTags,
      sort_by: sortBy,
      sort_order: sortOrder,
    }),
    [debouncedSearchQuery, selectedTags, sortBy, sortOrder, statusFilter],
  );

  const loadGames = useCallback(
    async (id: string, offset = 0, mode: "replace" | "append" = "replace") => {
      const requestId = ++requestIdRef.current;
      if (mode === "replace") {
        setLoading(true);
        setGames([]);
        setHasMore(false);
      }
      else {
        setLoadingMore(true);
      }
      try {
        const result = await GetCategoryGames({
          category_id: id,
          limit: PAGE_SIZE,
          offset,
          ...queryParams,
        });
        if (requestId !== requestIdRef.current) {
          return;
        }
        setTotal(result.total || 0);
        setHasMore(Boolean(result.has_more));
        setGames(previous =>
          mode === "append"
            ? [...previous, ...(result.games || [])]
            : result.games || [],
        );
      }
      catch (error) {
        if (requestId === requestIdRef.current) {
          console.error("Failed to load games for category:", error);
          toast.error(t("category.toast.loadGamesFailed"));
        }
      }
      finally {
        if (requestId === requestIdRef.current) {
          setLoading(false);
          setLoadingMore(false);
        }
      }
    },
    [queryParams, t],
  );

  const loadNextGames = useCallback(() => {
    if (!category || !hasMore || loading || loadingMore) {
      return;
    }
    void loadGames(category.id, games.length, "append");
  }, [category, games.length, hasMore, loadGames, loading, loadingMore]);

  const onBack = () => {
    navigate({ to: "/categories" });
  };

  const handleRemoveGame = async (gameId: string) => {
    if (!category)
      return;
    try {
      await RemoveGameFromCategory(gameId, category.id);
      await loadGames(category.id);
      await loadCategory(category.id);
    }
    catch (error) {
      console.error("Failed to remove game from category:", error);
      toast.error(t("category.toast.removeGameFailed"));
    }
  };

  const openAddGameModal = async () => {
    setAllGames([]);
    setCandidateSearchQuery("");
    setCandidateHasMore(false);
    setIsAddGameModalOpen(true);
  };

  const loadCandidates = useCallback(
    async (offset = 0, mode: "replace" | "append" = "replace") => {
      if (!category) {
        return;
      }
      const requestId = ++candidateRequestIdRef.current;
      setCandidateLoading(true);
      try {
        const result = await SearchCategoryGameCandidates({
          category_id: category.id,
          limit: CANDIDATE_PAGE_SIZE,
          offset,
          search_query: debouncedCandidateSearchQuery.trim(),
        });
        if (requestId !== candidateRequestIdRef.current) {
          return;
        }
        setCandidateHasMore(Boolean(result.has_more));
        setAllGames(previous =>
          mode === "append"
            ? [...previous, ...(result.games || [])]
            : result.games || [],
        );
      }
      catch (error) {
        if (requestId === candidateRequestIdRef.current) {
          console.error("Failed to load candidate games:", error);
          toast.error(t("category.toast.loadAllGamesFailed"));
        }
      }
      finally {
        if (requestId === candidateRequestIdRef.current) {
          setCandidateLoading(false);
        }
      }
    },
    [category, debouncedCandidateSearchQuery, t],
  );

  const handleAddGameToCategory = async (gameId: string) => {
    if (!category)
      return;
    try {
      await AddGameToCategory(gameId, category.id);
      setAllGames(prev => prev.filter(g => g.id !== gameId));
      await loadGames(category.id);
      await loadCategory(category.id);
    }
    catch (error) {
      console.error("Failed to add game to category:", error);
      toast.error(t("category.toast.addGameFailed"));
    }
  };

  const statusFilterLabel = statusFilter
    ? t(
        statusOptions.find(option => option.value === statusFilter)?.label
        || "",
      )
    : "";
  const gameCountText = statusFilterLabel
    ? t("category.filteredGameCount", {
        count: total,
        status: statusFilterLabel,
      })
    : t("category.gameCount", { count: total });

  const selectedGameIdSet = useMemo(
    () => new Set(selectedGameIds),
    [selectedGameIds],
  );

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
      games.forEach((game) => {
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

  const handleBatchRemove = async () => {
    if (!category || selectedGameIds.length === 0)
      return;
    try {
      await RemoveGamesFromCategory(selectedGameIds, category.id);
      await Promise.all([loadGames(category.id), loadCategory(category.id)]);
      toast.success(
        t("category.toast.batchRemoveSuccess", {
          count: selectedGameIds.length,
        }),
      );
      setSelectedGameIds([]);
      setBatchMode(false);
    }
    catch (error) {
      console.error("Failed to batch remove games:", error);
      toast.error(t("category.toast.batchRemoveFailed"));
    }
  };

  useEffect(() => {
    if (categoryId) {
      const init = async () => {
        setBatchMode(false);
        setSelectedGameIds([]);
        await Promise.all([loadCategory(categoryId), loadGames(categoryId)]);
      };
      init();
    }
  }, [categoryId, loadGames]);

  useEffect(() => {
    if (isAddGameModalOpen) {
      void loadCandidates(0, "replace");
    }
  }, [isAddGameModalOpen, loadCandidates]);

  if (loading && !category) {
    if (!showSkeleton) {
      return null;
    }
    return <CategorySkeleton />;
  }

  if (!category) {
    return (
      <div className="flex flex-col items-center justify-center h-full space-y-4 text-brand-500">
        <div className="i-mdi-alert-circle-outline text-6xl" />
        <p className="text-xl">{t("category.notFound")}</p>
        <button
          type="button"
          onClick={onBack}
          className="text-neutral-600 hover:underline"
        >
          {t("category.backToList")}
        </button>
      </div>
    );
  }

  return (
    <div
      className={`h-full w-full overflow-y-auto p-8 transition-opacity duration-300 ${loading ? "opacity-50 pointer-events-none" : "opacity-100"}`}
    >
      {/* Back Button */}
      <button
        type="button"
        onClick={onBack}
        className="flex rounded-md items-center text-brand-600 hover:text-brand-900 dark:text-brand-400 dark:hover:text-brand-200 transition-colors mb-6"
      >
        <div className="i-mdi-arrow-left text-2xl mr-1" />
        <span>{t("category.back")}</span>
      </button>

      <div className="flex flex-col gap-6">
        <div className="flex justify-between items-center">
          <div>
            <h1 className="text-4xl font-bold text-brand-900 dark:text-white flex items-center gap-3">
              {(category.emoji || "").trim() && (
                <span className="text-3xl leading-none">{category.emoji}</span>
              )}
              {category.name}
              {category.is_system && (
                <span className="text-sm bg-neutral-100 text-neutral-800 px-2 py-1 rounded-md dark:bg-neutral-900 dark:text-neutral-300 align-middle">
                  {t("category.systemTag")}
                </span>
              )}
            </h1>
            <p className="text-brand-500 dark:text-brand-400 mt-2">
              {gameCountText}
            </p>
          </div>
        </div>

        <FilterBar
          searchQuery={searchQuery}
          onSearchChange={setSearchQuery}
          searchPlaceholder={t("library.searchPlaceholder")}
          sortBy={sortBy}
          onSortByChange={val => setSortBy(val as CategorySortBy)}
          sortOptions={sortOptions.map(opt => ({
            ...opt,
            label: t(opt.label),
          }))}
          sortOrder={sortOrder}
          onSortOrderChange={setSortOrder}
          statusFilter={statusFilter}
          onStatusFilterChange={setStatusFilter}
          statusOptions={statusOptions.map(opt => ({
            ...opt,
            label: t(opt.label),
          }))}
          storageKey="category"
          batchMode={batchMode}
          onBatchModeChange={handleBatchModeChange}
          selectedCount={selectedGameIds.length}
          onSelectAll={handleSelectAll}
          onClearSelection={handleClearSelection}
          filterMenuExtraActive={selectedTags.length > 0 || Boolean(tagInput)}
          filterMenuExtra={(
            <TagFilterMenu
              selectedTags={selectedTags}
              tagInput={tagInput}
              tagSuggestions={tagSuggestions}
              onTagInputChange={setTagInput}
              onSelectTag={selectTag}
              onRemoveTag={removeTag}
              onClearTagFilter={clearTagFilter}
            />
          )}
          batchActions={(
            <button
              type="button"
              onClick={handleBatchRemove}
              disabled={selectedGameIds.length === 0}
              className={`glass-panel flex items-center gap-2 px-3 py-2 text-sm
                          bg-white dark:bg-brand-800 border border-brand-200 dark:border-brand-700
                          rounded-lg hover:bg-brand-100 dark:hover:bg-brand-700 text-error-600 dark:text-error-400
                          ${selectedGameIds.length === 0 ? "opacity-50 cursor-not-allowed" : ""}`}
            >
              <div className="i-mdi-delete text-lg" />
              {t("category.batchRemoveBtn")}
            </button>
          )}
          actionButton={(
            <button
              type="button"
              onClick={openAddGameModal}
              className="glass-btn-neutral flex items-center rounded-lg bg-neutral-600 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-700 focus:outline-none focus:ring-4 focus:ring-neutral-300 dark:bg-neutral-600 dark:hover:bg-neutral-700 dark:focus:ring-neutral-800"
            >
              <div className="i-mdi-plus mr-2 text-lg" />
              {t("category.addGameBtn")}
            </button>
          )}
        />
      </div>

      <div className="mt-6">
        {games.length > 0 ? (
          <>
            <VirtualGameGrid
              games={games}
              searchQuery={debouncedSearchQuery}
              selectionMode={batchMode}
              selectedGameIds={selectedGameIdSet}
              onSelectChange={setGameSelection}
              onNearEnd={loadNextGames}
              renderOverlay={game =>
                !batchMode && (
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleRemoveGame(game.id);
                    }}
                    className="absolute top-2 right-2 p-1 bg-error-500 text-white rounded-full opacity-0 group-hover:opacity-100 transition-opacity shadow-md hover:bg-error-600"
                    title={t("category.removeFromCategory")}
                  >
                    <div className="i-mdi-close text-sm" />
                  </button>
                )}
            />
            {loadingMore && (
              <div className="flex justify-center py-3 text-sm text-brand-500 dark:text-brand-400">
                <div className="i-mdi-loading animate-spin mr-2" />
                {t("common.loading", "加载中...")}
              </div>
            )}
          </>
        ) : total > 0 ? (
          <div className="flex flex-col items-center justify-center h-64 text-brand-500 dark:text-brand-400">
            <div className="i-mdi-magnify text-6xl mb-4" />
            <p className="text-lg">{t("category.noMatchingGames")}</p>
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center h-64 text-brand-500 dark:text-brand-400">
            <div className="i-mdi-gamepad-variant-outline text-6xl mb-4" />
            <p className="text-lg">{t("category.emptyCategory")}</p>
            <button
              type="button"
              onClick={openAddGameModal}
              className="mt-4 text-neutral-600 hover:underline dark:text-neutral-400"
            >
              {t("category.addFirstGame")}
            </button>
          </div>
        )}
      </div>

      <AddGameToCategoryModal
        isOpen={isAddGameModalOpen}
        allGames={allGames}
        loading={candidateLoading}
        hasMore={candidateHasMore}
        onSearchChange={setCandidateSearchQuery}
        onLoadMore={() => loadCandidates(allGames.length, "append")}
        onClose={() => setIsAddGameModalOpen(false)}
        onAddGame={handleAddGameToCategory}
      />
    </div>
  );
}
