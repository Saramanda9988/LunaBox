import type { TFunction } from "i18next";
import type { models } from "../../../wailsjs/go/models";
import { useNavigate } from "@tanstack/react-router";
import { memo, useCallback } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { enums } from "../../../wailsjs/go/models";
import { useAppStore } from "../../store";
import { formatLocalDate } from "../../utils/time";
import { ProxyImage } from "../ui/ProxyImage";

function HighlightText({ text, query }: { text: string; query: string }) {
  if (!query || !text) {
    return <>{text}</>;
  }
  const q = query.toLowerCase();
  const idx = text.toLowerCase().indexOf(q);
  if (idx === -1) {
    return <>{text}</>;
  }
  return (
    <>
      {text.slice(0, idx)}
      <mark className="bg-yellow-300/80 dark:bg-yellow-500/50 text-inherit rounded-[2px] px-[1px]">
        {text.slice(idx, idx + query.length)}
      </mark>
      {text.slice(idx + query.length)}
    </>
  );
}

// ─────────────────────────────────────────────────────────────────────────────

function formatSortFieldValue(
  game: models.Game,
  sortBy: enums.GameListSortBy | null | undefined,
  t: TFunction,
): string | null {
  if (!sortBy || sortBy === enums.GameListSortBy.NAME) {
    return null;
  }
  switch (sortBy) {
    case enums.GameListSortBy.LAST_PLAYED_AT:
      return game.last_played_at
        ? formatLocalDate(game.last_played_at)
        : t("common.never");
    case enums.GameListSortBy.CREATED_AT:
      return formatLocalDate(game.created_at);
    case enums.GameListSortBy.RATING:
      return `${(game.rating ?? 0).toFixed(1)}/10.0`;
    case enums.GameListSortBy.RELEASE_DATE:
      return game.release_date || t("common.unknownDate");
    default:
      return null;
  }
}

interface GameCardProps {
  game: models.Game;
  selectionMode?: boolean;
  selected?: boolean;
  onSelectChange?: (selected: boolean) => void;
  /** 当前搜索词，用于高亮游戏名和开发商 */
  searchQuery?: string;
  /** 当前排序维度；非 null 且非 NAME 时，在封面底部显示对应字段值 */
  displaySortField?: enums.GameListSortBy | null;
}

function GameCardComponent({
  game,
  selectionMode = false,
  selected = false,
  onSelectChange,
  searchQuery = "",
  displaySortField = null,
}: GameCardProps) {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const startGame = useAppStore(state => state.startGame);
  const gameRuntime = useAppStore(state =>
    game.id ? state.gameRuntimes[game.id] : undefined,
  );
  const isCurrentGameRunning = Boolean(gameRuntime);
  const isCurrentGameEnding = gameRuntime?.state === "ending";

  const handleToggleSelect = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      onSelectChange?.(!selected);
    },
    [onSelectChange, selected],
  );

  const handleStartGame = useCallback(
    async (e: React.MouseEvent) => {
      e.stopPropagation();
      if (isCurrentGameRunning) {
        return;
      }
      if (game.id) {
        try {
          const started = await startGame(game);
          // if (started) {
          //   toast.success(t("gameCard.startSuccess", { name: game.name }));
          // }
          // else {
          //   toast.error(
          //     t("gameCard.startFailedNotLaunched", { name: game.name }),
          //   );
          // }
          if (!started) {
            toast.error(
              t("gameCard.startFailedNotLaunched", { name: game.name }),
            );
          }
        }
        catch (error) {
          console.error("Failed to start game:", error);
          toast.error(t("gameCard.startFailedLog", { name: game.name }));
        }
      }
    },
    [game, isCurrentGameRunning, startGame, t],
  );

  const handleViewDetails = useCallback(() => {
    navigate({ to: `/game/${game.id}` });
  }, [game.id, navigate]);

  const isCompleted = game.status === enums.GameStatus.COMPLETED;
  const companyDisplay = game.company || t("common.unknownDeveloper");
  const sortFieldText = formatSortFieldValue(game, displaySortField, t);

  return (
    <div
      className={`glass-card group relative flex w-full flex-col overflow-hidden rounded-xl border border-brand-100 bg-white shadow-sm transition-shadow duration-200 hover:shadow-lg dark:border-brand-700 dark:bg-brand-800 ${selectionMode ? "cursor-pointer" : ""} ${selectionMode && selected ? "ring-2 ring-neutral-500 dark:ring-neutral-400" : ""}`}
      onClick={selectionMode ? handleToggleSelect : undefined}
    >
      {selectionMode && (
        <button
          type="button"
          onClick={handleToggleSelect}
          className={`absolute left-2 top-2 z-10 flex h-6 w-6 items-center justify-center rounded-full border
                      ${
        selected
          ? "bg-neutral-600 text-white border-neutral-600"
          : "bg-white/90 text-transparent border-brand-300 dark:bg-brand-800/90 dark:border-brand-600"
        }
                      shadow-sm`}
        >
          <div className="i-mdi-check text-sm" />
        </button>
      )}
      <div className="relative aspect-[3/3.6] w-full overflow-hidden bg-brand-200 dark:bg-brand-700">
        {game.cover_url ? (
          <ProxyImage
            src={game.cover_url}
            alt={game.name}
            className="h-full w-full object-cover object-center transition-transform duration-300 group-hover:scale-110"
            decoding="async"
            loading="lazy"
          />
        ) : (
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

        {/* 当前排序字段值（封面底部覆盖条） */}
        {sortFieldText && (
          <div className="pointer-events-none absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/75 via-black/45 to-transparent px-2 pt-5 pb-1.5">
            <p className="truncate text-xs font-semibold text-white drop-shadow-sm">
              {sortFieldText}
            </p>
          </div>
        )}

        {!selectionMode && (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-black/40 opacity-0 backdrop-blur-[2px] transition-opacity duration-200 group-hover:opacity-100">
            <button
              type="button"
              onClick={handleStartGame}
              disabled={isCurrentGameRunning}
              aria-label={t("gameCard.startGame")}
              className="flex h-8 w-8 items-center justify-center rounded-full bg-neutral-600 text-white shadow-lg transition-transform hover:scale-110 hover:bg-neutral-500 active:scale-95 disabled:cursor-not-allowed disabled:opacity-65 disabled:hover:scale-100"
            >
              <div
                className={
                  isCurrentGameEnding
                    ? "i-mdi-loading animate-spin text-lg"
                    : isCurrentGameRunning
                      ? "i-mdi-gamepad-variant text-lg"
                      : "i-mdi-play text-lg"
                }
              />
            </button>
            <button
              type="button"
              onClick={handleViewDetails}
              className="flex h-8 w-8 items-center justify-center rounded-full bg-white/20 text-white backdrop-blur-md transition-transform hover:scale-110 hover:bg-white/30 active:scale-95"
            >
              <div className="i-mdi-information-variant text-lg" />
            </button>
          </div>
        )}
      </div>

      <div className="px-2 pt-1 pb-2">
        <h3 className="truncate text-sm font-bold text-brand-900 dark:text-white leading-tight">
          <HighlightText text={game.name} query={searchQuery} />
        </h3>
        <p className="truncate text-xs text-brand-500 dark:text-brand-400 leading-tight">
          <HighlightText text={companyDisplay} query={searchQuery} />
        </p>
      </div>
    </div>
  );
}

export const GameCard = memo(GameCardComponent);
