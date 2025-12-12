import { models } from '../../wailsjs/go/models'

interface GameCardProps {
  game: models.Game
}

export function GameCard({ game }: GameCardProps) {
  return (
    <div className="group relative flex flex-col w-48 transition-transform hover:scale-105">
      <div className="aspect-[3/4] w-full overflow-hidden rounded-lg bg-gray-200 dark:bg-gray-700 shadow-md">
        {game.cover_url ? (
          <img
            src={game.cover_url}
            alt={game.name}
            className="h-full w-full object-cover object-center"
          />
        ) : (
          <div className="flex h-full items-center justify-center text-gray-400">
            <div className="i-mdi-image-off text-4xl" />
          </div>
        )}
        <div className="absolute inset-0 bg-black/0 group-hover:bg-black/10 transition-colors" />
      </div>
      <div className="mt-2">
        <h3 className="text-sm font-medium text-gray-900 dark:text-white truncate" title={game.name}>
          {game.name}
        </h3>
        <p className="text-xs text-gray-500 dark:text-gray-400 truncate">{game.company || 'Unknown Developer'}</p>
      </div>
    </div>
  )
}
