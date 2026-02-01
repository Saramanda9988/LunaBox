import type { ChartData, ChartOptions } from "chart.js";
import { Line } from "react-chartjs-2";

interface HorizontalScrollChartProps {
  data: ChartData<"line">;
  options?: ChartOptions<"line">;
  className?: string; // Wrapper class for height, e.g. "h-80" or "h-96"
  threshold?: number; // Number of data points before scrolling is enabled
  itemWidth?: number; // Width per data point in pixels
}

export function HorizontalScrollChart({
  data,
  options,
  className = "h-80",
  threshold = 31,
  itemWidth = 40,
}: HorizontalScrollChartProps) {
  const dataPointCount = data.labels?.length || 0;
  const isScrollable = dataPointCount > threshold;

  // Ensure tooltips behave correctly (snap to nearest x-axis point)
  const defaultInteraction = {
    mode: "index" as const,
    intersect: false,
  };

  const mergedOptions: ChartOptions<"line"> = {
    ...options,
    maintainAspectRatio: false,
    responsive: true,
    interaction: options?.interaction || defaultInteraction,
  };

  const containerStyle: React.CSSProperties = {
    minWidth: "100%",
    width: isScrollable ? `${dataPointCount * itemWidth}px` : "100%",
    height: "100%",
  };

  return (
    <div className={`w-full ${className}`}>
      <div className="h-full w-full overflow-x-auto overflow-y-hidden scrollbar-thin scrollbar-thumb-brand-200 dark:scrollbar-thumb-brand-700 scrollbar-track-transparent">
        <div style={containerStyle}>
          <Line options={mergedOptions} data={data} />
        </div>
      </div>
    </div>
  );
}
