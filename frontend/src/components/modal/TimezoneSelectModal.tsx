import { useState } from "react";
import { createPortal } from "react-dom";

interface TimezoneSelectModalProps {
  isOpen: boolean;
  onConfirm: (timezone: string) => void;
}

// 常见时区列表
const COMMON_TIMEZONES = [
  { value: "Asia/Shanghai", label: "中国标准时间 (UTC+8)" },
  { value: "Asia/Tokyo", label: "日本标准时间 (UTC+9)" },
  { value: "Asia/Seoul", label: "韩国标准时间 (UTC+9)" },
  { value: "Asia/Hong_Kong", label: "香港时间 (UTC+8)" },
  { value: "Asia/Taipei", label: "台北时间 (UTC+8)" },
  { value: "Asia/Singapore", label: "新加坡时间 (UTC+8)" },
  { value: "Asia/Bangkok", label: "曼谷时间 (UTC+7)" },
  { value: "Asia/Dubai", label: "迪拜时间 (UTC+4)" },
  { value: "Europe/London", label: "伦敦时间 (UTC+0)" },
  { value: "Europe/Paris", label: "巴黎时间 (UTC+1)" },
  { value: "Europe/Berlin", label: "柏林时间 (UTC+1)" },
  { value: "Europe/Moscow", label: "莫斯科时间 (UTC+3)" },
  { value: "America/New_York", label: "纽约时间 (UTC-5)" },
  { value: "America/Chicago", label: "芝加哥时间 (UTC-6)" },
  { value: "America/Denver", label: "丹佛时间 (UTC-7)" },
  { value: "America/Los_Angeles", label: "洛杉矶时间 (UTC-8)" },
  { value: "America/Sao_Paulo", label: "圣保罗时间 (UTC-3)" },
  { value: "Australia/Sydney", label: "悉尼时间 (UTC+10)" },
  { value: "Pacific/Auckland", label: "奥克兰时间 (UTC+12)" },
  { value: "UTC", label: "世界协调时间 (UTC)" },
];

export function TimezoneSelectModal({ isOpen, onConfirm }: TimezoneSelectModalProps) {
  // 尝试从浏览器获取当前时区
  const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const [selectedTimezone, setSelectedTimezone] = useState(browserTimezone || "Asia/Shanghai");

  if (!isOpen)
    return null;

  const handleConfirm = () => {
    onConfirm(selectedTimezone);
  };

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className="w-full max-w-lg rounded-xl bg-white p-6 shadow-xl dark:bg-brand-800 border border-brand-200 dark:border-brand-700">
        <div className="flex items-start gap-4 mb-6">
          <div className="p-2 rounded-full bg-primary-100 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400">
            <div className="i-mdi-earth text-2xl" />
          </div>
          <div className="flex-1">
            <h3 className="text-xl font-bold text-brand-900 dark:text-white mb-2">选择时区</h3>
            <p className="text-brand-600 dark:text-brand-400 text-sm leading-relaxed">
              检测到您尚未配置时区。为了正确统计游戏时长，请选择您所在的时区。
            </p>
          </div>
        </div>

        <div className="mb-6">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-2">
            当前检测到的时区:
            {" "}
            <span className="font-semibold text-primary-600 dark:text-primary-400">{browserTimezone || "未知"}</span>
          </label>
          <select
            value={selectedTimezone}
            onChange={e => setSelectedTimezone(e.target.value)}
            className="w-full px-4 py-2.5 bg-white dark:bg-brand-700 border border-brand-200 dark:border-brand-600 rounded-lg text-brand-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-transparent outline-none transition-all"
          >
            {COMMON_TIMEZONES.map(tz => (
              <option key={tz.value} value={tz.value}>
                {tz.label}
              </option>
            ))}
          </select>
          <p className="mt-2 text-xs text-brand-500 dark:text-brand-500">
            如果列表中没有您的时区，可以稍后在设置中手动配置
          </p>
        </div>

        <div className="flex justify-end gap-3">
          <button
            onClick={handleConfirm}
            className="px-6 py-2.5 text-sm font-medium text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors shadow-sm shadow-primary-200 dark:shadow-none"
          >
            确认并重启应用
          </button>
        </div>
      </div>
    </div>,
    document.body,
  );
}
