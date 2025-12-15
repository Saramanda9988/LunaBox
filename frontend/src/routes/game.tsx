import { createRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Route as rootRoute } from './__root'
import { GetGameByID } from '../../wailsjs/go/service/GameService'
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
  component: GameDetailComponent,
})

function GameDetailComponent() {
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
      y: {
        beginAtZero: true,
      },
    },
  }

  return (
    <div className="space-y-8 max-w-8xl mx-auto">
      {/* Back Button */}
      <button
        onClick={() => window.history.back()}
        className="flex rounded-md items-center text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
      >
        <div className="i-mdi-arrow-left text-2xl mr-1" />
        <span>返回</span>
      </button>

      {/* Header Section */}
      <div className="flex gap-6 items-center">
        <div className="w-60 flex-shrink-0 rounded-lg overflow-hidden shadow-lg bg-gray-200 dark:bg-gray-800">
          {game.cover_url ? (
            <img src={game.cover_url} alt={game.name} className="w-full h-auto block" />
          ) : (
            <div className="w-full h-64 flex items-center justify-center text-gray-400">
              No Cover
            </div>
          )}
        </div>
        
        <div className="flex-1 space-y-4">
          <h1 className="text-4xl font-bold text-gray-900 dark:text-white">{game.name}</h1>
          
          <div className="grid grid-cols-4 gap-4 text-sm text-gray-600 dark:text-gray-400">
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
            <div className="font-semibold mb-2 text-gray-900 dark:text-white">简介</div>
            <p className="text-gray-600 dark:text-gray-400 text-sm leading-relaxed line-clamp-4">
              {game.summary || '暂无简介'}
            </p>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-gray-200 dark:border-gray-700">
        <nav className="-mb-px flex space-x-8">
          {['stats', 'intro', 'edit', 'backup'].map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`
                whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm
                ${activeTab === tab
                  ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'}
              `}
            >
              {tab === 'stats' && '游戏统计'}
              {tab === 'intro' && '简介'}
              {tab === 'edit' && '编辑'}
              {tab === 'backup' && '备份'}
            </button>
          ))}
        </nav>
      </div>

      {/* Content */}
      {activeTab === 'stats' && (
        <div className="space-y-8">
          <h2 className="text-xl font-bold text-gray-900 dark:text-white">游戏统计</h2>
          
          <div className="grid grid-cols-3 gap-6">
            <div className="bg-white dark:bg-gray-800 p-6 rounded-lg shadow-sm">
              <div className="text-sm text-gray-500 dark:text-gray-400 mb-2">累计游戏次数</div>
              <div className="text-2xl font-bold text-gray-900 dark:text-white">
                {stats?.total_play_count || 0}
              </div>
            </div>
            <div className="bg-white dark:bg-gray-800 p-6 rounded-lg shadow-sm">
              <div className="text-sm text-gray-500 dark:text-gray-400 mb-2">今日游戏时长</div>
              <div className="text-2xl font-bold text-gray-900 dark:text-white">
                {formatDuration(stats?.today_play_time || 0)}
              </div>
            </div>
            <div className="bg-white dark:bg-gray-800 p-6 rounded-lg shadow-sm">
              <div className="text-sm text-gray-500 dark:text-gray-400 mb-2">累计总时长</div>
              <div className="text-2xl font-bold text-gray-900 dark:text-white">
                {formatDuration(stats?.total_play_time || 0)}
              </div>
            </div>
          </div>

          <div className="bg-white dark:bg-gray-800 p-6 rounded-lg shadow-sm">
            <div className="h-80">
              <Line options={chartOptions} data={chartData} />
            </div>
          </div>
        </div>
      )}
      
      {activeTab !== 'stats' && (
        <div className="text-center py-12 text-gray-500">
          功能开发中...
        </div>
      )}
    </div>
  )
}
