import { createRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import toast from 'react-hot-toast'
import { useAppStore } from '../store'
import { Route as rootRoute } from './__root'
import { appconf } from '../../wailsjs/go/models'

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/settings',
  component: SettingsPage,
})

function SettingsPage() {
  const { config, fetchConfig, updateConfig } = useAppStore()
  const [formData, setFormData] = useState<appconf.AppConfig | null>(null)

  useEffect(() => {
    fetchConfig()
  }, [fetchConfig])

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

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (formData) {
      try {
        await updateConfig(formData)
        toast.success('设置已保存')
      } catch {
        toast.error('保存设置失败')
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
    </div>
  )
}
