import { create } from 'zustand'
import { vo } from '../wailsjs/go/models'
import { GetHomePageData } from '../wailsjs/go/service/HomeService'

interface AppState {
  isSidebarOpen: boolean
  toggleSidebar: () => void
  homeData: vo.HomePageData | null
  isLoading: boolean
  fetchHomeData: () => Promise<void>
}

export const useAppStore = create<AppState>((set) => ({
  isSidebarOpen: true,
  toggleSidebar: () => set((state) => ({ isSidebarOpen: !state.isSidebarOpen })),
  homeData: null,
  isLoading: false,
  fetchHomeData: async () => {
    console.log('Start fetching home data...')
    set({ isLoading: true })
    try {
      const data = await GetHomePageData()
      console.log('Fetched home data:', data)
      set({ homeData: data })
    } catch (error) {
      console.error('Failed to fetch home data:', error)
    } finally {
      set({ isLoading: false })
    }
  },
}))
