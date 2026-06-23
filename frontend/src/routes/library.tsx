import type { models, vo } from "../../wailsjs/go/models";
import type { ImportSource } from "../components/modal/GameImportModal";
import type { GameStatusFilter } from "../consts/options";
import { createRoute, useNavigate } from "@tanstack/react-router";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { enums } from "../../wailsjs/go/models";
import {
  AddGamesToCategories,
  GetCategories,
} from "../../wailsjs/go/service/CategoryService";
import {
  BatchUpdateStatus,
  DeleteGames,
  GetGames,
} from "../../wailsjs/go/service/GameService";
import { FilterBar } from "../components/bar/FilterBar";
import { TagFilterMenu } from "../components/bar/TagFilterMenu";
import { VirtualGameGrid } from "../components/grid/VirtualGameGrid";
import { AddGameModal } from "../components/modal/AddGameModal";
import { AddToCategoryModal } from "../components/modal/AddToCategoryModal";
import { BatchImportModal } from "../components/modal/BatchImportModal";
import { ConfirmModal } from "../components/modal/ConfirmModal";
import { GameImportModal } from "../components/modal/GameImportModal";
import { LibrarySkeleton } from "../components/skeleton/LibrarySkeleton";
import { BetterDropdownMenu } from "../components/ui/better/BetterDropdownMenu";
import { ScrollToTopButton } from "../components/ui/ScrollToTopButton";
import { sortOptions, statusOptions } from "../consts/options";
import { useDebouncedValue } from "../hooks/useDebouncedValue";
import { usePageScrollControls } from "../hooks/usePageScrollControls";
import { useTagGameFilter } from "../hooks/useTagGameFilter";
import { useAppStore } from "../store";
import { Route as rootRoute } from "./__root";

interface LibrarySearch {
  tagFilter?: string;
  searchQuery?: string;
}

const LIBRARY_STORAGE_KEY = "library";
const PAGE_SIZE = 120;
const WINDOW_BUFFER_SIZE = PAGE_SIZE;
const WINDOW_REQUEST_SIZE = PAGE_SIZE * 2;
const WINDOW_KEEP_RADIUS = PAGE_SIZE * 4;
const LIBRARY_SORT_BY_VALUES = new Set<enums.GameListSortBy>([
  enums.GameListSortBy.NAME,
  enums.GameListSortBy.LAST_PLAYED_AT,
  enums.GameListSortBy.CREATED_AT,
  enums.GameListSortBy.RATING,
  enums.GameListSortBy.RELEASE_DATE,
]);
const LIBRARY_STATUS_VALUES = new Set(
  statusOptions.map(option => option.value),
);
const LIBRARY_SCROLL_RESTORATION_ID = "library-scroll";

interface GameListMetaCacheEntry {
  total: number;
}

interface VisibleGameRange {
  endIndex: number;
  startIndex: number;
}

const libraryGameListMetaCache = new Map<string, GameListMetaCacheEntry>();

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

function getWindowRequestForVisibleRange(
  visibleRange: VisibleGameRange | null,
  total: number,
) {
  if (!visibleRange || total <= 0) {
    return {
      limit: PAGE_SIZE,
      offset: 0,
    };
  }

  const startIndex = Math.min(Math.max(0, visibleRange.startIndex), total - 1);
  const endIndex = Math.min(
    Math.max(startIndex, visibleRange.endIndex),
    total - 1,
  );
  return getWindowRequest(startIndex, endIndex, total);
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

function readStoredLibrarySortBy() {
  const savedSortBy = readStoredValue(`${LIBRARY_STORAGE_KEY}_sortBy`);
  if (
    savedSortBy
    && LIBRARY_SORT_BY_VALUES.has(savedSortBy as enums.GameListSortBy)
  ) {
    return savedSortBy as enums.GameListSortBy;
  }
  return enums.GameListSortBy.CREATED_AT;
}

function readStoredLibrarySortOrder() {
  const savedSortOrder = readStoredValue(`${LIBRARY_STORAGE_KEY}_sortOrder`);
  return savedSortOrder === enums.SortOrder.ASC
    || savedSortOrder === enums.SortOrder.DESC
    ? (savedSortOrder as enums.SortOrder)
    : enums.SortOrder.DESC;
}

function readStoredLibrarySearchQuery() {
  return readStoredValue(`${LIBRARY_STORAGE_KEY}_searchQuery`) || "";
}

function writeStoredLibrarySearchQuery(value: string) {
  if (typeof window === "undefined") {
    return;
  }

  if (value) {
    window.localStorage.setItem(`${LIBRARY_STORAGE_KEY}_searchQuery`, value);
  }
  else {
    window.localStorage.removeItem(`${LIBRARY_STORAGE_KEY}_searchQuery`);
  }
}

function readStoredLibraryStatusFilter() {
  const savedStatusFilter = readStoredValue(
    `${LIBRARY_STORAGE_KEY}_statusFilter`,
  ) as GameStatusFilter | null;
  return savedStatusFilter && LIBRARY_STATUS_VALUES.has(savedStatusFilter)
    ? savedStatusFilter
    : "";
}

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/library",
  validateSearch: (search: Record<string, unknown>): LibrarySearch => ({
    tagFilter:
      typeof search.tagFilter === "string" ? search.tagFilter : undefined,
    searchQuery:
      typeof search.searchQuery === "string" ? search.searchQuery : undefined,
  }),
  component: LibraryPage,
});

function LibraryPage() {
  const navigate = useNavigate();
  const { tagFilter: routeTagFilter, searchQuery: routeSearchQuery }
    = Route.useSearch();
  const routeTagFilterValue = routeTagFilter?.trim() || "";
  const pageRef = useRef<HTMLDivElement | null>(null);
  const toolbarRef = useRef<HTMLDivElement | null>(null);
  const { t } = useTranslation();
  const [showSkeleton, setShowSkeleton] = useState(false);
  const [gamesByIndex, setGamesByIndex] = useState<Map<number, models.Game>>(
    () => new Map(),
  );
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [hasLoadedGames, setHasLoadedGames] = useState(false);
  const [hasShownMainContent, setHasShownMainContent] = useState(false);
  const [loadedQueryKey, setLoadedQueryKey] = useState("");
  const [loadingMore, setLoadingMore] = useState(false);
  const currentQueryKeyRef = useRef("");
  const gamesByIndexRef = useRef<ReadonlyMap<number, models.Game>>(new Map());
  const loadingWindowsRef = useRef(new Set<string>());
  const totalRef = useRef(0);
  const [isAddGameModalOpen, setIsAddGameModalOpen] = useState(false);
  const [isBatchImportOpen, setIsBatchImportOpen] = useState(false);
  const [importSource, setImportSource] = useState<ImportSource | null>(null);
  const visibleRangeRef = useRef<VisibleGameRange | null>(null);
  const [searchQuery, setSearchQuery] = useState(
    () => routeSearchQuery?.trim() || readStoredLibrarySearchQuery(),
  );
  const [sortBy, setSortBy] = useState<enums.GameListSortBy>(() =>
    readStoredLibrarySortBy(),
  );
  const [sortOrder, setSortOrder] = useState<enums.SortOrder>(() =>
    readStoredLibrarySortOrder(),
  );
  const [statusFilter, setStatusFilter] = useState<GameStatusFilter>(() =>
    readStoredLibraryStatusFilter(),
  );
  const storedSelectedTags = useAppStore(state => state.librarySelectedTags);
  const setStoredSelectedTags = useAppStore(
    state => state.setLibrarySelectedTags,
  );
  const showSortField = useAppStore(
    state => state.config?.show_sort_field_on_cover ?? false,
  );
  const patchLiveConfig = useAppStore(state => state.patchLiveConfig);
  const handleShowSortFieldChange = useCallback(
    (value: boolean) => {
      void patchLiveConfig({ show_sort_field_on_cover: value });
    },
    [patchLiveConfig],
  );
  const debouncedSearchQuery = useDebouncedValue(searchQuery, 250);
  const [batchMode, setBatchMode] = useState(false);
  const [selectedGameIds, setSelectedGameIds] = useState<string[]>([]);
  const enableTagTranslation = useAppStore(
    state => state.config?.enable_tag_translation ?? true,
  );
  const [allCategories, setAllCategories] = useState<vo.CategoryVO[]>([]);
  const [isBatchCategoryModalOpen, setIsBatchCategoryModalOpen]
    = useState(false);
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
  const loadedGames = useMemo(
    () => Array.from(gamesByIndex.values()),
    [gamesByIndex],
  );
  const loadedGameCount = gamesByIndex.size;

  useEffect(() => {
    gamesByIndexRef.current = gamesByIndex;
  }, [gamesByIndex]);

  useEffect(() => {
    totalRef.current = total;
  }, [total]);

  // 延迟显示骨架屏
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

  useEffect(() => {
    if (!loading || hasLoadedGames || total > 0) {
      setHasShownMainContent(true);
    }
  }, [hasLoadedGames, loading, total]);

  const clearRouteTagFilter = useCallback(() => {
    if (!routeTagFilter) {
      return;
    }
    void navigate({
      to: "/library",
      search: prev => ({ ...prev, tagFilter: undefined }),
      replace: true,
    });
  }, [navigate, routeTagFilter]);

  const clearRouteSearchQuery = useCallback(() => {
    if (!routeSearchQuery) {
      return;
    }
    void navigate({
      to: "/library",
      search: prev => ({ ...prev, searchQuery: undefined }),
      replace: true,
    });
  }, [navigate, routeSearchQuery]);

  const handleSearchChange = useCallback(
    (value: string) => {
      setSearchQuery(value);
      if (routeSearchQuery) {
        clearRouteSearchQuery();
      }
    },
    [clearRouteSearchQuery, routeSearchQuery],
  );

  const {
    selectedTags,
    tagInput,
    setTagInput,
    tagSuggestions,
    selectTag,
    removeTag,
    clearTagFilter,
  } = useTagGameFilter({
    enableTagTranslation,
    initialSelectedTags: routeTagFilterValue
      ? [routeTagFilterValue]
      : storedSelectedTags,
    onManualTagChange: clearRouteTagFilter,
  });
  const isPageReady = !(loading && total === 0 && loadedGameCount === 0);

  const { scrollToTop, showScrollTop } = usePageScrollControls({
    anchorRef: pageRef,
    enabled: isPageReady,
    toolbarRef,
  });

  useEffect(() => {
    setStoredSelectedTags(selectedTags);
  }, [selectedTags, setStoredSelectedTags]);

  // 通过路由参数进入库页面时，自动应用 tag 筛选
  useEffect(() => {
    if (!routeTagFilterValue) {
      return;
    }
    selectTag(routeTagFilterValue, { manual: false });
  }, [routeTagFilterValue, selectTag]);

  useEffect(() => {
    const incomingSearchQuery = routeSearchQuery?.trim();
    if (!incomingSearchQuery) {
      return;
    }
    writeStoredLibrarySearchQuery(incomingSearchQuery);
    setSearchQuery(incomingSearchQuery);
  }, [routeSearchQuery]);

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
  const queryKey = useMemo(() => JSON.stringify(queryParams), [queryParams]);
  const isSearchSettling = searchQuery.trim() !== debouncedSearchQuery.trim();
  const hasActiveGameFilters
    = debouncedSearchQuery.trim().length > 0
      || selectedTags.length > 0
      || Boolean(statusFilter);
  const isEmptyListWaiting
    = total === 0 && (loading || isSearchSettling || loadedQueryKey !== queryKey);

  const loadGamesWindow = useCallback(
    async (
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
        const response = await GetGames({
          limit,
          offset,
          ...queryParams,
        } as vo.GameListRequest);
        if (currentQueryKeyRef.current !== queryKey) {
          return;
        }

        const nextTotal = response.total || 0;
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
          (response.games || []).forEach((game, index) => {
            next.set(offset + index, game);
          });
          return next;
        });
        libraryGameListMetaCache.set(queryKey, { total: nextTotal });
        setHasLoadedGames(true);
        setLoadedQueryKey(queryKey);
      }
      catch (error) {
        if (currentQueryKeyRef.current === queryKey) {
          console.error("Failed to fetch games:", error);
          toast.error(t("library.toast.loadGamesFailed", "加载游戏失败"));
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

  const requestMissingVisibleWindow = useCallback(
    (range = visibleRangeRef.current) => {
      const currentTotal = totalRef.current;
      if (!range || currentTotal <= 0) {
        return;
      }

      const endIndex = Math.min(range.endIndex, currentTotal - 1);
      for (let index = range.startIndex; index <= endIndex; index++) {
        if (!gamesByIndexRef.current.has(index)) {
          const request = getWindowRequest(index, endIndex, currentTotal);
          void loadGamesWindow(request.offset, request.limit);
          return;
        }
      }
    },
    [loadGamesWindow],
  );

  const refreshFirstWindow = useCallback(() => {
    loadingWindowsRef.current.clear();
    setGamesByIndex(new Map());
    setTotal(0);
    setHasLoadedGames(false);
    setLoadedQueryKey("");
    void loadGamesWindow(0, PAGE_SIZE, { force: true, reset: true });
  }, [loadGamesWindow]);

  const invalidateAndRefreshLibrary = useCallback(() => {
    libraryGameListMetaCache.clear();
    refreshFirstWindow();
  }, [refreshFirstWindow]);

  const handleVisibleRangeChange = useCallback(
    (startIndex: number, endIndex: number) => {
      const previousRange = visibleRangeRef.current;
      if (
        previousRange?.startIndex === startIndex
        && previousRange.endIndex === endIndex
      ) {
        return;
      }

      const nextRange = { endIndex, startIndex };
      visibleRangeRef.current = nextRange;
      requestMissingVisibleWindow(nextRange);
    },
    [requestMissingVisibleWindow],
  );

  useEffect(() => {
    requestMissingVisibleWindow();
  }, [gamesByIndex, requestMissingVisibleWindow, total]);

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

  const setGameSelection = useCallback((gameId: string, selected: boolean) => {
    setSelectedGameIds((prev) => {
      if (selected) {
        return prev.includes(gameId) ? prev : [...prev, gameId];
      }
      return prev.filter(id => id !== gameId);
    });
  }, []);

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

  const statusConfig = {
    [enums.GameStatus.NOT_STARTED]: {
      label: t("common.notStarted"),
      icon: "i-mdi-clock-outline",
      color: "bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300",
    },
    [enums.GameStatus.WANT_TO_PLAY]: {
      label: t("common.wantToPlay"),
      icon: "i-mdi-bookmark-outline",
      color: "bg-info-100 text-info-700 dark:bg-info-900 dark:text-info-300",
    },
    [enums.GameStatus.PLAYING]: {
      label: t("common.playing"),
      icon: "i-mdi-gamepad-variant",
      color:
        "bg-neutral-100 text-neutral-700 dark:bg-neutral-900 dark:text-neutral-300",
    },
    [enums.GameStatus.COMPLETED]: {
      label: t("common.completed"),
      icon: "i-mdi-trophy",
      color:
        "bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300",
    },
    [enums.GameStatus.ON_HOLD]: {
      label: t("common.onHold"),
      icon: "i-mdi-pause-circle-outline",
      color:
        "bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300",
    },
  };

  const handleBatchStatusUpdate = async (newStatus: string) => {
    if (selectedGameIds.length === 0)
      return;
    try {
      await BatchUpdateStatus(selectedGameIds, newStatus);
      invalidateAndRefreshLibrary();
      const label
        = statusConfig[newStatus as keyof typeof statusConfig]?.label
          ?? newStatus;
      toast.success(
        t("library.toast.batchStatusUpdated", {
          count: selectedGameIds.length,
          status: label,
        }),
      );
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
      toast.success(
        t("library.toast.batchAddFavSuccess", {
          count: selectedGameIds.length,
        }),
      );
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
      message: t("library.toast.batchDeleteConfirmMsg", {
        count: selectedGameIds.length,
      }),
      type: "danger",
      onConfirm: async () => {
        try {
          await DeleteGames(selectedGameIds);
          invalidateAndRefreshLibrary();
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
    currentQueryKeyRef.current = queryKey;
    loadingWindowsRef.current.clear();
    setGamesByIndex(new Map());
    setSelectedGameIds([]);
    setLoadingMore(false);

    const cached = libraryGameListMetaCache.get(queryKey);
    if (cached) {
      setTotal(cached.total);
      setHasLoadedGames(true);
      setLoadedQueryKey(queryKey);
      if (cached.total > 0) {
        const request = getWindowRequestForVisibleRange(
          visibleRangeRef.current,
          cached.total,
        );
        void loadGamesWindow(request.offset, request.limit, {
          force: true,
          reset: true,
        });
      }
      else {
        setLoading(false);
      }
      return;
    }

    setTotal(0);
    setHasLoadedGames(false);
    setLoadedQueryKey("");
    void loadGamesWindow(0, PAGE_SIZE, { force: true, reset: true });
  }, [loadGamesWindow, queryKey]);

  useEffect(() => {
    currentQueryKeyRef.current = queryKey;
  }, [queryKey]);

  if (!hasShownMainContent && !hasLoadedGames && loading && total === 0) {
    if (!showSkeleton) {
      return null;
    }
    return <LibrarySkeleton />;
  }

  return (
    <div
      ref={pageRef}
      data-scroll-restoration-id={LIBRARY_SCROLL_RESTORATION_ID}
      className="h-full w-full overflow-y-auto scrollbar-stable p-8"
    >
      <div className="mx-auto max-w-8xl space-y-6">
        <div className="flex flex-col items-left justify-between">
          <h1 className="text-4xl font-bold text-brand-900 dark:text-white">
            {t("library.title")}
          </h1>
          <p className="text-brand-500 dark:text-brand-400 mt-2">
            {gameCountText}
          </p>
        </div>

        <div ref={toolbarRef}>
          <FilterBar
            searchQuery={searchQuery}
            onSearchChange={handleSearchChange}
            searchPlaceholder={t("library.searchPlaceholder")}
            disableStoredSearchQuery={Boolean(routeSearchQuery?.trim())}
            sortBy={sortBy}
            onSortByChange={val => setSortBy(val as enums.GameListSortBy)}
            sortOptions={sortOptions.map(opt => ({
              ...opt,
              label: t(opt.label),
            }))}
            sortOrder={sortOrder}
            onSortOrderChange={setSortOrder}
            showSortField={showSortField}
            onShowSortFieldChange={handleShowSortFieldChange}
            statusFilter={statusFilter}
            onStatusFilterChange={setStatusFilter}
            statusOptions={statusOptions.map(opt => ({
              ...opt,
              label: t(opt.label),
            }))}
            storageKey="library"
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
        </div>

        {isEmptyListWaiting ? (
          <div className="flex-1 flex items-center justify-center w-full text-brand-500 dark:text-brand-400">
            <div className="flex flex-col items-center">
              <div className="i-mdi-loading animate-spin text-4xl mb-2" />
              <p>{t("common.loading", "加载中...")}</p>
            </div>
          </div>
        ) : total === 0 ? (
          <div className="flex-1 flex items-center justify-center w-full">
            <div className="flex flex-col items-center justify-center py-20 text-brand-500 dark:text-brand-400">
              {hasActiveGameFilters ? (
                <>
                  <div className="i-mdi-magnify text-6xl mb-4" />
                  <p className="text-xl">{t("library.notFound")}</p>
                </>
              ) : (
                <>
                  <div className="i-mdi-gamepad-variant-outline text-6xl mb-4" />
                  <p className="text-xl">{t("library.emptyState")}</p>
                  <p className="text-sm mt-2">
                    {t("library.emptyStateAction")}
                  </p>
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
                </>
              )}
            </div>
          </div>
        ) : (
          <div className="relative">
            <div
              className={`transition-opacity duration-200 ${
                loading ? "pointer-events-none opacity-60" : "opacity-100"
              }`}
            >
              <VirtualGameGrid
                gamesByIndex={gamesByIndex}
                scrollRestorationId={LIBRARY_SCROLL_RESTORATION_ID}
                totalItems={total}
                visibleRangeResetKey={queryKey}
                searchQuery={debouncedSearchQuery}
                selectionMode={batchMode}
                selectedGameIds={selectedGameIdSet}
                onSelectChange={setGameSelection}
                onVisibleRangeChange={handleVisibleRangeChange}
                displaySortField={showSortField ? sortBy : null}
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
        )}
      </div>

      <AddGameModal
        isOpen={isAddGameModalOpen}
        onClose={() => setIsAddGameModalOpen(false)}
        onGameAdded={invalidateAndRefreshLibrary}
      />

      <GameImportModal
        isOpen={importSource !== null}
        source={importSource || "potatovn"}
        onClose={() => setImportSource(null)}
        onImportComplete={invalidateAndRefreshLibrary}
      />

      <BatchImportModal
        isOpen={isBatchImportOpen}
        onClose={() => setIsBatchImportOpen(false)}
        onImportComplete={invalidateAndRefreshLibrary}
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

      <ScrollToTopButton
        visible={showScrollTop}
        onClick={scrollToTop}
        label={t("common.backToTop", "回到顶部")}
      />
    </div>
  );
}
