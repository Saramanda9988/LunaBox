import type { models, vo } from "../../wailsjs/go/models";
import { createRoute, useNavigate } from "@tanstack/react-router";
import { useCallback, useEffect, useMemo, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { enums } from "../../wailsjs/go/models";
import { GetGames } from "../../wailsjs/go/service/GameService";
import { StartGameWithTracking } from "../../wailsjs/go/service/StartService";
import { BetterEdgeIconButton } from "../components/ui/better/BetterEdgeIconButton";
import { useAppStore } from "../store";
import { formatDuration, formatLocalDateTime } from "../utils/time";
import { Route as rootRoute } from "./__root";

const RECENT_GAME_LIMIT = 12;
const CAROUSEL_INTERVAL_MS = 4000;

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: HomePage,
});

function hasRecentPlayTime(game: models.Game) {
  return Boolean(game.last_played_at);
}

function HomePage() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const homeData = useAppStore(state => state.homeData);
  const fetchHomeData = useAppStore(state => state.fetchHomeData);
  const isLoading = useAppStore(state => state.isLoading);
  const config = useAppStore(state => state.config);
  const [recentGames, setRecentGames] = useState<models.Game[]>([]);
  const [activeGameId, setActiveGameId] = useState<string | null>(null);
  const [playingGameId, setPlayingGameId] = useState<string | null>(null);
  const [isPickerExpanded, setIsPickerExpanded] = useState(false);
  const [isCarouselPaused, setIsCarouselPaused] = useState(false);

  const loadRecentGames = useCallback(async () => {
    try {
      const response = await GetGames({
        limit: RECENT_GAME_LIMIT,
        offset: 0,
        search_query: "",
        tags: [],
        sort_by: enums.GameListSortBy.LAST_PLAYED_AT,
        sort_order: enums.SortOrder.DESC,
      } as vo.GameListRequest);
      setRecentGames((response?.games || []).filter(hasRecentPlayTime));
    }
    catch (error) {
      console.error("Failed to fetch recent games:", error);
    }
  }, []);

  useEffect(() => {
    void fetchHomeData();
    void loadRecentGames();
  }, [fetchHomeData, loadRecentGames]);

  // 每次 homeData 刷新后都以服务端状态为准，避免本地乐观状态卡住
  useEffect(() => {
    setPlayingGameId(
      homeData?.last_played?.is_playing ? homeData.last_played.game.id : null,
    );
  }, [homeData]);

  const carouselGames = useMemo(() => {
    const games = [...recentGames];
    const lastPlayed = homeData?.last_played;

    if (
      lastPlayed?.game.id
      && !games.some(game => game.id === lastPlayed.game.id)
    ) {
      games.unshift({
        ...lastPlayed.game,
        last_played_at: lastPlayed.last_played_at,
      } as models.Game);
    }

    return games;
  }, [homeData?.last_played, recentGames]);

  useEffect(() => {
    setActiveGameId(current =>
      current && carouselGames.some(game => game.id === current)
        ? current
        : (carouselGames[0]?.id ?? null),
    );
  }, [carouselGames]);

  useEffect(() => {
    if (carouselGames.length <= 1 || isCarouselPaused) {
      return;
    }

    const timer = window.setInterval(() => {
      setActiveGameId((current) => {
        const currentIndex = carouselGames.findIndex(
          game => game.id === current,
        );
        const nextIndex
          = currentIndex >= 0 ? (currentIndex + 1) % carouselGames.length : 0;
        return carouselGames[nextIndex]?.id ?? current;
      });
    }, CAROUSEL_INTERVAL_MS);

    return () => window.clearInterval(timer);
  }, [carouselGames, isCarouselPaused]);

  const selectedGame = useMemo(() => {
    if (carouselGames.length === 0) {
      return null;
    }

    return (
      carouselGames.find(game => game.id === activeGameId) ?? carouselGames[0]
    );
  }, [activeGameId, carouselGames]);

  const selectedLastPlayedAt = useMemo(() => {
    if (!selectedGame) {
      return null;
    }
    const lastPlayed = homeData?.last_played;
    return selectedGame.id === lastPlayed?.game.id
      ? lastPlayed.last_played_at
      : selectedGame.last_played_at;
  }, [homeData?.last_played, selectedGame]);

  const lastPlayedForSelected = homeData?.last_played;
  const selectedTotalPlayedDur
    = selectedGame
      && lastPlayedForSelected
      && selectedGame.id === lastPlayedForSelected.game.id
      ? lastPlayedForSelected.total_played_dur
      : 0;
  const isSelectedGamePlaying = Boolean(
    selectedGame?.id && playingGameId === selectedGame.id,
  );
  const showGameBackground
    = !config?.background_enabled || !config?.background_hide_game_cover;
  const showHeroCover
    = !config?.background_enabled || !config?.background_hide_game_hero_cover;
  const hasCoverPicker = carouselGames.length > 1;
  const showCoverPicker = hasCoverPicker && isPickerExpanded;
  const contentBottomClass = showCoverPicker ? "bottom-52" : "bottom-8";

  const openGameDetail = useCallback(
    (gameId?: string) => {
      if (!gameId) {
        return;
      }
      navigate({ to: "/game/$gameId", params: { gameId } });
    },
    [navigate],
  );

  const handleContinuePlay = useCallback(async () => {
    if (!selectedGame?.id) {
      return;
    }

    try {
      const success = await StartGameWithTracking(selectedGame.id);
      if (success) {
        setPlayingGameId(selectedGame.id);
        setActiveGameId(selectedGame.id);
        toast.success(t("home.toast.launching", { name: selectedGame.name }));
        void fetchHomeData();
        void loadRecentGames();
      }
    }
    catch (err) {
      console.error("Failed to launch game:", err);
      toast.error(t("home.toast.launchFailed"));
    }
  }, [fetchHomeData, loadRecentGames, selectedGame, t]);

  if (isLoading) {
    return null;
  }

  if (!homeData) {
    return (
      <div className="flex h-full flex-col items-center justify-center space-y-4">
        <p className="text-brand-500">{t("home.noData")}</p>
        <button
          onClick={() => fetchHomeData()}
          className="px-4 py-2 bg-neutral-500 text-white rounded hover:bg-neutral-600 transition-colors"
        >
          {t("home.retry")}
        </button>
      </div>
    );
  }

  const lastPlayed = homeData.last_played;

  if (!lastPlayed || !selectedGame) {
    return (
      <div className="h-full relative flex flex-col items-center justify-center">
        <div className="absolute top-6 left-8">
          <h1 className="text-4xl font-bold text-brand-900 dark:text-white drop-shadow-lg">
            {t("home.title")}
          </h1>
          <p className="mt-2 text-brand-600 dark:text-white/80 drop-shadow">
            {t("home.welcome")}
          </p>
        </div>
        <div className="glass-card absolute top-6 right-6 flex items-center gap-2 bg-white/80 dark:bg-brand-800/80 backdrop-blur-sm px-4 py-3 rounded-xl shadow-lg">
          <span className="i-mdi-clock-outline text-xl text-neutral-500" />
          <div>
            <div className="text-xs text-brand-500 dark:text-brand-400">
              {t("home.todayPlayTime")}
            </div>
            <div className="text-lg font-bold text-neutral-600 dark:text-neutral-400">
              {formatDuration(homeData.today_play_time_sec, t)}
            </div>
          </div>
        </div>
        <span className="i-mdi-gamepad-variant-outline text-8xl text-brand-300 dark:text-brand-700 mb-4" />
        <h2 className="text-2xl font-bold text-brand-700 dark:text-brand-300 mb-2 drop-shadow-lg">
          {t("home.noPlayRecord")}
        </h2>
        <p className="text-brand-600 dark:text-white/70 mb-6 drop-shadow">
          {t("home.noPlayRecordHint")}
        </p>
        <button
          onClick={() => navigate({ to: "/library" })}
          className="glass-btn-neutral flex items-center gap-2 px-6 py-3 bg-neutral-600 hover:bg-neutral-700 text-white rounded-xl shadow-lg transition-all hover:scale-105 font-medium"
        >
          <span className="i-mdi-gamepad-variant text-xl" />
          {t("home.browseLibrary")}
        </button>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col">
      <div className="flex-1 relative overflow-hidden">
        {/* 仅在未启用自定义背景或未选择隐藏游戏封面时显示 */}
        {showGameBackground && (
          <div className="absolute inset-0">
            <img
              key={selectedGame.id}
              src={selectedGame.cover_url}
              alt=""
              referrerPolicy="no-referrer"
              className="w-full h-full object-cover"
              draggable="false"
              onDragStart={e => e.preventDefault()}
            />
            {/* 整体柔和毛玻璃遮罩，使用统一不透明度替代复杂的渐变叠加以保持暗黑模式下的干净通透 */}
            <div className="absolute inset-0 backdrop-blur-lg bg-white/50 dark:bg-black/60" />

            {/* 仅保留底部极其轻柔的渐变，确保文字高对比度可读性 */}
            <div className="absolute inset-0 bg-gradient-to-t from-black/10 via-transparent to-transparent dark:from-black/40 pointer-events-none" />
          </div>
        )}
        <div className="absolute top-6 left-8 z-10">
          <h1 className="text-4xl font-bold text-brand-900 dark:text-white drop-shadow-lg">
            {t("home.title")}
          </h1>
          <p className="mt-2 text-brand-600 dark:text-white/80 drop-shadow">
            {t("home.welcomeBack")}
          </p>
        </div>
        <div className="glass-card absolute top-6 right-6 flex items-center gap-2 bg-white/80 dark:bg-brand-800/80 backdrop-blur-sm px-4 py-3 rounded-xl shadow-lg z-10">
          <span className="i-mdi-clock-outline text-xl text-neutral-500" />
          <div>
            <div className="text-xs text-brand-500 dark:text-brand-400">
              {t("home.todayPlayTime")}
            </div>
            <div className="text-lg font-bold text-neutral-600 dark:text-neutral-400">
              {formatDuration(homeData.today_play_time_sec, t)}
            </div>
          </div>
        </div>
        <div
          className={`absolute ${contentBottomClass} left-8 max-w-lg z-10 transition-[bottom] duration-300`}
        >
          {showHeroCover && selectedGame.cover_url && (
            <div className="mb-4">
              <img
                src={selectedGame.cover_url}
                alt={selectedGame.name}
                referrerPolicy="no-referrer"
                className="max-h-72 max-w-full sm:max-w-sm md:max-w-md lg:max-w-lg rounded-2xl shadow-[0_8px_30px_rgb(0,0,0,0.3)] object-contain hover:scale-105 origin-left transition-transform duration-300 cursor-pointer ring-2 ring-white/20 dark:ring-white/10"
                onClick={() => openGameDetail(selectedGame.id)}
                draggable="false"
              />
            </div>
          )}
          <h1
            className="text-4xl font-bold text-brand-900 dark:text-white mb-2 cursor-pointer hover:text-neutral-600 dark:hover:text-neutral-300 transition-colors drop-shadow-lg"
            onClick={() => openGameDetail(selectedGame.id)}
          >
            {selectedGame.name}
          </h1>
          <p className="text-brand-700 dark:text-white/80 text-sm drop-shadow">
            {isSelectedGamePlaying
              ? t("home.playingNow")
              : t("home.lastPlayed", {
                  time: formatLocalDateTime(
                    selectedLastPlayedAt,
                    config?.time_zone,
                  ),
                })}
          </p>
          {selectedTotalPlayedDur > 0 && !isSelectedGamePlaying && (
            <p className="text-brand-600 dark:text-white/70 text-sm mt-1 drop-shadow">
              {t("home.totalPlayTime")}
              {formatDuration(selectedTotalPlayedDur, t)}
            </p>
          )}
        </div>
        {isSelectedGamePlaying ? (
          <div
            className={`absolute ${contentBottomClass} right-8 flex items-center gap-2 px-6 py-3 bg-success-600 text-white rounded-xl shadow-lg font-medium z-10 transition-[bottom] duration-300`}
          >
            <span className="i-mdi-gamepad-variant text-xl animate-pulse" />
            {t("home.gaming")}
          </div>
        ) : (
          <button
            type="button"
            onClick={handleContinuePlay}
            className={`absolute ${contentBottomClass} right-8 flex items-center gap-2 px-6 py-3 bg-neutral-600 hover:bg-neutral-700 text-white rounded-xl shadow-lg transition-all hover:scale-105 font-medium z-10`}
          >
            <span className="i-mdi-play text-xl" />
            {t("home.continueGame")}
          </button>
        )}

        {showCoverPicker && (
          <div
            className="absolute inset-x-0 bottom-0 z-20"
            onMouseEnter={() => setIsCarouselPaused(true)}
            onMouseLeave={() => setIsCarouselPaused(false)}
          >
            <div className="pointer-events-none absolute inset-x-0 bottom-0 h-52 bg-gradient-to-t from-black/40 via-black/15 to-transparent dark:from-black/60" />
            <div className="relative px-8 pb-14">
              <div className="scrollbar-hide flex gap-3 overflow-x-auto pb-1">
                {carouselGames.map((game) => {
                  const isActive = game.id === selectedGame.id;
                  return (
                    <button
                      type="button"
                      key={game.id}
                      onClick={() => setActiveGameId(game.id)}
                      className={`group relative h-32 w-24 shrink-0 overflow-hidden rounded-xl border bg-white/30 shadow-lg transition-all duration-300 dark:bg-black/20 ${
                        isActive
                          ? "border-primary-300 opacity-100 ring-2 ring-primary-400/70"
                          : "border-white/30 opacity-75 hover:-translate-y-1 hover:opacity-100 hover:border-white/60"
                      }`}
                      title={game.name}
                      aria-label={t("home.selectGame", {
                        name: game.name,
                      })}
                    >
                      {game.cover_url ? (
                        <img
                          src={game.cover_url}
                          alt={game.name}
                          referrerPolicy="no-referrer"
                          className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
                          draggable="false"
                          onDragStart={e => e.preventDefault()}
                        />
                      ) : (
                        <div className="flex h-full w-full items-center justify-center bg-brand-200 text-brand-400 dark:bg-brand-800/70 dark:text-white/50">
                          <span className="i-mdi-image-off text-3xl" />
                        </div>
                      )}
                      <div className="absolute inset-x-0 bottom-0 h-1/2 bg-gradient-to-t from-black/60 to-transparent" />
                      {isActive && (
                        <div className="absolute inset-x-2 bottom-2 h-1 rounded-full bg-primary-300 shadow-[0_0_16px_rgba(196,181,253,0.8)]" />
                      )}
                    </button>
                  );
                })}
              </div>
            </div>
          </div>
        )}

        {hasCoverPicker && (
          <BetterEdgeIconButton
            placement="bottom"
            icon={isPickerExpanded ? "i-mdi-chevron-down" : "i-mdi-chevron-up"}
            onClick={() => setIsPickerExpanded(value => !value)}
            title={
              isPickerExpanded
                ? t("home.collapseCoverPicker")
                : t("home.expandCoverPicker")
            }
            aria-label={
              isPickerExpanded
                ? t("home.collapseCoverPicker")
                : t("home.expandCoverPicker")
            }
            className="absolute bottom-0 left-1/2 z-30 -translate-x-1/2"
          />
        )}
      </div>
    </div>
  );
}
