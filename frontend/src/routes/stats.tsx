import { createFileRoute, createRoute, RootRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Route as rootRoute } from './__root'
import {
  CategoryScale,
  Chart as ChartJS,
  Legend,
  LineElement,
  LinearScale,
  PointElement,
  Title,
  Tooltip,
} from 'chart.js'
import { Line } from 'react-chartjs-2'
import { GetGlobalPeriodStats } from '../../wailsjs/go/service/StatsService'
import { enums, vo } from '../../wailsjs/go/models'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
)

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/stats',
  component: StatsPage,
})

function StatsPage() {
  const [dimension, setDimension] = useState<enums.Period>(enums.Period.WEEK)
  const [stats, setStats] = useState<vo.PeriodStats | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    loadStats(dimension)
  }, [dimension])

  const loadStats = async (dim: enums.Period) => {
    setLoading(true)
    try {
      const data = await GetGlobalPeriodStats(dim)
      setStats(data)
    } catch (error) {
      console.error('Failed to load stats:', error)
    } finally {
      setLoading(false)
    }
  }

  const formatDuration = (seconds: number) => {
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    return `${hours}h ${minutes}m`
  }

  const formatDurationHours = (seconds: number) => {
    return Number((seconds / 3600).toFixed(1))
  }

  if (!stats && loading) {
    return <div className="p-6">Loading...</div>
  }

  if (!stats) {
    return <div className="p-6">No data available</div>
  }

  // Chart 1: Total Play Duration Trend
  const totalTrendData = {
    labels: stats.timeline.map((p) => p.label),
    datasets: [
      {
        label: '总游玩时长 (小时)',
        data: stats.timeline.map((p) => formatDurationHours(p.duration)),
        borderColor: 'rgb(75, 192, 192)',
        backgroundColor: 'rgba(75, 192, 192, 0.5)',
        tension: 0.3,
      },
    ],
  }

  // Chart 2: Game Play Duration Trend (Multi-line)
  const gameTrendData = {
    labels: stats.timeline.map((p) => p.label), // Assuming all series share the same timeline
    datasets: stats.leaderboard_series.map((series, index) => {
      const colors = [
        'rgb(255, 99, 132)',
        'rgb(54, 162, 235)',
        'rgb(255, 206, 86)',
        'rgb(75, 192, 192)',
        'rgb(153, 102, 255)',
      ]
      const color = colors[index % colors.length]
      return {
        label: series.game_name,
        data: series.points.map((p) => formatDurationHours(p.duration)),
        borderColor: color,
        backgroundColor: color.replace('rgb', 'rgba').replace(')', ', 0.5)'),
        tension: 0.3,
      }
    }),
  }

  const chartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        position: 'top' as const,
      },
    },
    scales: {
      y: {
        beginAtZero: true,
        title: {
          display: true,
          text: '小时',
        },
      },
    },
  }

  return (
    <div className="space-y-6 max-w-8xl mx-auto p-8">
      <div className="flex items-center justify-between">
        <h1 className="text-4xl font-bold text-gray-900 dark:text-white">统计</h1>
      </div>
      <div className="flex justify-between items-center">
        <div className="flex space-x-2 bg-gray-100 dark:bg-gray-800 p-1 rounded-lg">
          {[
            { label: '周', value: enums.Period.WEEK },
            { label: '月', value: enums.Period.MONTH },
            { label: '年', value: enums.Period.YEAR },
          ].map((item) => (
            <button
              key={item.value}
              onClick={() => setDimension(item.value)}
              className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
                dimension === item.value
                  ? 'bg-white dark:bg-gray-700 text-blue-600 dark:text-blue-400 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
              }`}
            >
              {item.label}
            </button>
          ))}
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="bg-white dark:bg-gray-800 p-6 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700">
          <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">总游玩次数</h3>
          <p className="text-3xl font-bold text-gray-900 dark:text-white">{stats.total_play_count}</p>
        </div>
        <div className="bg-white dark:bg-gray-800 p-6 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700">
          <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">总游玩时长</h3>
          <p className="text-3xl font-bold text-gray-900 dark:text-white">{formatDuration(stats.total_play_duration)}</p>
        </div>
      </div>

      {/* Leaderboard */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Top 1 Game Card */}
        {stats.play_time_leaderboard.length > 0 && (
          <div className="lg:col-span-1 bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6 flex flex-col items-center text-center relative overflow-hidden">
            <div className="absolute top-0 left-0 w-full h-1.5 bg-gradient-to-r from-yellow-400 to-orange-500" />
            <div className="w-10 h-10 bg-yellow-100 dark:bg-yellow-900/30 text-yellow-600 dark:text-yellow-400 rounded-full flex items-center justify-center text-lg font-bold mb-4 shadow-sm">
              #1
            </div>
            <div className="relative group">
              <img
                src={stats.play_time_leaderboard[0].cover_url}
                alt={stats.play_time_leaderboard[0].game_name}
                className="w-40 h-56 object-cover rounded-lg shadow-md mb-4 transition-transform group-hover:scale-105 bg-gray-200 dark:bg-gray-700"
              />
            </div>
            <h3 className="text-lg font-bold text-gray-900 dark:text-white mb-2 line-clamp-2 px-2">
              {stats.play_time_leaderboard[0].game_name}
            </h3>
            <p className="text-2xl font-mono font-semibold text-blue-600 dark:text-blue-400">
              {formatDuration(stats.play_time_leaderboard[0].total_duration)}
            </p>
          </div>
        )}

        {/* Other Games List */}
        <div className={`${stats.play_time_leaderboard.length > 0 ? 'lg:col-span-2' : 'lg:col-span-3'} bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 overflow-hidden flex flex-col`}>
          <div className="p-6 border-b border-gray-200 dark:border-gray-700">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
              {stats.play_time_leaderboard.length > 0 ? '排行榜' : '游玩时长排行榜'}
            </h3>
          </div>
          <div className="overflow-x-auto flex-1">
            <table className="w-full text-left text-sm">
              <thead className="bg-gray-50 dark:bg-gray-700/50">
                <tr>
                  <th className="px-6 py-3 font-medium text-gray-500 dark:text-gray-400 w-20">排名</th>
                  <th className="px-6 py-3 font-medium text-gray-500 dark:text-gray-400">游戏</th>
                  <th className="px-6 py-3 font-medium text-gray-500 dark:text-gray-400 text-right">时长</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                {stats.play_time_leaderboard.slice(1).map((game, index) => (
                  <tr key={game.game_id} className="hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors">
                    <td className="px-6 py-4 text-gray-500 dark:text-gray-400 font-medium">#{index + 2}</td>
                    <td className="px-6 py-4">
                      <div className="flex items-center">
                        <img
                          src={game.cover_url}
                          alt={game.game_name}
                          className="w-10 h-14 object-cover rounded shadow-sm mr-4 bg-gray-200 dark:bg-gray-700"
                        />
                        <span className="font-medium text-gray-900 dark:text-white line-clamp-1">{game.game_name}</span>
                      </div>
                    </td>
                    <td className="px-6 py-4 text-gray-900 dark:text-white text-right font-mono">
                      {formatDuration(game.total_duration)}
                    </td>
                  </tr>
                ))}
                {stats.play_time_leaderboard.length <= 1 && (
                  <tr>
                    <td colSpan={3} className="px-6 py-12 text-center text-gray-500 dark:text-gray-400">
                      {stats.play_time_leaderboard.length === 0 ? '暂无数据' : '暂无更多排行数据'}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      {/* Charts */}
      <div className="space-y-6">
        <div className="bg-white dark:bg-gray-800 p-6 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">游玩时长趋势</h3>
          <div className="h-96 w-full">
            <Line options={chartOptions} data={totalTrendData} />
          </div>
        </div>
        <div className="bg-white dark:bg-gray-800 p-6 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">热门游戏趋势</h3>
          <div className="h-96 w-full">
            <Line options={chartOptions} data={gameTrendData} />
          </div>
        </div>
      </div>
    </div>
  )
}
