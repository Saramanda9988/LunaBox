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
    <div className="p-6 space-y-6 overflow-y-auto h-full">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">统计数据</h1>
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

      {/* Leaderboard */}
      <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div className="p-6 border-b border-gray-200 dark:border-gray-700">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white">游玩时长排行榜</h3>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="bg-gray-50 dark:bg-gray-700/50">
              <tr>
                <th className="px-6 py-3 font-medium text-gray-500 dark:text-gray-400">排名</th>
                <th className="px-6 py-3 font-medium text-gray-500 dark:text-gray-400">游戏名称</th>
                <th className="px-6 py-3 font-medium text-gray-500 dark:text-gray-400 text-right">时长</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
              {stats.play_time_leaderboard.map((game, index) => (
                <tr key={game.game_id} className="hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors">
                  <td className="px-6 py-4 text-gray-500 dark:text-gray-400 w-16">#{index + 1}</td>
                  <td className="px-6 py-4 font-medium text-gray-900 dark:text-white">{game.game_name}</td>
                  <td className="px-6 py-4 text-gray-900 dark:text-white text-right font-mono">
                    {formatDuration(game.total_duration)}
                  </td>
                </tr>
              ))}
              {stats.play_time_leaderboard.length === 0 && (
                <tr>
                  <td colSpan={3} className="px-6 py-8 text-center text-gray-500 dark:text-gray-400">
                    暂无数据
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
