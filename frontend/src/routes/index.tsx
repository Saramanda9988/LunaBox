import { createRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useAppStore } from '../store'
import { GameCard } from '../components/GameCard'
import { Route as rootRoute } from './__root'
import { models } from '../../wailsjs/go/models'

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: HomeComponent,
})

function HomeComponent() {
  const { homeData, fetchHomeData, isLoading } = useAppStore()
  const [showWeekly, setShowWeekly] = useState(false)

  useEffect(() => {
    fetchHomeData()
  }, [fetchHomeData])

  if (isLoading) {
    return <div className="flex h-full items-center justify-center">Loading...</div>
  }

  if (!homeData) {
    return (
      <div className="flex h-full flex-col items-center justify-center space-y-4">
        <p className="text-gray-500">暂无数据</p>
        <button 
          onClick={() => fetchHomeData()}
          className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 transition-colors"
        >
          重试
        </button>
      </div>
    )
  }

  const formatTime = (seconds: number) => {
    // TODO: format with minutes and hours properly
    const hours = Math.floor(seconds / 3600)
    return `${hours}小时`
  }

  return (
    <div className="space-y-8">
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-4xl font-bold text-gray-900 dark:text-white">首页</h1>
          <p className="mt-2 text-gray-500 dark:text-gray-400">欢迎回来</p>
        </div>
        <div 
          className="flex items-center space-x-2 bg-white dark:bg-gray-800 p-4 rounded-lg shadow-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors select-none"
          onClick={() => setShowWeekly(!showWeekly)}
          title="点击切换今日/本周"
        >
          <div className="i-mdi-swap-horizontal text-2xl text-gray-900 dark:text-white" />
          <div>
            <div className="text-sm font-medium text-gray-900 dark:text-white">
              {showWeekly ? '本周游玩时间:' : '今日游玩时间:'}
            </div>
            <div className="text-sm text-gray-500 dark:text-gray-400">
              {formatTime(showWeekly ? homeData.weekly_play_time_sec : homeData.today_play_time_sec)}
            </div>
          </div>
        </div>
      </div>

      <section>
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-4">最近游玩</h2>
        <div className="flex flex-wrap gap-6">
          {homeData.recent_games && homeData.recent_games.length > 0 ? (
            homeData.recent_games.map((game: models.Game) => (
              <GameCard key={game.id} game={game} />
            ))
          ) : (
            <p className="text-gray-500">暂无最近游玩记录</p>
          )}
        </div>
      </section>
      
      {/* Can add Recently Added section if needed, based on models */}
       <section>
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-4">最近添加</h2>
        <div className="flex flex-wrap gap-6">
          {homeData.recently_added && homeData.recently_added.length > 0 ? (
            homeData.recently_added.map((game: models.Game) => (
              <GameCard key={game.id} game={game} />
            ))
          ) : (
            <p className="text-gray-500">暂无最近添加记录</p>
          )}
        </div>
      </section>
    </div>
  )
}
