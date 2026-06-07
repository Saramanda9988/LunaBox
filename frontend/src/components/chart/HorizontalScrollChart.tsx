import type { ChartData, ChartOptions } from "chart.js";
import type { CSSProperties } from "react";
import { useEffect, useRef, useState } from "react";
import { Line } from "react-chartjs-2";

interface HorizontalScrollChartProps {
  data: ChartData<"line">;
  options?: ChartOptions<"line">;
  className?: string; // Wrapper class for height, e.g. "h-80" or "h-96"
  minPointWidth?: number; // Minimum width per data point before horizontal scrolling starts
}

export function HorizontalScrollChart({
  data,
  options,
  className = "h-80",
  minPointWidth = 20,
}: HorizontalScrollChartProps) {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const [containerWidth, setContainerWidth] = useState(0);
  const dataPointCount
    = data.labels?.length
      || Math.max(0, ...data.datasets.map(dataset => dataset.data.length));
  const minChartWidth = dataPointCount > 1 ? dataPointCount * minPointWidth : 0;
  const chartWidth
    = containerWidth > 0 ? Math.max(containerWidth, minChartWidth) : undefined;
  const isScrollable = containerWidth > 0 && minChartWidth > containerWidth;

  useEffect(() => {
    const wrapper = wrapperRef.current;
    if (!wrapper)
      return;

    const updateContainerWidth = () => {
      setContainerWidth(wrapper.getBoundingClientRect().width);
    };

    updateContainerWidth();
    const resizeObserver = new ResizeObserver(updateContainerWidth);
    resizeObserver.observe(wrapper);

    return () => resizeObserver.disconnect();
  }, []);

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
  const xScale = options?.scales?.x as { type?: string } | undefined;
  const chartKey = `${xScale?.type ?? "category"}-${
    data.labels ? "labels" : "points"
  }`;

  const containerStyle: CSSProperties = {
    height: "100%",
    minWidth: "100%",
    width: chartWidth ? `${chartWidth}px` : "100%",
  };

  return (
    <div ref={wrapperRef} className={`w-full ${className}`}>
      <div
        className={`h-full w-full overflow-y-hidden scrollbar-thin scrollbar-thumb-brand-200 dark:scrollbar-thumb-brand-700 scrollbar-track-transparent ${
          isScrollable ? "overflow-x-auto" : "overflow-x-hidden"
        }`}
      >
        <div className="relative" style={containerStyle}>
          <Line key={chartKey} options={mergedOptions} data={data} />
        </div>
      </div>
    </div>
  );
}
