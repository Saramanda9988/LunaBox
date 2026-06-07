import type { ChartData, ChartOptions, TooltipItem } from "chart.js";
import {
  CategoryScale,
  Chart as ChartJS,
  Legend,
  LinearScale,
  LineElement,
  PointElement,
  TimeScale,
  Title,
  Tooltip,
} from "chart.js";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useChartTheme } from "../../hooks/useChartTheme";
import {
  formatDuration,
  formatDurationChart,
  parseDateOnlyToLocalTimestamp,
} from "../../utils/time";
import { HorizontalScrollChart } from "./HorizontalScrollChart";
import "chartjs-adapter-date-fns";

ChartJS.register(
  CategoryScale,
  LinearScale,
  TimeScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
);

interface DurationLineChartProps {
  data: ChartData<"line">;
  hasPlayData: boolean;
  className?: string;
  minPointWidth?: number;
  scaleType?: "category" | "time";
  showLegend?: boolean;
  timeRange?: {
    start?: string;
    end?: string;
  };
  yAxisTitle?: string;
}

const DAY_MS = 24 * 60 * 60 * 1000;

function parseLocalDateValue(dateString?: string) {
  return parseDateOnlyToLocalTimestamp(dateString);
}

function createPaddedTimeRange(
  timeRange?: DurationLineChartProps["timeRange"],
) {
  const start = parseLocalDateValue(timeRange?.start);
  const end = parseLocalDateValue(timeRange?.end);

  if (start === undefined || end === undefined)
    return undefined;

  if (start === end) {
    return {
      min: start - 7 * DAY_MS,
      max: end + 7 * DAY_MS,
    };
  }

  const span = Math.max(DAY_MS, end - start);
  const padding = Math.min(14 * DAY_MS, Math.max(DAY_MS, span * 0.04));
  return {
    min: start - padding,
    max: end + padding,
  };
}

export function DurationLineChart({
  data,
  hasPlayData,
  className = "h-80",
  minPointWidth,
  scaleType = "category",
  showLegend = true,
  timeRange,
  yAxisTitle,
}: DurationLineChartProps) {
  const { t } = useTranslation();
  const { textColor, gridColor } = useChartTheme();
  const paddedTimeRange = useMemo(
    () => createPaddedTimeRange(timeRange),
    [timeRange],
  );

  const options = useMemo<ChartOptions<"line">>(
    () => ({
      responsive: true,
      maintainAspectRatio: false,
      resizeDelay: 100,
      interaction: {
        mode: "index" as const,
        intersect: false,
      },
      plugins: {
        legend: {
          display: showLegend,
          position: "top" as const,
          labels: {
            color: textColor,
          },
        },
        title: {
          display: false,
        },
        tooltip: {
          callbacks: {
            label: (context: TooltipItem<"line">) => {
              const label = context.dataset.label
                ? `${context.dataset.label}: `
                : "";
              return `${label}${formatDuration(Number(context.parsed.y || 0), t)}`;
            },
          },
        },
      },
      scales: {
        x:
          scaleType === "time"
            ? {
                type: "time",
                min: paddedTimeRange?.min,
                max: paddedTimeRange?.max,
                time: {
                  minUnit: "day",
                  tooltipFormat: "yyyy-MM-dd",
                  displayFormats: {
                    day: "yyyy-MM-dd",
                    month: "yyyy-MM",
                    year: "yyyy",
                  },
                },
                grid: {
                  color: gridColor,
                },
                ticks: {
                  autoSkip: true,
                  autoSkipPadding: 12,
                  color: textColor,
                  maxRotation: 45,
                  minRotation: 0,
                },
              }
            : {
                type: "category",
                bounds: "ticks",
                grid: {
                  color: gridColor,
                },
                ticks: {
                  autoSkip: true,
                  autoSkipPadding: 12,
                  color: textColor,
                  maxRotation: 45,
                  minRotation: 0,
                },
              },
        y: {
          beginAtZero: true,
          max: hasPlayData ? undefined : 600,
          title: {
            display: !!yAxisTitle,
            text: yAxisTitle,
            color: textColor,
          },
          grid: {
            color: gridColor,
          },
          ticks: {
            color: textColor,
            stepSize: hasPlayData ? undefined : 60,
            callback: value => formatDurationChart(Number(value), t),
          },
        },
      },
    }),
    [
      gridColor,
      hasPlayData,
      paddedTimeRange,
      scaleType,
      showLegend,
      t,
      textColor,
      yAxisTitle,
    ],
  );

  return (
    <HorizontalScrollChart
      data={data}
      options={options}
      className={className}
      minPointWidth={minPointWidth}
    />
  );
}
