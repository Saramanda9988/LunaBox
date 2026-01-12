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
 * 将 UTC 时间字符串转换为本地时间的 Date 对象
 * 后端存储的是 UTC 时间（无时区信息），需要显式处理为本地时间
 * 
 * @param utcTimeString - 后端返回的时间（可以是字符串、Date对象或time.Time）
 * @returns Date 对象（本地时间）
 */
export const parseUTCTime = (utcTimeString: any): Date => {
  if (!utcTimeString) return new Date()
  
  // 如果已经是 Date 对象，直接返回
  if (utcTimeString instanceof Date) {
    return utcTimeString
  }
  
  // 将任何类型转换为字符串
  const timeStr = String(utcTimeString)
  
  // 确保时间字符串以 'Z' 结尾，表示 UTC 时间
  const utcStr = timeStr.includes('Z') 
    ? timeStr 
    : `${timeStr.replace(' ', 'T')}Z`
  
  return new Date(utcStr)
}

/**
 * 格式化时间为本地日期字符串
 * @param utcTimeString - UTC 时间（支持 string、Date、time.Time）
 * @param options - Intl.DateTimeFormat 选项
 */
export const formatLocalDate = (
  utcTimeString: any,
  options?: Intl.DateTimeFormatOptions
): string => {
  const date = parseUTCTime(utcTimeString)
  return date.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    ...options,
  })
}

/**
 * 格式化时间为本地日期时间字符串
 * @param utcTimeString - UTC 时间（支持 string、Date、time.Time）
 * @param options - Intl.DateTimeFormat 选项
 */
export const formatLocalDateTime = (
  utcTimeString: any,
  options?: Intl.DateTimeFormatOptions
): string => {
  const date = parseUTCTime(utcTimeString)
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
 * @param utcTimeString - UTC 时间（支持 string、Date、time.Time）
 */
export const formatLocalTime = (utcTimeString: any): string => {
  const date = parseUTCTime(utcTimeString)
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  })
}
