import { appconf } from '../../../wailsjs/go/models'
import { BetterSwitch } from '../ui/BetterSwitch'

interface AutoBackupSettingsProps {
  formData: appconf.AppConfig
  onChange: (data: appconf.AppConfig) => void
}

export function AutoBackupSettingsPanel({ formData, onChange }: AutoBackupSettingsProps) {
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex-1">
          <label htmlFor="auto_backup_db" className="text-sm font-medium text-brand-700 dark:text-brand-300 cursor-pointer">
            应用退出时自动备份数据库
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
            在应用关闭时自动创建数据库备份，保护您的游戏库数据
          </p>
        </div>
        <BetterSwitch
          id="auto_backup_db"
          checked={formData.auto_backup_db || false}
          onCheckedChange={(checked) => onChange({ ...formData, auto_backup_db: checked } as appconf.AppConfig)}
        />
      </div>

      <div className="flex items-center justify-between">
        <div className="flex-1">
          <label htmlFor="auto_backup_game_save" className="text-sm font-medium text-brand-700 dark:text-brand-300 cursor-pointer">
            游戏退出时自动备份存档
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
            在游戏进程结束时自动备份存档目录（需要先设置存档目录）
          </p>
        </div>
        <BetterSwitch
          id="auto_backup_game_save"
          checked={formData.auto_backup_game_save || false}
          onCheckedChange={(checked) => onChange({ ...formData, auto_backup_game_save: checked } as appconf.AppConfig)}
        />
      </div>

      <div className="flex items-center justify-between">
        <div className="flex-1">
          <label htmlFor="auto_upload_db_to_cloud" className={`text-sm font-medium cursor-pointer ${formData.cloud_backup_enabled ? 'text-brand-700 dark:text-brand-300' : 'text-brand-400 dark:text-brand-500'}`}>
            自动上传数据库备份到云端
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
            在创建数据库备份后自动上传到云存储（需要先启用云备份）
          </p>
        </div>
        <BetterSwitch
          id="auto_upload_db_to_cloud"
          checked={formData.auto_upload_db_to_cloud || false}
          onCheckedChange={(checked) => onChange({ ...formData, auto_upload_db_to_cloud: checked } as appconf.AppConfig)}
          disabled={!formData.cloud_backup_enabled}
        />
      </div>

      <div className="flex items-center justify-between">
        <div className="flex-1">
          <label htmlFor="auto_upload_game_save_to_cloud" className={`text-sm font-medium cursor-pointer ${formData.cloud_backup_enabled ? 'text-brand-700 dark:text-brand-300' : 'text-brand-400 dark:text-brand-500'}`}>
            自动上传游戏存档备份到云端
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
            在游戏退出并备份存档后自动上传到云存储（需要先启用云备份）
          </p>
        </div>
        <BetterSwitch
          id="auto_upload_game_save_to_cloud"
          checked={formData.auto_upload_game_save_to_cloud || false}
          onCheckedChange={(checked) => onChange({ ...formData, auto_upload_game_save_to_cloud: checked } as appconf.AppConfig)}
          disabled={!formData.cloud_backup_enabled}
        />
      </div>

      <div className="pt-4 border-t border-brand-300 dark:border-brand-700 grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="space-y-2">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">本地游戏备份保留数量</label>
          <input
            type="number"
            name="local_backup_retention"
            value={formData.local_backup_retention || 10}
            onChange={(e) => onChange({ ...formData, local_backup_retention: parseInt(e.target.value) || 0 } as appconf.AppConfig)}
            className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
          />
          <p className="text-xs text-brand-500 dark:text-brand-400">每个游戏在本地保留的最大备份数量</p>
        </div>

        <div className="space-y-2">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">本地数据库备份保留数量</label>
          <input
            type="number"
            name="local_db_backup_retention"
            value={formData.local_db_backup_retention || 5}
            onChange={(e) => onChange({ ...formData, local_db_backup_retention: parseInt(e.target.value) || 0 } as appconf.AppConfig)}
            className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
          />
          <p className="text-xs text-brand-500 dark:text-brand-400">数据库在本地保留的最大备份数量</p>
        </div>
      </div>
    </div>
  )
}
