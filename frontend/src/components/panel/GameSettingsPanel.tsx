import type { appconf } from "../../../wailsjs/go/models";
import { toast } from "react-hot-toast";
import { SelectGameExecutable } from "../../../wailsjs/go/service/GameService";
import { BetterButton } from "../ui/BetterButton";
import { BetterSwitch } from "../ui/BetterSwitch";

interface GameSettingsPanelProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

export function GameSettingsPanel({ formData, onChange }: GameSettingsPanelProps) {
  const handleSelectLocaleEmulatorPath = async () => {
    try {
      const path = await SelectGameExecutable();
      if (path) {
        onChange({ ...formData, locale_emulator_path: path } as appconf.AppConfig);
      }
    }
    catch (error) {
      console.error("Failed to select Locale Emulator:", error);
      toast.error("选择 Locale Emulator 失败");
    }
  };

  const handleSelectMagpiePath = async () => {
    try {
      const path = await SelectGameExecutable();
      if (path) {
        onChange({ ...formData, magpie_path: path } as appconf.AppConfig);
      }
    }
    catch (error) {
      console.error("Failed to select Magpie:", error);
      toast.error("选择 Magpie 失败");
    }
  };
  return (
    <>
      <div className="flex items-center justify-between p-2">
        <div className="flex-1">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            仅记录活跃游玩时长
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
            启用后，仅当游戏窗口处于前台焦点时才会记录游玩时间。这可以更准确地统计实际游玩时长，排除挂机和后台运行时间。
          </p>
        </div>
        <BetterSwitch
          id="record_active_time_only"
          checked={formData.record_active_time_only || false}
          onCheckedChange={checked =>
            onChange({ ...formData, record_active_time_only: checked } as appconf.AppConfig)}
        />
      </div>

      <div className="mt-4 p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700 rounded-lg">
        <div className="flex items-start gap-2">
          <span className="i-mdi-alert text-amber-600 dark:text-amber-400 text-lg mt-0.5" />
          <div className="text-xs text-amber-700 dark:text-amber-300">
            <p className="font-medium mb-1">注意：</p>
            <ul className="list-disc list-inside space-y-1 ml-2">
              <li>开启此选项将记录游戏进程的非完整运行时间（不包括后台时间）</li>
              <li>已有的游玩记录不会受到此设置影响</li>
              <li>某些全屏游戏可能影响焦点检测的准确性</li>
            </ul>
          </div>
        </div>
      </div>

      {/* 自动进程检测 */}
      <div className="mt-6 border-t border-brand-200 dark:border-brand-700 pt-6">
        <div className="flex items-center justify-between p-2">
          <div className="flex-1">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              自动进程检测
            </label>
            <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
              启用后，当启动器进程快速退出时，LunaBox 会自动提示您选择实际的游戏进程。
              禁用后，将直接使用已保存的进程名或可执行程序句柄进行监控，适合单exe游戏或不需要复杂检测的场景。
            </p>
          </div>
          <BetterSwitch
            id="auto_detect_game_process"
            checked={formData.auto_detect_game_process ?? true}
            onCheckedChange={checked =>
              onChange({ ...formData, auto_detect_game_process: checked } as appconf.AppConfig)}
          />
        </div>
      </div>

      {/* Locale Emulator 配置 */}
      <div className="mt-6 border-t border-brand-200 dark:border-brand-700 pt-6">
        <h3 className="text-sm font-semibold text-brand-900 dark:text-white mb-4">游戏启动工具</h3>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-2">
              Locale Emulator 路径
            </label>
            <div className="flex gap-2">
              <input
                type="text"
                value={formData.locale_emulator_path || ""}
                onChange={e => onChange({ ...formData, locale_emulator_path: e.target.value } as appconf.AppConfig)}
                placeholder="选择 LEProc.exe 文件路径"
                className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
              />
              <BetterButton onClick={handleSelectLocaleEmulatorPath} icon="i-mdi-file">
                选择
              </BetterButton>
            </div>
            <p className="mt-1 text-xs text-brand-500">
              设置后可在游戏编辑界面启用 Locale Emulator 转区启动
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-2">
              Magpie 路径
            </label>
            <div className="flex gap-2">
              <input
                type="text"
                value={formData.magpie_path || ""}
                onChange={e => onChange({ ...formData, magpie_path: e.target.value } as appconf.AppConfig)}
                placeholder="选择 Magpie.exe 文件路径"
                className="glass-input flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
              />
              <BetterButton onClick={handleSelectMagpiePath} icon="i-mdi-file">
                选择
              </BetterButton>
            </div>
            <p className="mt-1 text-xs text-brand-500">
              设置后可在游戏编辑界面启用 Magpie 超分辨率缩放启动
            </p>
          </div>
        </div>
      </div>
    </>
  );
}
