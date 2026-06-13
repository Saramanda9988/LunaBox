import type { vo } from "../../../wailsjs/go/models";
import { useTranslation } from "react-i18next";
import { formatDuration } from "../../utils/time";
import { ModalPortal } from "../ui/ModalPortal";
import { ProxyImage } from "../ui/ProxyImage";

interface StatsLeaderboardModalProps {
  isOpen: boolean;
  games: vo.GamePlayStats[];
  onClose: () => void;
}

export function StatsLeaderboardModal({
  isOpen,
  games,
  onClose,
}: StatsLeaderboardModalProps) {
  const { t } = useTranslation();

  if (!isOpen) {
    return null;
  }

  return (
    <ModalPortal>
      <div
        className="absolute inset-0 z-60 flex items-center justify-center bg-black/50 px-4 backdrop-blur-sm"
        onClick={onClose}
      >
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="stats-leaderboard-modal-title"
          className="flex max-h-[84vh] w-full max-w-4xl flex-col overflow-hidden rounded-xl bg-white shadow-2xl dark:bg-brand-800"
          onClick={event => event.stopPropagation()}
        >
          <div className="flex items-center justify-between border-b border-brand-200 p-5 dark:border-brand-700">
            <div className="min-w-0">
              <h3
                id="stats-leaderboard-modal-title"
                className="truncate text-lg font-bold text-brand-900 dark:text-white"
              >
                {t("stats.leaderboard.modalTitle")}
              </h3>
              <p className="mt-1 text-sm text-brand-500 dark:text-brand-400">
                {t("stats.leaderboard.countHint", { count: games.length })}
              </p>
            </div>
            <button
              type="button"
              onClick={onClose}
              aria-label={t("common.cancel")}
              className="i-mdi-close rounded-lg p-1 text-2xl text-brand-500 hover:bg-brand-100 hover:text-brand-700 focus:outline-none dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-200"
            />
          </div>

          <div className="flex-1 overflow-y-auto p-5">
            <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
              {games.map((game, index) => {
                const rank = index + 1;
                const isTopRank = rank <= 3;
                return (
                  <div
                    key={game.game_id}
                    className={`flex items-center gap-3 rounded-lg border p-3 transition-colors ${
                      isTopRank
                        ? "border-yellow-200/70 bg-yellow-50/80 dark:border-yellow-800/40 dark:bg-yellow-900/10"
                        : "border-brand-200 bg-brand-50/80 dark:border-brand-700 dark:bg-brand-900/30"
                    }`}
                  >
                    <span
                      className={`w-8 text-center text-sm font-semibold tabular-nums ${
                        isTopRank
                          ? "text-yellow-600 dark:text-yellow-300"
                          : "text-brand-500 dark:text-brand-400"
                      }`}
                    >
                      #
                      {rank}
                    </span>
                    <ProxyImage
                      src={game.cover_url}
                      alt={game.game_name}
                      className="h-14 w-10 flex-shrink-0 rounded bg-brand-200 object-cover shadow-sm dark:bg-brand-700"
                    />
                    <div className="min-w-0 flex-1">
                      <p className="line-clamp-1 text-sm font-medium text-brand-900 dark:text-white">
                        {game.game_name}
                      </p>
                      <p className="mt-0.5 font-mono text-xs text-brand-600 dark:text-brand-300">
                        {formatDuration(game.total_duration, t)}
                      </p>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          <div className="flex justify-end border-t border-brand-200 p-4 dark:border-brand-700">
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg bg-brand-700 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-brand-800 dark:bg-brand-600 dark:hover:bg-brand-500"
            >
              {t("common.confirm")}
            </button>
          </div>
        </div>
      </div>
    </ModalPortal>
  );
}
