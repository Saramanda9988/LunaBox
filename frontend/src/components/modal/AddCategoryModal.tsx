import { createPortal } from 'react-dom'

interface AddCategoryModalProps {
  isOpen: boolean
  value: string
  onChange: (value: string) => void
  onClose: () => void
  onSubmit: () => void
}

export function AddCategoryModal({
  isOpen,
  value,
  onChange,
  onClose,
  onSubmit,
}: AddCategoryModalProps) {
  if (!isOpen) return null

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
      <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl dark:bg-brand-800">
        <h3 className="text-xl font-bold text-brand-900 dark:text-white mb-4">新建收藏夹</h3>
        <input
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder="收藏夹名称"
          className="w-full p-2 border border-brand-300 rounded-lg mb-4 dark:bg-brand-700 dark:border-brand-600 dark:text-white focus:ring-2 focus:ring-blue-500"
          autoFocus
        />
        <div className="flex justify-end gap-2">
          <button
            onClick={onClose}
            className="px-4 py-2 text-brand-700 hover:bg-brand-100 rounded-lg dark:text-brand-300 dark:hover:bg-brand-700"
          >
            取消
          </button>
          <button
            onClick={onSubmit}
            disabled={!value.trim()}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            创建
          </button>
        </div>
      </div>
    </div>,
    document.body
  )
}