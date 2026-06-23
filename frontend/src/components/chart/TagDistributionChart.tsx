import type { ChartData, ChartOptions, TooltipItem } from "chart.js";
import {
  ArcElement,
  Chart as ChartJS,
  Legend,
  RadialLinearScale,
  Tooltip,
} from "chart.js";
import { useMemo } from "react";
import { PolarArea } from "react-chartjs-2";
import { useTranslation } from "react-i18next";
import { useChartTheme } from "../../hooks/useChartTheme";
import { formatDuration } from "../../utils/time";

ChartJS.register(ArcElement, RadialLinearScale, Tooltip, Legend);

interface TagPlayStats {
  name: string;
  total_duration: number;
  game_count: number;
}

interface TagDistributionChartProps {
  tags: TagPlayStats[];
  className?: string;
}

const TAG_COLORS = [
  "#f97316", // orange-500
  "#ef4444", // red-500
  "#ec4899", // pink-500
  "#a855f7", // purple-500
  "#6366f1", // indigo-500
  "#3b82f6", // blue-500
  "#06b6d4", // cyan-500
  "#10b981", // emerald-500
  "#84cc16", // lime-500
  "#eab308", // yellow-500
];

export function TagDistributionChart({
  tags,
  className = "",
}: TagDistributionChartProps) {
  const { t } = useTranslation();
  const { isDark } = useChartTheme();

  const totalDuration = useMemo(
    () => tags.reduce((sum, tag) => sum + tag.total_duration, 0),
    [tags],
  );

  const chartData: ChartData<"polarArea"> = useMemo(
    () => ({
      labels: tags.map(t => t.name),
      datasets: [
        {
          data: tags.map(t => t.total_duration),
          backgroundColor: tags.map(
            (_, i) => TAG_COLORS[i % TAG_COLORS.length],
          ),
          borderColor: isDark ? "#1f2937" : "#ffffff",
          borderWidth: 2,
        },
      ],
    }),
    [tags, isDark],
  );

  const options = useMemo<ChartOptions<"polarArea">>(
    () => ({
      responsive: true,
      maintainAspectRatio: false,
      layout: {
        padding: 0,
      },
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            label: (ctx: TooltipItem<"polarArea">) => {
              const value = Number(ctx.parsed.r || 0);
              const pct = totalDuration
                ? ((value / totalDuration) * 100).toFixed(1)
                : "0.0";
              return `${ctx.label}: ${formatDuration(value, t)} (${pct}%)`;
            },
          },
        },
      },
      scales: {
        r: {
          beginAtZero: true,
          display: false,
          ticks: { display: false },
        },
      },
    }),
    [t, totalDuration],
  );

  if (!tags.length) {
    return (
      <div
        className={`flex items-center justify-center text-sm text-brand-500 dark:text-brand-400 ${className}`}
      >
        {t("stats.tagDistribution.empty")}
      </div>
    );
  }

  return (
    <div className={`flex flex-col gap-5 ${className}`}>
      {/* Polar area - 居中置顶，固定尺寸保证可读 */}
      <div className="flex justify-center">
        <div className="relative aspect-square w-[200px] max-w-full">
          <PolarArea data={chartData} options={options} />
        </div>
      </div>

      {/* Top tags list - 两列网格 */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-2">
        {tags.map((tag, idx) => {
          const pct = totalDuration
            ? (tag.total_duration / totalDuration) * 100
            : 0;
          const color = TAG_COLORS[idx % TAG_COLORS.length];
          return (
            <div key={tag.name} className="flex items-center gap-3">
              <span className="w-6 text-xs text-brand-500 dark:text-brand-400 font-medium tabular-nums text-right flex-shrink-0">
                #
                {idx + 1}
              </span>
              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between gap-2 mb-1">
                  <span
                    className="text-sm font-medium text-brand-900 dark:text-white truncate"
                    title={tag.name}
                  >
                    {tag.name}
                  </span>
                  <span
                    className="text-xs font-mono text-brand-500 dark:text-brand-400 flex-shrink-0 whitespace-nowrap"
                    title={`${pct.toFixed(1)}%`}
                  >
                    {pct.toFixed(1)}
                    %
                  </span>
                </div>
                <div className="h-1.5 rounded-full bg-brand-100 dark:bg-brand-700 overflow-hidden">
                  <div
                    className="h-full rounded-full transition-all"
                    style={{
                      width: `${Math.max(2, pct)}%`,
                      backgroundColor: color,
                    }}
                  />
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export default TagDistributionChart;
