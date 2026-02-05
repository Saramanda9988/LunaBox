/**
 * 将秒数格式化为中文时间字符串 (X小时Y分钟)
 */
export function formatDuration(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (hours > 0) {
    return minutes > 0 ? `${hours}小时${minutes}分钟` : `${hours}小时`;
  }
  return `${minutes}分钟`;
}

/**
 * 将秒数格式化为简短时间字符串 (Xh Ym)
 * TODO: i18n
 */
export function formatDurationShort(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  return `${hours}h ${minutes}m`;
}

/**
 * 将秒数转换为小时数（保留1位小数）
 */
export function formatDurationHours(seconds: number): number {
  return Number((seconds / 3600).toFixed(1));
}

/**
 * 解析后端返回的时间为 Date 对象
 *
 * 数据流：
 * 1. Go time.Now() 获取本地时间（如 2026-02-05 13:33:00 +0800）
 * 2. Go DuckDB 驱动将其转换为 UTC epoch 微秒数存储（如 1738732380000000）
 * 3. DuckDB 将此微秒数存储在 TIMESTAMPTZ 列中
 * 4. 查询时，DuckDB 根据配置的时区（SET TimeZone）解释此微秒数：
 *    - 按日期聚合时（start_time::DATE）会转换为本地日期
 *    - 返回字符串时格式化为 RFC3339："2026-02-05T05:33:00Z"
 * 5. Wails 将字符串传递给前端
 * 6. 前端解析后，toLocaleString() 自动转换为用户本地时区显示
 *
 * 关键：TIMESTAMPTZ 存储的是 UTC 时间戳（INT64 微秒数），但查询时会使用
 * 数据库配置的时区进行日期聚合，确保统计数据按用户本地日期计算。
 *
 * @param timeValue - 后端返回的时间（RFC3339 字符串或 Date 对象）
 * @returns Date 对象
 */
export function parseTime(timeValue: any): Date {
  if (!timeValue)
    return new Date();

  // 如果已经是 Date 对象，直接返回
  if (timeValue instanceof Date) {
    return timeValue;
  }

  // 处理 Wails 的 time.Time 对象
  // 虽然 TypeScript 类型声明是对象，但实际序列化后就是 RFC3339 字符串
  let timeStr: string;
  if (typeof timeValue === "object") {
    timeStr = String(timeValue);
    if (timeStr === "[object Object]") {
      console.warn("parseTime: received empty time.Time object, returning current time");
      return new Date();
    }
  }
  else {
    timeStr = String(timeValue);
  }

  // 空字符串返回当前时间
  if (!timeStr) {
    return new Date();
  }

  // 将空格替换为 T（兼容 "2026-01-25 14:25:46" 格式）
  timeStr = timeStr.replace(" ", "T");

  // 直接解析 RFC3339 格式，JavaScript Date 会正确处理 UTC 时间（以 Z 结尾）
  // 例如 "2026-02-01T05:33:00Z" 会被解析为 UTC 时间，
  // 在格式化时（toLocaleString 等）会自动转换为本地时区显示
  return new Date(timeStr);
}

/**
 * @deprecated 使用 parseTime 替代
 * 保留此函数以兼容旧代码
 */
export const parseUTCTime = parseTime;

/**
 * 格式化时间为本地日期字符串
 * @param timeString - 时间（支持 string、Date）
 * @param options - Intl.DateTimeFormat 选项
 */
export function formatLocalDate(timeString: any, options?: Intl.DateTimeFormatOptions): string {
  const date = parseTime(timeString);
  return date.toLocaleDateString(undefined, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    ...options,
  });
}

/**
 * 格式化时间为本地日期时间字符串
 * @param timeString - 时间（支持 string、Date）
 * @param options - Intl.DateTimeFormat 选项
 */
export function formatLocalDateTime(timeString: any, options?: Intl.DateTimeFormatOptions): string {
  const date = parseTime(timeString);
  return date.toLocaleString(undefined, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
    ...options,
  });
}

/**
 * 格式化时间为本地时间字符串（仅时分秒）
 * @param timeString - 时间（支持 string、Date）
 */
export function formatLocalTime(timeString: any): string {
  const date = parseTime(timeString);
  return date.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

/**
 * 将 Date 对象格式化为 YYYY-MM-DD 格式（用于日期输入框或后端传参）
 *
 * @param date - Date 对象
 * @returns 格式化的日期字符串，如 "2026-01-19"
 *
 * @example
 * formatDateToYYYYMMDD(new Date()) // "2026-01-19"
 */
export function formatDateToYYYYMMDD(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

/**
 * 将 Date 对象格式化为带本地时区的 ISO 格式字符串（用于传递给后端）
 * 格式: YYYY-MM-DDTHH:mm:ss+HH:MM（RFC3339 格式，Go time.Time 可解析）
 *
 * 注意: 与 toISOString() 不同，这个函数返回的是本地时间带本地时区偏移
 * 后端会将此字符串解析为对应的本地时间
 *
 * @param date - Date 对象
 * @returns RFC3339 格式的时间字符串，如 "2026-01-19T14:30:00+08:00"
 *
 * @example
 * toLocalISOString(new Date()) // "2026-01-19T14:30:00+08:00"
 */
export function toLocalISOString(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  const seconds = String(date.getSeconds()).padStart(2, "0");

  // 获取本地时区偏移（分钟），并转换为 ±HH:MM 格式
  const tzOffset = -date.getTimezoneOffset(); // 注意：getTimezoneOffset 返回的是 UTC - 本地，所以要取反
  const tzSign = tzOffset >= 0 ? "+" : "-";
  const tzHours = String(Math.floor(Math.abs(tzOffset) / 60)).padStart(2, "0");
  const tzMinutes = String(Math.abs(tzOffset) % 60).padStart(2, "0");

  return `${year}-${month}-${day}T${hours}:${minutes}:${seconds}${tzSign}${tzHours}:${tzMinutes}`;
}
