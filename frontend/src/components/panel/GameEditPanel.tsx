import { models } from '../../../wailsjs/go/models'

interface GameEditFormProps {
  game: models.Game
  onGameChange: (game: models.Game) => void
  onDelete: () => void
  onSelectExecutable: () => void
  onSelectSaveDirectory: () => void
  onSelectCoverImage: () => void
  onUpdateFromRemote?: () => void
}

export function GameEditPanel({
  game,
  onGameChange,
  onDelete,
  onSelectExecutable,
  onSelectSaveDirectory,
  onSelectCoverImage,
  onUpdateFromRemote,
}: GameEditFormProps) {
  return (
    <div className="mx-auto bg-white dark:bg-brand-800 p-8 rounded-lg shadow-sm">
      <div className="space-y-6">
        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            游戏名称
          </label>
          <input
            type="text"
            value={game.name}
            onChange={(e) => onGameChange({ ...game, name: e.target.value } as models.Game)}
            className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            封面图片
          </label>
          <div className="flex gap-2">
            <input
              type="text"
              value={game.cover_url}
              onChange={(e) => onGameChange({ ...game, cover_url: e.target.value } as models.Game)}
              placeholder="输入图片 URL 或选择本地图片"
              className="flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            />
            <button
              type="button"
              onClick={onSelectCoverImage}
              className="px-4 py-2 bg-brand-100 dark:bg-brand-700 text-brand-700 dark:text-brand-300 rounded-md hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors"
            >
              选择
            </button>
          </div>
          <p className="mt-1 text-xs text-brand-500">支持远端url获取和本地图片选取</p>
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            开发商
          </label>
          <input
            type="text"
            value={game.company}
            onChange={(e) => onGameChange({ ...game, company: e.target.value } as models.Game)}
            className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
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
              onChange={(e) => onGameChange({ ...game, path: e.target.value } as models.Game)}
              className="flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            />
            <button
              type="button"
              onClick={onSelectExecutable}
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
              onChange={(e) => onGameChange({ ...game, save_path: e.target.value } as models.Game)}
              placeholder="选择游戏存档所在目录"
              className="flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            />
            <button
              type="button"
              onClick={onSelectSaveDirectory}
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
            onChange={(e) => onGameChange({ ...game, summary: e.target.value } as models.Game)}
            rows={6}
            className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none resize-none"
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              数据源类型
            </label>
            <select
              value={game.source_type || ''}
              onChange={(e) => onGameChange({ ...game, source_type: e.target.value } as models.Game)}
              className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            >
              <option value="">无</option>
              <option value="local">本地</option>
              <option value="bangumi">Bangumi</option>
              <option value="vndb">VNDB</option>
              <option value="ymgal">月幕Galgame</option>
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              数据源ID
            </label>
            <input
              type="text"
              value={game.source_id || ''}
              onChange={(e) => onGameChange({ ...game, source_id: e.target.value } as models.Game)}
              placeholder="远程数据源的ID"
              className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
            />
          </div>
        </div>

        <div className="flex justify-between pt-4">
          <div className="flex gap-4 justify-end w-full">
            {onUpdateFromRemote && (
              <button
                type="button"
                onClick={onUpdateFromRemote}
                className="px-6 py-2 bg-accent-500 text-white rounded-md hover:bg-accent-700 transition-colors focus:ring-2 focus:ring-offset-2 focus:ring-accent-500"
              >
                从远程更新
              </button>
            )}
            <button
              type="button"
              onClick={onDelete}
              className="px-6 py-2 bg-error-500 text-white rounded-md hover:bg-error-700 transition-colors focus:ring-2 focus:ring-offset-2 focus:ring-error-500"
            >
              删除
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
