import { create } from 'zustand'
import { vo, appconf } from '../wailsjs/go/models'
import { GetHomePageData } from '../wailsjs/go/service/HomeService'
import { GetAppConfig, UpdateAppConfig } from '../wailsjs/go/service/ConfigService'

interface AISummaryCache {
  [dimension: string]: string
}

interface AppState {
  isSidebarOpen: boolean
  toggleSidebar: () => void
  setSidebarOpen: (open: boolean) => void
  homeData: vo.HomePageData | null
  config: appconf.AppConfig | null
  isLoading: boolean
  fetchHomeData: () => Promise<void>
  fetchConfig: () => Promise<void>
  updateConfig: (config: appconf.AppConfig) => Promise<void>
  // AI Summary 缓存
  aiSummaryCache: AISummaryCache
  setAISummary: (dimension: string, summary: string) => void
  getAISummary: (dimension: string) => string | undefined
}

export const useAppStore = create<AppState>((set, get) => ({
  isSidebarOpen: true,
  toggleSidebar: () => {
    const newState = !get().isSidebarOpen
    set({ isSidebarOpen: newState })
    // 保存到配置
    const config = get().config
    if (config) {
      const newConfig = { ...config, sidebar_open: newState }
      UpdateAppConfig(newConfig).catch(console.error)
    }
  },
  setSidebarOpen: (open: boolean) => set({ isSidebarOpen: open }),
  homeData: null,
  config: null,
  isLoading: false,
  fetchHomeData: async () => {
    set({ isLoading: true })
    try {
      const data = await GetHomePageData()
      set({ homeData: data })
    } catch (error) {
      console.error('Failed to fetch home data:', error)
    } finally {
      set({ isLoading: false })
    }
  },
  fetchConfig: async () => {
    try {
      const config = await GetAppConfig()
      set({ config, isSidebarOpen: config.sidebar_open })
    } catch (error) {
      console.error('Failed to fetch config:', error)
    }
  },
  updateConfig: async (config: appconf.AppConfig) => {
    try {
      await UpdateAppConfig(config)
      set({ config })
    } catch (error) {
      console.error('Failed to update config:', error)
    }
  },
  // AI Summary 缓存
  aiSummaryCache: {},
  setAISummary: (dimension: string, summary: string) => {
    set((state) => ({
      aiSummaryCache: { ...state.aiSummaryCache, [dimension]: summary },
    }))
  },
  getAISummary: (dimension: string) => {
    return undefined // 这个方法不需要，直接用 selector 访问
  },
}))
