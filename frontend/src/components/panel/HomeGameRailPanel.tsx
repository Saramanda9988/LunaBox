import type { models, vo } from "../../../wailsjs/go/models";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useHorizontalRailScrollControls } from "../../hooks/useHorizontalRailScrollControls";
import { formatDuration } from "../../utils/time";
import { BetterEdgeIconButton } from "../ui/better/BetterEdgeIconButton";
import { SlideButton } from "../ui/SlideButton";

type HomeGameRailPanelTab = "recent" | "stats";

interface HomeGameRailPanelProps {
  games: models.Game[];
  isExpanded: boolean;
  libraryStats: vo.PeriodStats | null;
  onExpandedChange: (isExpanded: boolean) => void;
  onPauseChange: (isPaused: boolean) => void;
  onSelectGame: (gameId: string) => void;
  selectedGameId: string;
}

export function HomeGameRailPanel({
  games,
  isExpanded,
  libraryStats,
  onExpandedChange,
  onPauseChange,
  onSelectGame,
  selectedGameId,
}: HomeGameRailPanelProps) {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<HomeGameRailPanelTab>("recent");
  const { canScrollNext, canScrollPrev, hasOverflow, scrollerRef, scrollRail }
    = useHorizontalRailScrollControls({
      contentVersion: games.length,
      enabled: isExpanded && activeTab === "recent",
    });

  const tabOptions = useMemo(
    () => [
      {
        label: t("home.recentPlayed"),
        value: "recent" as const,
        icon: <span className="i-mdi-history text-base" />,
      },
      {
        label: t("stats.library.sectionTitle"),
        value: "stats" as const,
        icon: <span className="i-mdi-library-shelves text-base" />,
      },
    ],
    [t],
  );

  const libraryOverviewItems = useMemo(
    () => [
      {
        value: libraryStats?.all_sessions_count ?? "--",
        label: t("stats.library.totalSessions"),
        icon: "i-mdi-counter",
      },
      {
        value: libraryStats
          ? formatDuration(libraryStats.all_sessions_duration, t)
          : "--",
        label: t("stats.library.totalDuration"),
        icon: "i-mdi-timer-outline",
      },
      {
        value: libraryStats?.library_games_count ?? "--",
        label: t("stats.library.totalGames"),
        icon: "i-mdi-gamepad-square-outline",
      },
      {
        value: libraryStats?.all_completed_games_count ?? "--",
        label: t("stats.library.completedGames"),
        icon: "i-mdi-trophy-outline",
      },
    ],
    [libraryStats, t],
  );

  return (
    <div
      className={`absolute inset-x-0 bottom-0 z-20 transition-all duration-300 ease-out ${
        isExpanded ? "translate-y-0" : "translate-y-full"
      }`}
      onMouseEnter={() => isExpanded && onPauseChange(true)}
      onMouseLeave={() => isExpanded && onPauseChange(false)}
    >
      <BetterEdgeIconButton
        placement="bottom"
        icon={isExpanded ? "i-mdi-chevron-down" : "i-mdi-chevron-up"}
        onClick={() => onExpandedChange(!isExpanded)}
        aria-label={
          isExpanded
            ? t("home.collapseCoverPicker")
            : t("home.expandCoverPicker")
        }
        blurClassName="backdrop-blur-[2px]"
        className="absolute left-1/2 top-0 z-30 -translate-x-1/2 -translate-y-full border-b-0 border-white/15 dark:border-white/8"
        surfaceClassName="bg-white/10 hover:bg-white/18 dark:bg-black/12 dark:hover:bg-black/20 data-glass:bg-white/10 data-glass:hover:bg-white/18 data-glass:dark:bg-black/12 data-glass:dark:hover:bg-black/20"
      />
      <div className="pointer-events-none absolute inset-x-0 bottom-0 h-72 bg-gradient-to-t from-black/25 via-black/8 to-transparent dark:from-black/40 dark:via-black/10" />
      <div
        className={`relative overflow-hidden rounded-t-2xl bg-white/10 px-1 pb-4 pt-5 shadow-lg backdrop-blur-[2px] transition-opacity duration-200 dark:bg-black/12 ${
          isExpanded ? "opacity-100" : "pointer-events-none opacity-0"
        }`}
        aria-hidden={!isExpanded}
      >
        {activeTab === "recent" ? (
          <div className="relative">
            <div
              ref={scrollerRef}
              className="scrollbar-hide -my-2 w-full overflow-x-auto px-0.5 py-2 scroll-smooth [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]"
            >
              <div
                className={`flex gap-1 ${
                  hasOverflow ? "w-max justify-start" : "w-full justify-center"
                }`}
              >
                {games.map((game) => {
                  const isActive = game.id === selectedGameId;
                  return (
                    <button
                      type="button"
                      key={game.id}
                      onClick={() => onSelectGame(game.id)}
                      tabIndex={isExpanded ? 0 : -1}
                      className={`group relative h-48 w-36 shrink-0 snap-center rounded-xl border p-[2px] shadow-lg transition-all duration-300 hover:scale-[1.03] hover:shadow-xl ${
                        isActive
                          ? "border-transparent opacity-100 shadow-[0_0_24px_rgba(244,63,94,0.38)]"
                          : "border-white/30 bg-white/30 opacity-75 hover:-translate-y-1 hover:opacity-100 hover:border-white/60 dark:bg-black/20"
                      }`}
                      aria-label={t("home.selectGame", {
                        name: game.name,
                      })}
                    >
                      {isActive && (
                        <span
                          className="pointer-events-none absolute inset-0 overflow-hidden rounded-xl"
                          aria-hidden="true"
                        >
                          <span className="absolute left-1/2 top-1/2 h-[22rem] w-[22rem] -translate-x-1/2 -translate-y-1/2">
                            <span
                              className="absolute inset-0 animate-spin bg-[conic-gradient(from_0deg,#ef4444_0deg,#a855f7_90deg,#dc2626_180deg,#7e22ce_270deg,#ef4444_360deg)] opacity-95 blur-[1px]"
                              style={{ animationDuration: "3s" }}
                            />
                          </span>
                        </span>
                      )}
                      <div className="relative z-10 h-full w-full rounded-[0.65rem] bg-brand-200 dark:bg-brand-800/70">
                        {game.cover_url ? (
                          <img
                            src={game.cover_url}
                            alt={game.name}
                            referrerPolicy="no-referrer"
                            className="h-full w-full rounded-[0.65rem] object-cover"
                            draggable="false"
                            onDragStart={e => e.preventDefault()}
                          />
                        ) : (
                          <div className="flex h-full w-full items-center justify-center rounded-[0.65rem] text-brand-400 dark:text-white/50">
                            <span className="i-mdi-image-off text-3xl" />
                          </div>
                        )}
                        <div className="absolute inset-x-0 bottom-0 h-1/2 rounded-b-[0.65rem] bg-gradient-to-t from-black/60 to-transparent" />
                      </div>
                    </button>
                  );
                })}
              </div>
            </div>

            {canScrollPrev && (
              <BetterEdgeIconButton
                placement="left"
                icon="i-mdi-chevron-left"
                onClick={() => scrollRail(-1)}
                aria-label={t("home.scrollRecentPrev", "向前查看更多最近游玩")}
                className="absolute left-0 top-1/2 z-10 -translate-y-1/2"
              />
            )}

            {canScrollNext && (
              <BetterEdgeIconButton
                placement="right"
                icon="i-mdi-chevron-right"
                onClick={() => scrollRail(1)}
                aria-label={t("home.scrollRecentNext", "向后查看更多最近游玩")}
                className="absolute right-0 top-1/2 z-10 -translate-y-1/2"
              />
            )}
          </div>
        ) : (
          <div className="grid h-48 grid-cols-4 items-center gap-4 px-8 py-2">
            {libraryOverviewItems.map(item => (
              <div
                key={item.label}
                className="flex h-full min-w-0 flex-col justify-center gap-3 px-4"
              >
                <span
                  className={`${item.icon} text-3xl text-brand-500/80 dark:text-brand-300/80`}
                  aria-hidden="true"
                />
                <div>
                  <p className="whitespace-normal break-words text-2xl font-bold leading-tight text-brand-900 drop-shadow dark:text-white">
                    {item.value}
                  </p>
                  <p className="mt-1 text-sm text-brand-600 drop-shadow dark:text-white/70">
                    {item.label}
                  </p>
                </div>
              </div>
            ))}
          </div>
        )}

        <div className="mt-2 flex justify-center">
          <SlideButton
            options={tabOptions}
            value={activeTab}
            onChange={setActiveTab}
            className="bg-white/35 dark:bg-black/30"
            disabled={!isExpanded}
          />
        </div>
      </div>
    </div>
  );
}
