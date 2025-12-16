import { Link } from '@tanstack/react-router'
import { useAppStore } from '../store'

export function SideBar() {
  const { isSidebarOpen, toggleSidebar } = useAppStore()

  const navItems = [
    { to: '/', label: '首页', icon: 'i-mdi-home' },
    { to: '/library', label: '游戏库', icon: 'i-mdi-gamepad-variant' },
    { to: '/stats', label: '统计', icon: 'i-mdi-chart-bar' },
    { to: '/categories', label: '收藏', icon: 'i-mdi-format-list-bulleted' },
  ]

  return (
    <aside
      className={`flex flex-col bg-white dark:bg-brand-800 transition-all duration-300 border-r border-brand-200 dark:border-brand-700 ${
        isSidebarOpen ? 'w-64' : 'w-16'
      }`}
    >
      <div className={`flex items-center h-16 border-b border-brand-200 dark:border-brand-700 ${isSidebarOpen ? 'justify-between px-4' : 'justify-center'}`}>
        {isSidebarOpen && <span className="text-xl font-bold">LunaBox</span>}
        <button
          onClick={toggleSidebar}
          className="p-2 rounded hover:bg-brand-100 dark:hover:bg-brand-700 focus:outline-none"
        >
          <div className="i-mdi-menu text-xl" />
        </button>
      </div>

      <nav className="flex-1 py-4">
        <ul className="space-y-2 px-2">
          {navItems.map((item) => (
            <li key={item.to}>
              <Link
                to={item.to}
                className={`flex items-center p-2 rounded hover:bg-brand-100 dark:hover:bg-brand-700 text-brand-700 dark:text-brand-300 no-underline [&.active]:bg-brand-200 [&.active]:text-brand-900 dark:[&.active]:bg-brand-700 dark:[&.active]:text-brand-100 ${isSidebarOpen ? '' : 'justify-center'}`}
              >
                <div className={`${item.icon} text-xl`} />
                {isSidebarOpen && <span className="ml-3">{item.label}</span>}
              </Link>
            </li>
          ))}
        </ul>
      </nav>

      <div className={`p-4 border-t border-brand-200 dark:border-brand-700 ${isSidebarOpen ? '' : 'flex justify-center'}`}>
        <Link
          to="/settings"
          className={`flex items-center rounded hover:bg-brand-100 dark:hover:bg-brand-700 p-2 text-brand-700 dark:text-brand-300 no-underline [&.active]:bg-brand-200 [&.active]:text-brand-900 dark:[&.active]:bg-brand-700 dark:[&.active]:text-brand-100 ${isSidebarOpen ? 'w-full' : ''}`}
        >
          <div className="i-mdi-cog text-xl" />
          {isSidebarOpen && <span className="ml-3">设置</span>}
        </Link>
      </div>
    </aside>
  )
}
