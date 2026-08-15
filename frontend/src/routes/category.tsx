import type { models, vo } from "../../src/bindings/models";
import type { GameCardLayout } from "../components/card/GameCard";
import type { GameStatusFilter } from "../consts/options";
import { createRoute, useNavigate } from "@tanstack/react-router";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  AddGameToCategory,
  DeleteCategory,
  GetCategoryByID,
  GetCategoryGames,
  RemoveGameFromCategory,
  RemoveGamesFromCategory,
  SearchCategoryGameCandidates,
  UpdateCategory,
} from "../../bindings/lunabox/internal/service/categoryservice";
import { enums } from "../../src/bindings/models";
import {
  getCategoryGameListMetaCache,
  invalidateCategoryGameLists,
  setCategoryGameListMetaCache,
  useGameCacheStore,
} from "../cache/gameCache";
import { FilterBar } from "../components/bar/FilterBar";
import { TagFilterMenu } from "../components/bar/TagFilterMenu";
import { VirtualGameGrid } from "../components/grid/VirtualGameGrid";
import { AddGameToCategoryModal } from "../components/modal/AddGameToCategoryModal";
import { CategoryModal } from "../components/modal/CategoryModal";
import { ConfirmModal } from "../components/modal/ConfirmModal";
import { CategorySkeleton } from "../components/skeleton/CategorySkeleton";
import { BetterDropdownMenu } from "../components/ui/better/BetterDropdownMenu";
import { ScrollToTopButton } from "../components/ui/ScrollToTopButton";
import { CATEGORY_NAME_MAX_LENGTH } from "../consts/category";
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
  enums.GameListSortBy.GameListSortByName,
  enums.GameListSortBy.GameListSortByCompany,
  enums.GameListSortBy.GameListSortByLastPlayedAt,
  enums.GameListSortBy.GameListSortByCreatedAt,
  enums.GameListSortBy.GameListSortByRating,
  enums.GameListSortBy.GameListSortByReleaseDate,
]);
const CATEGORY_STATUS_VALUES = new Set(
  statusOptions.map(option => option.value),
);

interface VisibleGameRange {
  endIndex: number;
  startIndex: number;
}

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

function readStoredCategorySortBy() {
  const savedSortBy = readStoredValue(`${CATEGORY_STORAGE_KEY}_sortBy`);
  if (
    savedSortBy
    && CATEGORY_SORT_BY_VALUES.has(savedSortBy as enums.GameListSortBy)
  ) {
    return savedSortBy as enums.GameListSortBy;
  }
  return enums.GameListSortBy.GameListSortByCreatedAt;
}

function readStoredCategorySortOrder() {
  const savedSortOrder = readStoredValue(`${CATEGORY_STORAGE_KEY}_sortOrder`);
  return savedSortOrder === enums.SortOrder.SortOrderAsc
    || savedSortOrder === enums.SortOrder.SortOrderDesc
    ? (savedSortOrder as enums.SortOrder)
    : enums.SortOrder.SortOrderDesc;
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

function readStoredCategoryStatusFilterInverted() {
  return (
    readStoredValue(`${CATEGORY_STORAGE_KEY}_statusFilterInverted`) === "true"
  );
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
  const [isEditCategoryModalOpen, setIsEditCategoryModalOpen] = useState(false);
  const [isDeleteCategoryConfirmOpen, setIsDeleteCategoryConfirmOpen]
    = useState(false);
  const [editCategoryName, setEditCategoryName] = useState("");
  const [allGames, setAllGames] = useState<models.Game[]>([]);
  const [candidateSearchQuery, setCandidateSearchQuery] = useState("");
  const [candidateHasMore, setCandidateHasMore] = useState(false);
  const [candidateLoading, setCandidateLoading] = useState(false);
  const candidateRequestIdRef = useRef(0);
  const [visibleRange, setVisibleRange] = useState<VisibleGameRange | null>(
    null,
  );
  const visibleRangeRef = useRef<VisibleGameRange | null>(null);
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
  const [statusFilterInverted, setStatusFilterInverted] = useState(
    () =>
      Boolean(readStoredCategoryStatusFilter())
      && readStoredCategoryStatusFilterInverted(),
  );
  const [tagFilterInverted, setTagFilterInverted] = useState(false);
  const categoryGamesRevision = useGameCacheStore(
    state => state.categoryRevision,
  );
  const showSortField = useAppStore(
    state => state.config?.show_sort_field_on_cover ?? false,
  );
  const gameCardLayout = useAppStore(state =>
    state.config?.game_card_layout === "landscape" ? "landscape" : "portrait",
  ) as GameCardLayout;
  const patchLiveConfig = useAppStore(state => state.patchLiveConfig);
  const handleShowSortFieldChange = useCallback(
    (value: boolean) => {
      void patchLiveConfig({ show_sort_field_on_cover: value });
    },
    [patchLiveConfig],
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

  useEffect(() => {
    if (selectedTags.length === 0 && tagFilterInverted) {
      setTagFilterInverted(false);
    }
  }, [selectedTags.length, tagFilterInverted]);

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

  const loadCategory = useCallback(
    async (id: string) => {
      try {
        const result = await GetCategoryByID(id);
        setCategory(result);
        loadedCategoryIdRef.current = id;
      }
      catch (error) {
        console.error("Failed to load category:", error);
        toast.error(t("category.toast.loadCategoryFailed"));
      }
    },
    [t],
  );

  const queryParams = useMemo(
    () => ({
      search_query: debouncedSearchQuery.trim(),
      ...(statusFilter
        ? { exclude_status: statusFilterInverted, status: statusFilter }
        : {}),
      tags: selectedTags,
      exclude_tags: tagFilterInverted && selectedTags.length > 0,
      sort_by: sortBy,
      sort_order: sortOrder,
    }),
    [
      debouncedSearchQuery,
      selectedTags,
      sortBy,
      sortOrder,
      statusFilter,
      statusFilterInverted,
      tagFilterInverted,
    ],
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
      const requestKey = `${categoryGamesRevision}:${queryKey}:${offset}:${limit}`;
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
        if (
          currentQueryKeyRef.current !== queryKey
          || useGameCacheStore.getState().categoryRevision
          !== categoryGamesRevision
        ) {
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
        setCategoryGameListMetaCache(queryKey, nextTotal);
        setLoadedQueryKey(queryKey);
      }
      catch (error) {
        if (
          currentQueryKeyRef.current === queryKey
          && useGameCacheStore.getState().categoryRevision
          === categoryGamesRevision
        ) {
          console.error("Failed to load games for category:", error);
          toast.error(t("category.toast.loadGamesFailed"));
        }
      }
      finally {
        loadingWindowsRef.current.delete(requestKey);
        if (
          currentQueryKeyRef.current === queryKey
          && useGameCacheStore.getState().categoryRevision
          === categoryGamesRevision
        ) {
          setLoading(false);
          setLoadingMore(loadingWindowsRef.current.size > 0);
        }
      }
    },
    [categoryGamesRevision, queryKey, queryParams, t],
  );

  const handleVisibleRangeChange = useCallback(
    (startIndex: number, endIndex: number) => {
      const nextRange = { endIndex, startIndex };
      visibleRangeRef.current = nextRange;
      setVisibleRange((previous) => {
        if (
          previous?.startIndex === startIndex
          && previous.endIndex === endIndex
        ) {
          return previous;
        }
        return nextRange;
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

  const openEditCategoryModal = () => {
    if (!category)
      return;
    setEditCategoryName(category.name.slice(0, CATEGORY_NAME_MAX_LENGTH));
    setIsEditCategoryModalOpen(true);
  };

  const handleUpdateCategory = async () => {
    if (!category || !editCategoryName.trim())
      return;
    try {
      await UpdateCategory(category.id, editCategoryName, category.emoji || "");
      setCategory(current =>
        current ? { ...current, name: editCategoryName } : current,
      );
      setIsEditCategoryModalOpen(false);
      setEditCategoryName("");
      toast.success(t("categories.toast.updateSuccess"));
    }
    catch (error) {
      console.error("Failed to update category:", error);
      toast.error(t("categories.toast.updateFailed"));
    }
  };

  const handleDeleteCategory = async () => {
    if (!category)
      return;
    try {
      await DeleteCategory(category.id);
      toast.success(t("categories.toast.deleteSuccess"));
      navigate({ to: "/categories" });
    }
    catch (error) {
      console.error("Failed to delete category:", error);
      toast.error(t("categories.toast.deleteFailed"));
    }
  };

  const handleRemoveGame = async (gameId: string) => {
    if (!category)
      return;
    try {
      await RemoveGameFromCategory(gameId, category.id);
      invalidateCategoryGameLists();
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
      const requestRevision = categoryGamesRevision;
      setCandidateLoading(true);
      try {
        const result = await SearchCategoryGameCandidates({
          category_id: category.id,
          limit: CANDIDATE_PAGE_SIZE,
          offset,
          search_query: debouncedCandidateSearchQuery.trim(),
        });
        if (
          requestId !== candidateRequestIdRef.current
          || useGameCacheStore.getState().categoryRevision !== requestRevision
        ) {
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
        if (
          requestId === candidateRequestIdRef.current
          && useGameCacheStore.getState().categoryRevision === requestRevision
        ) {
          console.error("Failed to load candidate games:", error);
          toast.error(t("category.toast.loadAllGamesFailed"));
        }
      }
      finally {
        if (
          requestId === candidateRequestIdRef.current
          && useGameCacheStore.getState().categoryRevision === requestRevision
        ) {
          setCandidateLoading(false);
        }
      }
    },
    [category, categoryGamesRevision, debouncedCandidateSearchQuery, t],
  );

  const handleAddGameToCategory = async (gameId: string) => {
    if (!category)
      return;
    try {
      await AddGameToCategory(gameId, category.id);
      setAllGames(prev => prev.filter(g => g.id !== gameId));
      invalidateCategoryGameLists();
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
    ? t(
        statusFilterInverted
          ? "category.excludedStatusGameCount"
          : "category.filteredGameCount",
        {
          count: total,
          status: statusFilterLabel,
        },
      )
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
      invalidateCategoryGameLists();
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
        setBatchMode(false);
        setSelectedGameIds([]);
        if (shouldLoadCategory) {
          setLoading(true);
        }
        setLoadingMore(false);

        const cached = getCategoryGameListMetaCache(queryKey);
        if (cached?.revision === categoryGamesRevision) {
          setTotal(cached.total);
          setLoadedQueryKey(queryKey);
          if (shouldLoadCategory) {
            await loadCategory(categoryId);
          }
          if (cached.total > 0) {
            const request = getWindowRequestForVisibleRange(
              visibleRangeRef.current,
              cached.total,
            );
            void loadGamesWindow(categoryId, request.offset, request.limit, {
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
  }, [
    categoryGamesRevision,
    categoryId,
    loadCategory,
    loadGamesWindow,
    queryKey,
  ]);

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
        <div className="flex items-start justify-between gap-6">
          <div className="min-w-0 flex-1">
            <h1 className="flex min-w-0 flex-wrap items-center gap-3 text-4xl font-bold text-brand-900 dark:text-white">
              {(category.emoji || "").trim() && (
                <span className="shrink-0 text-3xl leading-none">
                  {category.emoji}
                </span>
              )}
              <span className="min-w-0 break-words [overflow-wrap:anywhere]">
                {category.name}
              </span>
              {category.is_system && (
                <span className="shrink-0 rounded-md bg-neutral-100 px-2 py-1 align-middle text-sm text-neutral-800 dark:bg-neutral-900 dark:text-neutral-300">
                  {t("category.systemTag")}
                </span>
              )}
            </h1>
            <p className="text-brand-500 dark:text-brand-400 mt-2">
              {gameCountText}
            </p>
          </div>
          {!category.is_system && (
            <BetterDropdownMenu
              align="end"
              menuWidth="min-w-[140px]"
              ariaLabel={t("common.action")}
              trigger={(
                <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full border border-brand-200 bg-white text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-500 dark:border-brand-700 dark:bg-brand-800 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white data-glass:border-transparent data-glass:bg-white/10 data-glass:text-brand-900 data-glass:backdrop-blur-12 data-glass:backdrop-saturate-180 data-glass:hover:bg-white/10 data-glass:hover:text-brand-900 data-glass:dark:bg-black/12 data-glass:dark:text-white data-glass:dark:hover:bg-black/12 data-glass:dark:hover:text-white">
                  <div
                    className="i-mdi-dots-horizontal text-2xl"
                    aria-hidden="true"
                  />
                </div>
              )}
              items={[
                {
                  key: "rename",
                  label: t("categories.rename"),
                  icon: "i-mdi-pencil",
                  onClick: openEditCategoryModal,
                },
                {
                  key: "delete",
                  label: t("common.delete"),
                  icon: "i-mdi-delete",
                  iconColor: "text-error-500 dark:text-error-400",
                  dividerBefore: true,
                  onClick: () => setIsDeleteCategoryConfirmOpen(true),
                },
              ]}
            />
          )}
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
            showSortField={showSortField}
            onShowSortFieldChange={handleShowSortFieldChange}
            statusFilter={statusFilter}
            onStatusFilterChange={setStatusFilter}
            statusFilterInverted={statusFilterInverted}
            onStatusFilterInvertedChange={setStatusFilterInverted}
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
                inverted={tagFilterInverted}
                onTagInputChange={setTagInput}
                onSelectTag={selectTag}
                onRemoveTag={removeTag}
                onClearTagFilter={clearTagFilter}
                onInvertedChange={setTagFilterInverted}
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
                visibleRangeResetKey={queryKey}
                searchQuery={debouncedSearchQuery}
                selectionMode={batchMode}
                selectedGameIds={selectedGameIdSet}
                onSelectChange={setGameSelection}
                onVisibleRangeChange={handleVisibleRangeChange}
                displaySortField={showSortField ? sortBy : null}
                cardLayout={gameCardLayout}
                renderOverlay={game =>
                  !batchMode && (
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleRemoveGame(game.id);
                      }}
                      className="absolute top-2 right-2 p-1 bg-error-500 text-white rounded-full opacity-0 group-hover:opacity-100 transition-opacity shadow-md hover:bg-error-600"
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

      <CategoryModal
        isOpen={isEditCategoryModalOpen}
        value={editCategoryName}
        onChange={setEditCategoryName}
        onClose={() => {
          setIsEditCategoryModalOpen(false);
          setEditCategoryName("");
        }}
        onSubmit={handleUpdateCategory}
        mode="edit"
      />

      <ConfirmModal
        isOpen={isDeleteCategoryConfirmOpen}
        title={t("categories.toast.deleteTitle")}
        message={t("categories.toast.deleteMsg", { name: category.name })}
        type="danger"
        onClose={() => setIsDeleteCategoryConfirmOpen(false)}
        onConfirm={handleDeleteCategory}
      />

      <ScrollToTopButton
        visible={showScrollTop}
        onClick={scrollToTop}
        label={t("common.backToTop", "回到顶部")}
      />
    </div>
  );
}
