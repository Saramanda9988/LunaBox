import { useState, useEffect } from 'react'
import { toast } from 'react-hot-toast'
import { models } from '../../../wailsjs/go/models'
import { GetPlaySessions, DeletePlaySession } from '../../../wailsjs/go/service/TimerService'
import { AddPlaySessionModal } from '../modal/AddPlaySessionModal'
import { ConfirmModal } from '../modal/ConfirmModal'
import { formatDuration } from '../../utils/time'

interface PlaySessionPanelProps {
  gameId: string
}

export function PlaySessionPanel({ gameId }: PlaySessionPanelProps) {
  const [sessions, setSessions] = useState<models.PlaySession[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isAddModalOpen, setIsAddModalOpen] = useState(false)
  const [deleteSessionId, setDeleteSessionId] = useState<string | null>(null)

  const loadSessions = async () => {
    try {
      const data = await GetPlaySessions(gameId)
      setSessions(data || [])
    } catch (error) {
      console.error('Failed to load play sessions:', error)
      toast.error('加载游玩记录失败')
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    loadSessions()
  }, [gameId])

  const handleDelete = async () => {
    if (!deleteSessionId) return
    
    try {
      await DeletePlaySession(deleteSessionId)
      toast.success('删除成功')
      loadSessions()
    } catch (error) {
      console.error('Failed to delete play session:', error)
      toast.error('删除失败')
    } finally {
      setDeleteSessionId(null)
    }
  }

  const formatDate = (dateValue: any) => {
    // time.Time 在 wails 中会被序列化为 ISO 字符串
    const dateStr = typeof dateValue === 'string' ? dateValue : String(dateValue)
    const date = new Date(dateStr)
    return date.toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit'
    })
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <div className="i-mdi-loading animate-spin text-2xl text-brand-500" />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <h3 className="text-lg font-medium text-brand-900 dark:text-white">
          游玩记录
        </h3>
        <button
          onClick={() => setIsAddModalOpen(true)}
          className="flex items-center gap-1 px-3 py-1.5 bg-neutral-600 text-white rounded-md hover:bg-neutral-700 transition-colors text-sm"
        >
          <div className="i-mdi-plus text-lg" />
          添加记录
        </button>
      </div>

      {sessions.length === 0 ? (
        <div className="text-center py-8 text-brand-500">
          <div className="i-mdi-clock-outline text-4xl mx-auto mb-2" />
          <p>暂无游玩记录</p>
          <p className="text-sm mt-1">点击上方按钮手动添加记录</p>
        </div>
      ) : (
        <div className="space-y-2">
          {sessions.map((session) => (
            <div
              key={session.id}
              className="flex items-center justify-between p-3 bg-brand-50 dark:bg-brand-700/50 rounded-lg"
            >
              <div className="flex-1">
                <div className="text-sm text-brand-900 dark:text-white">
                  {formatDate(session.start_time)}
                </div>
                <div className="text-xs text-brand-500 dark:text-brand-400">
                  时长: {formatDuration(session.duration)}
                </div>
              </div>
              <button
                onClick={() => setDeleteSessionId(session.id)}
                className="p-1.5 text-brand-400 hover:text-error-500 hover:bg-error-50 dark:hover:bg-error-900/20 rounded transition-colors"
                title="删除记录"
              >
                <div className="i-mdi-delete-outline text-lg" />
              </button>
            </div>
          ))}
        </div>
      )}

      <AddPlaySessionModal
        isOpen={isAddModalOpen}
        gameId={gameId}
        onClose={() => setIsAddModalOpen(false)}
        onSuccess={loadSessions}
      />

      <ConfirmModal
        isOpen={!!deleteSessionId}
        title="删除游玩记录"
        message="确定要删除这条游玩记录吗？此操作不可撤销。"
        confirmText="确认删除"
        type="danger"
        onClose={() => setDeleteSessionId(null)}
        onConfirm={handleDelete}
      />
    </div>
  )
}
