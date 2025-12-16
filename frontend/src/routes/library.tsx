import { createRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Route as rootRoute } from './__root'
import { GetGames } from '../../wailsjs/go/service/GameService'
import { models } from '../../wailsjs/go/models'
import { GameCard } from '../components/card/GameCard'
import { AddGameModal } from '../components/modal/AddGameModal'

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/library',
  component: LibraryPage,
})

function LibraryPage() {
  const [games, setGames] = useState<models.Game[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isAddGameModalOpen, setIsAddGameModalOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [sortBy, setSortBy] = useState<'name' | 'created_at'>('created_at')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')

  useEffect(() => {
    loadGames()
  }, [])

  const loadGames = async () => {
    try {
      const result = await GetGames()
      setGames(result || [])
    } catch (error) {
      console.error('Failed to load games:', error)
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
    <div className="h-full w-full flex flex-col p-8">
      <div className="flex items-center justify-between">
        <h1 className="text-4xl font-bold text-gray-900 dark:text-white">游戏库</h1>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-4 my-4">
        <div className="relative flex-1 max-w-md">
          <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none">
            <div className="i-mdi-magnify text-gray-500" />
          </div>
          <input
            type="text"
            className="block w-auto p-2 pl-10 text-sm text-gray-900 border border-gray-300 rounded-lg bg-gray-50 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500"
            placeholder="搜索游戏..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        </div>

        <div className="flex items-center gap-2">
          <select
            value={sortBy}
            onChange={(e) => setSortBy(e.target.value as any)}
            className="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block p-2 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500"
          >
            <option value="name">名称</option>
            <option value="created_at">添加时间</option>
          </select>

          <button
            onClick={() => setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc')}
            className="p-2 text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700"
            title={sortOrder === 'asc' ? '升序' : '降序'}
          >
            <div className={sortOrder === 'asc' ? "i-mdi-sort-ascending text-xl" : "i-mdi-sort-descending text-xl"} />
          </button>

          <button
            className="p-2 text-gray-400 cursor-not-allowed rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700"
            title="筛选 (即将推出)"
            disabled
          >
            <div className="i-mdi-filter text-xl" />
          </button>

          <button
            onClick={() => setIsAddGameModalOpen(true)}
            className="flex items-center rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-4 focus:ring-blue-300 dark:bg-blue-600 dark:hover:bg-blue-700 dark:focus:ring-blue-800"
          >
            <div className="i-mdi-plus mr-2 text-lg" />
            添加游戏
          </button>
        </div>
      </div>

      {games.length === 0 ? (
        <div className="flex-1 flex items-center justify-center w-full">
          <div className="flex flex-col items-center justify-center py-20 text-gray-500 dark:text-gray-400">
            <div className="i-mdi-gamepad-variant-outline text-6xl mb-4" />
            <p className="text-xl">暂无游戏</p>
            <p className="text-sm mt-2">添加一些游戏开始吧</p>
            <button
              onClick={() => setIsAddGameModalOpen(true)}
              className="mt-4 rounded-lg bg-blue-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-4 focus:ring-blue-300 dark:bg-blue-600 dark:hover:bg-blue-700 dark:focus:ring-blue-800"
            >
              立即添加
            </button>
          </div>
        </div>
      ) : filteredGames.length === 0 ? (
        <div className="flex-1 flex items-center justify-center w-full text-gray-500 dark:text-gray-400">
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
    </div>
  )
}
