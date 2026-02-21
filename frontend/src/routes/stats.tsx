import { createRoute } from "@tanstack/react-router";
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
import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { enums, vo } from "../../wailsjs/go/models";
import { AISummarize } from "../../wailsjs/go/service/AiService";
import { GetGlobalPeriodStats } from "../../wailsjs/go/service/StatsService";
import { AiSummaryCard } from "../components/card/AiSummaryCard";
import { HorizontalScrollChart } from "../components/chart/HorizontalScrollChart";
import { TemplateExportModal } from "../components/modal/TemplateExportModal";
import { StatsSkeleton } from "../components/skeleton/StatsSkeleton";
import { CollapsibleSection } from "../components/ui/CollapsibleSection";
import { SlideButton } from "../components/ui/SlideButton";
import { useChartTheme } from "../hooks/useChartTheme";
import { useAppStore } from "../store";
import { formatDateToYYYYMMDD, formatDurationHours, formatDurationShort } from "../utils/time";
import { Route as rootRoute } from "./__root";

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
);

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/stats",
  component: StatsPage,
});

function StatsPage() {
  const { t } = useTranslation();
  const ref = useRef<HTMLDivElement>(null);
  const { textColor, gridColor } = useChartTheme();
  const [dimension, setDimension] = useState<enums.Period>(enums.Period.WEEK);
  const [stats, setStats] = useState<vo.PeriodStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [showSkeleton, setShowSkeleton] = useState(false);
  const [aiLoading, setAiLoading] = useState(false);
  const [showTemplateModal, setShowTemplateModal] = useState(false);

  // Custom date range
  const [customDateRange, setCustomDateRange] = useState(false);
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");

  // Delay skeleton display to avoid flash
  useEffect(() => {
    let timer: number;
    if (loading) {
      timer = window.setTimeout(() => {
        setShowSkeleton(true);
      }, 300);
    }
    else {
      setShowSkeleton(false);
    }
    return () => clearTimeout(timer);
  }, [loading]);

  // Get cached AI summary from store
  const { aiSummaryCache, setAISummary } = useAppStore();
  const aiSummary = aiSummaryCache[dimension] || "";

  const handleAISummarize = useCallback(async () => {
    setAiLoading(true);
    setAISummary(dimension, "");
    try {
      const result = await AISummarize({ dimension });
      setAISummary(dimension, result.summary);
    }
    catch (err) {
      console.error("AI summarize failed:", err);
      setAISummary(dimension, "");
      toast.error(t("stats.ai.summarizeFailed"));
    }
    finally {
      setAiLoading(false);
    }
  }, [dimension, setAISummary, t]);

  const loadStats = async (dim: enums.Period, start?: string, end?: string) => {
    setLoading(true);
    try {
      const req = new vo.PeriodStatsRequest({
        dimension: dim,
        start_date: start || "",
        end_date: end || "",
      });
      const data = await GetGlobalPeriodStats(req);
      setStats(data);
    }
    catch (error) {
      console.error("Failed to load stats:", error);
      toast.error(t("stats.toast.loadStatsFailed"));
    }
    finally {
      setLoading(false);
    }
  };

  const handleApplyDateRange = () => {
    if (!startDate || !endDate) {
      toast.error(t("stats.toast.selectDateRange"));
      return;
    }
    if (new Date(startDate) >= new Date(endDate)) {
      toast.error(t("stats.toast.startBeforeEnd"));
      return;
    }
    loadStats(enums.Period.DAY, startDate, endDate);
  };

  const handleResetDateRange = () => {
    setCustomDateRange(false);
    setStartDate("");
    setEndDate("");
    loadStats(dimension);
  };

  useEffect(() => {
    if (!customDateRange) {
      loadStats(dimension);
    }
  }, [dimension]);

  // When switching to custom date range, initialize dates to today
  useEffect(() => {
    if (customDateRange && !startDate && !endDate) {
      const today = formatDateToYYYYMMDD(new Date());
      setStartDate(today);
      setEndDate(today);
    }
  }, [customDateRange]);

  if (loading && !stats) {
    if (!showSkeleton) {
      return null;
    }
    return <StatsSkeleton />;
  }

  if (!stats) {
    return null;
  }

  // Chart 1: Total Play Duration Trend
  const totalTrendData = {
    labels: stats.timeline.map(p => p.label),
    datasets: [
      {
        label: t("stats.totalDurationDataset"),
        data: stats.timeline.map(p => formatDurationHours(p.duration)),
        borderColor: "rgb(75, 192, 192)",
        backgroundColor: "rgba(75, 192, 192, 0.5)",
        tension: 0.3,
      },
    ],
  };

  // Chart 2: Game Play Duration Trend (Multi-line)
  const gameTrendData = {
    labels: stats.timeline.map(p => p.label),
    datasets: stats.leaderboard_series.map((series, index) => {
      const colors = [
        "rgb(255, 99, 132)",
        "rgb(54, 162, 235)",
        "rgb(255, 206, 86)",
        "rgb(75, 192, 192)",
        "rgb(153, 102, 255)",
      ];
      const color = colors[index % colors.length];
      return {
        label: series.game_name,
        data: series.points.map(p => formatDurationHours(p.duration)),
        borderColor: color,
        backgroundColor: color.replace("rgb", "rgba").replace(")", ", 0.5)"),
        tension: 0.3,
      };
    }),
  };

  const chartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        position: "top" as const,
        labels: {
          color: textColor,
        },
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
        title: {
          display: true,
          text: t("stats.chartYAxis"),
          color: textColor,
        },
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
    <div
      id="stats-container"
      ref={ref}
      className={`space-y-6 max-w-8xl mx-auto p-8 transition-opacity duration-300 ${loading ? "opacity-50 pointer-events-none" : "opacity-100"}`}
    >
      <div className="flex items-center justify-between">
        <h1 className="text-4xl font-bold text-brand-900 dark:text-white">{t("stats.title")}</h1>
      </div>
      <div className="flex justify-between items-center no-export">
        <div className="flex items-center space-x-4">
          <SlideButton
            options={[
              { label: t("stats.period.week"), value: enums.Period.WEEK },
              { label: t("stats.period.month"), value: enums.Period.MONTH },
            ]}
            value={customDateRange ? "" as enums.Period : dimension}
            onChange={(value) => {
              setDimension(value);
              if (customDateRange) {
                setCustomDateRange(false);
                setStartDate("");
                setEndDate("");
              }
            }}
            disabled={loading}
          />

          {/* Custom Date Range Toggle */}
          <button
            type="button"
            onClick={() => setCustomDateRange(!customDateRange)}
            className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors flex items-center gap-1 ${customDateRange
              ? "bg-neutral-100 dark:bg-neutral-900 text-neutral-600 dark:text-neutral-400"
              : "text-brand-600 dark:text-brand-400 hover:text-brand-900 dark:hover:text-brand-200"
            }`}
          >
            <span className="i-mdi-calendar-range text-lg" />
            {t("stats.customRange")}
          </button>
        </div>
        <div className="flex space-x-2 items-center">
          <button type="button" onClick={() => setShowTemplateModal(true)} className="flex justify-end i-mdi-image-filter-hdr text-2xl text-brand-600 dark:text-brand-400 hover:text-brand-900 dark:hover:text-brand-200 transition-colors" title={t("stats.exportTitle")} />
          <button type="button" onClick={handleAISummarize} className="flex justify-end i-mdi-robot-happy text-2xl text-brand-600 dark:text-brand-400 hover:text-brand-900 dark:hover:text-brand-200 transition-colors" title={t("stats.aiSummarizeTitle")} />
        </div>
      </div>

      {/* Custom Date Range Picker */}
      {customDateRange && (
        <div className="glass-panel flex items-center gap-4 p-4 bg-white dark:bg-brand-800 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700 no-export">
          <div className="flex items-center gap-2">
            <label className="text-sm text-brand-600 dark:text-brand-400">{t("stats.startDate")}</label>
            <input
              type="date"
              value={startDate}
              onChange={e => setStartDate(e.target.value)}
              className="px-3 py-1.5 rounded-md border border-brand-300 dark:border-brand-600 bg-white dark:bg-brand-700 text-brand-900 dark:text-white text-sm"
            />
          </div>
          <div className="flex items-center gap-2">
            <label className="text-sm text-brand-600 dark:text-brand-400">{t("stats.endDate")}</label>
            <input
              type="date"
              value={endDate}
              onChange={e => setEndDate(e.target.value)}
              className="px-3 py-1.5 rounded-md border border-brand-300 dark:border-brand-600 bg-white dark:bg-brand-700 text-brand-900 dark:text-white text-sm"
            />
          </div>
          <button
            type="button"
            onClick={handleApplyDateRange}
            className="px-4 py-1.5 bg-neutral-600 hover:bg-neutral-700 text-white rounded-md text-sm font-medium transition-colors"
          >
            {t("stats.applyBtn")}
          </button>
          <button
            type="button"
            onClick={handleResetDateRange}
            className="px-4 py-1.5 bg-brand-200 dark:bg-brand-700 hover:bg-brand-300 dark:hover:bg-brand-600 text-brand-700 dark:text-brand-300 rounded-md text-sm font-medium transition-colors"
          >
            {t("stats.resetBtn")}
          </button>
        </div>
      )}

      {/* AI Summary Card */}
      {(aiLoading || aiSummary) && (
        <AiSummaryCard
          aiSummary={aiSummary}
          aiLoading={aiLoading}
        />
      )}

      {/* Library Summary */}
      <CollapsibleSection
        title={t("stats.library.sectionTitle")}
        icon="i-mdi-library-shelves"
        defaultOpen={false}
      >
        <div className="flex items-center justify-between">
          <div className="text-center">
            <p className="text-2xl font-bold text-brand-900 dark:text-white">{stats.library_games_count}</p>
            <p className="text-sm text-brand-500 dark:text-brand-400 mt-1">{t("stats.library.totalGames")}</p>
          </div>
          <div className="text-center">
            <p className="text-2xl font-bold text-brand-900 dark:text-white">{stats.all_sessions_count}</p>
            <p className="text-sm text-brand-500 dark:text-brand-400 mt-1">{t("stats.library.totalSessions")}</p>
          </div>
          <div className="text-center">
            <p className="text-2xl font-bold text-brand-900 dark:text-white">{formatDurationShort(stats.all_sessions_duration)}</p>
            <p className="text-sm text-brand-500 dark:text-brand-400 mt-1">{t("stats.library.totalDuration")}</p>
          </div>
          <div className="text-center">
            <p className="text-2xl font-bold text-brand-900 dark:text-white">{stats.all_completed_games_count}</p>
            <p className="text-sm text-brand-500 dark:text-brand-400 mt-1">{t("stats.library.completedGames")}</p>
          </div>
        </div>
      </CollapsibleSection>

      {/* Summary Cards */}
      <div className="flex flex-wrap gap-6">
        <div className="flex-1 min-w-[150px] glass-card bg-white dark:bg-brand-800 p-6 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700">
          <h3 className="text-sm font-medium text-brand-500 dark:text-brand-400 mb-2">{t("stats.summary.totalPlayCount")}</h3>
          <p className="text-3xl font-bold text-brand-900 dark:text-white">{stats.total_play_count}</p>
        </div>
        <div className="flex-1 min-w-[150px] glass-card bg-white dark:bg-brand-800 p-6 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700">
          <h3 className="text-sm font-medium text-brand-500 dark:text-brand-400 mb-2">{t("stats.summary.totalPlayDuration")}</h3>
          <p className="text-3xl font-bold text-brand-900 dark:text-white">{formatDurationShort(stats.total_play_duration)}</p>
        </div>
        <div className="flex-1 min-w-[150px] glass-card bg-white dark:bg-brand-800 p-6 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700">
          <h3 className="text-sm font-medium text-brand-500 dark:text-brand-400 mb-2">{t("stats.summary.gamesPlayed")}</h3>
          <p className="text-3xl font-bold text-brand-900 dark:text-white">{stats.total_games_count}</p>
        </div>
        <div className="flex-1 min-w-[150px] glass-card bg-white dark:bg-brand-800 p-6 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700">
          <h3 className="text-sm font-medium text-brand-500 dark:text-brand-400 mb-2">{t("stats.summary.completedGames")}</h3>
          <p className="text-3xl font-bold text-brand-900 dark:text-white">{stats.completed_games_count}</p>
        </div>
      </div>

      {/* Leaderboard */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {/* Top 1 Game Card */}
        {stats.play_time_leaderboard.length > 0 && (
          <div className="glass-card md:col-span-1 lg:col-span-1 bg-white dark:bg-brand-800 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700 p-6 flex flex-col items-center text-center relative overflow-hidden">
            <div className="absolute top-0 left-0 w-full h-1.5 bg-gradient-to-r from-yellow-400 to-orange-500" />
            <div className="w-10 h-10 bg-yellow-100 dark:bg-yellow-900/30 text-yellow-600 dark:text-yellow-400 rounded-full flex items-center justify-center text-lg font-bold mb-4 shadow-sm">
              #1
            </div>
            <div className="relative group">
              <img
                src={stats.play_time_leaderboard[0].cover_url}
                alt={stats.play_time_leaderboard[0].game_name}
                referrerPolicy="no-referrer"
                className="w-full h-auto block object-cover rounded-lg shadow-md mb-4 transition-transform group-hover:scale-105 bg-brand-200 dark:bg-brand-700"
                draggable="false"
                onDragStart={e => e.preventDefault()}
              />
            </div>
            <h3 className="text-lg font-bold text-brand-900 dark:text-white mb-2 line-clamp-2 px-2">
              {stats.play_time_leaderboard[0].game_name}
            </h3>
            <p className="text-2xl font-mono font-semibold text-neutral-600 dark:text-neutral-400">
              {formatDurationShort(stats.play_time_leaderboard[0].total_duration)}
            </p>
          </div>
        )}

        {/* Other Games List */}
        <div className={`glass-card ${stats.play_time_leaderboard.length > 0 ? "md:col-span-1 lg:col-span-2" : "md:col-span-2 lg:col-span-3"} bg-white dark:bg-brand-800 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700 overflow-hidden flex flex-col`}>
          <div className="p-6 border-b border-brand-200 dark:border-brand-700">
            <h3 className="text-lg font-semibold text-brand-900 dark:text-white">
              {stats.play_time_leaderboard.length > 0 ? t("stats.leaderboard.title") : t("stats.leaderboard.fullTitle")}
            </h3>
          </div>
          <div className="overflow-x-auto flex-1">
            <table className="w-full text-left text-sm">
              <thead className="data-glass:bg-white/5 data-glass:dark:bg-black/5 bg-brand-50 dark:bg-brand-700/50">
                <tr>
                  <th className="px-6 py-3 font-medium text-brand-500 dark:text-brand-400 w-20">{t("stats.leaderboard.rankCol")}</th>
                  <th className="px-6 py-3 font-medium text-brand-500 dark:text-brand-400">{t("stats.leaderboard.gameCol")}</th>
                  <th className="px-6 py-3 font-medium text-brand-500 dark:text-brand-400 text-right">{t("stats.leaderboard.durationCol")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-brand-200 dark:divide-brand-700">
                {stats.play_time_leaderboard.slice(1).map((game, index) => (
                  <tr key={game.game_id} className="hover:bg-brand-50 dark:hover:bg-brand-700/50 transition-colors data-glass:hover:bg-white/5 data-glass:hover:dark:bg-black/5">
                    <td className="px-6 py-4 text-brand-500 dark:text-brand-400 font-medium">
                      #
                      {index + 2}
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex items-center">
                        <img
                          src={game.cover_url}
                          alt={game.game_name}
                          referrerPolicy="no-referrer"
                          className="w-10 h-14 object-cover rounded shadow-sm mr-4 bg-brand-200 dark:bg-brand-700 data-glass:bg-white/5 data-glass:dark:bg-black/5"
                          draggable="false"
                          onDragStart={e => e.preventDefault()}
                        />
                        <span className="font-medium text-brand-900 dark:text-white line-clamp-1">{game.game_name}</span>
                      </div>
                    </td>
                    <td className="px-6 py-4 text-brand-900 dark:text-white text-right font-mono">
                      {formatDurationShort(game.total_duration)}
                    </td>
                  </tr>
                ))}
                {stats.play_time_leaderboard.length <= 1 && (
                  <tr>
                    <td colSpan={3} className="px-6 py-12 text-center text-brand-500 dark:text-brand-400">
                      {stats.play_time_leaderboard.length === 0 ? t("stats.leaderboard.noData") : t("stats.leaderboard.noMoreData")}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      {/* Charts */}
      <div className="space-y-6">
        <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700">
          <h3 className="text-lg font-semibold text-brand-900 dark:text-white mb-4">{t("stats.charts.totalTrend")}</h3>
          <HorizontalScrollChart
            data={totalTrendData}
            options={chartOptions}
            className="h-96"
          />
        </div>
        <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700">
          <h3 className="text-lg font-semibold text-brand-900 dark:text-white mb-4">{t("stats.charts.gameTrend")}</h3>
          <HorizontalScrollChart
            data={gameTrendData}
            options={chartOptions}
            className="h-96"
          />
        </div>
      </div>

      {/* Template Export Modal */}
      <TemplateExportModal
        isOpen={showTemplateModal}
        onClose={() => setShowTemplateModal(false)}
        stats={stats}
        aiSummary={aiSummary}
      />
    </div>
  );
}
