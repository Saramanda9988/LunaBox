import { createRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Route as rootRoute } from './__root'
import { GetGames } from '../../wailsjs/go/service/GameService'
import { models } from '../../wailsjs/go/models'
import { GameCard } from '../components/GameCard'

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/library',
  component: LibraryComponent,
})

function LibraryComponent() {
  const [games, setGames] = useState<models.Game[]>([])
  const [isLoading, setIsLoading] = useState(true)

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

  if (isLoading) {
    return <div className="flex h-full items-center justify-center">Loading...</div>
  }

  return (
    <div className="h-full w-full flex flex-col">
      <div className="flex flex-col space-y-2">
        <h1 className="text-4xl font-bold text-gray-900 dark:text-white">游戏库</h1>
      </div>

      {games.length === 0 ? (
        <div className="flex-1 flex items-center justify-center w-full">
          <div className="flex flex-col items-center justify-center py-20 text-gray-500 dark:text-gray-400">
            <div className="i-mdi-gamepad-variant-outline text-6xl mb-4" />
            <p className="text-xl">暂无游戏</p>
            <p className="text-sm mt-2">添加一些游戏开始吧</p>
          </div>
        </div>
      ) : (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(12rem,1fr))] gap-6">
          {games.map((game) => (
            <div key={game.id} className="flex justify-center">
              <GameCard game={game} />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
