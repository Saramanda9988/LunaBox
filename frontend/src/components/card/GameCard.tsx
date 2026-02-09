import type { models } from "../../../wailsjs/go/models";
import { useNavigate } from "@tanstack/react-router";
import { toast } from "react-hot-toast";
import { enums } from "../../../wailsjs/go/models";
import { StartGameWithTracking } from "../../../wailsjs/go/service/StartService";

interface GameCardProps {
  game: models.Game;
  selectionMode?: boolean;
  selected?: boolean;
  onSelectChange?: (selected: boolean) => void;
}

export function GameCard({
  game,
  selectionMode = false,
  selected = false,
  onSelectChange,
}: GameCardProps) {
  const navigate = useNavigate();

  const handleToggleSelect = (e: React.MouseEvent) => {
    e.stopPropagation();
    onSelectChange?.(!selected);
  };

  const handleStartGame = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (game.id) {
      try {
        const started = await StartGameWithTracking(game.id);
        if (started) {
          toast.success(`${game.name} 启动成功`);
        }
        else {
          toast.error(`${game.name} 启动失败（未能启动）`);
        }
      }
      catch (error) {
        console.error("Failed to start game:", error);
        const notyfication = `${game.name} 启动失败, 查询日志获得帮助`;
        toast.error(notyfication);
      }
    }
  };

  const handleViewDetails = () => {
    navigate({ to: `/game/${game.id}` });
  };

  const isCompleted = game.status === enums.GameStatus.COMPLETED;

  return (
    <div
      className={`glass-card group relative flex w-full flex-col overflow-hidden rounded-xl border border-brand-100 bg-white shadow-sm transition-all duration-300 hover:shadow-xl dark:border-brand-700 dark:bg-brand-800 ${selectionMode ? "cursor-pointer" : ""} ${selectionMode && selected ? "ring-2 ring-neutral-500 dark:ring-neutral-400" : ""}`}
      onClick={selectionMode ? handleToggleSelect : undefined}
    >
      {selectionMode && (
        <button
          type="button"
          onClick={handleToggleSelect}
          className={`absolute left-2 top-2 z-10 flex h-6 w-6 items-center justify-center rounded-full border
                      ${selected
          ? "bg-neutral-600 text-white border-neutral-600"
          : "bg-white/90 text-transparent border-brand-300 dark:bg-brand-800/90 dark:border-brand-600"}
                      shadow-sm`}
          title={selected ? "取消选择" : "选择"}
        >
          <div className="i-mdi-check text-sm" />
        </button>
      )}
      <div className="relative aspect-[3/3.6] w-full overflow-hidden bg-brand-200 dark:bg-brand-700">
        {game.cover_url
          ? (
              <img
                src={game.cover_url}
                alt={game.name}
                referrerPolicy="no-referrer"
                className="h-full w-full object-cover object-center transition-transform duration-500 group-hover:scale-110"
              />
            )
          : (
              <div className="flex h-full items-center justify-center text-brand-400">
                <div className="i-mdi-image-off text-4xl" />
              </div>
            )}

        {/* 已通关奖杯标识 */}
        {isCompleted && (
          <div className="absolute top-1.5 right-1.5 flex h-6 w-6 items-center justify-center rounded-full bg-yellow-500 shadow-lg">
            <div className="i-mdi-trophy text-sm text-white" />
          </div>
        )}

        {!selectionMode && (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-black/40 opacity-0 backdrop-blur-[2px] transition-all duration-300 group-hover:opacity-100">
            <button
              onClick={handleStartGame}
              className="flex h-8 w-8 items-center justify-center rounded-full bg-neutral-600 text-white shadow-lg transition-transform hover:scale-110 hover:bg-neutral-500 active:scale-95"
              title="启动游戏"
            >
              <div className="i-mdi-play text-lg" />
            </button>
            <button
              onClick={handleViewDetails}
              className="flex h-8 w-8 items-center justify-center rounded-full bg-white/20 text-white backdrop-blur-md transition-transform hover:scale-110 hover:bg-white/30 active:scale-95"
              title="查看详情"
            >
              <div className="i-mdi-information-variant text-lg" />
            </button>
          </div>
        )}
      </div>

      <div className="px-2 pt-1 pb-2">
        <h3 className="truncate text-sm font-bold text-brand-900 dark:text-white leading-tight" title={game.name}>
          {game.name}
        </h3>
        <p className="truncate text-xs text-brand-500 dark:text-brand-400 leading-tight">
          {game.company || "Unknown Developer"}
        </p>
      </div>
    </div>
  );
}
