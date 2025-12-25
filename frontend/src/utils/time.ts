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
