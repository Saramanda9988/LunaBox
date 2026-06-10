import type { models, vo } from "../../wailsjs/go/models";
import { createRoute, useNavigate } from "@tanstack/react-router";
import { FastAverageColor } from "fast-average-color";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
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
const CAROUSEL_INTERVAL_MS = 6500;
const BACKGROUND_CROSSFADE_MS = 1200;
const HERO_FADE_OUT_MS = 280;
const HERO_FADE_IN_DELAY_MS = 90;
const DEFAULT_HERO_ACCENT_RGB = "71, 85, 105";
const DEFAULT_HERO_ACCENT_VALUE: [number, number, number, number] = [
  71,
  85,
  105,
  255,
];

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: HomePage,
});

function hasRecentPlayTime(game: models.Game) {
  return Boolean(game.last_played_at);
}

interface HeroSnapshot {
  game: models.Game;
  isPlaying: boolean;
  lastPlayedAt: Parameters<typeof formatLocalDateTime>[0];
  totalPlayedDur: number;
}

function clampColorChannel(value: number) {
  return Math.max(32, Math.min(224, Math.round(value)));
}

function formatHeroAccentRgb(value: number[]) {
  const [rawRed, rawGreen, rawBlue] = value;
  const neutral = [71, 85, 105];
  const red = clampColorChannel(rawRed * 0.72 + neutral[0] * 0.28);
  const green = clampColorChannel(rawGreen * 0.72 + neutral[1] * 0.28);
  const blue = clampColorChannel(rawBlue * 0.72 + neutral[2] * 0.28);

  return `${red}, ${green}, ${blue}`;
}

function getCoverAccentRgb(coverUrl: string) {
  return new Promise<string>((resolve, reject) => {
    const fac = new FastAverageColor();
    const image = new Image();

    image.crossOrigin = "anonymous";
    image.referrerPolicy = "no-referrer";

    image.onload = () => {
      try {
        const color = fac.getColor(image, {
          algorithm: "sqrt",
          defaultColor: DEFAULT_HERO_ACCENT_VALUE,
          mode: "speed",
          silent: true,
        });
        resolve(formatHeroAccentRgb(color.value));
      }
      catch (error) {
        reject(error);
      }
      finally {
        fac.destroy();
      }
    };

    image.onerror = (error) => {
      fac.destroy();
      reject(error);
    };

    image.src = coverUrl;
  });
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
  const [isHeroVisible, setIsHeroVisible] = useState(false);
  const [displayedHeroSnapshot, setDisplayedHeroSnapshot]
    = useState<HeroSnapshot | null>(null);
  const [previousBackgroundUrl, setPreviousBackgroundUrl] = useState<
    string | null
  >(null);
  const [isBackgroundCrossfading, setIsBackgroundCrossfading] = useState(false);
  const [heroAccentRgb, setHeroAccentRgb] = useState(DEFAULT_HERO_ACCENT_RGB);
  const backgroundUrlRef = useRef<string | null>(null);
  const backgroundFrameRef = useRef<number | null>(null);
  const backgroundTimerRef = useRef<number | null>(null);
  const displayedHeroSnapshotRef = useRef<HeroSnapshot | null>(null);
  const pendingHeroSnapshotRef = useRef<HeroSnapshot | null>(null);
  const heroFrameRef = useRef<number | null>(null);
  const heroTimerRef = useRef<number | null>(null);
  const colorCacheRef = useRef(new Map<string, string>());

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

  useLayoutEffect(() => {
    const nextBackgroundUrl = selectedGame?.cover_url || null;
    const currentBackgroundUrl = backgroundUrlRef.current;

    if (backgroundFrameRef.current !== null) {
      window.cancelAnimationFrame(backgroundFrameRef.current);
      backgroundFrameRef.current = null;
    }
    if (backgroundTimerRef.current !== null) {
      window.clearTimeout(backgroundTimerRef.current);
      backgroundTimerRef.current = null;
    }

    if (
      currentBackgroundUrl
      && nextBackgroundUrl
      && currentBackgroundUrl !== nextBackgroundUrl
    ) {
      setPreviousBackgroundUrl(currentBackgroundUrl);
      setIsBackgroundCrossfading(true);

      backgroundFrameRef.current = window.requestAnimationFrame(() => {
        setIsBackgroundCrossfading(false);
        backgroundFrameRef.current = null;
      });

      backgroundTimerRef.current = window.setTimeout(() => {
        setPreviousBackgroundUrl(null);
        backgroundTimerRef.current = null;
      }, BACKGROUND_CROSSFADE_MS);
    }
    else if (!nextBackgroundUrl) {
      setPreviousBackgroundUrl(null);
      setIsBackgroundCrossfading(false);
    }

    backgroundUrlRef.current = nextBackgroundUrl;
  }, [selectedGame?.cover_url]);

  useEffect(() => {
    return () => {
      if (backgroundFrameRef.current !== null) {
        window.cancelAnimationFrame(backgroundFrameRef.current);
      }
      if (backgroundTimerRef.current !== null) {
        window.clearTimeout(backgroundTimerRef.current);
      }
    };
  }, []);

  useEffect(() => {
    const coverUrl = selectedGame?.cover_url;

    if (!coverUrl) {
      setHeroAccentRgb(DEFAULT_HERO_ACCENT_RGB);
      return;
    }

    const cachedColor = colorCacheRef.current.get(coverUrl);
    if (cachedColor) {
      setHeroAccentRgb(cachedColor);
      return;
    }

    let isCancelled = false;

    void getCoverAccentRgb(coverUrl)
      .then((rgb) => {
        colorCacheRef.current.set(coverUrl, rgb);
        if (!isCancelled) {
          setHeroAccentRgb(rgb);
        }
      })
      .catch(() => {
        if (!isCancelled) {
          setHeroAccentRgb(DEFAULT_HERO_ACCENT_RGB);
        }
      });

    return () => {
      isCancelled = true;
    };
  }, [selectedGame?.cover_url]);

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
  const showGameBackground
    = !config?.background_enabled || !config?.background_hide_game_cover;
  const showHeroCover
    = !config?.background_enabled || !config?.background_hide_game_hero_cover;
  const hasCoverPicker = carouselGames.length > 1;
  const showCoverPicker = hasCoverPicker && isPickerExpanded;
  const contentBottomClass = showCoverPicker ? "bottom-72" : "bottom-8";
  const heroCoverMotionClass = isHeroVisible
    ? "duration-[760ms] opacity-100 translate-y-0 scale-100 blur-0 delay-75"
    : "duration-[220ms] opacity-0 translate-y-5 scale-[0.99] blur-[2px] delay-0";
  const heroMetaMotionClass = isHeroVisible
    ? "duration-[680ms] opacity-100 translate-y-0 blur-0 delay-150"
    : "duration-[180ms] opacity-0 translate-y-4 blur-[2px] delay-0";
  const heroAccentStyle = useMemo(
    () => ({ backgroundColor: `rgb(${heroAccentRgb})` }),
    [heroAccentRgb],
  );

  useLayoutEffect(() => {
    pendingHeroSnapshotRef.current = currentHeroSnapshot;
  }, [currentHeroSnapshot]);

  useLayoutEffect(() => {
    const nextHeroSnapshot = pendingHeroSnapshotRef.current;
    const visibleSnapshot = displayedHeroSnapshotRef.current;
    const hasNewHero = Boolean(nextHeroSnapshot?.game.id);
    const hasVisibleHero = Boolean(visibleSnapshot?.game.id);
    const hasChangedHero
      = hasVisibleHero
        && Boolean(nextHeroSnapshot?.game.id)
        && visibleSnapshot?.game.id !== nextHeroSnapshot?.game.id;

    if (heroFrameRef.current !== null) {
      window.cancelAnimationFrame(heroFrameRef.current);
      heroFrameRef.current = null;
    }
    if (heroTimerRef.current !== null) {
      window.clearTimeout(heroTimerRef.current);
      heroTimerRef.current = null;
    }

    if (!hasNewHero) {
      setIsHeroVisible(false);
      setDisplayedHeroSnapshot(null);
      displayedHeroSnapshotRef.current = null;
      return;
    }

    if (!hasVisibleHero || !hasChangedHero) {
      setDisplayedHeroSnapshot(nextHeroSnapshot);
      displayedHeroSnapshotRef.current = nextHeroSnapshot;
      setIsHeroVisible(false);

      heroFrameRef.current = window.requestAnimationFrame(() => {
        setIsHeroVisible(true);
        heroFrameRef.current = null;
      });
      return;
    }

    setIsHeroVisible(false);

    heroTimerRef.current = window.setTimeout(() => {
      setDisplayedHeroSnapshot(nextHeroSnapshot);
      displayedHeroSnapshotRef.current = nextHeroSnapshot;

      heroTimerRef.current = window.setTimeout(() => {
        heroFrameRef.current = window.requestAnimationFrame(() => {
          setIsHeroVisible(true);
          heroFrameRef.current = null;
        });
        heroTimerRef.current = null;
      }, HERO_FADE_IN_DELAY_MS);
    }, HERO_FADE_OUT_MS);
  }, [currentHeroId]);

  useLayoutEffect(() => {
    if (
      currentHeroSnapshot
      && displayedHeroSnapshotRef.current?.game.id === currentHeroSnapshot.game.id
    ) {
      displayedHeroSnapshotRef.current = currentHeroSnapshot;
      setDisplayedHeroSnapshot(currentHeroSnapshot);
    }
  }, [currentHeroSnapshot]);

  useEffect(() => {
    return () => {
      if (heroFrameRef.current !== null) {
        window.cancelAnimationFrame(heroFrameRef.current);
      }
      if (heroTimerRef.current !== null) {
        window.clearTimeout(heroTimerRef.current);
        heroTimerRef.current = null;
      }
    };
  }, []);

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

  const renderHeroContent = useCallback(
    (
      snapshot: HeroSnapshot,
      isInteractive: boolean,
      coverMotionClass = "",
      metaMotionClass = "",
    ) => {
      return (
        <>
          {showHeroCover && snapshot.game.cover_url && (
            <div
              className={`w-fit max-w-full transition-all duration-300 ease-out ${
                showCoverPicker
                  ? "mb-0 max-h-0 -translate-y-2 overflow-hidden opacity-0"
                  : "mb-4 max-h-72 translate-y-0 overflow-visible opacity-100"
              }`}
            >
              <img
                src={snapshot.game.cover_url}
                alt={snapshot.game.name}
                referrerPolicy="no-referrer"
                className={`block max-h-72 max-w-full sm:max-w-sm md:max-w-md lg:max-w-lg rounded-2xl shadow-[0_8px_30px_rgb(0,0,0,0.3)] object-contain ring-2 ring-white/20 dark:ring-white/10 transition-[opacity,transform,filter] ease-out will-change-transform ${
                  isInteractive ? "cursor-pointer" : ""
                } ${coverMotionClass}`}
                onClick={
                  isInteractive
                    ? () => openGameDetail(snapshot.game.id)
                    : undefined
                }
                draggable="false"
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
            <p className="text-brand-700 dark:text-white/80 text-sm drop-shadow">
              {snapshot.isPlaying
                ? t("home.playingNow")
                : t("home.lastPlayed", {
                    time: formatLocalDateTime(
                      snapshot.lastPlayedAt,
                      config?.time_zone,
                    ),
                  })}
            </p>
            {snapshot.totalPlayedDur > 0 && !snapshot.isPlaying && (
              <p className="text-brand-600 dark:text-white/70 text-sm mt-1 drop-shadow">
                {t("home.totalPlayTime")}
                {formatDuration(snapshot.totalPlayedDur, t)}
              </p>
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
            {selectedGame.cover_url && (
              <img
                src={selectedGame.cover_url}
                alt=""
                referrerPolicy="no-referrer"
                className="absolute inset-0 h-full w-full object-cover"
                draggable="false"
                onDragStart={e => e.preventDefault()}
              />
            )}
            {previousBackgroundUrl
              && previousBackgroundUrl !== selectedGame.cover_url && (
              <img
                src={previousBackgroundUrl}
                alt=""
                referrerPolicy="no-referrer"
                className={`absolute inset-0 h-full w-full object-cover transition-opacity duration-[1200ms] ease-in-out ${
                  isBackgroundCrossfading ? "opacity-100" : "opacity-0"
                }`}
                draggable="false"
                onDragStart={e => e.preventDefault()}
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
        {isSelectedGamePlaying ? (
          <div
            className={`absolute ${contentBottomClass} right-8 flex items-center gap-2 px-6 py-3 bg-success-600 text-white rounded-xl shadow-lg font-medium z-10 transition-[bottom] duration-300 ease-out`}
          >
            <span className="i-mdi-gamepad-variant text-xl animate-pulse" />
            {t("home.gaming")}
          </div>
        ) : (
          <button
            type="button"
            onClick={handleContinuePlay}
            className={`absolute ${contentBottomClass} right-8 flex items-center gap-2 px-6 py-3 bg-neutral-600 hover:bg-neutral-700 text-white rounded-xl shadow-lg transition-all duration-300 hover:scale-105 font-medium z-10`}
          >
            <span className="i-mdi-play text-xl" />
            {t("home.continueGame")}
          </button>
        )}

        {hasCoverPicker && (
          <div
            className={`absolute inset-x-0 bottom-0 z-20 transition-all duration-300 ease-out ${
              showCoverPicker
                ? "translate-y-0 opacity-100"
                : "pointer-events-none translate-y-full opacity-0"
            }`}
            aria-hidden={!showCoverPicker}
            onMouseEnter={() => showCoverPicker && setIsCarouselPaused(true)}
            onMouseLeave={() => showCoverPicker && setIsCarouselPaused(false)}
          >
            <div className="pointer-events-none absolute inset-x-0 bottom-0 h-72 bg-gradient-to-t from-black/40 via-black/15 to-transparent dark:from-black/60" />
            <div className="relative px-8 pb-12">
              <div className="scrollbar-hide -my-3 flex gap-1 overflow-x-auto px-1 py-3">
                {carouselGames.map((game) => {
                  const isActive = game.id === selectedGame.id;
                  return (
                    <button
                      type="button"
                      key={game.id}
                      onClick={() => setActiveGameId(game.id)}
                      tabIndex={showCoverPicker ? 0 : -1}
                      className={`group relative h-48 w-36 shrink-0 rounded-xl border p-[2px] shadow-lg transition-all duration-300 hover:scale-[1.03] hover:shadow-xl ${
                        isActive
                          ? "border-transparent opacity-100 shadow-[0_0_24px_rgba(244,63,94,0.38)]"
                          : "border-white/30 bg-white/30 opacity-75 hover:-translate-y-1 hover:opacity-100 hover:border-white/60 dark:bg-black/20"
                      }`}
                      title={game.name}
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
