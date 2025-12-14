import { models } from '../../wailsjs/go/models'
import { StartGameWithTracking } from '../../wailsjs/go/service/TimerService'

interface GameCardProps {
  game: models.Game
}

export function GameCard({ game }: GameCardProps) {
  const handleStartGame = async (e: React.MouseEvent) => {
    e.stopPropagation()
    if (game.id) {
      try {
        await StartGameWithTracking(game.id)
      } catch (error) {
        console.error('Failed to start game:', error)
      }
    }
  }

  return (
    <div className="group relative flex w-full flex-col overflow-hidden rounded-xl border border-gray-100 bg-white shadow-sm transition-all duration-300 hover:shadow-xl dark:border-gray-700 dark:bg-gray-800">
      <div className="relative aspect-[3/3.6] w-full overflow-hidden bg-gray-200 dark:bg-gray-700">
        {game.cover_url ? (
          <img
            src={game.cover_url}
            alt={game.name}
            className="h-full w-full object-cover object-center transition-transform duration-500 group-hover:scale-110"
          />
        ) : (
          <div className="flex h-full items-center justify-center text-gray-400">
            <div className="i-mdi-image-off text-4xl" />
          </div>
        )}
        
        {/* Hover Overlay */}
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-black/40 opacity-0 backdrop-blur-[2px] transition-all duration-300 group-hover:opacity-100">
          <button
            onClick={handleStartGame}
            className="flex h-8 w-8 items-center justify-center rounded-full bg-blue-600 text-white shadow-lg transition-transform hover:scale-110 hover:bg-blue-500 active:scale-95"
            title="启动游戏"
          >
            <div className="i-mdi-play text-lg" />
          </button>
          <button
            className="flex h-8 w-8 items-center justify-center rounded-full bg-white/20 text-white backdrop-blur-md transition-transform hover:scale-110 hover:bg-white/30 active:scale-95"
            title="查看详情"
          >
            <div className="i-mdi-information-variant text-lg" />
          </button>
        </div>
      </div>
      
      <div className="px-2 pt-1 pb-2">
        <h3 className="truncate text-sm font-bold text-gray-900 dark:text-white leading-tight" title={game.name}>
          {game.name}
        </h3>
        <p className="truncate text-xs text-gray-500 dark:text-gray-400 leading-tight">
          {game.company || 'Unknown Developer'}
        </p>
      </div>
    </div>
  )
}
