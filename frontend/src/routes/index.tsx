import { createRoute, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import toast from "react-hot-toast";
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
  const { homeData, fetchHomeData, isLoading, config } = useAppStore();
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
        toast.success(`正在启动 ${homeData.last_played.game.name}`);
      }
    }
    catch (err) {
      console.error("Failed to launch game:", err);
      toast.error("启动游戏失败");
    }
  };

  if (isLoading) {
    return null;
  }

  if (!homeData) {
    return (
      <div className="flex h-full flex-col items-center justify-center space-y-4">
        <p className="text-brand-500">暂无数据</p>
        <button
          onClick={() => fetchHomeData()}
          className="px-4 py-2 bg-neutral-500 text-white rounded hover:bg-neutral-600 transition-colors"
        >
          重试
        </button>
      </div>
    );
  }

  const lastPlayed = homeData.last_played;

  if (!lastPlayed) {
    return (
      <div className="h-full relative flex flex-col items-center justify-center">
        <div className="absolute top-6 left-8">
          <h1 className="text-4xl font-bold text-brand-900 dark:text-white drop-shadow-lg">首页</h1>
          <p className="mt-2 text-brand-600 dark:text-white/80 drop-shadow">欢迎回来!</p>
        </div>
        <div className="glass-card absolute top-6 right-6 flex items-center gap-2 bg-white/80 dark:bg-brand-800/80 backdrop-blur-sm px-4 py-3 rounded-xl shadow-lg">
          <span className="i-mdi-clock-outline text-xl text-neutral-500" />
          <div>
            <div className="text-xs text-brand-500 dark:text-brand-400">今日游玩时间</div>
            <div className="text-lg font-bold text-neutral-600 dark:text-neutral-400">
              {formatDuration(homeData.today_play_time_sec)}
            </div>
          </div>
        </div>
        <span className="i-mdi-gamepad-variant-outline text-8xl text-brand-300 dark:text-brand-700 mb-4" />
        <h2 className="text-2xl font-bold text-brand-700 dark:text-brand-300 mb-2 drop-shadow-lg">还没有游玩记录</h2>
        <p className="text-brand-600 dark:text-white/70 mb-6 drop-shadow">去游戏库选择一款游戏开始吧</p>
        <button
          onClick={() => navigate({ to: "/library" })}
          className="glass-btn-neutral flex items-center gap-2 px-6 py-3 bg-neutral-600 hover:bg-neutral-700 text-white rounded-xl shadow-lg transition-all hover:scale-105 font-medium"
        >
          <span className="i-mdi-gamepad-variant text-xl" />
          浏览游戏库
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
            />
            {/* 整体柔和遮罩 - 浅色模式用浅色，深色模式用深色 */}
            <div className="absolute inset-0 bg-brand-100/30 dark:bg-black/30" />
            {/* 从左到右的渐变遮罩 */}
            <div className="absolute inset-0 bg-gradient-to-r from-transparent via-brand-100/40 to-brand-100/80 dark:via-black/20 dark:to-brand-900/70" />
            {/* 底部渐变遮罩 */}
            <div className="absolute inset-0 bg-gradient-to-t from-brand-100/90 via-transparent to-transparent dark:from-brand-900/60" />
            {/* 顶部轻微渐变 */}
            <div className="absolute inset-0 bg-gradient-to-b from-brand-100/40 via-transparent to-transparent dark:from-black/20" />
          </div>
        )}
        <div className="absolute top-6 left-8">
          <h1 className="text-4xl font-bold text-brand-900 dark:text-white drop-shadow-lg">首页</h1>
          <p className="mt-2 text-brand-600 dark:text-white/80 drop-shadow">欢迎回来</p>
        </div>
        <div className="glass-card absolute top-6 right-6 flex items-center gap-2 bg-white/80 dark:bg-brand-800/80 backdrop-blur-sm px-4 py-3 rounded-xl shadow-lg">
          <span className="i-mdi-clock-outline text-xl text-neutral-500" />
          <div>
            <div className="text-xs text-brand-500 dark:text-brand-400">今日游玩时间</div>
            <div className="text-lg font-bold text-neutral-600 dark:text-neutral-400">
              {formatDuration(homeData.today_play_time_sec)}
            </div>
          </div>
        </div>
        <div className="absolute bottom-8 left-8 max-w-lg">
          <h1
            className="text-4xl font-bold text-brand-900 dark:text-white mb-2 cursor-pointer hover:text-neutral-600 dark:hover:text-neutral-300 transition-colors drop-shadow-lg"
            onClick={() => navigate({ to: "/game/$gameId", params: { gameId: lastPlayed.game.id } })}
          >
            {lastPlayed.game.name}
          </h1>
          <p className="text-brand-700 dark:text-white/80 text-sm drop-shadow">
            {isPlaying ? "正在游玩中..." : `上次游玩：${formatLocalDateTime(lastPlayed.last_played_at, config?.time_zone)}`}
          </p>
          {lastPlayed.total_played_dur > 0 && !isPlaying && (
            <p className="text-brand-600 dark:text-white/70 text-sm mt-1 drop-shadow">
              总游玩时长：
              {formatDuration(lastPlayed.total_played_dur)}
            </p>
          )}
        </div>
        {isPlaying
          ? (
              <div className="absolute bottom-8 right-8 flex items-center gap-2 px-6 py-3 bg-success-600 text-white rounded-xl shadow-lg font-medium">
                <span className="i-mdi-gamepad-variant text-xl animate-pulse" />
                正在游戏
              </div>
            )
          : (
              <button
                onClick={handleContinuePlay}
                className="absolute bottom-8 right-8 flex items-center gap-2 px-6 py-3 bg-neutral-600 hover:bg-neutral-700 text-white rounded-xl shadow-lg transition-all hover:scale-105 font-medium"
              >
                <span className="i-mdi-play text-xl" />
                继续游戏
              </button>
            )}
      </div>
    </div>
  );
}
