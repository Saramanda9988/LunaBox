import { createRoute } from '@tanstack/react-router'
import { useEffect, useState, useRef } from 'react'
import toast from 'react-hot-toast'
import { useAppStore } from '../store'
import { GetVersionInfo } from '../../wailsjs/go/service/VersionService'
import { Route as rootRoute } from './__root'
import { appconf, enums } from '../../wailsjs/go/models'
import { TestS3Connection, TestOneDriveConnection, GetOneDriveAuthURL, StartOneDriveAuth } from '../../wailsjs/go/service/BackupService'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import { DBBackupPanel } from '../components/panel/DBBackupPanel'
import { SettingsSkeleton } from '../components/skeleton/SettingsSkeleton'
import { BetterSwitch } from '../components/ui/BetterSwitch'

const PROMPT_LABELS: Record<string, string> = {
  DEFAULT_SYSTEM: '幽默评论员',
  MEOW_ZAKO: '雌小鬼猫娘',
  STRICT_TUTOR: '严厉导师',
}

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/settings',
  component: SettingsPage,
})

function SettingsPage() {
  const { config, fetchConfig, updateConfig } = useAppStore()
  const [formData, setFormData] = useState<appconf.AppConfig | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [showSkeleton, setShowSkeleton] = useState(false)
  const [testingS3, setTestingS3] = useState(false)
  const [testingOneDrive, setTestingOneDrive] = useState(false)
  const [authorizingOneDrive, setAuthorizingOneDrive] = useState(false)
  const [versionInfo, setVersionInfo] = useState<Record<string, string> | null>(null)
  const isInitialMount = useRef(true)

  useEffect(() => {
    const init = async () => {
      setIsLoading(true)
      await fetchConfig()
      try {
        const info = await GetVersionInfo()
        setVersionInfo(info)
      } catch (err) {
        console.error('Failed to fetch version info:', err)
      }
      setIsLoading(false)
    }
    init()
  }, [fetchConfig])

  // 延迟显示骨架屏
  useEffect(() => {
    let timer: number
    if (isLoading) {
      timer = window.setTimeout(() => {
        setShowSkeleton(true)
      }, 300)
    } else {
      setShowSkeleton(false)
    }
    return () => clearTimeout(timer)
  }, [isLoading])

  useEffect(() => {
    if (config && isInitialMount.current) {
      setFormData({ ...config } as appconf.AppConfig)
      isInitialMount.current = false
    }
  }, [config])

  // 自动保存逻辑
  useEffect(() => {
    if (!formData || isInitialMount.current) return

    // 检查是否有实际变化，避免无意义的保存
    const hasChanges = JSON.stringify(formData) !== JSON.stringify(config)
    if (!hasChanges) return

    const timer = setTimeout(() => {
      updateConfig(formData)
    }, 250) // 0.25秒防抖

    return () => clearTimeout(timer)
  }, [formData, updateConfig, config])

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    if (!formData) return
    const { name, value } = e.target
    setFormData({ ...formData, [name]: value } as appconf.AppConfig)
  }

  const handleTestS3 = async () => {
    if (!formData) return
    setTestingS3(true)
    try {
      await TestS3Connection(formData)
      toast.success('S3 连接测试成功')
    } catch (err: any) {
      toast.error('S3 连接测试失败: ' + err)
    } finally {
      setTestingS3(false)
    }
  }

  const handleTestOneDrive = async () => {
    if (!formData) return
    setTestingOneDrive(true)
    try {
      await TestOneDriveConnection(formData)
      toast.success('OneDrive 连接测试成功')
    } catch (err: any) {
      toast.error('OneDrive 连接测试失败: ' + err)
    } finally {
      setTestingOneDrive(false)
    }
  }

  const handleOneDriveAuth = async () => {
    if (!formData) return
    setAuthorizingOneDrive(true)
    try {
      const authURL = await GetOneDriveAuthURL()
      BrowserOpenURL(authURL)
      const refreshToken = await StartOneDriveAuth()
      setFormData({ ...formData, onedrive_refresh_token: refreshToken } as appconf.AppConfig)
      toast.success('OneDrive 授权成功')
    } catch (err: any) {
      toast.error('授权失败: ' + err)
    } finally {
      setAuthorizingOneDrive(false)
    }
  }

  if (isLoading && (!config || !formData)) {
    if (!showSkeleton) {
      return <div className="min-h-screen bg-brand-100 dark:bg-brand-900" />
    }
    return <SettingsSkeleton />
  }

  if (!config || !formData) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[80vh] space-y-4 text-brand-500">
        <div className="i-mdi-cog-outline text-6xl animate-spin-slow" />
        <p className="text-xl">正在准备设置...</p>
      </div>
    )
  }

  return (
    <div className={`space-y-8 max-w-8xl mx-auto p-8 transition-opacity duration-300 ${isLoading ? 'opacity-50 pointer-events-none' : 'opacity-100'}`}>
      <div className="flex items-center justify-between">
        <h1 className="text-4xl font-bold text-brand-900 dark:text-white">设置</h1>
      </div>
      
      <h2 className="text-lg font-semibold text-brand-900 dark:text-white flex items-center gap-2">
        <span className="i-mdi-database-settings text-xl"/>
        基础配置
      </h2>
      
      <div className="space-y-6">
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

        {/* 云备份配置 */}
        <div className="pt-6 border-t border-brand-200 dark:border-brand-700">
          <h2 className="text-lg font-semibold text-brand-900 dark:text-white mb-4 flex items-center gap-2">
            <span className="i-mdi-cloud-upload text-xl"/>
            云备份配置
          </h2>
          
          <div className="space-y-4">
            <div className="flex items-center justify-between p-4 bg-brand-50 dark:bg-brand-800/50 rounded-lg">
              <div className="flex flex-col">
                <label htmlFor="cloud_backup_enabled" className="text-sm font-medium text-brand-700 dark:text-brand-300 cursor-pointer">
                  启用云备份
                </label>
                <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">将您的数据同步到云端存储</p>
              </div>
              <BetterSwitch
                id="cloud_backup_enabled"
                checked={formData.cloud_backup_enabled || false}
                onCheckedChange={(checked) => setFormData({ ...formData, cloud_backup_enabled: checked } as appconf.AppConfig)}
              />
            </div>

            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">云存储提供商</label>
              <select
                value={formData.cloud_backup_provider || 's3'}
                onChange={(e) => setFormData({ ...formData, cloud_backup_provider: e.target.value } as appconf.AppConfig)}
                className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
              >
                <option value="s3">S3 兼容存储</option>
                <option value="onedrive">OneDrive</option>
              </select>
            </div>

            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">备份密码</label>
              <input
                type="password"
                name="backup_password"
                value={formData.backup_password || ''}
                onChange={handleChange}
                placeholder="用于生成用户标识和加密备份"
                className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
              />
              <p className="text-xs text-brand-500 dark:text-brand-400">
                重要：请牢记此密码，忘记将无法恢复云端备份
                {formData.backup_user_id && <span className="ml-2">用户ID: {formData.backup_user_id.substring(0, 8)}...</span>}
              </p>
            </div>

            {/* S3 配置 */}
            {formData.cloud_backup_provider === 's3' && (
              <div className="space-y-4 p-4 bg-brand-100 dark:bg-brand-800 rounded-lg">
                <h3 className="text-sm font-medium text-brand-800 dark:text-brand-200">S3 配置</h3>
                <div className="space-y-2">
                  <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">S3 端点 (Endpoint)</label>
                  <input type="text" name="s3_endpoint" value={formData.s3_endpoint || ''} onChange={handleChange} placeholder="https://s3.example.com" className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white" />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">区域 (Region)</label>
                    <input type="text" name="s3_region" value={formData.s3_region || ''} onChange={handleChange} placeholder="us-east-1" className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white" />
                  </div>
                  <div className="space-y-2">
                    <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">存储桶 (Bucket)</label>
                    <input type="text" name="s3_bucket" value={formData.s3_bucket || ''} onChange={handleChange} placeholder="lunabox-backup" className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white" />
                  </div>
                </div>
                <div className="space-y-2">
                  <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">Access Key</label>
                  <input type="text" name="s3_access_key" value={formData.s3_access_key || ''} onChange={handleChange} className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white" />
                </div>
                <div className="space-y-2">
                  <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">Secret Key</label>
                  <input type="password" name="s3_secret_key" value={formData.s3_secret_key || ''} onChange={handleChange} className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white" />
                </div>
                <div className="flex justify-end">
                  <button type="button" onClick={handleTestS3} disabled={testingS3} className="px-3 py-1.5 text-sm bg-brand-100 text-brand-700 rounded-md hover:bg-brand-200 dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600 disabled:opacity-50">
                    {testingS3 ? '测试中...' : '测试连接'}
                  </button>
                </div>
              </div>
            )}

            {/* OneDrive 配置 */}
            {formData.cloud_backup_provider === 'onedrive' && (
              <div className="space-y-4 p-4 bg-brand-50 dark:bg-brand-800 rounded-lg">
                <h3 className="text-sm font-medium text-brand-800 dark:text-brand-200">OneDrive 配置</h3>
                <div className="space-y-2">
                  <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">授权状态</label>
                  {formData.onedrive_refresh_token ? (
                    <div className="flex items-center gap-2">
                      <span className="text-green-600 dark:text-green-400 flex items-center gap-1">
                        <span className="i-mdi-check-circle text-lg" />
                        已授权
                      </span>
                      <button type="button" onClick={() => setFormData({ ...formData, onedrive_refresh_token: '' } as appconf.AppConfig)} className="px-2 py-1 text-xs text-red-600 hover:bg-red-100 dark:hover:bg-red-900 rounded">
                        取消授权
                      </button>
                    </div>
                  ) : (
                    <div className="space-y-3">
                      <button type="button" onClick={handleOneDriveAuth} disabled={authorizingOneDrive} className="px-3 py-1.5 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 flex items-center gap-2">
                        {authorizingOneDrive ? (<><span className="i-mdi-loading animate-spin" />等待授权中...</>) : (<><span className="i-mdi-microsoft" />授权 OneDrive</>)}
                      </button>
                      {authorizingOneDrive && <p className="text-xs text-brand-500 dark:text-brand-400">请在浏览器中完成授权，授权成功后会自动返回</p>}
                    </div>
                  )}
                </div>
                {formData.onedrive_refresh_token && (
                  <div className="flex justify-end">
                    <button type="button" onClick={handleTestOneDrive} disabled={testingOneDrive} className="px-3 py-1.5 text-sm bg-brand-100 text-brand-700 rounded-md hover:bg-brand-200 dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600 disabled:opacity-50">
                      {testingOneDrive ? '测试中...' : '测试连接'}
                    </button>
                  </div>
                )}
              </div>
            )}

            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">保留备份数量</label>
              <input
                type="number"
                value={formData.cloud_backup_retention || 20}
                onChange={(e) => setFormData({ ...formData, cloud_backup_retention: parseInt(e.target.value) || 20 } as appconf.AppConfig)}
                min={1}
                max={100}
                className="w-32 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
              />
              <p className="text-xs text-brand-500 dark:text-brand-400">云端每个游戏保留的最大备份数量</p>
            </div>
          </div>
        </div>

        {/* 自动备份配置 */}
        <div className="pt-6 border-t border-brand-200 dark:bg-brand-800/50">
          <h2 className="text-lg font-semibold text-brand-900 dark:text-white mb-4 flex items-center gap-2">
            <span className="i-mdi-backup-restore text-xl"/>
            自动备份
          </h2>
          
          <div className="space-y-4 bg-brand-200 dark:bg-brand-800/50 p-4 rounded-lg">
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
                onCheckedChange={(checked) => setFormData({ ...formData, auto_backup_db: checked } as appconf.AppConfig)}
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
                onCheckedChange={(checked) => setFormData({ ...formData, auto_backup_game_save: checked } as appconf.AppConfig)}
              />
            </div>

            <div className="flex items-center justify-between">
              <div className="flex-1">
                <label htmlFor="auto_upload_to_cloud" className={`text-sm font-medium cursor-pointer ${formData.cloud_backup_enabled ? 'text-brand-700 dark:text-brand-300' : 'text-brand-400 dark:text-brand-500'}`}>
                  自动上传备份到云端
                </label>
                <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
                  在创建本地备份后自动上传到云存储（需要先启用云备份）
                </p>
              </div>
              <BetterSwitch
                id="auto_upload_to_cloud"
                checked={formData.auto_upload_to_cloud || false}
                onCheckedChange={(checked) => setFormData({ ...formData, auto_upload_to_cloud: checked } as appconf.AppConfig)}
                disabled={!formData.cloud_backup_enabled}
              />
            </div>
          </div>
        </div>

        {/* AI 配置 */}
        <div className="pt-6 border-t border-brand-200 dark:border-brand-700">
          <h2 className="text-lg font-semibold text-brand-900 dark:text-white mb-4 flex items-center gap-2">
            <span className="i-mdi-robot-happy text-xl"/>
            AI 配置
          </h2>
          <div className="space-y-4">
            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">AI 服务商</label>
              <select name="ai_provider" value={formData.ai_provider || ''} onChange={handleChange} className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white">
                <option value="">请选择</option>
                <option value="openai">OpenAI</option>
                <option value="deepseek">DeepSeek</option>
                <option value="custom">自定义 (OpenAI兼容)</option>
              </select>
            </div>
            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">API Base URL</label>
              <input type="text" name="ai_base_url" value={formData.ai_base_url || ''} onChange={handleChange} placeholder="https://api.openai.com/v1" className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white" />
            </div>
            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">API Key</label>
              <input type="password" name="ai_api_key" value={formData.ai_api_key || ''} onChange={handleChange} placeholder="sk-..." className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white" />
            </div>
            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">模型名称</label>
              <input type="text" name="ai_model" value={formData.ai_model || ''} onChange={handleChange} placeholder="gpt-3.5-turbo" className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white" />
            </div>
            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">系统提示语 (System Prompt)</label>
              <div className="flex gap-2 mb-2">
                {Object.entries(enums.PromptType).map(([name, prompt]) => (
                  <button
                    key={name}
                    onClick={() => setFormData({ ...formData, ai_system_prompt: prompt } as appconf.AppConfig)}
                    className="px-2 py-1 text-xs bg-brand-100 dark:bg-brand-700 text-brand-600 dark:text-brand-300 rounded hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors"
                  >
                    {PROMPT_LABELS[name] || name}
                  </button>
                ))}
              </div>
              <textarea
                name="ai_system_prompt"
                value={formData.ai_system_prompt || ''}
                onChange={(e) => setFormData({ ...formData, ai_system_prompt: e.target.value } as appconf.AppConfig)}
                rows={4}
                placeholder="输入自定义的 AI 系统提示语..."
                className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white text-sm"
              />
              <p className="text-xs text-brand-500 dark:text-brand-400">AI 将根据此提示语来生成统计总结。你可以点击上方预设快速填充。</p>
            </div>
          </div>
        </div>
      </div>

      {/* 数据库备份 - 使用独立组件 */}
      <div className="pt-6 border-t border-brand-200 dark:border-brand-700">
        <h2 className="text-lg font-semibold text-brand-900 dark:text-white mb-4 flex items-center gap-2">
          <span className="i-mdi-database-refresh text-xl"/>
          数据库备份
        </h2>
        <DBBackupPanel />
      </div>

      <div>
        <p className="text-xs text-brand-500 dark:text-brand-400">
          Lunabox made with LunaRain_079 &amp; Contributors.
        </p>
        {versionInfo && (
          <p className="text-xs text-brand-400 dark:text-brand-500 mt-1">
            Version {versionInfo.version} ({versionInfo.commit}) | {versionInfo.buildMode} | Built at {versionInfo.buildTime}
          </p>
        )}
      </div>
    </div>
  )
}
