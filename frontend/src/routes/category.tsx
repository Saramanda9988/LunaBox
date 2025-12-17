import { createRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Route as rootRoute } from './__root'
import {
  AddGameToCategory,
  GetCategoryByID,
  GetGamesByCategory, 
  RemoveGameFromCategory
} from '../../wailsjs/go/service/CategoryService'
import {models, vo} from '../../wailsjs/go/models'
import {GetGames} from "../../wailsjs/go/service/GameService";
import {GameCard} from "../components/card/GameCard";
import {AddGameToCategoryModal} from "../components/modal/AddGameToCategoryModal";
import { FilterBar } from '../components/FilterBar'
import { toast } from 'react-hot-toast'

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/categories/$categoryId',
  component: CategoryDetailPage,
})

function CategoryDetailPage() {
  const navigate = useNavigate()
  const { categoryId } = Route.useParams()
  const [category, setCategory] = useState<vo.CategoryVO | null>(null)
  const [games, setGames] = useState<models.Game[]>([])
  const [isAddGameModalOpen, setIsAddGameModalOpen] = useState(false)
  const [allGames, setAllGames] = useState<models.Game[]>([])
  const [searchQuery, setSearchQuery] = useState('')
  const [sortBy, setSortBy] = useState<'name' | 'created_at'>('created_at')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')

  useEffect(() => {
    if (categoryId) {
      loadCategory(categoryId)
      loadGames(categoryId)
    }
  }, [categoryId])

  const loadCategory = async (id: string) => {
    try {
      const result = await GetCategoryByID(id)
      setCategory(result)
    } catch (error) {
      console.error('Failed to load category:', error)
      toast.error('加载收藏夹失败')
    }
  }

  const loadGames = async (id: string) => {
    try {
      const result = await GetGamesByCategory(id)
      setGames(result || [])
    } catch (error) {
      console.error('Failed to load games for category:', error)
      toast.error('加载文件夹中游戏失败')
    }
  }

  const onBack = () => {
    navigate({ to: '/categories' })
  }

  const handleRemoveGame = async (gameId: string) => {
    if (!category) return
    try {
      await RemoveGameFromCategory(gameId, category.id)
      await loadGames(category.id)
      await loadCategory(category.id)
    } catch (error) {
      console.error('Failed to remove game from category:', error)
      toast.error('从收藏夹中移除游戏失败')
    }
  }

  const openAddGameModal = async () => {
    try {
      const result = await GetGames()
      const currentGameIds = new Set(games.map(g => g.id))
      setAllGames(result.filter(g => !currentGameIds.has(g.id)) || [])
      setIsAddGameModalOpen(true)
    } catch (error) {
      console.error('Failed to load all games:', error)
      toast.error('加载库中所有游戏失败')
    }
  }

  const handleAddGameToCategory = async (gameId: string) => {
    if (!category) return
    try {
      await AddGameToCategory(gameId, category.id)
      setAllGames(prev => prev.filter(g => g.id !== gameId))
      await loadGames(category.id)
      await loadCategory(category.id)
    } catch (error) {
      console.error('Failed to add game to category:', error)
      toast.error('添加游戏到收藏夹失败')
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

  if (!category) {
    return <div className="flex h-full items-center justify-center">Loading...</div>
  }

  return (
      <div className="h-full w-full overflow-y-auto p-8">
        {/* Back Button */}
        <button
            onClick={onBack}
            className="flex rounded-md items-center text-brand-600 hover:text-brand-900 dark:text-brand-400 dark:hover:text-brand-200 transition-colors mb-6"
        >
          <div className="i-mdi-arrow-left text-2xl mr-1" />
          <span>返回</span>
        </button>

        <div className="flex flex-col gap-6">
          <div className="flex justify-between items-center">
            <div>
              <h1 className="text-4xl font-bold text-brand-900 dark:text-white flex items-center gap-3">
                {category.name}
                {category.is_system && <span className="text-sm bg-blue-100 text-blue-800 px-2 py-1 rounded-md dark:bg-blue-900 dark:text-blue-300 align-middle">系统</span>}
              </h1>
              <p className="text-brand-500 dark:text-brand-400 mt-2">
                共 {games.length} 个游戏
              </p>
            </div>
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
            actionButton={
              <button
                onClick={openAddGameModal}
                className="flex items-center rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-4 focus:ring-blue-300 dark:bg-blue-600 dark:hover:bg-blue-700 dark:focus:ring-blue-800"
              >
                <div className="i-mdi-plus mr-2 text-lg" />
                添加游戏
              </button>
            }
          />
        </div>

        <div className="mt-6">
          {games.length > 0 ? (
              filteredGames.length > 0 ? (
                  <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-8">
                    {filteredGames.map((game) => (
                        <div key={game.id} className="relative group">
                          <GameCard game={game} />
                          <button
                              onClick={(e) => {
                                e.stopPropagation()
                                handleRemoveGame(game.id)
                              }}
                              className="absolute top-2 right-2 p-1 bg-red-500 text-white rounded-full opacity-0 group-hover:opacity-100 transition-opacity shadow-md hover:bg-red-600"
                              title="从收藏夹移除"
                          >
                            <div className="i-mdi-close text-sm" />
                          </button>
                        </div>
                    ))}
                  </div>
              ) : (
                  <div className="flex flex-col items-center justify-center h-64 text-brand-500 dark:text-brand-400">
                    <div className="i-mdi-magnify text-6xl mb-4" />
                    <p className="text-lg">未找到匹配的游戏</p>
                  </div>
              )
          ) : (
              <div className="flex flex-col items-center justify-center h-64 text-brand-500 dark:text-brand-400">
                <div className="i-mdi-gamepad-variant-outline text-6xl mb-4" />
                <p className="text-lg">这个收藏夹还没有游戏</p>
                <button
                    onClick={openAddGameModal}
                    className="mt-4 text-blue-600 hover:underline dark:text-blue-400"
                >
                  添加游戏
                </button>
              </div>
          )}
        </div>

        <AddGameToCategoryModal
          isOpen={isAddGameModalOpen}
          allGames={allGames}
          onClose={() => setIsAddGameModalOpen(false)}
          onAddGame={handleAddGameToCategory}
        />
      </div>
  )
}


