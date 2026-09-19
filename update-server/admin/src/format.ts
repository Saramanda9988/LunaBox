import type { EventType } from "./types";

const numberFormatter = new Intl.NumberFormat("zh-CN");
const dateTimeFormatter = new Intl.DateTimeFormat("zh-CN", {
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hour12: false,
});

export const eventLabels: Record<EventType, string> = {
  update_available: "发现更新",
  download_started: "开始下载",
  download_verified: "校验完成",
  install_success: "安装成功",
  install_failed: "安装失败",
};

export function formatNumber(value: number | null | undefined): string {
  return numberFormatter.format(Number(value ?? 0));
}

export function formatBytes(value: number | null | undefined): string {
  let size = Number(value ?? 0);
  if (size < 1024)
    return `${formatNumber(size)} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let unit = -1;
  do {
    size /= 1024;
    unit += 1;
  } while (size >= 1024 && unit < units.length - 1);
  return `${size.toLocaleString("zh-CN", { maximumFractionDigits: size >= 100 ? 0 : 1 })} ${units[unit]}`;
}

export function formatDateTime(value: string | null | undefined): string {
  if (!value)
    return "—";
  const normalized = value.includes("T") ? value : `${value.replace(" ", "T")}Z`;
  const date = new Date(normalized);
  return Number.isNaN(date.getTime()) ? value : dateTimeFormatter.format(date);
}

export function successRate(success: number, failed: number): number {
  const finished = success + failed;
  return finished > 0 ? Math.round(success / finished * 1000) / 10 : 0;
}

export function shortID(value: string | null): string {
  return value ? value.slice(0, 8) : "—";
}
