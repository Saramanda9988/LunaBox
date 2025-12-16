import { RouterProvider, createRouter } from '@tanstack/react-router'
import { Route as rootRoute } from './routes/__root'
import { Route as indexRoute } from './routes/index'
import { Route as libraryRoute } from './routes/library'
import { Route as gameRoute } from './routes/game'
import { Route as statsRoute } from './routes/stats'

const routeTree = rootRoute.addChildren([indexRoute, libraryRoute, gameRoute, statsRoute])

const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

function App() {
  return <RouterProvider router={router} />
}

export default App
