import { RouterProvider, createRouter } from '@tanstack/react-router'
import { useEffect } from 'react'
import { useAppStore } from './store'
import { Route as rootRoute } from './routes/__root'
import { Route as indexRoute } from './routes/index'
import { Route as libraryRoute } from './routes/library'
import { Route as gameRoute } from './routes/game'
import { Route as statsRoute } from './routes/stats'
import { Route as categoriesRoute } from './routes/categories'
import { Route as categoryRoute } from "./routes/category";
import { Route as settingsRoute } from './routes/settings'

const routeTree = rootRoute.addChildren([indexRoute, libraryRoute, gameRoute, statsRoute, categoriesRoute, categoryRoute, settingsRoute])

const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

function App() {
  const { config, fetchConfig } = useAppStore()

  useEffect(() => {
    fetchConfig()
  }, [fetchConfig])

  useEffect(() => {
    if (!config) return

    const root = window.document.documentElement
    const applyTheme = (theme: string) => {
      root.classList.remove('light', 'dark')
      root.classList.add(theme)
    }

    if (config.theme === 'system') {
      const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
      applyTheme(mediaQuery.matches ? 'dark' : 'light')

      const handler = (e: MediaQueryListEvent) => {
        applyTheme(e.matches ? 'dark' : 'light')
      }

      mediaQuery.addEventListener('change', handler)
      return () => mediaQuery.removeEventListener('change', handler)
    } else {
      applyTheme(config.theme)
    }
  }, [config?.theme])

  return <RouterProvider router={router} />
}

export default App
