import { createRootRoute, Outlet } from '@tanstack/react-router'
import { SideBar } from '../components/SideBar'

export const Route = createRootRoute({
  component: () => (
    <div className="flex h-screen w-full bg-gray-100 dark:bg-gray-900 text-gray-900 dark:text-gray-100 overflow-hidden">
      <SideBar />
      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  ),
})
