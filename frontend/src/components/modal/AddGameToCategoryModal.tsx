import {models} from "../../../wailsjs/go/models";

interface AddGameToCategoryModalProps {
  isOpen: boolean
  allGames: models.Game[]
  onClose: () => void
  onAddGame: (gameId: string) => void
}

export function AddGameToCategoryModal({
  isOpen,
  allGames,
  onClose,
  onAddGame,
}: AddGameToCategoryModalProps) {
  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
      <div className="w-full max-w-2xl h-[80vh] rounded-xl bg-white flex flex-col shadow-xl dark:bg-gray-800">
        <div className="p-6 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
          <h3 className="text-xl font-bold text-gray-900 dark:text-white">添加游戏到收藏夹</h3>
          <button
            onClick={onClose}
            className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-white"
          >
            <div className="i-mdi-close text-xl" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-6">
          {allGames.length > 0 ? (
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-4">
              {allGames.map((game) => (
                <button
                  key={game.id}
                  onClick={() => onAddGame(game.id)}
                  className="flex flex-col items-center p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors text-left group"
                >
                  <div className="w-full aspect-[3/4] rounded-lg overflow-hidden bg-gray-200 dark:bg-gray-700 mb-2 relative">
                    {game.cover_url ? (
                      <img src={game.cover_url} alt={game.name} className="w-full h-full object-cover" />
                    ) : (
                      <div className="w-full h-full flex items-center justify-center text-gray-400">
                        <div className="i-mdi-image-off text-2xl" />
                      </div>
                    )}
                    <div className="absolute inset-0 bg-black/40 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity">
                      <div className="i-mdi-plus text-white text-3xl" />
                    </div>
                  </div>
                  <span className="text-sm font-medium text-gray-900 dark:text-white line-clamp-2 w-full">
                    {game.name}
                  </span>
                </button>
              ))}
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center h-full text-gray-500">
              <p>没有可添加的游戏</p>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}