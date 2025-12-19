import { createRoute } from '@tanstack/react-router'
import { useEffect, useState, useRef } from 'react'
import { Route as rootRoute } from './__root'
import { GetGames } from '../../wailsjs/go/service/GameService'
import { models } from '../../wailsjs/go/models'
import { GameCard } from '../components/card/GameCard'
import { AddGameModal } from '../components/modal/AddGameModal'
import { ImportModal } from '../components/modal/ImportModal'
import { PlayniteImportModal } from '../components/modal/PlayniteImportModal'
import { FilterBar } from '../components/FilterBar'
import toast from 'react-hot-toast'

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/library',
  component: LibraryPage,
})

function LibraryPage() {
  const [games, setGames] = useState<models.Game[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isAddGameModalOpen, setIsAddGameModalOpen] = useState(false)
  const [isImportModalOpen, setIsImportModalOpen] = useState(false)
  const [isPlayniteImportModalOpen, setIsPlayniteImportModalOpen] = useState(false)
  const [isDropdownOpen, setIsDropdownOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [sortBy, setSortBy] = useState<'name' | 'created_at'>('created_at')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')
  const dropdownRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    loadGames()
  }, [])

  // 点击外部关闭下拉菜单
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsDropdownOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const loadGames = async () => {
    try {
      const result = await GetGames()
      setGames(result || [])
    } catch (error) {

    } finally {
      setIsLoading(false)
    }
  }

  const filteredGames = games
    .filter((game) => {
      if (!searchQuery) return true
      return game.name.toLowerCase().includes(searchQuery.toLowerCase())
    })
    .sort((a, b) => {
      let comparison = 0
      switch (sortBy) {
        case 'name':
          comparison = a.name.localeCompare(b.name)
          break
        case 'created_at':
          comparison = (a.created_at || '').localeCompare(b.created_at || '')
          break
      }
      return sortOrder === 'asc' ? comparison : -comparison
    })

  if (isLoading) {
    return <div className="flex h-full items-center justify-center">Loading...</div>
  }

  return (
    <div className="space-y-6 max-w-8xl mx-auto p-8">
      <div className="flex items-center justify-between">
        <h1 className="text-4xl font-bold text-brand-900 dark:text-white">游戏库</h1>
      </div>

      <FilterBar
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        searchPlaceholder="搜索游戏..."
        sortBy={sortBy}
        onSortByChange={(val) => setSortBy(val as any)}
        sortOptions={[
          { label: '名称', value: 'name' },
          { label: '添加时间', value: 'created_at' },
        ]}
        sortOrder={sortOrder}
        onSortOrderChange={setSortOrder}
        extraButtons={
          <button
            className="p-2 text-brand-400 cursor-not-allowed rounded-lg hover:bg-brand-100 dark:hover:bg-brand-700"
            title="筛选 (即将推出)"
            disabled
          >
            <div className="i-mdi-filter text-xl" />
          </button>
        }
        actionButton={
          <div className="relative" ref={dropdownRef}>
            <button
              onClick={() => setIsDropdownOpen(!isDropdownOpen)}
              className="flex items-center rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-4 focus:ring-blue-300 dark:bg-blue-600 dark:hover:bg-blue-700 dark:focus:ring-blue-800"
            >
              <div className="i-mdi-plus mr-2 text-lg" />
              添加游戏
              <div className="i-mdi-chevron-down ml-2 text-lg" />
            </button>

            {/* Dropdown Menu */}
            {isDropdownOpen && (
              <div className="absolute right-0 mt-2 w-56 origin-top-right rounded-lg bg-white shadow-lg ring-1 ring-black/5 dark:bg-brand-700 dark:ring-white/10 z-50">
                <div className="py-1">
                  <button
                    onClick={() => {
                      setIsAddGameModalOpen(true)
                      setIsDropdownOpen(false)
                    }}
                    className="flex w-full items-center px-4 py-3 text-sm text-brand-700 hover:bg-brand-100 dark:text-brand-200 dark:hover:bg-brand-600"
                  >
                    <div className="i-mdi-gamepad-variant mr-3 text-xl text-blue-500" />
                    <div className="text-left">
                      <div className="font-medium">手动添加</div>
                      <div className="text-xs text-brand-400 dark:text-brand-400">
                        选择可执行文件并搜索元数据
                      </div>
                    </div>
                  </button>
                  <button
                    onClick={() => {
                      setIsImportModalOpen(true)
                      setIsDropdownOpen(false)
                    }}
                    className="flex w-full items-center px-4 py-3 text-sm text-brand-700 hover:bg-brand-100 dark:text-brand-200 dark:hover:bg-brand-600"
                  >
                    <div className="i-mdi-database-import mr-3 text-xl text-green-500" />
                    <div className="text-left">
                      <div className="font-medium">从 PotatoVN 导入</div>
                      <div className="text-xs text-brand-400 dark:text-brand-400">
                        导入 PotatoVN 导出的 ZIP 文件
                      </div>
                    </div>
                  </button>
                  <button
                    onClick={() => {
                      setIsPlayniteImportModalOpen(true)
                      setIsDropdownOpen(false)
                    }}
                    className="flex w-full items-center px-4 py-3 text-sm text-brand-700 hover:bg-brand-100 dark:text-brand-200 dark:hover:bg-brand-600"
                  >
                    <div className="i-mdi-application-import mr-3 text-xl text-purple-500" />
                    <div className="text-left">
                      <div className="font-medium">从 Playnite 导入</div>
                      <div className="text-xs text-brand-400 dark:text-brand-400">
                        导入 Playnite 导出的 JSON 文件
                      </div>
                    </div>
                  </button>
                </div>
              </div>
            )}
          </div>
        }
      />

      {games.length === 0 ? (
        <div className="flex-1 flex items-center justify-center w-full">
          <div className="flex flex-col items-center justify-center py-20 text-brand-500 dark:text-brand-400">
            <div className="i-mdi-gamepad-variant-outline text-6xl mb-4" />
            <p className="text-xl">暂无游戏</p>
            <p className="text-sm mt-2">添加一些游戏开始吧</p>
            <div className="flex flex-col gap-3 mt-4">
              <button
                onClick={() => setIsImportModalOpen(true)}
                className="rounded-lg border border-green-600 px-5 py-2.5 text-sm font-medium text-green-600 hover:bg-green-50 focus:outline-none focus:ring-4 focus:ring-green-300 dark:border-green-500 dark:text-green-500 dark:hover:bg-green-900/20"
              >
                从 PotatoVN 导入
              </button>
              <button
                onClick={() => setIsPlayniteImportModalOpen(true)}
                className="rounded-lg border border-purple-600 px-5 py-2.5 text-sm font-medium text-purple-600 hover:bg-purple-50 focus:outline-none focus:ring-4 focus:ring-purple-300 dark:border-purple-500 dark:text-purple-500 dark:hover:bg-purple-900/20"
              >
                从 Playnite 导入
              </button>
            </div>
          </div>
        </div>
      ) : filteredGames.length === 0 ? (
        <div className="flex-1 flex items-center justify-center w-full text-brand-500 dark:text-brand-400">
          <div className="flex flex-col items-center">
            <div className="i-mdi-magnify text-4xl mb-2" />
            <p>未找到匹配的游戏</p>
          </div>
        </div>
      ) : (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(max(8rem,11%),1fr))] gap-3">
          {filteredGames.map((game) => (
            <GameCard key={game.id} game={game} />
          ))}
        </div>
      )}

      <AddGameModal
        isOpen={isAddGameModalOpen}
        onClose={() => setIsAddGameModalOpen(false)}
        onGameAdded={loadGames}
      />

      <ImportModal
        isOpen={isImportModalOpen}
        onClose={() => setIsImportModalOpen(false)}
        onImportComplete={loadGames}
      />

      <PlayniteImportModal
        isOpen={isPlayniteImportModalOpen}
        onClose={() => setIsPlayniteImportModalOpen(false)}
        onImportComplete={loadGames}
      />
    </div>
  )
}

