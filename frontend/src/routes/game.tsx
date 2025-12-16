import { createRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Route as rootRoute } from './__root'
import { useChartTheme } from '../hooks/useChartTheme'
import { GetGameByID, UpdateGame, SelectGameExecutable, DeleteGame } from '../../wailsjs/go/service/GameService'
import { GetGameStats } from '../../wailsjs/go/service/StatsService'
import { models, vo } from '../../wailsjs/go/models'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
} from 'chart.js'
import { Line } from 'react-chartjs-2'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend
)

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/game/$gameId',
  component: GameDetailPage,
})

function GameDetailPage() {
  const { textColor, gridColor } = useChartTheme()
  const navigate = useNavigate()
  const { gameId } = Route.useParams()
  const [game, setGame] = useState<models.Game | null>(null)
  const [stats, setStats] = useState<vo.GameDetailStats | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [activeTab, setActiveTab] = useState('stats')

  useEffect(() => {
    const loadData = async () => {
      try {
        const [gameData, statsData] = await Promise.all([
          GetGameByID(gameId),
          GetGameStats(gameId),
        ])
        setGame(gameData)
        setStats(statsData)
      } catch (error) {
        console.error('Failed to load game data:', error)
      } finally {
        setIsLoading(false)
      }
    }
    loadData()
  }, [gameId])

  if (isLoading) {
    return <div className="flex h-full items-center justify-center">Loading...</div>
  }

  if (!game) {
    return <div className="flex h-full items-center justify-center">Game not found</div>
  }

  const formatDuration = (seconds: number) => {
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    if (hours > 0) {
        return `${hours}小时${minutes}分钟`
    }
    return `${minutes}分钟`
  }

  const chartData = {
    labels: stats?.recent_play_history?.map(h => h.date) || [],
    datasets: [
      {
        label: '游戏时长 (分钟)',
        data: stats?.recent_play_history?.map(h => h.duration / 60) || [],
        borderColor: 'rgb(59, 130, 246)',
        backgroundColor: 'rgba(59, 130, 246, 0.5)',
        tension: 0.3,
      },
    ],
  }

  const chartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        display: false,
      },
      title: {
        display: false,
      },
    },
    scales: {
      x: {
        grid: {
          color: gridColor,
        },
        ticks: {
          color: textColor,
        },
      },
      y: {
        beginAtZero: true,
        grid: {
          color: gridColor,
        },
        ticks: {
          color: textColor,
        },
      },
    },
  }

  const handleUpdateGame = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!game) return
    
    try {
      await UpdateGame(game)
      const updatedGame = await GetGameByID(game.id)
      setGame(updatedGame)
      alert('更新成功')
    } catch (error) {
      console.error('Failed to update game:', error)
      alert('更新失败')
    }
  }

  const handleSelectExecutable = async () => {
    try {
      const path = await SelectGameExecutable()
      if (path && game) {
        setGame({ ...game, path } as models.Game)
      }
    } catch (error) {
      console.error('Failed to select executable:', error)
    }
  }

  const handleDeleteGame = async () => {
    if (!game) return

    // TODO: pop window使用专门的样式
    if (window.confirm(`确定要删除游戏 "${game.name}" 吗？此操作无法撤销。`)) {
      try {
        await DeleteGame(game.id)
        alert('删除成功')
        navigate({ to: '/library' })
      } catch (error) {
        // TODO: 弹窗
        console.error('Failed to delete game:', error)
        alert('删除失败')
      }
    }
  }

  return (
    <div className="space-y-8 max-w-8xl mx-auto p-8">
      {/* Back Button */}
      <button
        onClick={() => window.history.back()}
        className="flex rounded-md items-center text-brand-600 hover:text-brand-900 dark:text-brand-400 dark:hover:text-brand-200 transition-colors"
      >
        <div className="i-mdi-arrow-left text-2xl mr-1" />
        <span>返回</span>
      </button>

      {/* Header Section */}
      <div className="flex gap-6 items-center">
        <div className="w-60 flex-shrink-0 rounded-lg overflow-hidden shadow-lg bg-brand-200 dark:bg-brand-800">
          {game.cover_url ? (
            <img src={game.cover_url} alt={game.name} className="w-full h-auto block" />
          ) : (
            <div className="w-full h-64 flex items-center justify-center text-brand-400">
              No Cover
            </div>
          )}
        </div>
        
        <div className="flex-1 space-y-4">
          <h1 className="text-4xl font-bold text-brand-900 dark:text-white">{game.name}</h1>
          
          <div className="grid grid-cols-4 gap-4 text-sm text-brand-600 dark:text-brand-400">
            <div>
              <div className="font-semibold mb-1">数据来源</div>
              <div>{game.source_type}</div>
            </div>
            <div>
              <div className="font-semibold mb-1">开发</div>
              <div>{game.company || '-'}</div>
            </div>
            <div>
              <div className="font-semibold mb-1">添加时间</div>
              <div>{new Date(game.created_at).toLocaleDateString()}</div>
            </div>
            {/* Placeholders for missing data */}
          </div>

          <div className="mt-4">
            <div className="font-semibold mb-2 text-brand-900 dark:text-white">简介</div>
            <p className="text-brand-600 dark:text-brand-400 text-sm leading-relaxed line-clamp-10">
              {game.summary || '暂无简介'}
            </p>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-brand-200 dark:border-brand-700">
        <nav className="-mb-px flex space-x-8">
          {['stats', 'edit', 'backup'].map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`
                whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm
                ${activeTab === tab
                  ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                  : 'border-transparent text-brand-500 hover:text-brand-700 hover:border-brand-300 dark:text-brand-400 dark:hover:text-brand-300'}
              `}
            >
              {tab === 'stats' && '游戏统计'}
              {tab === 'edit' && '编辑'}
              {tab === 'backup' && '备份'}
            </button>
          ))}
        </nav>
      </div>

      {/* Content */}
      {activeTab === 'stats' && (
        <div className="space-y-8">
          <div className="grid grid-cols-3 gap-6">
            <div className="bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
              <div className="text-sm text-brand-500 dark:text-brand-400 mb-2">累计游戏次数</div>
              <div className="text-2xl font-bold text-brand-900 dark:text-white">
                {stats?.total_play_count || 0}
              </div>
            </div>
            <div className="bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
              <div className="text-sm text-brand-500 dark:text-brand-400 mb-2">今日游戏时长</div>
              <div className="text-2xl font-bold text-brand-900 dark:text-white">
                {formatDuration(stats?.today_play_time || 0)}
              </div>
            </div>
            <div className="bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
              <div className="text-sm text-brand-500 dark:text-brand-400 mb-2">累计总时长</div>
              <div className="text-2xl font-bold text-brand-900 dark:text-white">
                {formatDuration(stats?.total_play_time || 0)}
              </div>
            </div>
          </div>

          <div className="bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
            <div className="h-80">
              <Line options={chartOptions} data={chartData} />
            </div>
          </div>
        </div>
      )}
      
      {activeTab === 'edit' && game && (
        <div className="mx-auto bg-white dark:bg-brand-800 p-8 rounded-lg shadow-sm">
          <form onSubmit={handleUpdateGame} className="space-y-6">
            <div>
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
                游戏名称
              </label>
              <input
                type="text"
                value={game.name}
                onChange={(e) => setGame({ ...game, name: e.target.value } as models.Game)}
                className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
                封面图片 URL
              </label>
              <input
                type="text"
                value={game.cover_url}
                onChange={(e) => setGame({ ...game, cover_url: e.target.value } as models.Game)}
                className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
                开发商
              </label>
              <input
                type="text"
                value={game.company}
                onChange={(e) => setGame({ ...game, company: e.target.value } as models.Game)}
                className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
                游戏路径
              </label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={game.path}
                  onChange={(e) => setGame({ ...game, path: e.target.value } as models.Game)}
                  className="flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none"
                />
                <button
                  type="button"
                  onClick={handleSelectExecutable}
                  className="px-4 py-2 bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 rounded-md hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors"
                >
                  选择
                </button>
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
                简介
              </label>
              <textarea
                value={game.summary}
                onChange={(e) => setGame({ ...game, summary: e.target.value } as models.Game)}
                rows={6}
                className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none resize-none"
              />
            </div>

            <div className="flex justify-end pt-4">
              <button
                type="button"
                onClick={handleDeleteGame}
                className="px-6 py-2 mx-1 bg-red-600 text-white rounded-md hover:bg-red-700 transition-colors focus:ring-2 focus:ring-offset-2 focus:ring-red-500"
              >
                删除游戏
              </button>
              <button
                type="submit"
                className="px-6 py-2 mx-1  bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
              >
                保存更改
              </button>
            </div>
          </form>
        </div>
      )}

      {(activeTab === 'backup') && (
        <div className="text-center py-12 text-brand-500">
          功能开发中...
        </div>
      )}
    </div>
  )
}
