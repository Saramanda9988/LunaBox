import { createRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Route as rootRoute } from './__root'
import { useChartTheme } from '../hooks/useChartTheme'
import { GetGameByID, UpdateGame, SelectGameExecutable, DeleteGame, SelectSaveDirectory } from '../../wailsjs/go/service/GameService'
import { GetGameStats } from '../../wailsjs/go/service/StatsService'
import { models, vo, enums } from '../../wailsjs/go/models'
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
import { toast } from 'react-hot-toast'
import { GameBackupPanel } from '../components/panel/GameBackupPanel'
import { ConfirmModal } from '../components/modal/ConfirmModal'
import { GameDetailSkeleton } from '../components/skeleton/GameDetailSkeleton'

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
  const [showSkeleton, setShowSkeleton] = useState(false)
  const [activeTab, setActiveTab] = useState('stats')
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false)

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
        toast.error('加载游戏数据失败')
      } finally {
        setIsLoading(false)
      }
    }
    loadData()
  }, [gameId])

  // 延迟显示骨架屏
  useEffect(() => {
    let timer: number
    if (isLoading) {
      timer = window.setTimeout(() => {
        setShowSkeleton(true)
      }, 300)
    } else {
      setShowSkeleton(false)
    }
    return () => clearTimeout(timer)
  }, [isLoading])

  if (isLoading && !game) {
    if (!showSkeleton) {
      return <div className="min-h-screen bg-brand-100 dark:bg-brand-900" />
    }
    return <GameDetailSkeleton />
  }

  if (!game) {
    return (
      <div className="flex flex-col items-center justify-center h-full space-y-4 text-brand-500">
        <div className="i-mdi-gamepad-variant-outline text-6xl" />
        <p className="text-xl">未找到该游戏</p>
        <button onClick={() => navigate({ to: '/library' })} className="text-blue-600 hover:underline">返回库</button>
      </div>
    )
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
      toast.success('更新成功')
    } catch (error) {
      console.error('Failed to update game:', error)
      toast.error('更新失败')
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
      toast.error('选择可执行文件失败')
    }
  }

  const handleDeleteGame = async () => {
    if (!game) return
    setIsDeleteModalOpen(true)
  }

  const confirmDeleteGame = async () => {
    if (!game) return
    try {
      await DeleteGame(game.id)
      toast.success('删除成功')
      navigate({ to: '/library' })
    } catch (error) {
      console.error('Failed to delete game:', error)
      toast.error('删除失败')
    }
  }

  const handleSelectSaveDirectory = async () => {
    try {
      const path = await SelectSaveDirectory()
      if (path && game) {
        setGame({ ...game, save_path: path } as models.Game)
      }
    } catch (error) {
      console.error('Failed to select save directory:', error)
      toast.error('选择存档目录失败')
    }
  }

  const statusConfig = {
    [enums.GameStatus.NOT_STARTED]: { label: '未开始', icon: 'i-mdi-clock-outline', color: 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300' },
    [enums.GameStatus.PLAYING]: { label: '游玩中', icon: 'i-mdi-gamepad-variant', color: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300' },
    [enums.GameStatus.COMPLETED]: { label: '已通关', icon: 'i-mdi-trophy', color: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300' },
    [enums.GameStatus.ON_HOLD]: { label: '搁置', icon: 'i-mdi-pause-circle-outline', color: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300' },
  }

  const handleStatusChange = async (newStatus: string) => {
    if (!game) return
    const updatedGame = { ...game, status: newStatus } as models.Game
    setGame(updatedGame)
    try {
      await UpdateGame(updatedGame)
      toast.success('状态已更新')
    } catch (error) {
      console.error('Failed to update status:', error)
      toast.error('状态更新失败')
    }
  }

  return (
    <div className={`space-y-8 max-w-8xl mx-auto p-8 transition-opacity duration-300 ${isLoading ? 'opacity-50 pointer-events-none' : 'opacity-100'}`}>
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
        <div className="relative w-60 flex-shrink-0 rounded-lg overflow-hidden shadow-lg bg-brand-200 dark:bg-brand-800">
          {game.cover_url ? (
            <img
              src={game.cover_url}
              alt={game.name}
              className="w-full h-auto block"
              referrerPolicy="no-referrer"
            />
          ) : (
            <div className="w-full h-64 flex items-center justify-center text-brand-400">
              No Cover
            </div>
          )}
          {/* 已通关奖杯标识 */}
          {game.status === enums.GameStatus.COMPLETED && (
            <div className="absolute top-3 right-3 flex h-10 w-10 items-center justify-center rounded-full bg-yellow-500 shadow-lg">
              <div className="i-mdi-trophy text-xl text-white" />
            </div>
          )}
        </div>
        
        <div className="flex-1 space-y-4">
          <div className="flex items-center gap-4 flex-wrap">
            <h1 className="text-4xl font-bold text-brand-900 dark:text-white">{game.name}</h1>
            {/* 状态标签组 */}
            <div className="flex gap-1.5">
              {Object.entries(statusConfig).map(([key, config]) => {
                const isActive = (game.status || enums.GameStatus.NOT_STARTED) === key
                return (
                  <button
                    key={key}
                    onClick={() => handleStatusChange(key)}
                    className={`flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-medium transition-all ${
                      isActive 
                        ? config.color + ' ring-2 ring-offset-1 ring-brand-400 dark:ring-offset-brand-900' 
                        : 'bg-brand-100 text-brand-500 dark:bg-brand-700 dark:text-brand-400 hover:bg-brand-200 dark:hover:bg-brand-600'
                    }`}
                    title={config.label}
                  >
                    <div className={`${config.icon} text-sm`} />
                    {isActive && <span>{config.label}</span>}
                  </button>
                )
              })}
            </div>
          </div>
          
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
                存档目录
              </label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={game.save_path || ''}
                  onChange={(e) => setGame({ ...game, save_path: e.target.value } as models.Game)}
                  placeholder="选择游戏存档所在目录"
                  className="flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-blue-500 outline-none"
                />
                <button
                  type="button"
                  onClick={handleSelectSaveDirectory}
                  className="px-4 py-2 bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 rounded-md hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors"
                >
                  选择
                </button>
              </div>
              <p className="mt-1 text-xs text-brand-500">设置存档目录后可使用备份功能</p>
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

      {activeTab === 'backup' && (
        <GameBackupPanel gameId={gameId} savePath={game?.save_path} />
      )}

      <ConfirmModal
        isOpen={isDeleteModalOpen}
        title="删除游戏"
        message={`确定要删除游戏 "${game.name}" 吗？此操作将从库中移除该游戏，但不会删除本地游戏文件。`}
        confirmText="确认删除"
        type="danger"
        onClose={() => setIsDeleteModalOpen(false)}
        onConfirm={confirmDeleteGame}
      />
    </div>
  )
}
