import { createRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useState, useRef } from 'react'
import { Route as rootRoute } from './__root'
import { GetGameByID, UpdateGame, SelectGameExecutable, DeleteGame, SelectSaveDirectory, SelectCoverImage, UpdateGameFromRemote } from '../../wailsjs/go/service/GameService'
import { models, enums } from '../../wailsjs/go/models'
import { toast } from 'react-hot-toast'
import { GameBackupPanel } from '../components/panel/GameBackupPanel'
import { GameEditPanel } from '../components/panel/GameEditPanel'
import { GameStatsPanel } from '../components/panel/GameStatsPanel'
import { ConfirmModal } from '../components/modal/ConfirmModal'
import { GameDetailSkeleton } from '../components/skeleton/GameDetailSkeleton'

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/game/$gameId',
  component: GameDetailPage,
})

function GameDetailPage() {
  const navigate = useNavigate()
  const { gameId } = Route.useParams()
  const [game, setGame] = useState<models.Game | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [showSkeleton, setShowSkeleton] = useState(false)
  const [activeTab, setActiveTab] = useState('stats')
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false)
  const isInitialMount = useRef(true)
  const originalGameData = useRef<models.Game | null>(null)

  useEffect(() => {
    const loadData = async () => {
      try {
        const gameData = await GetGameByID(gameId)
        setGame(gameData)
        originalGameData.current = gameData
        isInitialMount.current = false
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

  // 自动保存
  useEffect(() => {
    if (!game || isInitialMount.current) return

    const hasChanges = JSON.stringify(game) !== JSON.stringify(originalGameData.current)
    if (!hasChanges) return

    const timer = setTimeout(async () => {
      try {
        await UpdateGame(game)
        originalGameData.current = game
      } catch (error) {
        console.error('Failed to auto-save game:', error)
        toast.error('保存失败' + (error as Error).message)
      }
    }, 500)

    return () => clearTimeout(timer)
  }, [game])

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
        <button onClick={() => navigate({ to: '/library' })} className="text-neutral-600 hover:underline">返回库</button>
      </div>
    )
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

  const handleSelectCoverImage = async () => {
    if (!game) return
    try {
      const coverUrl = await SelectCoverImage(game.id)
      if (coverUrl) {
        setGame({ ...game, cover_url: coverUrl } as models.Game)
      }
    } catch (error) {
      console.error('Failed to select cover image:', error)
      toast.error('选择封面图片失败')
    }
  }

  const handleUpdateFromRemote = async () => {
    if (!game) return
    try {
      await UpdateGameFromRemote(game.id)
      const updatedGame = await GetGameByID(game.id)
      setGame(updatedGame)
      originalGameData.current = updatedGame
      toast.success('从远程更新成功')
    } catch (error) {
      console.error('Failed to update from remote:', error)
      toast.error('从远程更新失败: ' + error)
    }
  }

  const statusConfig = {
    [enums.GameStatus.NOT_STARTED]: { label: '未开始', icon: 'i-mdi-clock-outline', color: 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300' },
    [enums.GameStatus.PLAYING]: { label: '游玩中', icon: 'i-mdi-gamepad-variant', color: 'bg-neutral-100 text-neutral-700 dark:bg-neutral-900 dark:text-neutral-300' },
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
        </div>
        
        <div className="flex-1 space-y-4">
          <div className="flex flex-col gap-3">
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
              <div>{new Date(String(game.created_at)).toLocaleDateString()}</div>
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
                  ? 'border-neutral-500 text-neutral-600 dark:text-neutral-400'
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
        <GameStatsPanel gameId={gameId} />
      )}
      
      {activeTab === 'edit' && game && (
        <GameEditPanel
          game={game}
          onGameChange={setGame}
          onDelete={handleDeleteGame}
          onSelectExecutable={handleSelectExecutable}
          onSelectSaveDirectory={handleSelectSaveDirectory}
          onSelectCoverImage={handleSelectCoverImage}
          onUpdateFromRemote={handleUpdateFromRemote}
        />
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
