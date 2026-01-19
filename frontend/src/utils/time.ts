/**
 * 将秒数格式化为中文时间字符串 (X小时Y分钟)
 */
export const formatDuration = (seconds: number): string => {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (hours > 0) {
    return minutes > 0 ? `${hours}小时${minutes}分钟` : `${hours}小时`
  }
  return `${minutes}分钟`
}

/**
 * 将秒数格式化为简短时间字符串 (Xh Ym)
 * TODO: i18n
 */
export const formatDurationShort = (seconds: number): string => {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  return `${hours}h ${minutes}m`
}

/**
 * 将秒数转换为小时数（保留1位小数）
 */
export const formatDurationHours = (seconds: number): number => {
  return Number((seconds / 3600).toFixed(1))
}

/**
 * 解析后端返回的时间字符串为 Date 对象
 * 
 * 后端存储的是本地时间（TIMESTAMP 不带时区），返回格式如 "2025-01-19T01:00:00"
 * JavaScript 会将不带时区后缀的时间字符串解析为本地时间，这正是我们期望的行为
 * 
 * @param timeString - 后端返回的时间（可以是字符串、Date对象）
 * @returns Date 对象（本地时间）
 */
export const parseTime = (timeString: any): Date => {
  if (!timeString) return new Date()
  
  // 如果已经是 Date 对象，直接返回
  if (timeString instanceof Date) {
    return timeString
  }
  
  // 将任何类型转换为字符串
  const timeStr = String(timeString)
  
  // 如果字符串以 Z 结尾（UTC 时间），去掉 Z 作为本地时间处理
  // 这是为了兼容历史数据（如果有的话）
  const localStr = timeStr.endsWith('Z') 
    ? timeStr.slice(0, -1)
    : timeStr.replace(' ', 'T')
  
  return new Date(localStr)
}

/**
 * @deprecated 使用 parseTime 替代
 * 保留此函数以兼容旧代码
 */
export const parseUTCTime = parseTime

/**
 * 格式化时间为本地日期字符串
 * @param timeString - 时间（支持 string、Date）
 * @param options - Intl.DateTimeFormat 选项
 */
export const formatLocalDate = (
  timeString: any,
  options?: Intl.DateTimeFormatOptions
): string => {
  const date = parseTime(timeString)
  return date.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    ...options,
  })
}

/**
 * 格式化时间为本地日期时间字符串
 * @param timeString - 时间（支持 string、Date）
 * @param options - Intl.DateTimeFormat 选项
 */
export const formatLocalDateTime = (
  timeString: any,
  options?: Intl.DateTimeFormatOptions
): string => {
  const date = parseTime(timeString)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
    ...options,
  })
}

/**
 * 格式化时间为本地时间字符串（仅时分秒）
 * @param timeString - 时间（支持 string、Date）
 */
export const formatLocalTime = (timeString: any): string => {
  const date = parseTime(timeString)
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  })
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
export const formatDateToYYYYMMDD = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
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
export const toLocalISOString = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')
  
  // 获取本地时区偏移（分钟），并转换为 ±HH:MM 格式
  const tzOffset = -date.getTimezoneOffset() // 注意：getTimezoneOffset 返回的是 UTC - 本地，所以要取反
  const tzSign = tzOffset >= 0 ? '+' : '-'
  const tzHours = String(Math.floor(Math.abs(tzOffset) / 60)).padStart(2, '0')
  const tzMinutes = String(Math.abs(tzOffset) % 60).padStart(2, '0')
  
  return `${year}-${month}-${day}T${hours}:${minutes}:${seconds}${tzSign}${tzHours}:${tzMinutes}`
}
