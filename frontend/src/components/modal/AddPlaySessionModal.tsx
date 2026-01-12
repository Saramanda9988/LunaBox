import { useState } from 'react'
import { toast } from 'react-hot-toast'
import { AddPlaySession } from '../../../wailsjs/go/service/TimerService'

interface AddPlaySessionModalProps {
  isOpen: boolean
  gameId: string
  onClose: () => void
  onSuccess: () => void
}

export function AddPlaySessionModal({ isOpen, gameId, onClose, onSuccess }: AddPlaySessionModalProps) {
  const [date, setDate] = useState(() => {
    const today = new Date()
    return today.toISOString().split('T')[0]
  })
  const [hours, setHours] = useState(0)
  const [minutes, setMinutes] = useState(30)
  const [isSubmitting, setIsSubmitting] = useState(false)

  if (!isOpen) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    
    const totalMinutes = hours * 60 + minutes
    if (totalMinutes <= 0) {
      toast.error('游玩时长必须大于0')
      return
    }

    setIsSubmitting(true)
    try {
      // 构建开始时间（设置为当天中午12点）
      const startTime = new Date(date + 'T12:00:00')
      
      await AddPlaySession(gameId, startTime.toISOString(), totalMinutes)
      toast.success('游玩记录添加成功')
      onSuccess()
      onClose()
      // 重置表单
      setHours(0)
      setMinutes(30)
    } catch (error) {
      console.error('Failed to add play session:', error)
      toast.error('添加游玩记录失败')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div className="relative bg-white dark:bg-brand-800 rounded-lg shadow-xl w-full max-w-md mx-4 p-6">
        <h2 className="text-xl font-semibold text-brand-900 dark:text-white mb-4">
          添加游玩记录
        </h2>
        
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              游玩日期
            </label>
            <input
              type="date"
              value={date}
              onChange={(e) => setDate(e.target.value)}
              max={new Date().toISOString().split('T')[0]}
              className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              游玩时长
            </label>
            <div className="flex gap-2 items-center">
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <input
                    type="number"
                    value={hours}
                    onChange={(e) => setHours(Math.max(0, parseInt(e.target.value) || 0))}
                    min="0"
                    max="99"
                    className="w-20 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none text-center"
                  />
                  <span className="text-brand-600 dark:text-brand-400">小时</span>
                  <input
                    type="number"
                    value={minutes}
                    onChange={(e) => setMinutes(Math.max(0, Math.min(59, parseInt(e.target.value) || 0)))}
                    min="0"
                    max="59"
                    className="w-20 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none text-center"
                  />
                  <span className="text-brand-600 dark:text-brand-400">分钟</span>
                </div>
              </div>
            </div>
            <p className="mt-1 text-xs text-brand-500">
              总计: {hours > 0 ? `${hours}小时` : ''}{minutes > 0 ? `${minutes}分钟` : ''}{hours === 0 && minutes === 0 ? '0分钟' : ''}
            </p>
          </div>

          <div className="flex justify-end gap-3 pt-4">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-brand-600 dark:text-brand-400 hover:bg-brand-100 dark:hover:bg-brand-700 rounded-md transition-colors"
            >
              取消
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="px-4 py-2 bg-neutral-600 text-white rounded-md hover:bg-neutral-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isSubmitting ? '添加中...' : '添加'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
