import { createRoute, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { StartGameWithTracking } from "../../wailsjs/go/service/StartService";
import { useAppStore } from "../store";
import { formatDuration, formatLocalDateTime } from "../utils/time";
import { Route as rootRoute } from "./__root";

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: HomePage,
});

function HomePage() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const homeData = useAppStore(state => state.homeData);
  const fetchHomeData = useAppStore(state => state.fetchHomeData);
  const isLoading = useAppStore(state => state.isLoading);
  const config = useAppStore(state => state.config);
  const [isPlaying, setIsPlaying] = useState(false);

  useEffect(() => {
    fetchHomeData();
  }, [fetchHomeData]);

  // 同步后端的 is_playing 状态
  useEffect(() => {
    if (homeData?.last_played) {
      setIsPlaying(homeData.last_played.is_playing);
    }
  }, [homeData?.last_played?.is_playing]);

  const handleContinuePlay = async () => {
    if (!homeData?.last_played)
      return;
    try {
      const success = await StartGameWithTracking(homeData.last_played.game.id);
      if (success) {
        setIsPlaying(true);
        toast.success(t("home.toast.launching", { name: homeData.last_played.game.name }));
      }
    }
    catch (err) {
      console.error("Failed to launch game:", err);
      toast.error(t("home.toast.launchFailed"));
    }
  };

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

  if (!lastPlayed) {
    return (
      <div className="h-full relative flex flex-col items-center justify-center">
        <div className="absolute top-6 left-8">
          <h1 className="text-4xl font-bold text-brand-900 dark:text-white drop-shadow-lg">{t("home.title")}</h1>
          <p className="mt-2 text-brand-600 dark:text-white/80 drop-shadow">{t("home.welcome")}</p>
        </div>
        <div className="glass-card absolute top-6 right-6 flex items-center gap-2 bg-white/80 dark:bg-brand-800/80 backdrop-blur-sm px-4 py-3 rounded-xl shadow-lg">
          <span className="i-mdi-clock-outline text-xl text-neutral-500" />
          <div>
            <div className="text-xs text-brand-500 dark:text-brand-400">{t("home.todayPlayTime")}</div>
            <div className="text-lg font-bold text-neutral-600 dark:text-neutral-400">
              {formatDuration(homeData.today_play_time_sec, t)}
            </div>
          </div>
        </div>
        <span className="i-mdi-gamepad-variant-outline text-8xl text-brand-300 dark:text-brand-700 mb-4" />
        <h2 className="text-2xl font-bold text-brand-700 dark:text-brand-300 mb-2 drop-shadow-lg">{t("home.noPlayRecord")}</h2>
        <p className="text-brand-600 dark:text-white/70 mb-6 drop-shadow">{t("home.noPlayRecordHint")}</p>
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
        {(!config?.background_enabled || !config?.background_hide_game_cover) && (
          <div className="absolute inset-0">
            <img
              src={lastPlayed.game.cover_url}
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
          <h1 className="text-4xl font-bold text-brand-900 dark:text-white drop-shadow-lg">{t("home.title")}</h1>
          <p className="mt-2 text-brand-600 dark:text-white/80 drop-shadow">{t("home.welcomeBack")}</p>
        </div>
        <div className="glass-card absolute top-6 right-6 flex items-center gap-2 bg-white/80 dark:bg-brand-800/80 backdrop-blur-sm px-4 py-3 rounded-xl shadow-lg z-10">
          <span className="i-mdi-clock-outline text-xl text-neutral-500" />
          <div>
            <div className="text-xs text-brand-500 dark:text-brand-400">{t("home.todayPlayTime")}</div>
            <div className="text-lg font-bold text-neutral-600 dark:text-neutral-400">
              {formatDuration(homeData.today_play_time_sec, t)}
            </div>
          </div>
        </div>
        <div className="absolute bottom-8 left-8 max-w-lg z-10">
          <div className="mb-4">
            <img
              src={lastPlayed.game.cover_url}
              alt={lastPlayed.game.name}
              referrerPolicy="no-referrer"
              className="max-h-72 max-w-full sm:max-w-sm md:max-w-md lg:max-w-lg rounded-2xl shadow-[0_8px_30px_rgb(0,0,0,0.3)] object-contain hover:scale-105 origin-left transition-transform duration-300 cursor-pointer ring-2 ring-white/20 dark:ring-white/10"
              onClick={() => navigate({ to: "/game/$gameId", params: { gameId: lastPlayed.game.id } })}
              draggable="false"
            />
          </div>
          <h1
            className="text-4xl font-bold text-brand-900 dark:text-white mb-2 cursor-pointer hover:text-neutral-600 dark:hover:text-neutral-300 transition-colors drop-shadow-lg"
            onClick={() => navigate({ to: "/game/$gameId", params: { gameId: lastPlayed.game.id } })}
          >
            {lastPlayed.game.name}
          </h1>
          <p className="text-brand-700 dark:text-white/80 text-sm drop-shadow">
            {isPlaying ? t("home.playingNow") : t("home.lastPlayed", { time: formatLocalDateTime(lastPlayed.last_played_at, config?.time_zone) })}
          </p>
          {lastPlayed.total_played_dur > 0 && !isPlaying && (
            <p className="text-brand-600 dark:text-white/70 text-sm mt-1 drop-shadow">
              {t("home.totalPlayTime")}
              {formatDuration(lastPlayed.total_played_dur, t)}
            </p>
          )}
        </div>
        {isPlaying
          ? (
              <div className="absolute bottom-8 right-8 flex items-center gap-2 px-6 py-3 bg-success-600 text-white rounded-xl shadow-lg font-medium z-10">
                <span className="i-mdi-gamepad-variant text-xl animate-pulse" />
                {t("home.gaming")}
              </div>
            )
          : (
              <button
                onClick={handleContinuePlay}
                className="absolute bottom-8 right-8 flex items-center gap-2 px-6 py-3 bg-neutral-600 hover:bg-neutral-700 text-white rounded-xl shadow-lg transition-all hover:scale-105 font-medium z-10"
              >
                <span className="i-mdi-play text-xl" />
                {t("home.continueGame")}
              </button>
            )}
      </div>
    </div>
  );
}
