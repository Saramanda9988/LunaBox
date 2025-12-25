import { appconf } from '../../../wailsjs/go/models'

interface BasicSettingsProps {
  formData: appconf.AppConfig
  onChange: (data: appconf.AppConfig) => void
}

export function BasicSettingsPanel({ formData, onChange }: BasicSettingsProps) {
  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target
    onChange({ ...formData, [name]: value } as appconf.AppConfig)
  }

  return (
    <>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">Bangumi Access Token</label>
        <input
          type="text"
          name="access_token"
          value={formData.access_token || ''}
          onChange={handleChange}
          className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">如果您想使用Bangumi数据源，请一定填写</p>
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">VNDB Access Token</label>
        <input
          type="text"
          name="vndb_access_token"
          value={formData.vndb_access_token || ''}
          onChange={handleChange}
          className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
        />
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">主题</label>
        <select
          name="theme"
          value={formData.theme}
          onChange={handleChange}
          className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
        >
          <option value="light">浅色</option>
          <option value="dark">深色</option>
          <option value="system">跟随系统</option>
        </select>
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">语言</label>
        <select
          name="language"
          value={formData.language}
          onChange={handleChange}
          className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
        >
          <option value="zh-CN">简体中文</option>
          <option value="en-US">English</option>
        </select>
      </div>
    </>
  )
}
