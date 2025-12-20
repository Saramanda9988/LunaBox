import { createRootRoute, Outlet } from '@tanstack/react-router'
import { SideBar } from '../components/bar/SideBar'

export const Route = createRootRoute({
  component: () => (
    <div className="flex h-screen w-full bg-brand-100 dark:bg-brand-900 text-brand-900 dark:text-brand-100 overflow-hidden">
      <SideBar />
      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  ),
})
