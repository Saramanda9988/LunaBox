import { useState } from 'react'
import { models, vo, enums } from '../../wailsjs/go/models'
import { SelectGameExecutable, FetchMetadataByName, FetchMetadata, AddGame } from '../../wailsjs/go/service/GameService'

interface AddGameModalProps {
  isOpen: boolean
  onClose: () => void
  onGameAdded: () => void
}

export function AddGameModal({ isOpen, onClose, onGameAdded }: AddGameModalProps) {
  const [step, setStep] = useState<1 | 2 | 3>(1)
  const [executablePath, setExecutablePath] = useState('')
  const [gameName, setGameName] = useState('')
  const [metadataResults, setMetadataResults] = useState<vo.GameMetadataFromWebVO[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [manualId, setManualId] = useState('')
  const [manualSource, setManualSource] = useState<enums.SourceType>(enums.SourceType.BANGUMI)

  if (!isOpen) return null

  const handleSelectExecutable = async () => {
    try {
      const path = await SelectGameExecutable()
      if (path) {
        setExecutablePath(path)
        // Extract parent folder name
        // Windows path usually uses backslash, but Wails might return forward slash or mixed.
        // Let's handle both.
        const normalizedPath = path.replace(/\\/g, '/')
        const parts = normalizedPath.split('/')
        // If it's a file, the last part is filename, the one before is parent folder
        if (parts.length > 1) {
          setGameName(parts[parts.length - 2])
        }
      }
    } catch (error) {
      console.error('Failed to select executable:', error)
    }
  }

  const handleSearchByName = async () => {
    if (!gameName) return
    setIsLoading(true)
    try {
      const results = await FetchMetadataByName(gameName)
      setMetadataResults(results || [])
      setStep(2)
    } catch (error) {
      console.error('Failed to fetch metadata:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const handleSearchById = async () => {
    if (!manualId) return
    setIsLoading(true)
    try {
      const request = new vo.MetadataRequest({
        source: manualSource,
        id: manualId
      })
      const game = await FetchMetadata(request)
      if (game) {
        await saveGame(game)
      }
    } catch (error) {
      console.error('Failed to fetch metadata by ID:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const saveGame = async (game: models.Game) => {
    try {
      game.path = executablePath
      // Ensure other fields are set if missing?
      // The backend AddGame handles ID generation if empty.
      await AddGame(game)
      onGameAdded()
      resetAndClose()
    } catch (error) {
      console.error('Failed to save game:', error)
    }
  }

  const resetAndClose = () => {
    setStep(1)
    setExecutablePath('')
    setGameName('')
    setMetadataResults([])
    setManualId('')
    onClose()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
      <div className="w-full max-w-2xl rounded-xl bg-white p-6 shadow-2xl dark:bg-gray-800">
        <div className="mb-6 flex items-center justify-between">
          <h2 className="text-2xl font-bold text-gray-900 dark:text-white">添加游戏</h2>
          <button onClick={resetAndClose} className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200">
            <div className="i-mdi-close text-2xl" />
          </button>
        </div>

        {step === 1 && (
          <div className="space-y-6">
            <button
              onClick={handleSelectExecutable}
              className="flex w-full items-center justify-center rounded-lg bg-blue-500 py-4 text-white transition hover:bg-blue-600"
            >
              <div className="i-mdi-file-find mr-2 text-xl" />
              选择启动程序
            </button>

            <div>
              <input
                type="text"
                value={executablePath}
                readOnly
                placeholder="请选择一个可执行程序"
                className="w-full rounded-lg border border-gray-300 bg-gray-50 p-3 text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
              />
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-white">游戏名称 *</label>
              <input
                type="text"
                value={gameName}
                onChange={(e) => setGameName(e.target.value)}
                className="w-full rounded-lg border border-gray-300 bg-gray-50 p-3 text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
              />
            </div>

            <div className="flex justify-end space-x-4">
              <button
                onClick={resetAndClose}
                className="rounded-lg border border-gray-300 px-5 py-2.5 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
              >
                取消
              </button>
              <button
                onClick={handleSearchByName}
                disabled={!executablePath || !gameName || isLoading}
                className="rounded-lg bg-blue-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
              >
                {isLoading ? '搜索中...' : '确认'}
              </button>
            </div>
          </div>
        )}

        {step === 2 && (
          <div className="space-y-6">
            <p className="text-gray-600 dark:text-gray-300">哪个结果是您期望的？</p>
            
            <div className="grid max-h-[400px] grid-cols-2 gap-4 overflow-y-auto p-1">
              {metadataResults.filter(item => item.Game)
                .map((item, index) => (
                  <div
                    key={index}
                    onClick={() => saveGame(item.Game!)} // 使用非空断言，因为上面已过滤
                    className="cursor-pointer rounded-lg border border-gray-200 p-4 transition hover:border-blue-500 hover:shadow-md dark:border-gray-700 dark:hover:border-blue-400"
                  >
                    <div className="aspect-[3/4] w-full overflow-hidden rounded-md bg-gray-200 dark:bg-gray-700">
                      {item.Game!.cover_url ? (
                        <img src={item.Game!.cover_url} alt={item.Game!.name} className="h-full w-full object-cover" />
                      ) : (
                        <div className="flex h-full items-center justify-center text-gray-400">
                          <div className="i-mdi-image-off text-4xl" />
                        </div>
                      )}
                    </div>
                    <h3 className="mt-2 truncate text-lg font-bold text-gray-900 dark:text-white">{item.Game!.name}</h3>
                    <p className="text-sm text-gray-500 dark:text-gray-400">
                      来自 {item.Source}
                    </p>
                  </div>
                ))}
            </div>

            <div className="flex items-center justify-between border-t border-gray-200 pt-4 dark:border-gray-700">
               <button
                onClick={() => setStep(1)}
                className="text-sm text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
              >
                &larr; 返回上一步
              </button>
              <button
                onClick={() => setStep(3)}
                className="text-sm text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300"
              >
                都不是？输入id查找
              </button>
            </div>
          </div>
        )}

        {step === 3 && (
          <div className="space-y-6">
            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-white">数据源</label>
              <select
                value={manualSource}
                onChange={(e) => setManualSource(e.target.value as enums.SourceType)}
                className="w-full rounded-lg border border-gray-300 bg-gray-50 p-3 text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
              >
                <option value={enums.SourceType.BANGUMI}>Bangumi</option>
                <option value={enums.SourceType.VNDB}>VNDB</option>
              </select>
            </div>

            <div>
              <label className="mb-2 block text-sm font-medium text-gray-900 dark:text-white">游戏 ID</label>
              <input
                type="text"
                value={manualId}
                onChange={(e) => setManualId(e.target.value)}
                placeholder="请输入游戏 ID"
                className="w-full rounded-lg border border-gray-300 bg-gray-50 p-3 text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
              />
            </div>

            <div className="flex justify-end space-x-4">
              <button
                onClick={() => setStep(2)}
                className="rounded-lg border border-gray-300 px-5 py-2.5 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
              >
                返回
              </button>
              <button
                onClick={handleSearchById}
                disabled={!manualId || isLoading}
                className="rounded-lg bg-blue-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
              >
                {isLoading ? '搜索中...' : '确认'}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
