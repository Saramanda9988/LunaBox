import { useEffect, useState } from 'react'
import toast from 'react-hot-toast'
import { vo } from '../../../wailsjs/go/models'
import {
  CreateAndUploadDBBackup,
  GetDBBackups,
  GetCloudDBBackups,
  ScheduleDBRestore,
  DeleteDBBackup,
  UploadDBBackupToCloud,
  ScheduleDBRestoreFromCloud,
} from '../../../wailsjs/go/service/BackupService'
import { Quit } from '../../../wailsjs/runtime/runtime'
import { useAppStore } from '../../store'

export function DBBackupPanel() {
  const { config } = useAppStore()
  const [dbBackups, setDbBackups] = useState<vo.DBBackupStatus | null>(null)
  const [cloudDBBackups, setCloudDBBackups] = useState<vo.CloudBackupItem[]>([])
  const [isBackingUp, setIsBackingUp] = useState(false)
  const [restoringBackup, setRestoringBackup] = useState<string | null>(null)
  const [uploadingBackup, setUploadingBackup] = useState<string | null>(null)
  const [loadingLocal, setLoadingLocal] = useState(true)
  const [loadingCloud, setLoadingCloud] = useState(false)

  const cloudEnabled = config?.cloud_backup_enabled && config?.backup_user_id

  useEffect(() => {
    loadDBBackups()
  }, [])

  useEffect(() => {
    if (cloudEnabled) {
      loadCloudDBBackups()
    }
  }, [cloudEnabled])

  const loadDBBackups = async () => {
    setLoadingLocal(true)
    try {
      const backups = await GetDBBackups()
      setDbBackups(backups)
    } catch (err) {
      console.error('Failed to load DB backups:', err)
    } finally {
      setLoadingLocal(false)
    }
  }

  const loadCloudDBBackups = async () => {
    setLoadingCloud(true)
    try {
      const backups = await GetCloudDBBackups()
      setCloudDBBackups(backups || [])
    } catch (err) {
      console.error('Failed to load cloud DB backups:', err)
      setCloudDBBackups([])
    } finally {
      setLoadingCloud(false)
    }
  }

  const handleCreateBackup = async () => {
    if (isBackingUp) return
    setIsBackingUp(true)
    try {
      await CreateAndUploadDBBackup()
      await loadDBBackups()
      if (cloudEnabled) await loadCloudDBBackups()
      toast.success(cloudEnabled ? '数据库备份成功并已上传云端' : '数据库备份成功')
    } catch (err: any) {
      if (err.toString().includes('本地备份成功')) {
        await loadDBBackups()
        toast.success('本地备份成功')
        toast.error(err.toString())
      } else {
        toast.error('备份失败: ' + err)
      }
    } finally {
      setIsBackingUp(false)
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

  const handleUploadDBBackup = async (backupPath: string) => {
    setUploadingBackup(backupPath)
    try {
      await UploadDBBackupToCloud(backupPath)
      await loadCloudDBBackups()
      toast.success('已上传到云端')
    } catch (err: any) {
      toast.error('上传失败: ' + err)
    } finally {
      setUploadingBackup(null)
    }
  }

  const handleRestoreFromCloud = async (cloudKey: string) => {
    if (!confirm('确定要从云端恢复此备份吗？程序将退出并在下次启动时完成恢复。')) return
    setRestoringBackup(cloudKey)
    try {
      await ScheduleDBRestoreFromCloud(cloudKey)
      toast.success('已安排恢复，程序即将退出...')
      setTimeout(() => Quit(), 1500)
    } catch (err: any) {
      toast.error('安排恢复失败: ' + err)
      setRestoringBackup(null)
    }
  }

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  }

  const isDisabled = restoringBackup !== null || uploadingBackup !== null || isBackingUp

  return (
    <div className="space-y-6">
      {/* 备份操作区 */}
      <div className="bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h3 className="text-lg font-semibold text-brand-900 dark:text-white">数据库备份</h3>
            <p className="text-sm text-brand-500 dark:text-brand-400 mt-1">
              备份游戏库元数据、分类、游玩记录等应用数据
            </p>
          </div>
          <button
            onClick={handleCreateBackup}
            disabled={isDisabled}
            className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
          >
            {isBackingUp && <div className="i-mdi-loading animate-spin" />}
            {isBackingUp ? '备份中...' : '立即备份'}
          </button>
        </div>
        {dbBackups?.last_backup_time && (
          <p className="text-xs text-brand-500 dark:text-brand-400">
            上次备份: {new Date(dbBackups.last_backup_time).toLocaleString('zh-CN')}
          </p>
        )}
      </div>

      {/* 本地备份列表 */}
      <div className="bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
        <h3 className="text-lg font-semibold text-brand-900 dark:text-white mb-4">本地备份</h3>
        {loadingLocal ? (
          <div className="flex justify-center py-8">
            <div className="i-mdi-loading animate-spin text-2xl text-brand-500" />
          </div>
        ) : dbBackups?.backups && dbBackups.backups.length > 0 ? (
          <div className="space-y-3">
            {dbBackups.backups.map((backup) => (
              <div
                key={backup.path}
                className="flex items-center justify-between p-4 bg-brand-50 dark:bg-brand-700 rounded-lg"
              >
                <div className="flex items-center gap-4">
                  <div className="i-mdi-database text-2xl text-brand-500" />
                  <div>
                    <div className="font-medium text-brand-900 dark:text-white">{backup.name}</div>
                    <div className="text-sm text-brand-500">
                      {new Date(backup.created_at).toLocaleString('zh-CN')} · {formatFileSize(backup.size)}
                    </div>
                  </div>
                </div>
                <div className="flex gap-2">
                  {cloudEnabled && (
                    <button
                      onClick={() => handleUploadDBBackup(backup.path)}
                      disabled={isDisabled}
                      title="上传到云端"
                      className="p-2 text-blue-600 hover:bg-blue-100 dark:hover:bg-blue-900 rounded transition-colors disabled:opacity-50"
                    >
                      {uploadingBackup === backup.path ? (
                        <div className="i-mdi-loading text-xl animate-spin" />
                      ) : (
                        <div className="i-mdi-cloud-upload text-xl" />
                      )}
                    </button>
                  )}
                  <button
                    onClick={() => handleRestoreDB(backup.path)}
                    disabled={isDisabled}
                    title="恢复此备份"
                    className="p-2 text-green-600 hover:bg-green-100 dark:hover:bg-green-900 rounded transition-colors disabled:opacity-50"
                  >
                    {restoringBackup === backup.path ? (
                      <div className="i-mdi-loading text-xl animate-spin" />
                    ) : (
                      <div className="i-mdi-backup-restore text-xl" />
                    )}
                  </button>
                  <button
                    onClick={() => handleDeleteDBBackup(backup.path)}
                    disabled={isDisabled}
                    title="删除备份"
                    className="p-2 text-red-600 hover:bg-red-100 dark:hover:bg-red-900 rounded transition-colors disabled:opacity-50"
                  >
                    <div className="i-mdi-delete text-xl" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="text-center py-8 text-brand-500">暂无本地备份记录</div>
        )}
      </div>

      {/* 云端备份列表 */}
      {cloudEnabled && (
        <div className="bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm">
          <h3 className="text-lg font-semibold text-brand-900 dark:text-white mb-4 flex items-center gap-2">
            <div className="i-mdi-cloud text-xl text-blue-500" />
            云端备份
          </h3>
          {loadingCloud ? (
            <div className="flex justify-center py-8">
              <div className="i-mdi-loading animate-spin text-2xl text-brand-500" />
            </div>
          ) : cloudDBBackups.length > 0 ? (
            <div className="space-y-3">
              {cloudDBBackups.map((backup) => (
                <div
                  key={backup.key}
                  className="flex items-center justify-between p-4 bg-blue-50 dark:bg-blue-900/30 rounded-lg"
                >
                  <div className="flex items-center gap-4">
                    <div className="i-mdi-cloud-check text-2xl text-blue-500" />
                    <div>
                      <div className="font-medium text-brand-900 dark:text-white">{backup.name}</div>
                      <div className="text-sm text-brand-500">
                        {new Date(backup.created_at).toLocaleString('zh-CN')}
                      </div>
                    </div>
                  </div>
                  <div className="flex gap-2">
                    <button
                      onClick={() => handleRestoreFromCloud(backup.key)}
                      disabled={isDisabled}
                      title="从云端恢复"
                      className="p-2 text-green-600 hover:bg-green-100 dark:hover:bg-green-900 rounded transition-colors disabled:opacity-50"
                    >
                      {restoringBackup === backup.key ? (
                        <div className="i-mdi-loading text-xl animate-spin" />
                      ) : (
                        <div className="i-mdi-cloud-download text-xl" />
                      )}
                    </button>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="text-center py-8 text-brand-500">暂无云端备份记录</div>
          )}
        </div>
      )}

      {/* 云备份未配置提示 */}
      {!cloudEnabled && (
        <div className="bg-brand-50 dark:bg-brand-800 p-4 rounded-lg border border-brand-200 dark:border-brand-700">
          <div className="flex items-center gap-3">
            <div className="i-mdi-cloud-off-outline text-2xl text-brand-400" />
            <div>
              <div className="font-medium text-brand-700 dark:text-brand-300">云备份未启用</div>
              <div className="text-sm text-brand-500">在上方配置云备份后，可将数据库同步到云端</div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
