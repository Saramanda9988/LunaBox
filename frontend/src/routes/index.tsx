import type { models } from "../../src/bindings/models";
import { createRoute, useNavigate } from "@tanstack/react-router";
import { useCallback, useEffect, useMemo, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { GetGlobalPeriodStats } from "../../bindings/lunabox/internal/service/statsservice";
import { enums, vo } from "../../src/bindings/models";
import { HomeGameRailPanel } from "../components/panel/HomeGameRailPanel";
import { BetterButton } from "../components/ui/better/BetterButton";
import { GameCoverImage } from "../components/ui/GameCoverImage";
import { ProxyImage } from "../components/ui/ProxyImage";
import { useCrossfadeBackground } from "../hooks/useCrossfadeBackground";
import { useImageAccentRgb } from "../hooks/useImageAccentRgb";
import { useSnapshotVisibilityTransition } from "../hooks/useSnapshotVisibilityTransition";
import { useVerticalSwipe } from "../hooks/useVerticalSwipe";
import { isGameRuntimeVisible, useAppStore } from "../store";
import { proxiedImageSrc } from "../utils/imageProxy";
import { formatDuration, formatLocalDateTime } from "../utils/time";
import { Route as rootRoute } from "./__root";

const DEFAULT_HOME_GAME_CAROUSEL_INTERVAL_SEC = 6;
const MIN_HOME_GAME_CAROUSEL_INTERVAL_SEC = 4;
const BACKGROUND_CROSSFADE_MS = 1200;
const HERO_FADE_OUT_MS = 280;
const HERO_FADE_IN_DELAY_MS = 90;

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: HomePage,
});

interface HeroSnapshot {
  game: models.Game;
  isPlaying: boolean;
  lastPlayedAt: Parameters<typeof formatLocalDateTime>[0];
  totalPlayedDur: number;
}

function HomePage() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const homeData = useAppStore(state => state.homeData);
  const fetchHomeData = useAppStore(state => state.fetchHomeData);
  const isLoading = useAppStore(state => state.isLoading);
  const config = useAppStore(state => state.config);
  const startGame = useAppStore(state => state.startGame);
  const hasVisibleGameRuntime = useAppStore(state =>
    Object.values(state.gameRuntimes).some(isGameRuntimeVisible),
  );
  const [activeGameId, setActiveGameId] = useState<string | null>(null);
  const [isPickerExpanded, setIsPickerExpanded] = useState(false);
  const [isCarouselPaused, setIsCarouselPaused] = useState(false);
  const [libraryPreviewStats, setLibraryPreviewStats]
    = useState<vo.PeriodStats | null>(null);

  const loadLibraryPreviewStats = useCallback(async () => {
    try {
      const data = await GetGlobalPeriodStats(
        new vo.PeriodStatsRequest({
          dimension: enums.Period.All,
          start_date: "",
          end_date: "",
        }),
      );
      setLibraryPreviewStats(data);
    }
    catch (error) {
      console.error("Failed to fetch library preview stats:", error);
    }
  }, []);

  useEffect(() => {
    void fetchHomeData();
    void loadLibraryPreviewStats();
  }, [fetchHomeData, loadLibraryPreviewStats]);

  const carouselItems = useMemo(() => {
    const items = [...(homeData?.recent_played || [])];
    const lastPlayed = homeData?.last_played;

    if (
      lastPlayed?.game.id
      && !items.some(item => item.game.id === lastPlayed.game.id)
    ) {
      items.unshift(lastPlayed);
    }

    return items;
  }, [homeData?.last_played, homeData?.recent_played]);

  const carouselGames = useMemo(
    () =>
      carouselItems.map(
        item =>
          ({
            ...item.game,
            last_played_at: item.last_played_at,
          }) as models.Game,
      ),
    [carouselItems],
  );
  const hasCoverPicker = carouselGames.length > 1;
  const homeSwipeHandlers = useVerticalSwipe({
    enabled: hasCoverPicker,
    onSwipeDown: () => setIsPickerExpanded(false),
    onSwipeUp: () => setIsPickerExpanded(true),
  });
  const showCoverPicker = hasCoverPicker && isPickerExpanded;
  const isHomeGameCarouselEnabled
    = config?.home_game_carousel_enabled !== false;
  const homeGameCarouselIntervalMs
    = Math.max(
      MIN_HOME_GAME_CAROUSEL_INTERVAL_SEC,
      Number(
        config?.home_game_carousel_interval_sec
        || DEFAULT_HOME_GAME_CAROUSEL_INTERVAL_SEC,
      ),
    ) * 1000;

  useEffect(() => {
    setActiveGameId(current =>
      current && carouselItems.some(item => item.game.id === current)
        ? current
        : (carouselItems[0]?.game.id ?? null),
    );
  }, [carouselItems]);

  useEffect(() => {
    if (
      carouselGames.length <= 1
      || !isHomeGameCarouselEnabled
      || isPickerExpanded
      || isCarouselPaused
      || hasVisibleGameRuntime
    ) {
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
    }, homeGameCarouselIntervalMs);

    return () => window.clearInterval(timer);
  }, [
    carouselGames,
    hasVisibleGameRuntime,
    homeGameCarouselIntervalMs,
    isCarouselPaused,
    isHomeGameCarouselEnabled,
    isPickerExpanded,
  ]);

  const selectedCarouselItem = useMemo(() => {
    if (carouselItems.length === 0) {
      return null;
    }

    return (
      carouselItems.find(item => item.game.id === activeGameId)
      ?? carouselItems[0]
    );
  }, [activeGameId, carouselItems]);

  const selectedGame = useMemo(() => {
    if (!selectedCarouselItem) {
      return null;
    }

    return {
      ...selectedCarouselItem.game,
      last_played_at: selectedCarouselItem.last_played_at,
    } as models.Game;
  }, [selectedCarouselItem]);

  const selectedGameHasRuntime = useAppStore((state) => {
    if (!selectedGame?.id) {
      return false;
    }

    return Boolean(state.gameRuntimes[selectedGame.id]);
  });

  const selectedGameCoverSrc
    = selectedGame?.cover_url || selectedGame?.cover_source_url || "";
  const selectedGameAccentSrc = proxiedImageSrc(selectedGameCoverSrc);
  const heroAccentRgb = useImageAccentRgb(selectedGameAccentSrc);
  const { isBackgroundCrossfading, previousBackgroundUrl }
    = useCrossfadeBackground(selectedGameCoverSrc, {
      durationMs: BACKGROUND_CROSSFADE_MS,
    });

  const selectedLastPlayedAt = useMemo(() => {
    return selectedCarouselItem?.last_played_at ?? null;
  }, [selectedCarouselItem]);

  const selectedTotalPlayedDur = selectedCarouselItem
    ? selectedCarouselItem.total_played_dur
    : 0;
  const isSelectedGamePlaying = Boolean(
    selectedGame?.id
    && (selectedCarouselItem?.is_playing || selectedGameHasRuntime),
  );
  const currentHeroSnapshot = useMemo<HeroSnapshot | null>(() => {
    if (!selectedGame) {
      return null;
    }

    return {
      game: selectedGame,
      isPlaying: isSelectedGamePlaying,
      lastPlayedAt: selectedLastPlayedAt,
      totalPlayedDur: Number(selectedTotalPlayedDur || 0),
    };
  }, [
    isSelectedGamePlaying,
    selectedGame,
    selectedLastPlayedAt,
    selectedTotalPlayedDur,
  ]);
  const currentHeroId = currentHeroSnapshot?.game.id ?? null;
  const { displayedSnapshot: displayedHeroSnapshot, isVisible: isHeroVisible }
    = useSnapshotVisibilityTransition<HeroSnapshot>(
      currentHeroSnapshot,
      currentHeroId,
      {
        fadeInDelayMs: HERO_FADE_IN_DELAY_MS,
        fadeOutMs: HERO_FADE_OUT_MS,
      },
    );
  const showGameBackground
    = !config?.background_enabled || !config?.background_hide_game_cover;
  const showHeroCover
    = !config?.background_enabled || !config?.background_hide_game_hero_cover;
  const contentBottomClass = showCoverPicker ? "bottom-[18rem]" : "bottom-8";
  const heroCoverMotionClass = isHeroVisible
    ? "duration-[760ms] opacity-100 translate-y-0 scale-100 delay-75"
    : "duration-[220ms] opacity-0 translate-y-5 scale-[0.99] delay-0";
  const heroMetaMotionClass = isHeroVisible
    ? "duration-[680ms] opacity-100 translate-y-0 blur-0 delay-150"
    : "duration-[180ms] opacity-0 translate-y-4 blur-[2px] delay-0";
  const heroAccentStyle = useMemo(
    () => ({ backgroundColor: `rgb(${heroAccentRgb})` }),
    [heroAccentRgb],
  );

  useEffect(() => {
    if (!showCoverPicker) {
      setIsCarouselPaused(false);
    }
  }, [showCoverPicker]);

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
      const success = await startGame(selectedGame);
      if (success) {
        setActiveGameId(selectedGame.id);
      }
    }
    catch (err) {
      console.error("Failed to launch game:", err);
      toast.error(t("home.toast.launchFailed"));
    }
  }, [selectedGame, startGame, t]);

  const renderHeroContent = useCallback(
    (
      snapshot: HeroSnapshot,
      isInteractive: boolean,
      coverMotionClass = "",
      metaMotionClass = "",
    ) => {
      return (
        <>
          {showHeroCover
            && (snapshot.game.cover_url || snapshot.game.cover_source_url) && (
            <div
              className={`w-fit max-w-full transition-all duration-300 ease-out ${
                showCoverPicker
                  ? "mb-0 max-h-0 -translate-y-2 overflow-hidden opacity-0"
                  : "mb-4 max-h-72 translate-y-0 opacity-100"
              }`}
            >
              <GameCoverImage
                src={
                  snapshot.game.cover_url || snapshot.game.cover_source_url
                }
                fallbackSrc={snapshot.game.cover_source_url}
                alt={snapshot.game.name}
                isNSFW={snapshot.game.is_nsfw}
                revealNSFWOnHover
                className={`w-fit max-w-full rounded-2xl bg-brand-200 shadow-[0_8px_30px_rgb(0,0,0,0.3)] ring-2 ring-white/20 transition-[opacity,transform] ease-out will-change-transform dark:bg-brand-800 dark:ring-white/10 ${coverMotionClass}`}
                imageClassName={`block max-h-72 max-w-full object-contain transition-[filter] duration-300 sm:max-w-sm md:max-w-md lg:max-w-lg ${
                  isInteractive ? "cursor-pointer" : ""
                }`}
                onClick={
                  isInteractive
                    ? () => openGameDetail(snapshot.game.id)
                    : undefined
                }
              />
            </div>
          )}
          <div
            className={`transition-[opacity,transform,filter] ease-out will-change-transform ${metaMotionClass}`}
          >
            <h1
              className={`text-4xl font-bold text-brand-900 dark:text-white mb-2 drop-shadow-lg ${
                isInteractive
                  ? "cursor-pointer hover:text-neutral-600 dark:hover:text-neutral-300 transition-colors"
                  : ""
              }`}
              onClick={
                isInteractive
                  ? () => openGameDetail(snapshot.game.id)
                  : undefined
              }
            >
              {snapshot.game.name}
            </h1>
            {snapshot.isPlaying ? (
              <div className="flex min-h-[2.75rem] items-center justify-start">
                <p className="text-base font-medium text-brand-700 dark:text-white/85 drop-shadow">
                  {t("home.playingNow")}
                </p>
              </div>
            ) : (
              <div className="grid min-h-[2.75rem] content-start gap-1">
                <p className="min-h-5 text-brand-700 dark:text-white/80 text-sm drop-shadow">
                  {t("home.lastPlayed", {
                    time: formatLocalDateTime(
                      snapshot.lastPlayedAt,
                      config?.time_zone,
                    ),
                  })}
                </p>
                <p
                  className={`min-h-5 text-brand-600 dark:text-white/70 text-sm drop-shadow ${
                    snapshot.totalPlayedDur > 0 ? "opacity-100" : "opacity-0"
                  }`}
                  aria-hidden={snapshot.totalPlayedDur <= 0}
                >
                  {snapshot.totalPlayedDur > 0 ? (
                    <>
                      {t("home.totalPlayTime")}
                      {formatDuration(snapshot.totalPlayedDur, t)}
                    </>
                  ) : (
                    "\u00A0"
                  )}
                </p>
              </div>
            )}
          </div>
        </>
      );
    },
    [config?.time_zone, openGameDetail, showCoverPicker, showHeroCover, t],
  );

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
      <div
        className="flex-1 relative overflow-hidden [touch-action:pan-x]"
        {...homeSwipeHandlers}
      >
        {/* 仅在未启用自定义背景或未选择隐藏游戏封面时显示 */}
        {showGameBackground && (
          <div className="absolute inset-0">
            {(selectedGame.cover_url || selectedGame.cover_source_url) && (
              <ProxyImage
                src={selectedGameCoverSrc}
                fallbackSrc={selectedGame.cover_source_url}
                alt=""
                isNSFW={selectedGame.is_nsfw}
                className="absolute inset-0 h-full w-full object-cover"
              />
            )}
            {previousBackgroundUrl
              && previousBackgroundUrl !== selectedGameCoverSrc && (
              <ProxyImage
                src={previousBackgroundUrl}
                alt=""
                isNSFW={selectedGame.is_nsfw}
                className={`absolute inset-0 h-full w-full object-cover transition-opacity duration-[1200ms] ease-in-out ${
                  isBackgroundCrossfading ? "opacity-100" : "opacity-0"
                }`}
              />
            )}
            {/* 整体柔和毛玻璃遮罩，使用统一不透明度替代复杂的渐变叠加以保持暗黑模式下的干净通透 */}
            <div className="absolute inset-0 backdrop-blur-lg bg-white/50 dark:bg-black/60" />
            <div
              className="absolute inset-0 opacity-[0.08] transition-colors duration-[1400ms] ease-in-out dark:opacity-[0.12]"
              style={heroAccentStyle}
            />

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
        <div
          className={`absolute ${contentBottomClass} left-8 max-w-lg z-10 transition-[bottom] duration-300 ease-out`}
        >
          {displayedHeroSnapshot
            && renderHeroContent(
              displayedHeroSnapshot,
              true,
              heroCoverMotionClass,
              heroMetaMotionClass,
            )}
        </div>
        <div
          className={`absolute ${contentBottomClass} right-8 z-10 transition-[bottom] duration-300 ease-out`}
        >
          {isSelectedGamePlaying ? (
            <div className="inline-flex shrink-0 items-center justify-center gap-2 rounded-xl bg-success-600 px-6 py-3 text-sm font-medium text-white shadow-lg">
              <span className="i-mdi-gamepad-variant text-xl animate-pulse" />
              <span>{t("home.gaming")}</span>
            </div>
          ) : (
            <BetterButton
              variant="primary"
              size="lg"
              icon="i-mdi-play"
              onClick={handleContinuePlay}
              className="!h-auto gap-2 rounded-xl px-6 py-3 shadow-lg"
            >
              {t("home.continueGame")}
            </BetterButton>
          )}
        </div>

        {hasCoverPicker && (
          <HomeGameRailPanel
            games={carouselGames}
            isExpanded={isPickerExpanded}
            libraryStats={libraryPreviewStats}
            onExpandedChange={setIsPickerExpanded}
            onPauseChange={setIsCarouselPaused}
            onSelectGame={setActiveGameId}
            selectedGameId={selectedGame.id}
          />
        )}
      </div>
    </div>
  );
}
