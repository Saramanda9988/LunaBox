import type { appconf } from "../../../wailsjs/go/models";
import { BetterSelect } from "../ui/BetterSelect";
import { BetterSwitch } from "../ui/BetterSwitch";

interface BetterSelectOption {
  value: string;
  label: string;
}

const COMMON_TIMEZONES: BetterSelectOption[] = [
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

interface BasicSettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

export function BasicSettingsPanel({ formData, onChange }: BasicSettingsProps) {
  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value, type } = e.target;
    const newValue = type === "checkbox" ? (e.target as HTMLInputElement).checked : value;
    onChange({ ...formData, [name]: newValue } as appconf.AppConfig);
  };

  return (
    <>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">Bangumi Access Token</label>
        <input
          type="text"
          name="access_token"
          value={formData.access_token || ""}
          onChange={handleChange}
          className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">如果您想使用Bangumi数据源，请一定填写</p>
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">VNDB Access Token</label>
        <input
          type="text"
          name="vndb_access_token"
          value={formData.vndb_access_token || ""}
          onChange={handleChange}
          className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
        />
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">主题</label>
        <BetterSelect
          name="theme"
          value={formData.theme}
          onChange={value => onChange({ ...formData, theme: value } as appconf.AppConfig)}
          options={[
            { value: "light", label: "浅色" },
            { value: "dark", label: "深色" },
            { value: "system", label: "跟随系统" },
          ]}
        />
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">语言</label>
        <BetterSelect
          name="language"
          value={formData.language}
          onChange={value => onChange({ ...formData, language: value } as appconf.AppConfig)}
          options={[
            { value: "zh-CN", label: "简体中文" },
            { value: "en-US", label: "English" },
          ]}
        />
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">时区</label>
        <span className="text-xs text-brand-500 dark:text-brand-400">用于正确显示和记录游戏时长,修改后请重启应用</span>
        <BetterSelect
          name="timezone"
          value={formData.time_zone || "Asia/Shanghai"}
          onChange={value => onChange({ ...formData, time_zone: value } as appconf.AppConfig)}
          options={COMMON_TIMEZONES}
          placeholder="请选择时区"
        />
      </div>

      <div className="flex items-center justify-between p-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          关闭窗口时最小化到系统托盘
        </label>
        <BetterSwitch
          id="close_to_tray"
          checked={formData.close_to_tray || false}
          onCheckedChange={checked => onChange({ ...formData, close_to_tray: checked } as appconf.AppConfig)}
        />
      </div>
    </>
  );
}
