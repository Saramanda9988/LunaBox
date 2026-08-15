import type { models, service, vo } from "../../src/bindings/models";
import type { GameCardLayout } from "../components/card/GameCard";
import type { ImportSource } from "../components/modal/GameImportModal";
import type { GameStatusFilter } from "../consts/options";
import { createRoute, useNavigate } from "@tanstack/react-router";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  AddGamesToCategories,
  GetCategories,
} from "../../bindings/lunabox/internal/service/categoryservice";
import {
  BatchUpdateStatus,
  DeleteGames,
  GetGames,
} from "../../bindings/lunabox/internal/service/gameservice";
import {
  BatchImportGamesToSteam,
  GetGameSteamStatus,
} from "../../bindings/lunabox/internal/service/integrationservice";
import { enums } from "../../src/bindings/models";
import playniteIconUrl from "../assets/importers/playnite.png";
import potatovnIconUrl from "../assets/importers/potatovn.png";
import reinaManagerIconUrl from "../assets/importers/reinamanager.png";
import vniteIconUrl from "../assets/importers/vnite.png";
import yukihubIconUrl from "../assets/importers/yukihub.png";
import {
  getLibraryGameListCache,
  invalidateAllGameLists,
  invalidateCategoryGameLists,
  removeGamesFromCache,
  setLibraryGameListCache,
  useGameCacheStore,
} from "../cache/gameCache";
import { FilterBar } from "../components/bar/FilterBar";
import { GameFilterPresetMenu } from "../components/bar/GameFilterPresetMenu";
import { TagFilterMenu } from "../components/bar/TagFilterMenu";
import { VirtualGameGrid } from "../components/grid/VirtualGameGrid";
import { AddGameModal } from "../components/modal/AddGameModal";
import { AddToCategoryModal } from "../components/modal/AddToCategoryModal";
import { BatchImportModal } from "../components/modal/BatchImportModal";
import { ConfirmModal } from "../components/modal/ConfirmModal";
import { GameImportModal } from "../components/modal/GameImportModal";
import { SteamBatchImportModal } from "../components/modal/SteamBatchImportModal";
import { LibrarySkeleton } from "../components/skeleton/LibrarySkeleton";
import { BetterButton } from "../components/ui/better/BetterButton";
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
  enums.GameListSortBy.GameListSortByName,
  enums.GameListSortBy.GameListSortByCompany,
  enums.GameListSortBy.GameListSortByLastPlayedAt,
  enums.GameListSortBy.GameListSortByCreatedAt,
  enums.GameListSortBy.GameListSortByRating,
  enums.GameListSortBy.GameListSortByReleaseDate,
]);
const LIBRARY_STATUS_VALUES = new Set(
  statusOptions.map(option => option.value),
);
const LIBRARY_SCROLL_RESTORATION_ID = "library-scroll";
const EMPTY_STATE_IMPORT_OPTIONS = [
  {
    source: "potatovn",
    labelKey: "library.importPotatoVN",
    iconSrc: potatovnIconUrl,
  },
  {
    source: "playnite",
    labelKey: "library.importPlaynite",
    iconSrc: playniteIconUrl,
  },
  {
    source: "yukihub",
    labelKey: "library.importYukiHub",
    iconSrc: yukihubIconUrl,
  },
  {
    source: "vnite",
    labelKey: "library.importVnite",
    iconSrc: vniteIconUrl,
  },
  {
    source: "reinamanager",
    labelKey: "library.importReinaManager",
    iconSrc: reinaManagerIconUrl,
  },
  {
    source: "steam",
    labelKey: "library.importSteam",
    icon: "i-mdi-steam",
  },
] as const satisfies ReadonlyArray<{
  source: ImportSource;
  labelKey: string;
  iconSrc?: string;
  icon?: string;
}>;

interface VisibleGameRange {
  endIndex: number;
  startIndex: number;
}

function LibraryGridLoadingState({
  label,
  cardLayout,
}: {
  label: string;
  cardLayout: GameCardLayout;
}) {
  return (
    <div
      aria-label={label}
      className={
        cardLayout === "landscape"
          ? "grid grid-cols-[repeat(auto-fill,minmax(16rem,1fr))] gap-3"
          : "grid grid-cols-[repeat(auto-fill,minmax(max(8rem,11%),1fr))] gap-3"
      }
    >
      {[...Array.from({ length: 16 })].map((_, index) => (
        <div
          key={index}
          className="glass-card pointer-events-none flex w-full animate-pulse flex-col overflow-hidden rounded-xl border border-brand-100 bg-white shadow-sm data-glass:bg-white/2 dark:border-brand-700 dark:bg-brand-800 data-glass:dark:bg-black/2"
        >
          <div
            className={`w-full bg-brand-200/80 dark:bg-brand-700/80 ${
              cardLayout === "landscape" ? "aspect-video" : "aspect-[3/3.6]"
            }`}
          />
          <div className="space-y-1 px-2 pb-2 pt-1">
            <div className="h-4 w-4/5 rounded bg-brand-200 dark:bg-brand-700" />
            <div className="h-3 w-3/5 rounded bg-brand-200/80 dark:bg-brand-700/80" />
          </div>
        </div>
      ))}
    </div>
  );
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
  return enums.GameListSortBy.GameListSortByCreatedAt;
}

function readStoredLibrarySortOrder() {
  const savedSortOrder = readStoredValue(`${LIBRARY_STORAGE_KEY}_sortOrder`);
  return savedSortOrder === enums.SortOrder.SortOrderAsc
    || savedSortOrder === enums.SortOrder.SortOrderDesc
    ? (savedSortOrder as enums.SortOrder)
    : enums.SortOrder.SortOrderDesc;
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

function readStoredLibraryStatusFilterInverted() {
  return (
    readStoredValue(`${LIBRARY_STORAGE_KEY}_statusFilterInverted`) === "true"
  );
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
  const [statusFilterInverted, setStatusFilterInverted] = useState(
    () =>
      Boolean(readStoredLibraryStatusFilter())
      && readStoredLibraryStatusFilterInverted(),
  );
  const [tagFilterInverted, setTagFilterInverted] = useState(false);
  const libraryGamesRevision = useGameCacheStore(
    state => state.libraryRevision,
  );
  const storedSelectedTags = useAppStore(state => state.librarySelectedTags);
  const setStoredSelectedTags = useAppStore(
    state => state.setLibrarySelectedTags,
  );
  const showSortField = useAppStore(
    state => state.config?.show_sort_field_on_cover ?? false,
  );
  const gameCardLayout = useAppStore(state =>
    state.config?.game_card_layout === "landscape" ? "landscape" : "portrait",
  );
  const patchLiveConfig = useAppStore(state => state.patchLiveConfig);
  const fetchHomeData = useAppStore(state => state.fetchHomeData);
  const platformGOOS = useAppStore(state => state.platformGOOS);
  const handleShowSortFieldChange = useCallback(
    (value: boolean) => {
      void patchLiveConfig({ show_sort_field_on_cover: value });
    },
    [patchLiveConfig],
  );
  const debouncedSearchQuery = useDebouncedValue(searchQuery, 250);
  const [batchMode, setBatchMode] = useState(false);
  const [isOpeningRandomGame, setIsOpeningRandomGame] = useState(false);
  const [selectedGameIds, setSelectedGameIds] = useState<string[]>([]);
  const [isBatchImportingToSteam, setIsBatchImportingToSteam] = useState(false);
  const [isBatchSteamModalOpen, setIsBatchSteamModalOpen] = useState(false);
  const [isCheckingBatchSteam, setIsCheckingBatchSteam] = useState(false);
  const [batchSteamStatus, setBatchSteamStatus]
    = useState<service.SteamLaunchStatus | null>(null);
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
    replaceSelectedTags,
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

  useEffect(() => {
    if (selectedTags.length === 0 && tagFilterInverted) {
      setTagFilterInverted(false);
    }
  }, [selectedTags.length, tagFilterInverted]);

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

  const applyFilterPreset = useCallback(
    (preset: models.GameFilterPreset) => {
      replaceSelectedTags(preset.tags || []);
      setTagFilterInverted(
        (preset.tags?.length || 0) > 0 && preset.exclude_tags,
      );
      setStatusFilter(preset.status || "");
      setStatusFilterInverted(Boolean(preset.status) && preset.exclude_status);
      setTagInput("");

      if (preset.status) {
        window.localStorage.setItem(
          `${LIBRARY_STORAGE_KEY}_statusFilter`,
          preset.status,
        );
        if (preset.exclude_status) {
          window.localStorage.setItem(
            `${LIBRARY_STORAGE_KEY}_statusFilterInverted`,
            "true",
          );
        }
        else {
          window.localStorage.removeItem(
            `${LIBRARY_STORAGE_KEY}_statusFilterInverted`,
          );
        }
      }
      else {
        window.localStorage.removeItem(`${LIBRARY_STORAGE_KEY}_statusFilter`);
        window.localStorage.removeItem(
          `${LIBRARY_STORAGE_KEY}_statusFilterInverted`,
        );
      }
    },
    [replaceSelectedTags, setTagInput],
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
  const queryKey = useMemo(() => JSON.stringify(queryParams), [queryParams]);
  const isSearchSettling = searchQuery.trim() !== debouncedSearchQuery.trim();
  const hasActiveGameFilters
    = debouncedSearchQuery.trim().length > 0
      || selectedTags.length > 0
      || Boolean(statusFilter);
  const isEmptyListWaiting
    = total === 0 && (loading || isSearchSettling || loadedQueryKey !== queryKey);

  const handleOpenRandomGame = useCallback(async () => {
    if (isOpeningRandomGame || total <= 0) {
      return;
    }

    setIsOpeningRandomGame(true);
    try {
      const randomOffset = Math.floor(Math.random() * total);
      const response = await GetGames({
        ...queryParams,
        limit: 1,
        offset: randomOffset,
      } as vo.GameListRequest);
      const randomGame = response.games?.[0];

      if (!randomGame?.id) {
        toast.error(t("library.toast.randomGameFailed"));
        return;
      }

      await navigate({ to: `/game/${randomGame.id}` });
    }
    catch (error) {
      console.error("Failed to open a random game:", error);
      toast.error(t("library.toast.randomGameFailed"));
    }
    finally {
      setIsOpeningRandomGame(false);
    }
  }, [isOpeningRandomGame, navigate, queryParams, t, total]);

  const loadGamesWindow = useCallback(
    async (
      offset: number,
      limit: number,
      options: { force?: boolean; reset?: boolean } = {},
    ) => {
      const requestKey = `${libraryGamesRevision}:${queryKey}:${offset}:${limit}`;
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

      try {
        const response = await GetGames({
          limit,
          offset,
          ...queryParams,
        } as vo.GameListRequest);
        if (
          currentQueryKeyRef.current !== queryKey
          || useGameCacheStore.getState().libraryRevision !== libraryGamesRevision
        ) {
          return;
        }

        const nextTotal = response.total || 0;
        totalRef.current = nextTotal;
        setTotal(nextTotal);
        const nextGamesByIndex = options.reset
          ? new Map<number, models.Game>()
          : new Map(gamesByIndexRef.current);
        const keepStart = Math.max(0, offset - WINDOW_KEEP_RADIUS);
        const keepEnd = offset + limit + WINDOW_KEEP_RADIUS;
        for (const index of nextGamesByIndex.keys()) {
          if (index < keepStart || index > keepEnd) {
            nextGamesByIndex.delete(index);
          }
        }
        (response.games || []).forEach((game, index) => {
          nextGamesByIndex.set(offset + index, game);
        });
        gamesByIndexRef.current = nextGamesByIndex;
        setGamesByIndex(nextGamesByIndex);
        setLibraryGameListCache(queryKey, nextGamesByIndex, nextTotal);
        setHasLoadedGames(true);
        setLoadedQueryKey(queryKey);
      }
      catch (error) {
        if (
          currentQueryKeyRef.current === queryKey
          && useGameCacheStore.getState().libraryRevision === libraryGamesRevision
        ) {
          console.error("Failed to fetch games:", error);
          toast.error(t("library.toast.loadGamesFailed", "加载游戏失败"));
        }
      }
      finally {
        loadingWindowsRef.current.delete(requestKey);
        if (
          currentQueryKeyRef.current === queryKey
          && useGameCacheStore.getState().libraryRevision === libraryGamesRevision
        ) {
          setLoading(false);
        }
      }
    },
    [libraryGamesRevision, queryKey, queryParams, t],
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

  const invalidateAndRefreshLibrary = useCallback(() => {
    invalidateAllGameLists();
    void fetchHomeData({ showLoading: false, syncRuntime: false });
  }, [fetchHomeData]);

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
    [enums.GameStatus.StatusNotStarted]: {
      label: t("common.notStarted"),
      icon: "i-mdi-clock-outline",
      color: "bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300",
    },
    [enums.GameStatus.StatusWantToPlay]: {
      label: t("common.wantToPlay"),
      icon: "i-mdi-bookmark-outline",
      color: "bg-info-100 text-info-700 dark:bg-info-900 dark:text-info-300",
    },
    [enums.GameStatus.StatusPlaying]: {
      label: t("common.playing"),
      icon: "i-mdi-gamepad-variant",
      color:
        "bg-neutral-100 text-neutral-700 dark:bg-neutral-900 dark:text-neutral-300",
    },
    [enums.GameStatus.StatusCompleted]: {
      label: t("common.completed"),
      icon: "i-mdi-trophy",
      color:
        "bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300",
    },
    [enums.GameStatus.StatusOnHold]: {
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
      invalidateCategoryGameLists();
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
          removeGamesFromCache(selectedGameIds);
          void fetchHomeData({ showLoading: false, syncRuntime: false });
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

  const performBatchImportToSteam = async () => {
    const gameIds = [...selectedGameIds];
    if (gameIds.length === 0 || isBatchImportingToSteam)
      return;

    setIsBatchImportingToSteam(true);
    try {
      const result = await BatchImportGamesToSteam(gameIds);
      const failedItems = result.items.filter(
        item => Boolean(item.error) || !item.status.ready,
      );
      const failedGameIds = failedItems.map(item => item.game_id);
      const failureStates = new Set(
        failedItems.map(item => (item.error ? "error" : item.status.state)),
      );

      if (result.imported_count + result.existing_count > 0) {
        invalidateAndRefreshLibrary();
      }

      const summary = t("library.toast.batchSteamImportSummary", {
        existing: result.existing_count,
        imported: result.imported_count,
        skipped: result.failed_count,
      });
      if (result.failed_count === 0) {
        setSelectedGameIds([]);
        setBatchMode(false);
        setIsBatchSteamModalOpen(false);
        toast.success(summary);
        return;
      }

      setSelectedGameIds(failedGameIds);
      if (
        result.imported_count + result.existing_count === 0
        && failureStates.size === 1
        && failureStates.has("steam_running")
      ) {
        setBatchSteamStatus(failedItems[0].status);
        return;
      }

      setIsBatchSteamModalOpen(false);
      if (result.imported_count + result.existing_count > 0) {
        toast.success(summary);
      }
      else {
        toast.error(summary);
      }
    }
    catch (error) {
      console.error("Failed to batch import games into Steam:", error);
      setIsBatchSteamModalOpen(false);
      toast.error(t("library.toast.batchSteamImportFailed"));
    }
    finally {
      setIsBatchImportingToSteam(false);
    }
  };

  const handleBatchImportToSteam = () => {
    if (selectedGameIds.length === 0 || isBatchImportingToSteam)
      return;

    setIsBatchSteamModalOpen(true);
    setBatchSteamStatus(null);
    setIsCheckingBatchSteam(true);
    void GetGameSteamStatus(selectedGameIds[0])
      .then(setBatchSteamStatus)
      .catch((error) => {
        console.error("Failed to check Steam status:", error);
        setIsBatchSteamModalOpen(false);
        toast.error(t("steamImport.checkFailed", { error }));
      })
      .finally(() => {
        setIsCheckingBatchSteam(false);
      });
  };

  useLayoutEffect(() => {
    currentQueryKeyRef.current = queryKey;
    loadingWindowsRef.current.clear();
    setSelectedGameIds([]);

    const cached = getLibraryGameListCache(queryKey);
    if (cached?.revision === libraryGamesRevision) {
      const cachedGamesByIndex = new Map(cached.gamesByIndex);
      gamesByIndexRef.current = cachedGamesByIndex;
      totalRef.current = cached.total;
      setGamesByIndex(cachedGamesByIndex);
      setTotal(cached.total);
      setHasLoadedGames(true);
      setLoadedQueryKey(queryKey);
      setLoading(false);
      // 返回页面时 visibleRangeRef 尚未由虚拟列表恢复，不能把它当作顶部
      // 预取，否则第 0 页的窗口裁剪会删掉当前深处的缓存。实际可见范围
      // 会由 VirtualGameGrid 在布局恢复后通过 handleVisibleRangeChange 请求。
      return;
    }

    gamesByIndexRef.current = new Map();
    setGamesByIndex(new Map());
    setTotal(0);
    totalRef.current = 0;
    setHasLoadedGames(false);
    setLoadedQueryKey("");
    void loadGamesWindow(0, PAGE_SIZE, { force: true, reset: true });
  }, [libraryGamesRevision, loadGamesWindow, queryKey]);

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
      <div className="mx-auto flex min-h-full max-w-8xl flex-col gap-6">
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
            statusFilterInverted={statusFilterInverted}
            onStatusFilterInvertedChange={setStatusFilterInverted}
            statusOptions={statusOptions.map(opt => ({
              ...opt,
              label: t(opt.label),
            }))}
            storageKey="library"
            onRandomGame={handleOpenRandomGame}
            randomGameDisabled={
              loading
              || isSearchSettling
              || loadedQueryKey !== queryKey
              || total === 0
            }
            randomGameLoading={isOpeningRandomGame}
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
            filterPresetMenu={(
              <GameFilterPresetMenu
                tags={selectedTags}
                excludeTags={tagFilterInverted}
                status={statusFilter}
                excludeStatus={statusFilterInverted}
                enableTagTranslation={enableTagTranslation}
                onApplyPreset={applyFilterPreset}
              />
            )}
            batchActions={(
              <>
                {platformGOOS === "windows" && (
                  <button
                    type="button"
                    aria-label={t("library.batchImportToSteam")}
                    onClick={handleBatchImportToSteam}
                    disabled={
                      selectedGameIds.length === 0 || isBatchImportingToSteam
                    }
                    className={`glass-panel flex items-center gap-2 px-3 py-2 text-sm
                          bg-white dark:bg-brand-800 border border-brand-200 dark:border-brand-700
                          rounded-lg hover:bg-brand-100 dark:hover:bg-brand-700 text-brand-700 dark:text-brand-300
                          ${
                  selectedGameIds.length === 0
                  || isBatchImportingToSteam
                    ? "opacity-50 cursor-not-allowed"
                    : ""
                  }`}
                  >
                    <div
                      className={`${
                        isBatchImportingToSteam
                          ? "i-mdi-loading animate-spin"
                          : "i-mdi-steam"
                      } text-lg`}
                    />
                  </button>
                )}
                {/* 批量更新状态 */}
                <BetterDropdownMenu
                  title={t("library.setStatus")}
                  align="end"
                  menuWidth="min-w-[130px]"
                  disabled={selectedGameIds.length === 0}
                  trigger={(
                    <div
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
                    iconSrc: potatovnIconUrl,
                    dividerBefore: true,
                    onClick: () => setImportSource("potatovn"),
                  },
                  {
                    key: "playnite",
                    label: t("library.importPlaynite"),
                    description: t("library.importPlayniteDesc"),
                    iconSrc: playniteIconUrl,
                    onClick: () => setImportSource("playnite"),
                  },
                  {
                    key: "yukihub",
                    label: t("library.importYukiHub"),
                    description: t("library.importYukiHubDesc"),
                    iconSrc: yukihubIconUrl,
                    onClick: () => setImportSource("yukihub"),
                  },
                  {
                    key: "vnite",
                    label: t("library.importVnite"),
                    description: t("library.importVniteDesc"),
                    iconSrc: vniteIconUrl,
                    onClick: () => setImportSource("vnite"),
                  },
                  {
                    key: "reinamanager",
                    label: t("library.importReinaManager"),
                    description: t("library.importReinaManagerDesc"),
                    iconSrc: reinaManagerIconUrl,
                    onClick: () => setImportSource("reinamanager"),
                  },
                  {
                    key: "steam",
                    label: t("library.importSteam"),
                    description: t("library.importSteamDesc"),
                    icon: "i-mdi-steam",
                    iconColor: "text-slate-500",
                    onClick: () => setImportSource("steam"),
                  },
                ]}
              />
            )}
          />
        </div>

        {isEmptyListWaiting ? (
          <div className="w-full" aria-busy="true">
            <LibraryGridLoadingState
              label={t("common.loading", "加载中...")}
              cardLayout={gameCardLayout}
            />
          </div>
        ) : total === 0 ? (
          <div className="flex flex-1 items-center justify-center w-full">
            <div className="flex w-full flex-col items-center justify-center py-20 text-brand-500 dark:text-brand-400">
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
                  <div className="mt-5 grid w-full max-w-lg grid-cols-2 gap-3">
                    {EMPTY_STATE_IMPORT_OPTIONS.map(option => (
                      <BetterButton
                        key={option.source}
                        variant="secondary"
                        size="lg"
                        onClick={() => setImportSource(option.source)}
                        className="w-full rounded-full border-brand-300/70 bg-brand-100/55 px-5 shadow-sm hover:-translate-y-0.5 hover:border-brand-400 hover:bg-brand-150/85 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400/70 data-glass:bg-white/8 dark:border-brand-600 dark:bg-brand-800/55 dark:hover:border-brand-500 dark:hover:bg-brand-700/70"
                      >
                        {"iconSrc" in option ? (
                          <img
                            src={option.iconSrc}
                            alt=""
                            aria-hidden="true"
                            className="h-5 w-5 shrink-0 rounded-md object-cover"
                          />
                        ) : (
                          <span
                            aria-hidden="true"
                            className={`${option.icon} shrink-0 text-xl`}
                          />
                        )}
                        <span className="truncate">{t(option.labelKey)}</span>
                      </BetterButton>
                    ))}
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
                cardLayout={gameCardLayout}
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

      <SteamBatchImportModal
        isOpen={isBatchSteamModalOpen}
        selectedCount={selectedGameIds.length}
        status={batchSteamStatus}
        isChecking={isCheckingBatchSteam}
        isImporting={isBatchImportingToSteam}
        onClose={() => setIsBatchSteamModalOpen(false)}
        onImport={() => {
          void performBatchImportToSteam();
        }}
        onRetry={handleBatchImportToSteam}
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
