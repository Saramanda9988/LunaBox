import { appconf } from '../../../wailsjs/go/models'
import { BetterSwitch } from '../ui/BetterSwitch'

interface GameSettingsPanelProps {
  formData: appconf.AppConfig
  onChange: (data: appconf.AppConfig) => void
}

export function GameSettingsPanel({ formData, onChange }: GameSettingsPanelProps) {
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
          onCheckedChange={(checked) =>
            onChange({ ...formData, record_active_time_only: checked } as appconf.AppConfig)
          }
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
    </>
  )
}
