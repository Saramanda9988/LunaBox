import { Empty, Flex, Skeleton } from "antd";
import { lazy, Suspense } from "react";

const Column = lazy(async () => {
  const module = await import("@ant-design/plots");
  return { default: module.Column };
});

interface TrendChartProps {
  rows: Array<{ date: string; count: number }>;
}

export function TrendChart({ rows }: TrendChartProps) {
  const byDate = new Map(rows.map(row => [row.date, row.count]));
  const points = Array.from({ length: 30 }, (_, index) => {
    const date = new Date();
    date.setHours(12, 0, 0, 0);
    date.setDate(date.getDate() - (29 - index));
    const key = [
      date.getFullYear(),
      String(date.getMonth() + 1).padStart(2, "0"),
      String(date.getDate()).padStart(2, "0"),
    ].join("-");
    return { date: key, count: byDate.get(key) ?? 0 };
  });
  const total = points.reduce((sum, point) => sum + point.count, 0);

  if (total === 0) {
    return (
      <Flex className="chart-empty" align="center" justify="center">
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="近 30 天暂无成功更新" />
      </Flex>
    );
  }

  return (
    <Suspense fallback={<Skeleton.Node active className="chart-skeleton" />}>
      <Column
        data={points}
        xField="date"
        yField="count"
        height={280}
        axis={{
          x: { title: false, labelFormatter: (value: string) => value.slice(5) },
          y: { title: false },
        }}
        tooltip={{ title: (value: string) => value }}
        style={{ radiusTopLeft: 4, radiusTopRight: 4 }}
      />
    </Suspense>
  );
}
