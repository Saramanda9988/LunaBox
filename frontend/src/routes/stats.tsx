import { createRoute } from "@tanstack/react-router";
import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { enums, vo } from "../../wailsjs/go/models";
import { AISummarize } from "../../wailsjs/go/service/AiService";
import { GetGlobalPeriodStats } from "../../wailsjs/go/service/StatsService";
import { StatsToolbar } from "../components/bar/StatsToolbar";
import { AiSummaryCard } from "../components/card/AiSummaryCard";
import { DurationLineChart } from "../components/chart/DurationLineChart";
import { HourWeekDistribution } from "../components/chart/HourWeekDistribution";
import { TagDistributionChart } from "../components/chart/TagDistributionChart";
import { StatsLeaderboardModal } from "../components/modal/StatsLeaderboardModal";
import { TemplateExportModal } from "../components/modal/TemplateExportModal";
import { StatsSkeleton } from "../components/skeleton/StatsSkeleton";
import { ProxyImage } from "../components/ui/ProxyImage";
import { useAppStore } from "../store";
import {
  formatDateToYYYYMMDD,
  formatDuration,
  formatDurationCompact,
} from "../utils/time";
import { Route as rootRoute } from "./__root";

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/stats",
  component: StatsPage,
});

interface StatsContentProps {
  stats: vo.PeriodStats;
}

interface SummaryItem {
  value: string | number;
  label: string;
  suffix?: string;
  valueClassName?: string;
}

const GAME_TREND_COLORS = [
  "rgb(255, 99, 132)",
  "rgb(54, 162, 235)",
  "rgb(255, 206, 86)",
  "rgb(75, 192, 192)",
  "rgb(153, 102, 255)",
  "rgb(255, 159, 64)",
  "rgb(99, 255, 132)",
  "rgb(199, 99, 255)",
  "rgb(64, 224, 208)",
  "rgb(255, 105, 180)",
];

const StatsContent = memo(({ stats }: StatsContentProps) => {
  const { t } = useTranslation();
  const [showLeaderboardModal, setShowLeaderboardModal] = useState(false);

  const summaryItems = useMemo<SummaryItem[]>(
    () => [
      {
        value: stats.total_play_count,
        label: t("stats.summary.totalPlayCount"),
      },
      {
        value: formatDurationCompact(stats.total_play_duration, t),
        label: t("stats.summary.totalPlayDuration"),
        valueClassName: "whitespace-nowrap",
      },
      {
        value: stats.total_games_count,
        label: t("stats.summary.gamesPlayed"),
      },
      {
        value: stats.completed_games_count,
        label: t("stats.summary.completedGames"),
      },
      {
        value: formatDurationCompact(stats.avg_daily_duration, t),
        label: t("stats.summary.avgDailyDuration"),
        valueClassName: "whitespace-nowrap",
      },
      {
        value: formatDurationCompact(stats.avg_session_duration, t),
        label: t("stats.summary.avgSessionDuration"),
        valueClassName: "whitespace-nowrap",
      },
      {
        value: stats.max_streak,
        label: t("stats.summary.maxStreak"),
        suffix: t("stats.summary.dayUnit"),
      },
      {
        value: stats.new_games_count,
        label: t("stats.summary.newGames"),
      },
    ],
    [stats, t],
  );

  const timelineLabels = useMemo(
    () => stats.timeline.map(point => point.label),
    [stats.timeline],
  );

  const totalTrendDurations = useMemo(
    () => stats.timeline.map(point => point.duration),
    [stats.timeline],
  );

  const hasTotalTrendPlayData = useMemo(
    () => totalTrendDurations.some(duration => duration > 0),
    [totalTrendDurations],
  );

  const totalTrendData = useMemo(
    () => ({
      labels: timelineLabels,
      datasets: [
        {
          label: t("stats.totalDurationDataset"),
          data: totalTrendDurations,
          borderColor: "rgb(75, 192, 192)",
          backgroundColor: "rgba(75, 192, 192, 0.5)",
          tension: 0.3,
        },
      ],
    }),
    [t, timelineLabels, totalTrendDurations],
  );

  const gameTrendDurations = useMemo(
    () =>
      stats.leaderboard_series.flatMap(series =>
        series.points.map(point => point.duration),
      ),
    [stats.leaderboard_series],
  );

  const hasGameTrendPlayData = useMemo(
    () => gameTrendDurations.some(duration => duration > 0),
    [gameTrendDurations],
  );

  const gameTrendData = useMemo(
    () => ({
      labels: timelineLabels,
      datasets: stats.leaderboard_series.map((series, index) => {
        const color = GAME_TREND_COLORS[index % GAME_TREND_COLORS.length];
        return {
          label: series.game_name,
          data: series.points.map(point => point.duration),
          borderColor: color,
          backgroundColor: color.replace("rgb", "rgba").replace(")", ", 0.5)"),
          tension: 0.3,
        };
      }),
    }),
    [stats.leaderboard_series, timelineLabels],
  );

  const previewLeaderboard = useMemo(
    () => stats.play_time_leaderboard.slice(0, 10),
    [stats.play_time_leaderboard],
  );

  const modalLeaderboard = useMemo(
    () => stats.play_time_leaderboard.slice(0, 20),
    [stats.play_time_leaderboard],
  );

  const hasLeaderboard = previewLeaderboard.length > 0;
  const showMoreLeaderboard = previewLeaderboard.length >= 10;
  const hasTagDistribution = (stats.tag_distribution?.length ?? 0) > 0;
  return (
    <>
      {/* Summary Cards - compact 4/8 cols grid */}
      <div className="grid grid-cols-2 sm:grid-cols-4 2xl:grid-cols-8 gap-3">
        {summaryItems.map(item => (
          <div
            key={item.label}
            className="glass-card bg-white dark:bg-brand-800 px-4 py-3 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700"
          >
            <h3 className="text-xs font-medium text-brand-500 dark:text-brand-400 mb-1 truncate">
              {item.label}
            </h3>
            <p
              className={`text-xl font-bold text-brand-900 dark:text-white ${item.valueClassName ?? ""}`}
            >
              {item.value}
              {item.suffix && (
                <span className="text-xs font-normal text-brand-500 dark:text-brand-400 ml-1">
                  {item.suffix}
                </span>
              )}
            </p>
          </div>
        ))}
      </div>

      {/* Row: Leaderboard + Tag Distribution (container-query 2-col) */}
      <div className="grid grid-cols-1 @[1024px]:grid-cols-12 gap-6">
        <div className="@[1024px]:col-span-7 glass-card bg-white dark:bg-brand-800 p-5 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700 flex flex-col">
          <h3 className="text-base font-semibold text-brand-900 dark:text-white mb-3">
            {t("stats.leaderboard.fullTitle")}
          </h3>

          {!hasLeaderboard ? (
            <div className="px-6 py-12 text-center text-brand-500 dark:text-brand-400">
              {t("stats.leaderboard.noData")}
            </div>
          ) : (
            <div className="space-y-3 flex-1">
              {/* #1 hero - compact horizontal */}
              <div className="relative flex items-center gap-3 p-3 rounded-lg bg-gradient-to-r from-yellow-100/80 via-amber-50/70 to-yellow-50/20 dark:from-yellow-900/10 dark:via-orange-900/20 dark:to-transparent border border-yellow-200/60 dark:border-yellow-800/40 overflow-hidden">
                <div className="w-7 h-7 bg-yellow-100 dark:bg-yellow-900/40 text-yellow-600 dark:text-yellow-400 rounded-full flex items-center justify-center text-xs font-bold flex-shrink-0">
                  #1
                </div>
                <ProxyImage
                  src={previewLeaderboard[0].cover_url}
                  alt={previewLeaderboard[0].game_name}
                  className="w-10 h-14 object-cover rounded shadow-md flex-shrink-0 bg-brand-200 dark:bg-brand-700"
                />
                <div className="flex-1 min-w-0">
                  <h4 className="text-sm font-bold text-brand-900 dark:text-white line-clamp-1">
                    {previewLeaderboard[0].game_name}
                  </h4>
                  <p className="text-base font-mono font-semibold text-neutral-700 dark:text-neutral-300 mt-0.5">
                    {formatDuration(previewLeaderboard[0].total_duration, t)}
                  </p>
                </div>
              </div>

              {/* #2 - #10: two-column grid, denser rows */}
              {previewLeaderboard.length > 1 && (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                  {previewLeaderboard.slice(1).map((game, index) => (
                    <div
                      key={game.game_id}
                      className="flex items-center gap-2.5 p-2 rounded-lg bg-brand-50 dark:bg-brand-700/40 hover:bg-brand-100 dark:hover:bg-brand-700/60 transition-colors data-glass:bg-white/5 data-glass:dark:bg-black/5"
                    >
                      <span className="w-6 text-xs text-brand-500 dark:text-brand-400 font-medium tabular-nums text-center flex-shrink-0">
                        #
                        {index + 2}
                      </span>
                      <ProxyImage
                        src={game.cover_url}
                        alt={game.game_name}
                        className="w-7 h-10 object-cover rounded shadow-sm flex-shrink-0 bg-brand-200 dark:bg-brand-700"
                      />
                      <div className="flex-1 min-w-0">
                        <p className="text-xs font-medium text-brand-900 dark:text-white line-clamp-1">
                          {game.game_name}
                        </p>
                        <p className="text-[11px] font-mono text-brand-600 dark:text-brand-300 mt-0.5">
                          {formatDuration(game.total_duration, t)}
                        </p>
                      </div>
                    </div>
                  ))}
                  {showMoreLeaderboard && (
                    <button
                      type="button"
                      onClick={() => setShowLeaderboardModal(true)}
                      className="flex min-h-[3.5rem] items-center justify-start gap-3 rounded-lg border border-dashed border-brand-300 bg-transparent px-4 py-2 text-left transition-colors hover:border-brand-500 hover:bg-brand-50/40 data-glass:bg-transparent data-glass:hover:bg-white/5 dark:border-brand-600 dark:hover:border-brand-400 dark:hover:bg-brand-700/20"
                    >
                      <span className="i-mdi-format-list-numbered flex-shrink-0 text-3xl text-brand-500 dark:text-brand-300" />
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-brand-900 dark:text-white line-clamp-1">
                          {t("stats.leaderboard.viewMore")}
                        </p>
                        <p className="mt-1 text-xs text-brand-500 dark:text-brand-400">
                          {t("stats.leaderboard.countHint", {
                            count: modalLeaderboard.length,
                          })}
                        </p>
                      </div>
                    </button>
                  )}
                </div>
              )}
            </div>
          )}
        </div>

        {/* Tag Distribution - @[1024px]:col-span-5 */}
        {hasTagDistribution && (
          <div className="@[1024px]:col-span-5 glass-card bg-white dark:bg-brand-800 p-5 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700 flex flex-col">
            <h3 className="text-base font-semibold text-brand-900 dark:text-white mb-3">
              {t("stats.tagDistribution.title")}
            </h3>
            <div className="flex-1 min-h-0">
              <TagDistributionChart tags={stats.tag_distribution} />
            </div>
          </div>
        )}
      </div>

      {/* Row: Time-of-day distribution + Total trend (container-query 2-col) */}
      <div className="grid grid-cols-1 @[1024px]:grid-cols-12 gap-6">
        <div className="@[1024px]:col-span-5 glass-card bg-white dark:bg-brand-800 p-5 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700">
          <h3 className="text-base font-semibold text-brand-900 dark:text-white mb-3">
            {t("stats.timeOfDay.title")}
          </h3>
          <HourWeekDistribution
            hourly={stats.hourly_distribution}
            weekday={stats.weekday_distribution}
          />
        </div>
        <div className="@[1024px]:col-span-7 glass-card bg-white dark:bg-brand-800 p-5 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700 flex flex-col">
          <h3 className="text-base font-semibold text-brand-900 dark:text-white mb-3">
            {t("stats.charts.totalTrend")}
          </h3>
          <div className="flex-1 min-h-[18rem]">
            <DurationLineChart
              data={totalTrendData}
              hasPlayData={hasTotalTrendPlayData}
              yAxisTitle={t("stats.chartYAxis")}
              className="h-full"
            />
          </div>
        </div>
      </div>

      {/* Game Trend - full width */}
      <div className="glass-card bg-white dark:bg-brand-800 p-5 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700">
        <h3 className="text-base font-semibold text-brand-900 dark:text-white mb-3">
          {t("stats.charts.gameTrend")}
        </h3>
        <DurationLineChart
          data={gameTrendData}
          hasPlayData={hasGameTrendPlayData}
          yAxisTitle={t("stats.chartYAxis")}
          className="h-80"
        />
      </div>

      <StatsLeaderboardModal
        isOpen={showLeaderboardModal}
        games={modalLeaderboard}
        onClose={() => setShowLeaderboardModal(false)}
      />
    </>
  );
});

function StatsPage() {
  const { t } = useTranslation();
  const ref = useRef<HTMLDivElement>(null);
  const [dimension, setDimension] = useState<enums.Period>(enums.Period.WEEK);
  const [stats, setStats] = useState<vo.PeriodStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [showSkeleton, setShowSkeleton] = useState(false);
  const [aiLoading, setAiLoading] = useState(false);
  const [webSearchUsed, setWebSearchUsed] = useState(false);
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
  const aiSummaryCache = useAppStore(state => state.aiSummaryCache);
  const setAISummary = useAppStore(state => state.setAISummary);
  const aiSummary = aiSummaryCache[dimension] || "";

  const handleAISummarize = useCallback(async () => {
    setAiLoading(true);
    setWebSearchUsed(false);
    setAISummary(dimension, "");
    try {
      const result = await AISummarize({ dimension, spoiler_level: "" });
      setAISummary(dimension, result.summary);
      setWebSearchUsed(result.web_search_used ?? false);
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

  const loadStats = useCallback(
    async (dim: enums.Period, start?: string, end?: string) => {
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
    },
    [t],
  );

  const handleApplyDateRange = useCallback(() => {
    if (!startDate || !endDate) {
      toast.error(t("stats.toast.selectDateRange"));
      return;
    }
    if (new Date(startDate) >= new Date(endDate)) {
      toast.error(t("stats.toast.startBeforeEnd"));
      return;
    }
    setCustomDateRange(true);
    loadStats(enums.Period.WEEK, startDate, endDate);
  }, [endDate, loadStats, startDate, t]);

  const handleResetDateRange = useCallback(() => {
    setCustomDateRange(false);
    setStartDate("");
    setEndDate("");
    loadStats(dimension);
  }, [dimension, loadStats]);

  const handlePeriodChange = useCallback(
    (value: enums.Period) => {
      setDimension(value);
      if (customDateRange) {
        setCustomDateRange(false);
        setStartDate("");
        setEndDate("");
      }
    },
    [customDateRange],
  );

  const openTemplateModal = useCallback(() => {
    setShowTemplateModal(true);
  }, []);

  const closeTemplateModal = useCallback(() => {
    setShowTemplateModal(false);
  }, []);

  useEffect(() => {
    if (!customDateRange) {
      loadStats(dimension);
    }
  }, [customDateRange, dimension, loadStats]);

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

  return (
    <div
      id="stats-container"
      ref={ref}
      className="space-y-6 max-w-8xl mx-auto p-8"
    >
      <div className="flex items-center justify-between">
        <h1 className="text-4xl font-bold text-brand-900 dark:text-white">
          {t("stats.title")}
        </h1>
      </div>
      <StatsToolbar
        period={dimension}
        customRangeActive={customDateRange}
        startDate={startDate}
        endDate={endDate}
        loading={loading}
        aiLoading={aiLoading}
        onPeriodChange={handlePeriodChange}
        onStartDateChange={setStartDate}
        onEndDateChange={setEndDate}
        onApplyDateRange={handleApplyDateRange}
        onResetDateRange={handleResetDateRange}
        onExportReport={openTemplateModal}
        onAISummarize={handleAISummarize}
      />

      {/* AI Summary Card */}
      {(aiLoading || aiSummary) && (
        <AiSummaryCard
          aiSummary={aiSummary}
          aiLoading={aiLoading}
          webSearchUsed={webSearchUsed}
        />
      )}

      {stats && <StatsContent stats={stats} />}

      {/* Template Export Modal */}
      {stats && (
        <TemplateExportModal
          isOpen={showTemplateModal}
          onClose={closeTemplateModal}
          stats={stats}
          aiSummary={aiSummary}
        />
      )}
    </div>
  );
}
