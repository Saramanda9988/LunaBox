import type { CSSProperties } from "react";
import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useChartTheme } from "../../hooks/useChartTheme";
import { formatDuration } from "../../utils/time";

interface HeatmapCell {
  date: string; // YYYY-MM-DD
  duration: number; // seconds
}

interface PlayHeatmapProps {
  cells: HeatmapCell[];
  className?: string;
}

interface ColumnCell {
  date: Date;
  iso: string;
  duration: number;
  weekday: number; // 0 = Sun .. 6 = Sat
  isPlaceholder?: boolean;
}

const WEEKDAY_LABEL_WIDTH = 22;
const LABEL_GUTTER = 4;
const MONTH_ROW_HEIGHT = 12;
const ROW_GAP = 4;
const MIN_CELL_SIZE = 6;
const MAX_CELL_SIZE = 14;

function parseISODate(iso: string): Date | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (!m)
    return null;
  return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
}

function bucketize(duration: number, p50: number, p90: number): number {
  if (duration <= 0)
    return 0;
  if (duration < Math.max(60, p50 * 0.4))
    return 1;
  if (duration < p50)
    return 2;
  if (duration < p90)
    return 3;
  return 4;
}

export function PlayHeatmap({ cells, className = "" }: PlayHeatmapProps) {
  const { t, i18n } = useTranslation();
  const { isDark } = useChartTheme();

  const containerRef = useRef<HTMLDivElement>(null);
  const [containerWidth, setContainerWidth] = useState(0);

  useEffect(() => {
    if (!containerRef.current)
      return;
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setContainerWidth(entry.contentRect.width);
      }
    });
    observer.observe(containerRef.current);
    setContainerWidth(containerRef.current.clientWidth);
    return () => observer.disconnect();
  }, []);

  const { columns, monthLabels, activeDays } = useMemo(() => {
    if (!cells.length) {
      return {
        columns: [] as ColumnCell[][],
        monthLabels: [] as { col: number; label: string }[],
        activeDays: 0,
      };
    }

    const valid = cells
      .map((c) => {
        const d = parseISODate(c.date);
        return d ? { date: d, iso: c.date, duration: c.duration ?? 0 } : null;
      })
      .filter(
        (v): v is { date: Date; iso: string; duration: number } => v !== null,
      )
      .sort((a, b) => a.date.getTime() - b.date.getTime());

    if (!valid.length) {
      return {
        columns: [] as ColumnCell[][],
        monthLabels: [] as { col: number; label: string }[],
        activeDays: 0,
      };
    }

    const first = valid[0].date;
    const gridStart = new Date(first);
    gridStart.setDate(first.getDate() - first.getDay());

    const last = valid[valid.length - 1].date;
    const gridEnd = new Date(last);
    gridEnd.setDate(last.getDate() + (6 - last.getDay()));

    const dataMap = new Map<string, number>();
    for (const v of valid) dataMap.set(v.iso, v.duration);

    const allCells: ColumnCell[] = [];
    const cursor = new Date(gridStart);
    while (cursor.getTime() <= gridEnd.getTime()) {
      const y = cursor.getFullYear();
      const m = String(cursor.getMonth() + 1).padStart(2, "0");
      const d = String(cursor.getDate()).padStart(2, "0");
      const iso = `${y}-${m}-${d}`;
      const duration = dataMap.get(iso) ?? 0;
      const isPlaceholder
        = cursor.getTime() < first.getTime() || cursor.getTime() > last.getTime();
      allCells.push({
        date: new Date(cursor),
        iso,
        duration,
        weekday: cursor.getDay(),
        isPlaceholder,
      });
      cursor.setDate(cursor.getDate() + 1);
    }

    const cols: ColumnCell[][] = [];
    for (let i = 0; i < allCells.length; i += 7) {
      cols.push(allCells.slice(i, i + 7));
    }

    const months: { col: number; label: string }[] = [];
    let lastMonth = -1;
    const monthFormatter = new Intl.DateTimeFormat(i18n.language, {
      month: "short",
    });
    cols.forEach((col, idx) => {
      const firstReal = col.find(c => !c.isPlaceholder);
      if (!firstReal)
        return;
      const m = firstReal.date.getMonth();
      if (m !== lastMonth) {
        months.push({
          col: idx,
          label: monthFormatter.format(firstReal.date),
        });
        lastMonth = m;
      }
    });

    let active = 0;
    for (const c of allCells) {
      if (c.isPlaceholder)
        continue;
      if (c.duration > 0)
        active += 1;
    }

    return {
      columns: cols,
      monthLabels: months,
      activeDays: active,
    };
  }, [cells, i18n.language]);

  // 固定像素布局：根据可用宽度反推 cell + gap，月份标签与列严格对齐
  const layout = useMemo(() => {
    if (!columns.length || containerWidth <= 0) {
      return { cellSize: 12, cellGap: 3, gridWidth: 0 };
    }
    const innerWidth = Math.max(
      0,
      containerWidth - WEEKDAY_LABEL_WIDTH - LABEL_GUTTER,
    );
    // step = cell + gap, gap ≈ cell/4 → step ≈ cell * 1.25
    let size = Math.floor(innerWidth / columns.length / 1.25);
    size = Math.max(MIN_CELL_SIZE, Math.min(MAX_CELL_SIZE, size));
    let gap = Math.max(2, Math.round(size / 4));
    const fits = (s: number, g: number) =>
      columns.length * s + (columns.length - 1) * g <= innerWidth;
    while (!fits(size, gap) && gap > 1) gap -= 1;
    while (!fits(size, gap) && size > MIN_CELL_SIZE) size -= 1;
    const gridWidth
      = columns.length * size + Math.max(0, columns.length - 1) * gap;
    return { cellSize: size, cellGap: gap, gridWidth };
  }, [columns.length, containerWidth]);

  const buckets = useMemo(() => {
    const active = cells
      .map(c => c.duration ?? 0)
      .filter(d => d > 0)
      .sort((a, b) => a - b);
    if (!active.length)
      return { p50: 0, p90: 0 };
    const p50 = active[Math.floor(active.length * 0.5)];
    const p90
      = active[Math.min(active.length - 1, Math.floor(active.length * 0.9))];
    return { p50, p90 };
  }, [cells]);

  const colorScale = isDark
    ? ["rgba(255,255,255,0.06)", "#0f3a2e", "#14563f", "#1f8a5c", "#34d399"]
    : ["rgba(15,23,42,0.06)", "#bbf7d0", "#86efac", "#34d399", "#059669"];

  const weekdayLabels = [
    t("stats.heatmap.weekdays.sun"),
    t("stats.heatmap.weekdays.mon"),
    t("stats.heatmap.weekdays.tue"),
    t("stats.heatmap.weekdays.wed"),
    t("stats.heatmap.weekdays.thu"),
    t("stats.heatmap.weekdays.fri"),
    t("stats.heatmap.weekdays.sat"),
  ];

  if (!columns.length) {
    return (
      <div
        ref={containerRef}
        className={`flex min-h-16 items-center justify-center text-xs text-brand-500 dark:text-brand-400 ${className}`}
      >
        {t("stats.heatmap.empty")}
      </div>
    );
  }

  const { cellSize, cellGap, gridWidth } = layout;
  const step = cellSize + cellGap;
  const totalWidth = WEEKDAY_LABEL_WIDTH + LABEL_GUTTER + gridWidth;

  return (
    <div ref={containerRef} className={`w-full ${className}`}>
      <div className="mx-auto" style={{ width: totalWidth || undefined }}>
        <div className="mb-1 flex items-baseline justify-between text-[11px] text-brand-600 dark:text-white/70">
          <span className="font-medium text-brand-700 dark:text-white/85">
            {t("stats.heatmap.summary", { days: activeDays })}
          </span>
        </div>

        <div className="flex flex-col" style={{ gap: ROW_GAP }}>
          {/* Month labels */}
          <div className="flex" style={{ height: MONTH_ROW_HEIGHT }}>
            <div
              style={{
                width: WEEKDAY_LABEL_WIDTH + LABEL_GUTTER,
                flexShrink: 0,
              }}
            />
            <div className="relative" style={{ width: gridWidth }}>
              {monthLabels.map((m, idx) => {
                const isLast = idx === monthLabels.length - 1;
                const remaining = columns.length - m.col;
                const useRightAnchor = isLast && remaining <= 3;
                const style: CSSProperties = useRightAnchor
                  ? { right: 0 }
                  : { left: m.col * step };
                return (
                  <span
                    key={`${m.col}-${m.label}`}
                    className="absolute whitespace-nowrap text-[10px] leading-none text-brand-500 dark:text-brand-400"
                    style={style}
                  >
                    {m.label}
                  </span>
                );
              })}
            </div>
          </div>

          {/* Main grid */}
          <div className="flex">
            <div
              className="flex flex-shrink-0 flex-col"
              style={{
                gap: cellGap,
                width: WEEKDAY_LABEL_WIDTH,
                marginRight: LABEL_GUTTER,
              }}
            >
              {weekdayLabels.map((w, idx) => (
                <span
                  key={w + idx}
                  className="pr-1 text-right text-[10px] leading-none text-brand-500 dark:text-brand-400"
                  style={{
                    height: cellSize,
                    lineHeight: `${cellSize}px`,
                    visibility: idx % 2 === 1 ? "visible" : "hidden",
                  }}
                >
                  {w}
                </span>
              ))}
            </div>

            <div className="flex" style={{ gap: cellGap, width: gridWidth }}>
              {columns.map((col, ci) => (
                <div
                  key={ci}
                  className="flex flex-col"
                  style={{ gap: cellGap, width: cellSize, flexShrink: 0 }}
                >
                  {col.map((cell, ri) => {
                    if (cell.isPlaceholder) {
                      return (
                        <div
                          key={ri}
                          style={{ width: cellSize, height: cellSize }}
                        />
                      );
                    }
                    const level = bucketize(
                      cell.duration,
                      buckets.p50,
                      buckets.p90,
                    );
                    return (
                      <div
                        key={ri}
                        title={`${cell.iso} · ${cell.duration > 0 ? formatDuration(cell.duration, t) : t("stats.heatmap.noPlay")}`}
                        className="rounded-[2px] transition-transform hover:scale-125"
                        style={{
                          width: cellSize,
                          height: cellSize,
                          backgroundColor: colorScale[level],
                        }}
                      />
                    );
                  })}
                </div>
              ))}
            </div>
          </div>

          {/* Legend */}
          <div className="flex items-center justify-end gap-1 text-[10px] text-brand-500 dark:text-brand-400">
            <span>{t("stats.heatmap.less")}</span>
            {colorScale.map((c, idx) => (
              <span
                key={idx}
                className="rounded-[2px]"
                style={{
                  width: cellSize,
                  height: cellSize,
                  backgroundColor: c,
                }}
              />
            ))}
            <span>{t("stats.heatmap.more")}</span>
          </div>
        </div>
      </div>
    </div>
  );
}

export default PlayHeatmap;
