import type { models, vo } from "../../../wailsjs/go/models";
import {
  CategoryScale,
  Chart as ChartJS,
  Legend,
  LinearScale,
  LineElement,
  PointElement,
  Title,
  Tooltip,
} from "chart.js";
import { useCallback, useEffect, useState } from "react";
import { toast } from "react-hot-toast";
import { enums } from "../../../wailsjs/go/models";
import { DeletePlaySession, GetPlaySessions } from "../../../wailsjs/go/service/SessionService";
import { GetGameStats } from "../../../wailsjs/go/service/StatsService";
import { useChartTheme } from "../../hooks/useChartTheme";
import { useAppStore } from "../../store";
import { formatDuration, formatLocalDateTime } from "../../utils/time";
import { HorizontalScrollChart } from "../chart/HorizontalScrollChart";
import { AddPlaySessionModal } from "../modal/AddPlaySessionModal";
import { ConfirmModal } from "../modal/ConfirmModal";
import { SlideButton } from "../ui/SlideButton";

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
);

interface GameStatsPanelProps {
  gameId: string;
}

type ViewMode = "chart" | "sessions";

export function GameStatsPanel({ gameId }: GameStatsPanelProps) {
  const { config } = useAppStore();
  const { textColor, gridColor } = useChartTheme();
  const [stats, setStats] = useState<vo.GameDetailStats | null>(null);
  const [sessions, setSessions] = useState<models.PlaySession[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [viewMode, setViewMode] = useState<ViewMode>("chart");
  const [timeDimension, setTimeDimension] = useState<enums.Period>(enums.Period.WEEK);
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [deleteSessionId, setDeleteSessionId] = useState<string | null>(null);

  const loadStats = useCallback(async () => {
    try {
      const statsData = await GetGameStats({
        game_id: gameId,
        dimension: timeDimension,
        start_date: "",
        end_date: "",
      });
      setStats(statsData);
    }
    catch (error) {
      console.error("Failed to load game stats:", error);
      toast.error("加载统计数据失败");
    }
  }, [gameId, timeDimension]);

  const loadSessions = useCallback(async () => {
    try {
      const data = await GetPlaySessions(gameId);
      setSessions(data || []);
    }
    catch (error) {
      console.error("Failed to load play sessions:", error);
      toast.error("加载游玩记录失败");
    }
  }, [gameId]);

  useEffect(() => {
    const loadData = async () => {
      try {
        setIsLoading(true);
        await Promise.all([loadStats(), loadSessions()]);
      }
      finally {
        setIsLoading(false);
      }
    };
    loadData();
  }, [loadStats, loadSessions]);

  const handleDeleteSession = async () => {
    if (!deleteSessionId)
      return;

    try {
      await DeletePlaySession(deleteSessionId);
      toast.success("删除成功");
      await Promise.all([loadStats(), loadSessions()]);
    }
    catch (error) {
      console.error("Failed to delete play session:", error);
      toast.error("删除失败");
    }
    finally {
      setDeleteSessionId(null);
    }
  };

  const handleSessionAdded = async () => {
    await Promise.all([loadStats(), loadSessions()]);
  };
  const chartData = {
    labels: stats?.recent_play_history?.map(h => h.date) || [], // 后端已返回本地日期字符串，直接使用
    datasets: [
      {
        label: "游戏时长 (分钟)",
        data: stats?.recent_play_history?.map(h => h.duration / 60) || [],
        borderColor: "rgb(59, 130, 246)",
        backgroundColor: "rgba(59, 130, 246, 0.5)",
        tension: 0.3,
      },
    ],
  };

  const chartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    interaction: {
      mode: "index" as const,
      intersect: false,
    },
    plugins: {
      legend: {
        display: false,
      },
      title: {
        display: false,
      },
    },
    scales: {
      x: {
        grid: {
          color: gridColor,
        },
        ticks: {
          color: textColor,
        },
      },
      y: {
        beginAtZero: true,
        grid: {
          color: gridColor,
        },
        ticks: {
          color: textColor,
        },
      },
    },
  };

  return (
    <div className="space-y-8">
      {/* 统计卡片 */}
      <div className="grid grid-cols-3 gap-6">
        <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
          <div className="text-sm text-brand-500 dark:text-brand-400 mb-2">累计游戏次数</div>
          <div className="text-2xl font-bold text-brand-900 dark:text-white">
            {stats?.total_play_count ?? (isLoading ? "-" : 0)}
          </div>
        </div>
        <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
          <div className="text-sm text-brand-500 dark:text-brand-400 mb-2">今日游戏时长</div>
          <div className="text-2xl font-bold text-brand-900 dark:text-white">
            {stats ? formatDuration(stats.today_play_time) : (isLoading ? "-" : "0分钟")}
          </div>
        </div>
        <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
          <div className="text-sm text-brand-500 dark:text-brand-400 mb-2">累计总时长</div>
          <div className="text-2xl font-bold text-brand-900 dark:text-white">
            {stats ? formatDuration(stats.total_play_time) : (isLoading ? "-" : "0分钟")}
          </div>
        </div>
      </div>

      {/* 视图切换和操作栏 */}
      <div className="glass-card bg-white dark:bg-brand-800 rounded-lg shadow-sm">
        <div className="p-6">
          {isLoading && !stats
            ? (
                <div className="h-80 flex items-center justify-center">
                  <div className="i-mdi-loading animate-spin text-3xl text-brand-500" />
                </div>
              )
            : viewMode === "chart"
              ? (
                  <HorizontalScrollChart
                    data={chartData}
                    options={chartOptions}
                    className="h-80"
                  />
                )
              : (
                  <div className="space-y-2">
                    {sessions.length === 0
                      ? (
                          <div className="text-center py-12 text-brand-500">
                            <div className="i-mdi-clock-outline text-4xl mx-auto mb-2" />
                            <p>暂无游玩记录</p>
                          </div>
                        )
                      : (
                          sessions.map(session => (
                            <div
                              key={session.id}
                              className="flex items-center justify-between p-3 bg-brand-50 dark:bg-brand-700/50 rounded-lg"
                            >
                              <div className="flex-1">
                                <div className="text-sm text-brand-900 dark:text-white">
                                  {formatLocalDateTime(session.start_time, config?.time_zone)}
                                </div>
                                <div className="text-xs text-brand-500 dark:text-brand-400">
                                  时长:
                                  {" "}
                                  {formatDuration(session.duration)}
                                </div>
                              </div>
                              <button
                                onClick={() => setDeleteSessionId(session.id)}
                                className="p-1.5 text-brand-400 hover:text-error-500 hover:bg-error-50 dark:hover:bg-error-900/20 rounded transition-colors"
                                title="删除记录"
                                type="button"
                              >
                                <div className="i-mdi-delete-outline text-lg" />
                              </button>
                            </div>
                          ))
                        )}
                  </div>
                )}
        </div>
        <div className="flex items-center justify-between p-4 border-t-1 border-brand-200 dark:border-brand-700">
          <div className="flex gap-4">
            {/* Time Dimension Selector */}
            <SlideButton
              options={[
                { label: "周", value: enums.Period.WEEK },
                { label: "月", value: enums.Period.MONTH },
                { label: "全部", value: enums.Period.ALL },
              ]}
              value={timeDimension}
              onChange={setTimeDimension}
              disabled={isLoading}
            />

            {/* View Mode Selector */}
            <div className="flex gap-2">
              <button
                onClick={() => setViewMode("chart")}
                className={`glass-btn-neutral flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${viewMode === "chart"
                  ? "bg-neutral-600 text-white"
                  : "bg-brand-100 text-brand-700 dark:bg-brand-700 dark:text-brand-300 hover:bg-brand-200 dark:hover:bg-brand-600"
                }`}
                type="button"
              >
                <div className="i-mdi-chart-line text-lg" />
              </button>
              <button
                onClick={() => setViewMode("sessions")}
                className={`glass-btn-neutral flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${viewMode === "sessions"
                  ? "bg-neutral-600 text-white"
                  : "bg-brand-100 text-brand-700 dark:bg-brand-700 dark:text-brand-300 hover:bg-brand-200 dark:hover:bg-brand-600"
                }`}
                type="button"
              >
                <div className="i-mdi-format-list-bulleted text-lg" />
              </button>
            </div>
          </div>

          <button
            onClick={() => setIsAddModalOpen(true)}
            className="glass-btn-neutral flex items-center gap-1 px-3 py-1.5 bg-neutral-600 text-white rounded-md hover:bg-neutral-700 transition-colors text-sm"
            type="button"
          >
            <div className="i-mdi-plus text-lg" />
            手动添加
          </button>
        </div>
      </div>

      <AddPlaySessionModal
        isOpen={isAddModalOpen}
        gameId={gameId}
        onClose={() => setIsAddModalOpen(false)}
        onSuccess={handleSessionAdded}
      />

      <ConfirmModal
        isOpen={!!deleteSessionId}
        title="删除游玩记录"
        message="确定要删除这条游玩记录吗？此操作不可撤销。"
        confirmText="确认删除"
        type="danger"
        onClose={() => setDeleteSessionId(null)}
        onConfirm={handleDeleteSession}
      />
    </div>
  );
}
