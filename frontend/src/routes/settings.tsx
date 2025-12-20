import { createRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import toast from 'react-hot-toast'
import { useAppStore } from '../store'
import { Route as rootRoute } from './__root'
import { appconf, vo } from '../../wailsjs/go/models'
import { TestS3Connection, TestOneDriveConnection, GetOneDriveAuthURL, StartOneDriveAuth, GetDBBackups, ScheduleDBRestore, DeleteDBBackup } from '../../wailsjs/go/service/BackupService'
import { Quit, BrowserOpenURL } from '../../wailsjs/runtime/runtime'

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/settings',
  component: SettingsPage,
})

function SettingsPage() {
  const { config, fetchConfig, updateConfig } = useAppStore()
  const [formData, setFormData] = useState<appconf.AppConfig | null>(null)
  const [testingS3, setTestingS3] = useState(false)
  const [testingOneDrive, setTestingOneDrive] = useState(false)
  const [authorizingOneDrive, setAuthorizingOneDrive] = useState(false)
  const [dbBackups, setDbBackups] = useState<vo.DBBackupStatus | null>(null)
  const [restoringBackup, setRestoringBackup] = useState<string | null>(null)

  useEffect(() => {
    fetchConfig()
    loadDBBackups()
  }, [fetchConfig])

  const loadDBBackups = async () => {
    try {
      const backups = await GetDBBackups()
      setDbBackups(backups)
    } catch (err) {
      console.error('Failed to load DB backups:', err)
    }
  }

  const handleRestoreDB = async (backupPath: string) => {
    if (!confirm('确定要恢复到此备份吗？程序将退出并在下次启动时完成恢复。')) return
    setRestoringBackup(backupPath)
    try {
      await ScheduleDBRestore(backupPath)
      toast.success('已安排恢复，程序即将退出...')
      setTimeout(() => Quit(), 1500)
    } catch (err: any) {
      toast.error('安排恢复失败: ' + err)
      setRestoringBackup(null)
    }
  }

  const handleDeleteDBBackup = async (backupPath: string) => {
    if (!confirm('确定要删除此备份吗？')) return
    try {
      await DeleteDBBackup(backupPath)
      await loadDBBackups()
      toast.success('备份已删除')
    } catch (err: any) {
      toast.error('删除失败: ' + err)
    }
  }

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  }

  useEffect(() => {
    if (config) {
      setFormData({ ...config } as appconf.AppConfig)
    }
  }, [config])

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
      // 先打开浏览器进行授权
      const authURL = await GetOneDriveAuthURL()
      BrowserOpenURL(authURL)
      
      // 后台启动本地服务器等待回调，自动获取 token
      const refreshToken = await StartOneDriveAuth()
      setFormData({ ...formData, onedrive_refresh_token: refreshToken } as appconf.AppConfig)
      toast.success('OneDrive 授权成功')
    } catch (err: any) {
      toast.error('授权失败: ' + err)
    } finally {
      setAuthorizingOneDrive(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (formData) {
      try {
        await updateConfig(formData)
        toast.success('设置已保存')
      } catch (err: any) {
        toast.error('保存失败: ' + err)
      }
    }
  }

  if (!config || !formData) {
    return <div className="p-8">Loading...</div>
  }

  return (
    <div className="space-y-8 max-w-8xl mx-auto p-8">
      <div className="flex items-center justify-between">
        <h1 className="text-4xl font-bold text-brand-900 dark:text-white">设置</h1>
      </div>
      <h2 className="text-lg font-semibold text-brand-900 dark:text-white flex items-center">
        <span className="i-mdi-database-settings text-xl"/>
        基础配置
      </h2>
      <form onSubmit={handleSubmit} className="space-y-6">
        <div className="space-y-2">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            Access Token
          </label>
          <input
            type="text"
            name="access_token"
            value={formData.access_token || ''}
            onChange={handleChange}
            className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
          />
        </div>

        <div className="space-y-2">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            VNDB Access Token
          </label>
          <input
            type="text"
            name="vndb_access_token"
            value={formData.vndb_access_token || ''}
            onChange={handleChange}
            className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
          />
        </div>

        <div className="space-y-2">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            主题
          </label>
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
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            语言
          </label>
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
            <div className="flex items-center gap-3">
              <input
                type="checkbox"
                id="cloud_backup_enabled"
                name="cloud_backup_enabled"
                checked={formData.cloud_backup_enabled || false}
                onChange={(e) => setFormData({ ...formData, cloud_backup_enabled: e.target.checked } as appconf.AppConfig)}
                className="w-4 h-4 text-blue-600 rounded focus:ring-blue-500"
              />
              <label htmlFor="cloud_backup_enabled" className="text-sm font-medium text-brand-700 dark:text-brand-300">
                启用云备份
              </label>
            </div>

            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                云存储提供商
              </label>
              <select
                name="cloud_backup_provider"
                value={formData.cloud_backup_provider || 's3'}
                onChange={(e) => setFormData({ ...formData, cloud_backup_provider: e.target.value } as appconf.AppConfig)}
                className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
              >
                <option value="s3">S3 兼容存储</option>
                <option value="onedrive">OneDrive</option>
              </select>
            </div>

            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                备份密码
              </label>
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
              <div className="space-y-4 p-4 bg-brand-50 dark:bg-brand-800 rounded-lg">
                <h3 className="text-sm font-medium text-brand-800 dark:text-brand-200">S3 配置</h3>
                
                <div className="space-y-2">
                  <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                    S3 端点 (Endpoint)
                  </label>
                  <input
                    type="text"
                    name="s3_endpoint"
                    value={formData.s3_endpoint || ''}
                    onChange={handleChange}
                    placeholder="https://s3.example.com"
                    className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
                  />
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                      区域 (Region)
                    </label>
                    <input
                      type="text"
                      name="s3_region"
                      value={formData.s3_region || ''}
                      onChange={handleChange}
                      placeholder="us-east-1"
                      className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
                    />
                  </div>
                  <div className="space-y-2">
                    <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                      存储桶 (Bucket)
                    </label>
                    <input
                      type="text"
                      name="s3_bucket"
                      value={formData.s3_bucket || ''}
                      onChange={handleChange}
                      placeholder="lunabox-backup"
                      className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                    Access Key
                  </label>
                  <input
                    type="text"
                    name="s3_access_key"
                    value={formData.s3_access_key || ''}
                    onChange={handleChange}
                    className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
                  />
                </div>

                <div className="space-y-2">
                  <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                    Secret Key
                  </label>
                  <input
                    type="password"
                    name="s3_secret_key"
                    value={formData.s3_secret_key || ''}
                    onChange={handleChange}
                    className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
                  />
                </div>

                <div className="flex justify-end">
                  <button
                    type="button"
                    onClick={handleTestS3}
                    disabled={testingS3}
                    className="px-3 py-1.5 text-sm bg-brand-100 text-brand-700 rounded-md hover:bg-brand-200 focus:outline-none focus:ring-2 focus:ring-brand-500 dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {testingS3 ? '测试中...' : '测试连接'}
                  </button>
                </div>
              </div>
            )}

            {/* OneDrive 配置 */}
            {formData.cloud_backup_provider === 'onedrive' && (
              <div className="space-y-4 p-4 bg-brand-50 dark:bg-brand-800 rounded-lg">
                <h3 className="text-sm font-medium text-brand-800 dark:text-brand-200">OneDrive 配置</h3>
                
                {/* OAuth 授权状态 */}
                <div className="space-y-2">
                  <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                    授权状态
                  </label>
                  {formData.onedrive_refresh_token ? (
                    <div className="flex items-center gap-2">
                      <span className="text-green-600 dark:text-green-400 flex items-center gap-1">
                        <span className="i-mdi-check-circle text-lg" />
                        已授权
                      </span>
                      <button
                        type="button"
                        onClick={() => setFormData({ ...formData, onedrive_refresh_token: '' } as appconf.AppConfig)}
                        className="px-2 py-1 text-xs text-red-600 hover:bg-red-100 dark:hover:bg-red-900 rounded"
                      >
                        取消授权
                      </button>
                    </div>
                  ) : (
                    <div className="space-y-3">
                      <button
                        type="button"
                        onClick={handleOneDriveAuth}
                        disabled={authorizingOneDrive}
                        className="px-3 py-1.5 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                      >
                        {authorizingOneDrive ? (
                          <>
                            <span className="i-mdi-loading animate-spin" />
                            等待授权中...
                          </>
                        ) : (
                          <>
                            <span className="i-mdi-microsoft" />
                            授权 OneDrive
                          </>
                        )}
                      </button>
                      {authorizingOneDrive && (
                        <p className="text-xs text-brand-500 dark:text-brand-400">
                          请在浏览器中完成授权，授权成功后会自动返回
                        </p>
                      )}
                    </div>
                  )}
                </div>

                {formData.onedrive_refresh_token && (
                  <div className="flex justify-end">
                    <button
                      type="button"
                      onClick={handleTestOneDrive}
                      disabled={testingOneDrive}
                      className="px-3 py-1.5 text-sm bg-brand-100 text-brand-700 rounded-md hover:bg-brand-200 focus:outline-none focus:ring-2 focus:ring-brand-500 dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      {testingOneDrive ? '测试中...' : '测试连接'}
                    </button>
                  </div>
                )}
              </div>
            )}

            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                保留备份数量
              </label>
              <input
                type="number"
                name="cloud_backup_retention"
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

        {/* AI 配置 */}
        <div className="pt-6 border-t border-brand-200 dark:border-brand-700">
          <h2 className="text-lg font-semibold text-brand-900 dark:text-white mb-4 flex items-center gap-2">
            <span className="i-mdi-robot-happy text-xl"/>
            AI 配置
          </h2>
          
          <div className="space-y-4">
            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                AI 服务商
              </label>
              <select
                name="ai_provider"
                value={formData.ai_provider || ''}
                onChange={handleChange}
                className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
              >
                <option value="">请选择</option>
                <option value="openai">OpenAI</option>
                <option value="deepseek">DeepSeek</option>
                <option value="custom">自定义 (OpenAI兼容)</option>
              </select>
            </div>

            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                API Base URL
              </label>
              <input
                type="text"
                name="ai_base_url"
                value={formData.ai_base_url || ''}
                onChange={handleChange}
                placeholder="https://api.openai.com/v1"
                className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
              />
              <p className="text-xs text-brand-500 dark:text-brand-400">留空则使用默认地址</p>
            </div>

            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                API Key
              </label>
              <input
                type="password"
                name="ai_api_key"
                value={formData.ai_api_key || ''}
                onChange={handleChange}
                placeholder="sk-..."
                className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
              />
            </div>

            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                模型名称
              </label>
              <input
                type="text"
                name="ai_model"
                value={formData.ai_model || ''}
                onChange={handleChange}
                placeholder="gpt-3.5-turbo"
                className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-brand-700 dark:text-white"
              />
              <p className="text-xs text-brand-500 dark:text-brand-400">留空则使用默认模型</p>
            </div>
          </div>
        </div>

        <div className="pt-4">
          <button
            type="submit"
            className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
          >
            保存设置
          </button>
        </div>
      </form>

      {/* 数据库备份恢复 */}
      <div className="pt-6 border-t border-brand-200 dark:border-brand-700">
        <h2 className="text-lg font-semibold text-brand-900 dark:text-white mb-4 flex items-center gap-2">
          <span className="i-mdi-database-refresh text-xl"/>
          数据库备份恢复
        </h2>
        
        {dbBackups?.last_backup_time && (
          <p className="text-sm text-brand-600 dark:text-brand-400 mb-4">
            上次备份: {new Date(dbBackups.last_backup_time).toLocaleString('zh-CN')}
          </p>
        )}

        {dbBackups?.backups && dbBackups.backups.length > 0 ? (
          <div className="space-y-2">
            {dbBackups.backups.map((backup) => (
              <div
                key={backup.path}
                className="flex items-center justify-between p-3 bg-brand-50 dark:bg-brand-700 rounded-lg"
              >
                <div className="flex-1">
                  <p className="text-sm font-medium text-brand-900 dark:text-white">
                    {backup.name}
                  </p>
                  <p className="text-xs text-brand-500 dark:text-brand-400">
                    {new Date(backup.created_at).toLocaleString('zh-CN')} · {formatFileSize(backup.size)}
                  </p>
                </div>
                <div className="flex gap-2">
                  <button
                    onClick={() => handleRestoreDB(backup.path)}
                    disabled={restoringBackup !== null}
                    className="p-2 text-green-600 hover:bg-green-100 dark:hover:bg-green-900 rounded transition-colors"
                  >
                    {restoringBackup === backup.path ? <div className="i-mdi-loading text-xl animate-spin" /> : <div className="i-mdi-backup-restore text-xl" />}
                  </button>
                  <button
                    onClick={() => handleDeleteDBBackup(backup.path)}
                    disabled={restoringBackup !== null}
                    className="p-2 text-red-600 hover:bg-red-100 dark:hover:bg-red-900 rounded transition-colors"
                  >
                    <div className="i-mdi-delete text-xl" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-brand-500 dark:text-brand-400">
            暂无备份，点击侧边栏的云同步按钮创建备份
          </p>
        )}
      </div>
    </div>
  )
}
