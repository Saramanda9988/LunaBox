import { createRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Route as rootRoute } from './__root'
import { useChartTheme } from '../hooks/useChartTheme'
import { GetGameByID, UpdateGame, SelectGameExecutable, DeleteGame, SelectSaveDirectory } from '../../wailsjs/go/service/GameService'
import { GetGameStats } from '../../wailsjs/go/service/StatsService'
import { GetGameBackups, CreateBackup, RestoreBackup, DeleteBackup, OpenBackupFolder, GetCloudBackupStatus, UploadGameBackupToCloud, GetCloudGameBackups, RestoreFromCloud } from '../../wailsjs/go/service/BackupService'
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
import { toast } from 'react-hot-toast'

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
  const [backups, setBackups] = useState<models.GameBackup[]>([])
  const [isBackingUp, setIsBackingUp] = useState(false)
  const [cloudBackups, setCloudBackups] = useState<vo.CloudBackupItem[]>([])
  const [cloudStatus, setCloudStatus] = useState<vo.CloudBackupStatus | null>(null)
  const [isUploading, setIsUploading] = useState(false)
  const [isBackupsLoading, setIsBackupsLoading] = useState(false)
  const [isCloudBackupsLoading, setIsCloudBackupsLoading] = useState(false)

  useEffect(() => {
    const loadData = async () => {
      try {
        const [gameData, statsData, cloudStatusData] = await Promise.all([
          GetGameByID(gameId),
          GetGameStats(gameId),
          GetCloudBackupStatus(),
        ])
        setGame(gameData)
        setStats(statsData)
        setCloudStatus(cloudStatusData)
      } catch (error) {
        console.error('Failed to load game data:', error)
        toast.error('加载游戏数据失败')
      } finally {
        setIsLoading(false)
      }
    }
    loadData()
  }, [gameId])

  useEffect(() => {
    if (activeTab === 'backup' && gameId) {
      const loadLocalBackups = async () => {
        setIsBackupsLoading(true)
        try {
          const backupsData = await GetGameBackups(gameId)
          setBackups(backupsData || [])
        } catch (error) {
          console.error('Failed to load local backups:', error)
          toast.error('加载本地备份失败')
        } finally {
          setIsBackupsLoading(false)
        }
      }

      const loadCloudBackups = async () => {
        if (cloudStatus?.configured && cloudStatus?.enabled) {
          setIsCloudBackupsLoading(true)
          try {
            const cloudBackupsData = await GetCloudGameBackups(gameId)
            setCloudBackups(cloudBackupsData || [])
          } catch (error) {
            console.error('Failed to load cloud backups:', error)
            // 云端备份加载失败不影响主流程，可以不弹窗或者给个轻提示
          } finally {
            setIsCloudBackupsLoading(false)
          }
        }
      }

      loadLocalBackups()
      loadCloudBackups()
    }
  }, [activeTab, gameId, cloudStatus])

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

    // TODO: pop window使用专门的样式
    if (window.confirm(`确定要删除游戏 "${game.name}" 吗？此操作无法撤销。`)) {
      try {
        await DeleteGame(game.id)
        toast.success('删除成功')
        navigate({ to: '/library' })
      } catch (error) {
        // TODO: 弹窗
        console.error('Failed to delete game:', error)
        toast.error('删除失败')
      }
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

  const handleCreateBackup = async () => {
    if (!game) return
    if (!game.save_path) {
      toast.error('请先设置存档目录')
      return
    }
    setIsBackingUp(true)
    try {
      await CreateBackup(game.id)
      const backupsData = await GetGameBackups(game.id)
      setBackups(backupsData || [])
      toast.success('备份成功')
    } catch (error: any) {
      console.error('Failed to create backup:', error)
      toast.error(error?.message || '备份失败')
    } finally {
      setIsBackingUp(false)
    }
  }

  const handleRestoreBackup = async (backupId: string, createdAt: string) => {
    if (window.confirm(`确定要恢复到 ${new Date(createdAt).toLocaleString()} 的备份吗？当前存档将被覆盖。`)) {
      try {
        await RestoreBackup(backupId)
        toast.success('恢复成功')
      } catch (error: any) {
        console.error('Failed to restore backup:', error)
        toast.error(error?.message || '恢复失败')
      }
    }
  }

  const handleDeleteBackup = async (backupId: string) => {
    if (window.confirm('确定要删除此备份吗？')) {
      try {
        await DeleteBackup(backupId)
        const backupsData = await GetGameBackups(gameId)
        setBackups(backupsData || [])
        toast.success('删除成功')
      } catch (error) {
        console.error('Failed to delete backup:', error)
        toast.error('删除失败')
      }
    }
  }

  const handleOpenBackupFolder = async () => {
    try {
      await OpenBackupFolder(gameId)
    } catch (error) {
      console.error('Failed to open backup folder:', error)
    }
  }

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  }

  const handleUploadToCloud = async (backupId: string) => {
    if (!game) return
    setIsUploading(true)
    try {
      await UploadGameBackupToCloud(game.id, backupId)
      const cloudBackupsData = await GetCloudGameBackups(game.id)
      setCloudBackups(cloudBackupsData || [])
      toast.success('上传成功')
    } catch (error: any) {
      console.error('Failed to upload to cloud:', error)
      toast.error(error?.message || '上传失败')
    } finally {
      setIsUploading(false)
    }
  }

  const handleRestoreFromCloud = async (cloudKey: string, name: string) => {
    if (!game) return
    if (window.confirm(`确定要从云端恢复 ${name} 的备份吗？当前存档将被覆盖。`)) {
      try {
        await RestoreFromCloud(cloudKey, game.id)
        toast.success('恢复成功')
      } catch (error: any) {
        console.error('Failed to restore from cloud:', error)
        toast.error(error?.message || '恢复失败')
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

      {(activeTab === 'backup') && (
        <div className="space-y-6">
          {/* 备份操作区 */}
          <div className="bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
            <div className="flex items-center justify-between mb-4">
              <div>
                <h3 className="text-lg font-semibold text-brand-900 dark:text-white">存档备份</h3>
                <p className="text-sm text-brand-500 dark:text-brand-400 mt-1">
                  {game?.save_path ? `存档目录: ${game.save_path}` : '请先在编辑页面设置存档目录'}
                </p>
              </div>
              <div className="flex gap-2">
                <button
                  onClick={handleOpenBackupFolder}
                  className="px-4 py-2 text-brand-600 dark:text-brand-400 hover:bg-brand-100 dark:hover:bg-brand-700 rounded-md transition-colors"
                >
                  打开备份文件夹
                </button>
                <button
                  onClick={handleCreateBackup}
                  disabled={isBackingUp || !game?.save_path}
                  className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                >
                  {isBackingUp && <div className="i-mdi-loading animate-spin" />}
                  {isBackingUp ? '备份中...' : '立即备份'}
                </button>
              </div>
            </div>
          </div>

          {/* 本地备份历史列表 */}
          <div className="bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
            <h3 className="text-lg font-semibold text-brand-900 dark:text-white mb-4">本地备份</h3>
            {isBackupsLoading ? (
              <div className="flex justify-center py-8">
                <div className="i-mdi-loading animate-spin text-2xl text-brand-500" />
              </div>
            ) : backups.length === 0 ? (
              <div className="text-center py-8 text-brand-500">
                暂无本地备份记录
              </div>
            ) : (
              <div className="space-y-3">
                {backups.map((backup) => (
                  <div
                    key={backup.id}
                    className="flex items-center justify-between p-4 bg-brand-50 dark:bg-brand-700 rounded-lg"
                  >
                    <div className="flex items-center gap-4">
                      <div className="i-mdi-archive text-2xl text-brand-500" />
                      <div>
                        <div className="font-medium text-brand-900 dark:text-white">
                          {new Date(backup.created_at).toLocaleString()}
                        </div>
                        <div className="text-sm text-brand-500">
                          大小: {formatFileSize(backup.size)}
                        </div>
                      </div>
                    </div>
                    <div className="flex gap-2">
                      {cloudStatus?.configured && cloudStatus?.enabled && (
                        <button
                          onClick={() => handleUploadToCloud(backup.id)}
                          disabled={isUploading}
                          title="上传到云端"
                          className="p-2 text-blue-600 hover:bg-blue-100 dark:hover:bg-blue-900 rounded transition-colors disabled:opacity-50"
                        >
                          <div className={`i-mdi-cloud-upload text-xl ${isUploading ? 'animate-pulse' : ''}`} />
                        </button>
                      )}
                      <button
                        onClick={() => handleRestoreBackup(backup.id, backup.created_at)}
                        title="恢复备份"
                        className="p-2 text-green-600 hover:bg-green-100 dark:hover:bg-green-900 rounded transition-colors"
                      >
                        <div className="i-mdi-backup-restore text-xl" />
                      </button>
                      <button
                        onClick={() => handleDeleteBackup(backup.id)}
                        title="删除备份"
                        className="p-2 text-red-600 hover:bg-red-100 dark:hover:bg-red-900 rounded transition-colors"
                      >
                        <div className="i-mdi-delete text-xl" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* 云端备份列表 */}
          {cloudStatus?.configured && cloudStatus?.enabled && (
            <div className="bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
              <h3 className="text-lg font-semibold text-brand-900 dark:text-white mb-4 flex items-center gap-2">
                <div className="text-xl text-blue-500" />
                云端备份
              </h3>
              {isCloudBackupsLoading ? (
                <div className="flex justify-center py-8">
                  <div className="i-mdi-loading animate-spin text-2xl text-brand-500" />
                </div>
              ) : cloudBackups.length === 0 ? (
                <div className="text-center py-8 text-brand-500">
                  暂无云端备份记录
                </div>
              ) : (
                <div className="space-y-3">
                  {cloudBackups.map((backup) => (
                    <div
                      key={backup.key}
                      className="flex items-center justify-between p-4 bg-brand-50 dark:bg-brand-700 rounded-lg"
                    >
                      <div className="flex items-center gap-4">
                        <div className="i-mdi-cloud-check text-2xl text-blue-500" />
                        <div>
                          <div className="font-medium text-brand-900 dark:text-white">
                            {backup.name || new Date(backup.created_at).toLocaleString()}
                          </div>
                          <div className="text-sm text-brand-500">
                            {new Date(backup.created_at).toLocaleString()}
                          </div>
                        </div>
                      </div>
                      <div className="flex gap-2">
                        <button
                          onClick={() => handleRestoreFromCloud(backup.key, backup.name)}
                          title="从云端恢复"
                          className="p-2 text-green-600 hover:bg-green-100 dark:hover:bg-green-900 rounded transition-colors"
                        >
                          <div className="i-mdi-cloud-download text-xl" />
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* 云备份未配置提示 */}
          {(!cloudStatus?.configured || !cloudStatus?.enabled) && (
            <div className="bg-brand-50 dark:bg-brand-800 p-4 rounded-lg border border-brand-200 dark:border-brand-700">
              <div className="flex items-center gap-3">
                <div className="i-mdi-cloud-off-outline text-2xl text-brand-400" />
                <div>
                  <div className="font-medium text-brand-700 dark:text-brand-300">云备份未启用</div>
                  <div className="text-sm text-brand-500">前往设置页面配置云备份，将存档同步到云端</div>
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
