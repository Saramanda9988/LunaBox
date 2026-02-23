/**
 * 格式化文件大小为 B、KB、MB
 * @param bytes
 */
export function formatFileSize(bytes: number): string {
  if (bytes <= 0)
    return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / k ** i).toFixed(1)} ${sizes[i]}`;
}
