import type { models, vo } from "../../wailsjs/go/models";
import type { GameStatusFilter } from "../consts/options";
import { createRoute, useNavigate } from "@tanstack/react-router";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { enums } from "../../wailsjs/go/models";
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
import { ScrollToTopButton } from "../components/ui/ScrollToTopButton";
import { sortOptions, statusOptions } from "../consts/options";
import { useDebouncedValue } from "../hooks/useDebouncedValue";
import { usePageScrollControls } from "../hooks/usePageScrollControls";
import { useTagGameFilter } from "../hooks/useTagGameFilter";
import { useAppStore } from "../store";
import { Route as rootRoute } from "./__root";

const CATEGORY_STORAGE_KEY = "category";
const PAGE_SIZE = 120;
const CANDIDATE_PAGE_SIZE = 80;
const WINDOW_BUFFER_SIZE = PAGE_SIZE;
const WINDOW_REQUEST_SIZE = PAGE_SIZE * 2;
const WINDOW_KEEP_RADIUS = PAGE_SIZE * 4;
const CATEGORY_SORT_BY_VALUES = new Set<enums.GameListSortBy>([
  enums.GameListSortBy.NAME,
  enums.GameListSortBy.LAST_PLAYED_AT,
  enums.GameListSortBy.CREATED_AT,
  enums.GameListSortBy.RATING,
  enums.GameListSortBy.RELEASE_DATE,
]);
const CATEGORY_STATUS_VALUES = new Set(
  statusOptions.map(option => option.value),
);

interface CategoryGameListMetaCacheEntry {
  total: number;
}

const categoryGameListMetaCache = new Map<
  string,
  CategoryGameListMetaCacheEntry
>();

function getWindowRequest(startIndex: number, endIndex: number, total: number) {
  const bufferedStart = Math.max(0, startIndex - WINDOW_BUFFER_SIZE);
  const offset = Math.floor(bufferedStart / PAGE_SIZE) * PAGE_SIZE;
  const requestedEnd = Math.min(
    total,
    offset + WINDOW_REQUEST_SIZE,
    Math.max(endIndex + 1, offset + PAGE_SIZE),
  );
  return {
    limit: Math.max(1, requestedEnd - offset),
    offset,
  };
}

function isIndexedWindowLoaded(
  gamesByIndex: ReadonlyMap<number, models.Game>,
  offset: number,
  limit: number,
  total: number,
) {
  const end = Math.min(total, offset + limit);
  for (let index = offset; index < end; index++) {
    if (!gamesByIndex.has(index)) {
      return false;
    }
  }
  return end > offset;
}

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
    && CATEGORY_SORT_BY_VALUES.has(savedSortBy as enums.GameListSortBy)
  ) {
    return savedSortBy as enums.GameListSortBy;
  }
  return enums.GameListSortBy.CREATED_AT;
}

function readStoredCategorySortOrder() {
  const savedSortOrder = readStoredValue(`${CATEGORY_STORAGE_KEY}_sortOrder`);
  return savedSortOrder === enums.SortOrder.ASC
    || savedSortOrder === enums.SortOrder.DESC
    ? (savedSortOrder as enums.SortOrder)
    : enums.SortOrder.DESC;
}

function readStoredCategorySearchQuery() {
  return readStoredValue(`${CATEGORY_STORAGE_KEY}_searchQuery`) || "";
}

function readStoredCategoryStatusFilter() {
  const savedStatusFilter = readStoredValue(
    `${CATEGORY_STORAGE_KEY}_statusFilter`,
  ) as GameStatusFilter | null;
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
  const scrollRestorationId = `category-${categoryId}-scroll`;
  const pageRef = useRef<HTMLDivElement | null>(null);
  const toolbarRef = useRef<HTMLDivElement | null>(null);
  const { t } = useTranslation();
  const [category, setCategory] = useState<vo.CategoryVO | null>(null);
  const loadedCategoryIdRef = useRef<string | null>(null);
  const [gamesByIndex, setGamesByIndex] = useState<Map<number, models.Game>>(
    () => new Map(),
  );
  const [total, setTotal] = useState(0);
  const [loadingMore, setLoadingMore] = useState(false);
  const currentQueryKeyRef = useRef("");
  const gamesByIndexRef = useRef<ReadonlyMap<number, models.Game>>(new Map());
  const loadingWindowsRef = useRef(new Set<string>());
  const totalRef = useRef(0);
  const [loading, setLoading] = useState(true);
  const [loadedQueryKey, setLoadedQueryKey] = useState("");
  const [showSkeleton, setShowSkeleton] = useState(false);
  const [isAddGameModalOpen, setIsAddGameModalOpen] = useState(false);
  const [allGames, setAllGames] = useState<models.Game[]>([]);
  const [candidateSearchQuery, setCandidateSearchQuery] = useState("");
  const [candidateHasMore, setCandidateHasMore] = useState(false);
  const [candidateLoading, setCandidateLoading] = useState(false);
  const candidateRequestIdRef = useRef(0);
  const [visibleRange, setVisibleRange] = useState<{
    endIndex: number;
    startIndex: number;
  } | null>(null);
  const [searchQuery, setSearchQuery] = useState(() =>
    readStoredCategorySearchQuery(),
  );
  const [sortBy, setSortBy] = useState<enums.GameListSortBy>(() =>
    readStoredCategorySortBy(),
  );
  const [sortOrder, setSortOrder] = useState<enums.SortOrder>(() =>
    readStoredCategorySortOrder(),
  );
  const [statusFilter, setStatusFilter] = useState<GameStatusFilter>(() =>
    readStoredCategoryStatusFilter(),
  );
  const debouncedSearchQuery = useDebouncedValue(searchQuery, 250);
  const debouncedCandidateSearchQuery = useDebouncedValue(
    candidateSearchQuery,
    250,
  );
  const [batchMode, setBatchMode] = useState(false);
  const [selectedGameIds, setSelectedGameIds] = useState<string[]>([]);
  const enableTagTranslation = useAppStore(
    state => state.config?.enable_tag_translation ?? true,
  );
  const {
    selectedTags,
    tagInput,
    setTagInput,
    tagSuggestions,
    selectTag,
    removeTag,
    clearTagFilter,
  } = useTagGameFilter({ enableTagTranslation });
  const loadedGames = useMemo(
    () => Array.from(gamesByIndex.values()),
    [gamesByIndex],
  );
  const loadedGameCount = gamesByIndex.size;
  const isPageReady = !(loading && !category && loadedGameCount === 0);

  useEffect(() => {
    gamesByIndexRef.current = gamesByIndex;
  }, [gamesByIndex]);

  useEffect(() => {
    totalRef.current = total;
  }, [total]);

  const { scrollToTop, showScrollTop } = usePageScrollControls({
    anchorRef: pageRef,
    enabled: isPageReady,
    toolbarRef,
  });

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
      loadedCategoryIdRef.current = id;
    }
    catch (error) {
      console.error("Failed to load category:", error);
      toast.error(t("category.toast.loadCategoryFailed"));
    }
  };

  const queryParams = useMemo(
    () => ({
      search_query: debouncedSearchQuery.trim(),
      ...(statusFilter ? { status: statusFilter } : {}),
      tags: selectedTags,
      sort_by: sortBy,
      sort_order: sortOrder,
    }),
    [debouncedSearchQuery, selectedTags, sortBy, sortOrder, statusFilter],
  );
  const queryKey = useMemo(
    () => JSON.stringify({ categoryId, ...queryParams }),
    [categoryId, queryParams],
  );
  const isSearchSettling = searchQuery.trim() !== debouncedSearchQuery.trim();
  const hasActiveGameFilters
    = debouncedSearchQuery.trim().length > 0
      || selectedTags.length > 0
      || Boolean(statusFilter);
  const isEmptyListWaiting
    = total === 0 && (loading || isSearchSettling || loadedQueryKey !== queryKey);

  const loadGamesWindow = useCallback(
    async (
      id: string,
      offset: number,
      limit: number,
      options: { force?: boolean; reset?: boolean } = {},
    ) => {
      const requestKey = `${queryKey}:${offset}:${limit}`;
      if (!options.force) {
        if (loadingWindowsRef.current.has(requestKey)) {
          return;
        }
        if (
          totalRef.current > 0
          && isIndexedWindowLoaded(
            gamesByIndexRef.current,
            offset,
            limit,
            totalRef.current,
          )
        ) {
          return;
        }
      }

      loadingWindowsRef.current.add(requestKey);
      if (options.reset) {
        setLoading(true);
      }
      else {
        setLoadingMore(true);
      }

      try {
        const result = await GetCategoryGames({
          category_id: id,
          limit,
          offset,
          ...queryParams,
        } as vo.CategoryGameListRequest);
        if (currentQueryKeyRef.current !== queryKey) {
          return;
        }

        const nextTotal = result.total || 0;
        setTotal(nextTotal);
        setGamesByIndex((previous) => {
          const next = options.reset
            ? new Map<number, models.Game>()
            : new Map(previous);
          const keepStart = Math.max(0, offset - WINDOW_KEEP_RADIUS);
          const keepEnd = offset + limit + WINDOW_KEEP_RADIUS;
          for (const index of next.keys()) {
            if (index < keepStart || index > keepEnd) {
              next.delete(index);
            }
          }
          (result.games || []).forEach((game, index) => {
            next.set(offset + index, game);
          });
          return next;
        });
        categoryGameListMetaCache.set(queryKey, { total: nextTotal });
        setLoadedQueryKey(queryKey);
      }
      catch (error) {
        if (currentQueryKeyRef.current === queryKey) {
          console.error("Failed to load games for category:", error);
          toast.error(t("category.toast.loadGamesFailed"));
        }
      }
      finally {
        loadingWindowsRef.current.delete(requestKey);
        if (currentQueryKeyRef.current === queryKey) {
          setLoading(false);
          setLoadingMore(loadingWindowsRef.current.size > 0);
        }
      }
    },
    [queryKey, queryParams, t],
  );

  const refreshFirstWindow = useCallback(() => {
    if (!category) {
      return;
    }
    loadingWindowsRef.current.clear();
    setGamesByIndex(new Map());
    setTotal(0);
    setLoadedQueryKey("");
    void loadGamesWindow(category.id, 0, PAGE_SIZE, {
      force: true,
      reset: true,
    });
  }, [category, loadGamesWindow]);

  const handleVisibleRangeChange = useCallback(
    (startIndex: number, endIndex: number) => {
      setVisibleRange((previous) => {
        if (
          previous?.startIndex === startIndex
          && previous.endIndex === endIndex
        ) {
          return previous;
        }
        return { endIndex, startIndex };
      });
    },
    [],
  );

  useEffect(() => {
    if (!category || !visibleRange || total <= 0) {
      return;
    }

    const endIndex = Math.min(visibleRange.endIndex, total - 1);
    for (let index = visibleRange.startIndex; index <= endIndex; index++) {
      if (!gamesByIndex.has(index)) {
        const request = getWindowRequest(index, endIndex, total);
        void loadGamesWindow(category.id, request.offset, request.limit);
        return;
      }
    }
  }, [category, gamesByIndex, loadGamesWindow, total, visibleRange]);

  const onBack = () => {
    navigate({ to: "/categories" });
  };

  const handleRemoveGame = async (gameId: string) => {
    if (!category)
      return;
    try {
      await RemoveGameFromCategory(gameId, category.id);
      categoryGameListMetaCache.clear();
      refreshFirstWindow();
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
      categoryGameListMetaCache.clear();
      refreshFirstWindow();
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
      loadedGames.forEach((game) => {
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
      categoryGameListMetaCache.clear();
      refreshFirstWindow();
      await loadCategory(category.id);
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
        const shouldLoadCategory = loadedCategoryIdRef.current !== categoryId;
        currentQueryKeyRef.current = queryKey;
        loadingWindowsRef.current.clear();
        if (shouldLoadCategory) {
          setCategory(null);
        }
        setGamesByIndex(new Map());
        setVisibleRange(null);
        setBatchMode(false);
        setSelectedGameIds([]);
        if (shouldLoadCategory) {
          setLoading(true);
        }
        setLoadingMore(false);

        const cached = categoryGameListMetaCache.get(queryKey);
        if (cached) {
          setTotal(cached.total);
          setLoadedQueryKey(queryKey);
          if (shouldLoadCategory) {
            await loadCategory(categoryId);
          }
          setLoading(false);
          return;
        }

        setTotal(0);
        setLoadedQueryKey("");
        if (shouldLoadCategory) {
          await loadCategory(categoryId);
        }
        await loadGamesWindow(categoryId, 0, PAGE_SIZE, {
          force: true,
          reset: true,
        });
      };
      init();
    }
  }, [categoryId, loadGamesWindow, queryKey]);

  useEffect(() => {
    currentQueryKeyRef.current = queryKey;
  }, [queryKey]);

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
      ref={pageRef}
      data-scroll-restoration-id={scrollRestorationId}
      className="h-full w-full overflow-y-auto scrollbar-stable p-8"
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

        <div ref={toolbarRef}>
          <FilterBar
            searchQuery={searchQuery}
            onSearchChange={setSearchQuery}
            searchPlaceholder={t("library.searchPlaceholder")}
            sortBy={sortBy}
            onSortByChange={val => setSortBy(val as enums.GameListSortBy)}
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
                enableTagTranslation={enableTagTranslation}
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
      </div>

      <div className="mt-6">
        {isEmptyListWaiting ? (
          <div className="flex flex-col items-center justify-center h-64 text-brand-500 dark:text-brand-400">
            <div className="i-mdi-loading animate-spin text-6xl mb-4" />
            <p className="text-lg">{t("common.loading", "加载中...")}</p>
          </div>
        ) : total > 0 ? (
          <div className="relative">
            <div
              className={`transition-opacity duration-200 ${
                loading ? "pointer-events-none opacity-60" : "opacity-100"
              }`}
            >
              <VirtualGameGrid
                gamesByIndex={gamesByIndex}
                scrollRestorationId={scrollRestorationId}
                totalItems={total}
                searchQuery={debouncedSearchQuery}
                selectionMode={batchMode}
                selectedGameIds={selectedGameIdSet}
                onSelectChange={setGameSelection}
                onVisibleRangeChange={handleVisibleRangeChange}
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
            </div>
            {loading && (
              <div className="pointer-events-none absolute inset-x-0 top-0 z-10 flex justify-center py-3 text-sm text-brand-600 dark:text-brand-300">
                <div className="glass-panel flex items-center rounded-full border border-brand-200/70 bg-white/85 px-3 py-1.5 shadow-sm backdrop-blur dark:border-brand-700/70 dark:bg-brand-900/75">
                  <div className="i-mdi-loading animate-spin mr-2" />
                  {t("common.loading", "加载中...")}
                </div>
              </div>
            )}
            {loadingMore && (
              <div className="flex justify-center py-3 text-sm text-brand-500 dark:text-brand-400">
                <div className="i-mdi-loading animate-spin mr-2" />
                {t("common.loading", "加载中...")}
              </div>
            )}
          </div>
        ) : hasActiveGameFilters ? (
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

      <ScrollToTopButton
        visible={showScrollTop}
        onClick={scrollToTop}
        label={t("common.backToTop", "回到顶部")}
      />
    </div>
  );
}
