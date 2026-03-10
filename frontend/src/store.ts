import { create } from "zustand";

import type { appconf, models, vo } from "../wailsjs/go/models";

import { GetAppConfig, UpdateAppConfig } from "../wailsjs/go/service/ConfigService";
import { GetGames } from "../wailsjs/go/service/GameService";
import { GetHomePageData } from "../wailsjs/go/service/HomeService";

type AISummaryCache = {
  [dimension: string]: string;
};

type AppState = {
  isSidebarOpen: boolean;
  toggleSidebar: () => void;
  setSidebarOpen: (open: boolean) => void;
  homeData: vo.HomePageData | null;
  config: appconf.AppConfig | null;
  draftConfig: appconf.AppConfig | null;
  isLoading: boolean;
  fetchHomeData: () => Promise<void>;
  fetchConfig: () => Promise<void>;
  patchLiveConfig: (patch: Partial<appconf.AppConfig>) => Promise<void>;
  setDraftConfig: (config: appconf.AppConfig) => void;
  resetDraftConfig: () => void;
  saveDraftConfig: () => Promise<void>;
  // 游戏列表全局状态
  games: models.Game[];
  gamesLoading: boolean;
  fetchGames: () => Promise<void>;
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
    void get().patchLiveConfig({ sidebar_open: newState });
  },
  setSidebarOpen: (open: boolean) => set({ isSidebarOpen: open }),
  homeData: null,
  config: null,
  draftConfig: null,
  isLoading: false,
  games: [],
  gamesLoading: false,
  fetchHomeData: async () => {
    set({ isLoading: true });
    try {
      const data = await GetHomePageData();
      set({ homeData: data });
    }
    catch (error) {
      console.error("Failed to fetch home data:", error);
    }
    finally {
      set({ isLoading: false });
    }
  },
  fetchConfig: async () => {
    try {
      const config = await GetAppConfig();
      set({ config, draftConfig: { ...config }, isSidebarOpen: config.sidebar_open });
    }
    catch (error) {
      console.error("Failed to fetch config:", error);
    }
  },
  patchLiveConfig: async (patch: Partial<appconf.AppConfig>) => {
    const previousConfig = get().config;
    const previousDraftConfig = get().draftConfig;
    if (!previousConfig) {
      return;
    }

    const nextConfig = { ...previousConfig, ...patch } as appconf.AppConfig;
    const nextDraftConfig = previousDraftConfig
      ? ({ ...previousDraftConfig, ...patch } as appconf.AppConfig)
      : ({ ...nextConfig } as appconf.AppConfig);

    set({
      config: nextConfig,
      draftConfig: nextDraftConfig,
      isSidebarOpen: nextConfig.sidebar_open,
    });

    try {
      await UpdateAppConfig(nextConfig);
    }
    catch (error) {
      set({
        config: previousConfig,
        draftConfig: previousDraftConfig,
        isSidebarOpen: previousConfig.sidebar_open,
      });
      console.error("Failed to patch live config:", error);
    }
  },
  setDraftConfig: (config: appconf.AppConfig) => {
    set({ draftConfig: config });
  },
  resetDraftConfig: () => {
    const config = get().config;
    set({ draftConfig: config ? { ...config } as appconf.AppConfig : null });
  },
  saveDraftConfig: async () => {
    const draftConfig = get().draftConfig;
    if (!draftConfig) {
      return;
    }

    try {
      await UpdateAppConfig(draftConfig);
      set({ config: { ...draftConfig }, draftConfig: { ...draftConfig }, isSidebarOpen: draftConfig.sidebar_open });
    }
    catch (error) {
      console.error("Failed to save draft config:", error);
    }
  },
  // 游戏列表管理
  fetchGames: async () => {
    set({ gamesLoading: true });
    try {
      const result = await GetGames();
      set({ games: result || [] });
    }
    catch (error) {
      console.error("Failed to fetch games:", error);
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
