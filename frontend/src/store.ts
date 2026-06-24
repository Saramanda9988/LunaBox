import { create } from "zustand";

import type { appconf, launcher, models, vo } from "../wailsjs/go/models";

import { enums } from "../wailsjs/go/models";
import {
  GetAppConfig,
  UpdateAppConfig,
} from "../wailsjs/go/service/ConfigService";
import { GetGames } from "../wailsjs/go/service/GameService";
import { GetHomePageData } from "../wailsjs/go/service/HomeService";
import {
  StartGameWithOptions,
  StartGameWithTracking,
} from "../wailsjs/go/service/StartService";
import { GetGOOS } from "../wailsjs/go/service/VersionService";

type AISummaryCache = {
  [dimension: string]: string;
};

export type GameRuntimeState = "idle" | "launching" | "playing" | "ending";

export type GameRuntimeInfo = {
  game: models.Game | null;
  gameId: string;
  sessionId: string;
  startTime: unknown;
  state: GameRuntimeState;
};

export type GameRuntimeMap = Record<string, GameRuntimeInfo>;

export type GameRuntimeChangedEvent = {
  game?: models.Game | null;
  game_id?: string;
  session_id?: string;
  start_time?: unknown;
  state?: GameRuntimeState;
  reason?: string;
};

export type FetchHomeDataOptions = {
  showLoading?: boolean;
  syncRuntime?: boolean;
};

function normalizeLibraryTags(tags: string[]) {
  return [...new Set(tags.map(tag => tag.trim()).filter(Boolean))];
}

function areStringArraysEqual(left: string[], right: string[]) {
  return (
    left.length === right.length
    && left.every((value, index) => value === right[index])
  );
}

function withSidebarState(
  config: appconf.AppConfig,
  sidebarOpen: boolean,
): appconf.AppConfig {
  return { ...config, sidebar_open: sidebarOpen };
}

export function isGameRuntimeVisible(runtime?: GameRuntimeInfo | null) {
  return (
    runtime?.state === "launching"
    || runtime?.state === "playing"
    || runtime?.state === "ending"
  );
}

function getRuntimeStartTime(runtime: GameRuntimeInfo) {
  const timestamp = Date.parse(String(runtime.startTime ?? ""));
  return Number.isFinite(timestamp) ? timestamp : 0;
}

function getVisibleGameRuntimes(gameRuntimes: GameRuntimeMap) {
  return Object.values(gameRuntimes).filter(isGameRuntimeVisible);
}

function pickGameRuntime(
  gameRuntimes: GameRuntimeMap,
  preferredGameId: string,
): GameRuntimeInfo | undefined {
  if (preferredGameId && isGameRuntimeVisible(gameRuntimes[preferredGameId])) {
    return gameRuntimes[preferredGameId];
  }

  const visibleRuntimes = getVisibleGameRuntimes(gameRuntimes);
  if (visibleRuntimes.length === 0) {
    return undefined;
  }

  return [...visibleRuntimes].sort(
    (left, right) => getRuntimeStartTime(right) - getRuntimeStartTime(left),
  )[0];
}

function runtimeSelectionPatch(
  gameRuntimes: GameRuntimeMap,
  preferredGameId: string,
) {
  const gameRuntime = pickGameRuntime(gameRuntimes, preferredGameId);

  return {
    activeGameRuntimeId: gameRuntime?.gameId ?? "",
  };
}

type AppState = {
  isSidebarOpen: boolean;
  toggleSidebar: () => void;
  setSidebarOpen: (open: boolean) => void;
  homeData: vo.HomePageData | null;
  config: appconf.AppConfig | null;
  draftConfig: appconf.AppConfig | null;
  platformGOOS: string;
  isLoading: boolean;
  gameRuntimes: GameRuntimeMap;
  activeGameRuntimeId: string;
  fetchHomeData: (options?: FetchHomeDataOptions) => Promise<void>;
  fetchConfig: () => Promise<void>;
  fetchPlatformGOOS: () => Promise<void>;
  applyGameRuntimeEvent: (event: GameRuntimeChangedEvent) => void;
  setGameRuntimeFromHome: (recentPlayed: vo.LastPlayedGame[] | null) => void;
  selectNextGameRuntime: () => void;
  startGame: (
    game: models.Game,
    options?: launcher.LaunchOptions,
  ) => Promise<boolean>;
  patchLiveConfig: (patch: Partial<appconf.AppConfig>) => Promise<void>;
  applyCloudSyncStatus: (status: vo.CloudSyncStatus) => void;
  setDraftConfig: (config: appconf.AppConfig) => void;
  resetDraftConfig: () => void;
  saveDraftConfig: () => Promise<void>;
  // 游戏列表全局状态
  games: models.Game[];
  gamesLoading: boolean;
  librarySelectedTags: string[];
  fetchGames: (
    request?: Partial<vo.GameListRequest>,
  ) => Promise<vo.GameListResponse | null>;
  setLibrarySelectedTags: (tags: string[]) => void;
  // AI Summary 缓存
  aiSummaryCache: AISummaryCache;
  setAISummary: (dimension: string, summary: string) => void;
  getAISummary: (dimension: string) => string | undefined;
};

export const useAppStore = create<AppState>((set, get) => ({
  isSidebarOpen: true,
  toggleSidebar: () => {
    const newState = !get().isSidebarOpen;
    set({ isSidebarOpen: newState });
    const config = get().config;
    if (!config) {
      return;
    }

    void UpdateAppConfig(withSidebarState(config, newState)).catch((error) => {
      console.error("Failed to persist sidebar state:", error);
    });
  },
  setSidebarOpen: (open: boolean) => set({ isSidebarOpen: open }),
  homeData: null,
  config: null,
  draftConfig: null,
  platformGOOS: "",
  isLoading: false,
  gameRuntimes: {},
  activeGameRuntimeId: "",
  games: [],
  gamesLoading: false,
  librarySelectedTags: [],
  fetchHomeData: async (options = {}) => {
    const showLoading = options.showLoading !== false;
    if (showLoading) {
      set({ isLoading: true });
    }

    try {
      const data = await GetHomePageData();
      set({ homeData: data });
      if (options.syncRuntime !== false) {
        get().setGameRuntimeFromHome(data?.recent_played ?? null);
      }
    }
    catch (error) {
      console.error("Failed to fetch home data:", error);
    }
    finally {
      if (showLoading) {
        set({ isLoading: false });
      }
    }
  },
  fetchConfig: async () => {
    try {
      const config = await GetAppConfig();
      set({
        config,
        draftConfig: { ...config },
        isSidebarOpen: config.sidebar_open,
      });
    }
    catch (error) {
      console.error("Failed to fetch config:", error);
    }
  },
  fetchPlatformGOOS: async () => {
    try {
      const goos = await GetGOOS();
      set({ platformGOOS: goos });
    }
    catch (error) {
      console.error("Failed to fetch platform GOOS:", error);
    }
  },
  applyGameRuntimeEvent: (event: GameRuntimeChangedEvent) => {
    const state = event.state ?? "idle";
    const gameId = event.game_id ?? event.game?.id ?? "";

    if (!gameId) {
      if (state === "idle") {
        set({
          activeGameRuntimeId: "",
          gameRuntimes: {},
        });
      }
      return;
    }

    if (state === "idle") {
      set((currentState) => {
        const currentRuntime = currentState.gameRuntimes[gameId];
        if (
          event.session_id
          && currentRuntime?.sessionId
          && currentRuntime.sessionId !== event.session_id
        ) {
          return currentState;
        }

        const nextGameRuntimes = { ...currentState.gameRuntimes };
        delete nextGameRuntimes[gameId];
        const preferredGameId
          = currentState.activeGameRuntimeId === gameId
            ? ""
            : currentState.activeGameRuntimeId;

        return {
          gameRuntimes: nextGameRuntimes,
          ...runtimeSelectionPatch(nextGameRuntimes, preferredGameId),
        };
      });
      return;
    }

    set((currentState) => {
      const currentRuntime = currentState.gameRuntimes[gameId];
      const nextRuntime: GameRuntimeInfo = {
        game: event.game ?? currentRuntime?.game ?? null,
        gameId,
        sessionId: event.session_id ?? currentRuntime?.sessionId ?? "",
        startTime: event.start_time ?? currentRuntime?.startTime ?? null,
        state,
      };
      const nextGameRuntimes = {
        ...currentState.gameRuntimes,
        [gameId]: nextRuntime,
      };
      const preferredGameId
        = state === "launching"
          || !isGameRuntimeVisible(
            currentState.gameRuntimes[currentState.activeGameRuntimeId],
          )
          ? gameId
          : currentState.activeGameRuntimeId;

      return {
        gameRuntimes: nextGameRuntimes,
        ...runtimeSelectionPatch(nextGameRuntimes, preferredGameId),
      };
    });
  },
  setGameRuntimeFromHome: (recentPlayed: vo.LastPlayedGame[] | null) => {
    const playingItems = (recentPlayed ?? []).filter(
      item => item.is_playing && item.game?.id,
    );

    set((state) => {
      if (playingItems.length === 0) {
        if (getVisibleGameRuntimes(state.gameRuntimes).length === 0) {
          return state;
        }

        return {
          gameRuntimes: {},
          ...runtimeSelectionPatch({}, ""),
        };
      }

      const nextGameRuntimes: GameRuntimeMap = {};
      for (const item of playingItems) {
        const game = item.game;
        const currentRuntime = state.gameRuntimes[game.id];
        nextGameRuntimes[game.id] = {
          game,
          gameId: game.id,
          sessionId: currentRuntime?.sessionId ?? "",
          startTime: item.last_played_at,
          state:
            currentRuntime?.state === "launching"
            || currentRuntime?.state === "ending"
              ? currentRuntime.state
              : "playing",
        };
      }

      return {
        gameRuntimes: nextGameRuntimes,
        ...runtimeSelectionPatch(nextGameRuntimes, state.activeGameRuntimeId),
      };
    });
  },
  selectNextGameRuntime: () => {
    set((state) => {
      const runtimes = getVisibleGameRuntimes(state.gameRuntimes);
      if (runtimes.length <= 1) {
        return state;
      }

      const currentIndex = runtimes.findIndex(
        runtime => runtime.gameId === state.activeGameRuntimeId,
      );
      const nextRuntime = runtimes[(currentIndex + 1) % runtimes.length];

      return runtimeSelectionPatch(state.gameRuntimes, nextRuntime.gameId);
    });
  },
  startGame: async (game: models.Game, options?: launcher.LaunchOptions) => {
    const gameId = game.id;
    if (!gameId) {
      return false;
    }

    const previousGameRuntimes = get().gameRuntimes;
    const previousActiveGameRuntimeId = get().activeGameRuntimeId;
    const previousRuntime = previousGameRuntimes[gameId];
    if (isGameRuntimeVisible(previousRuntime)) {
      set(runtimeSelectionPatch(previousGameRuntimes, gameId));
      return true;
    }

    const optimisticStartTime = new Date().toISOString();
    const optimisticRuntime: GameRuntimeInfo = {
      game,
      gameId,
      sessionId: previousRuntime?.sessionId ?? "",
      startTime: optimisticStartTime,
      state: "launching",
    };
    const optimisticGameRuntimes = {
      ...previousGameRuntimes,
      [gameId]: optimisticRuntime,
    };
    set({
      gameRuntimes: optimisticGameRuntimes,
      ...runtimeSelectionPatch(optimisticGameRuntimes, gameId),
    });

    const rollbackOptimisticRuntime = () => {
      set((state) => {
        const runtime = state.gameRuntimes[gameId];
        if (
          !runtime
          || runtime.startTime !== optimisticStartTime
          || runtime.state !== "launching"
        ) {
          return state;
        }

        const nextGameRuntimes = { ...state.gameRuntimes };
        if (previousRuntime) {
          nextGameRuntimes[gameId] = previousRuntime;
        }
        else {
          delete nextGameRuntimes[gameId];
        }

        return {
          gameRuntimes: nextGameRuntimes,
          ...runtimeSelectionPatch(
            nextGameRuntimes,
            previousActiveGameRuntimeId,
          ),
        };
      });
    };

    try {
      const started = options
        ? await StartGameWithOptions(gameId, options)
        : await StartGameWithTracking(gameId);

      if (!started) {
        rollbackOptimisticRuntime();
      }

      return started;
    }
    catch (error) {
      rollbackOptimisticRuntime();
      throw error;
    }
  },
  patchLiveConfig: async (patch: Partial<appconf.AppConfig>) => {
    const previousConfig = get().config;
    const previousDraftConfig = get().draftConfig;
    if (!previousConfig) {
      return;
    }

    const nextSidebarOpen
      = typeof patch.sidebar_open === "boolean"
        ? patch.sidebar_open
        : get().isSidebarOpen;
    const nextConfig = withSidebarState(
      { ...previousConfig, ...patch } as appconf.AppConfig,
      nextSidebarOpen,
    );
    const nextDraftConfig = previousDraftConfig
      ? withSidebarState(
          { ...previousDraftConfig, ...patch } as appconf.AppConfig,
          nextSidebarOpen,
        )
      : ({ ...nextConfig } as appconf.AppConfig);

    set({
      config: nextConfig,
      draftConfig: nextDraftConfig,
      isSidebarOpen: nextSidebarOpen,
    });

    try {
      await UpdateAppConfig(nextConfig);
    }
    catch (error) {
      set({
        config: previousConfig,
        draftConfig: previousDraftConfig,
        isSidebarOpen: get().isSidebarOpen,
      });
      console.error("Failed to patch live config:", error);
    }
  },
  applyCloudSyncStatus: (status: vo.CloudSyncStatus) => {
    set((state) => {
      if (!state.config && !state.draftConfig) {
        return state;
      }

      const patch: Partial<appconf.AppConfig> = {
        last_cloud_sync_time: status.last_sync_time,
        last_cloud_sync_status: status.last_sync_status,
        last_cloud_sync_error: status.last_sync_error,
      };

      return {
        config: state.config
          ? ({ ...state.config, ...patch } as appconf.AppConfig)
          : null,
        draftConfig: state.draftConfig
          ? ({ ...state.draftConfig, ...patch } as appconf.AppConfig)
          : null,
      };
    });
  },
  setDraftConfig: (config: appconf.AppConfig) => {
    set({ draftConfig: config });
  },
  resetDraftConfig: () => {
    const config = get().config;
    const sidebarOpen = get().isSidebarOpen;
    set({
      draftConfig: config
        ? withSidebarState({ ...config } as appconf.AppConfig, sidebarOpen)
        : null,
    });
  },
  saveDraftConfig: async () => {
    const draftConfig = get().draftConfig;
    if (!draftConfig) {
      return;
    }

    const sidebarOpen = get().isSidebarOpen;
    const nextConfig = withSidebarState(
      { ...draftConfig } as appconf.AppConfig,
      sidebarOpen,
    );

    try {
      await UpdateAppConfig(nextConfig);
      set({
        config: nextConfig,
        draftConfig: { ...nextConfig },
        isSidebarOpen: sidebarOpen,
      });
    }
    catch (error) {
      console.error("Failed to save draft config:", error);
    }
  },
  // 游戏列表管理
  setLibrarySelectedTags: (tags: string[]) => {
    const nextTags = normalizeLibraryTags(tags);
    if (areStringArraysEqual(get().librarySelectedTags, nextTags)) {
      return;
    }
    set({ librarySelectedTags: nextTags });
  },
  fetchGames: async (request = {}) => {
    set({ gamesLoading: true });
    try {
      const result = await GetGames({
        limit: 120,
        offset: 0,
        search_query: "",
        tags: [],
        sort_by: enums.GameListSortBy.CREATED_AT,
        sort_order: enums.SortOrder.DESC,
        ...request,
      });
      set({ games: result?.games || [] });
      return result;
    }
    catch (error) {
      console.error("Failed to fetch games:", error);
      return null;
    }
    finally {
      set({ gamesLoading: false });
    }
  },
  // AI Summary 缓存
  aiSummaryCache: {},
  setAISummary: (dimension: string, summary: string) => {
    set(state => ({
      aiSummaryCache: { ...state.aiSummaryCache, [dimension]: summary },
    }));
  },
  getAISummary: () => {
    return undefined; // 这个方法不需要，直接用 selector 访问
  },
}));
