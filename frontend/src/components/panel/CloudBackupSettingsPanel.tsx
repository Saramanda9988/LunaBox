import type { appconf } from "../../../wailsjs/go/models";
import { useState } from "react";
import toast from "react-hot-toast";
import { GetOneDriveAuthURL, SetupCloudBackup, StartOneDriveAuth, TestOneDriveConnection, TestS3Connection } from "../../../wailsjs/go/service/BackupService";
import { GetAppConfig } from "../../../wailsjs/go/service/ConfigService";
import { BrowserOpenURL } from "../../../wailsjs/runtime";
import { PasswordInputModal } from "../modal/PasswordInputModal";
import { BetterSwitch } from "../ui/BetterSwitch";

interface CloudBackupSettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

export function CloudBackupSettingsPanel({ formData, onChange }: CloudBackupSettingsProps) {
  const [testingS3, setTestingS3] = useState(false);
  const [testingOneDrive, setTestingOneDrive] = useState(false);
  const [authorizingOneDrive, setAuthorizingOneDrive] = useState(false);
  const [showPasswordModal, setShowPasswordModal] = useState(false);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    onChange({ ...formData, [name]: value } as appconf.AppConfig);
  };

  const handleSetupBackupPassword = async (password: string, confirmPassword: string) => {
    if (password.length < 6) {
      toast.error("密码长度至少为6位");
      return;
    }

    if (password !== confirmPassword) {
      toast.error("两次输入的密码不一致");
      return;
    }

    try {
      const userID = await SetupCloudBackup(password);
      toast.success(`密码设置成功！用户ID: ${userID.substring(0, 8)}...`);

      // 重新加载配置以更新界面
      const updatedConfig = await GetAppConfig();
      onChange(updatedConfig);
    }
    catch (err: any) {
      toast.error(`设置失败: ${err}`);
    }
  };

  const handleTestS3 = async () => {
    setTestingS3(true);
    try {
      await TestS3Connection(formData);
      toast.success("S3 连接测试成功");
    }
    catch (err: any) {
      toast.error(`S3 连接测试失败: ${err}`);
    }
    finally {
      setTestingS3(false);
    }
  };

  const handleTestOneDrive = async () => {
    setTestingOneDrive(true);
    try {
      await TestOneDriveConnection(formData);
      toast.success("OneDrive 连接测试成功");
    }
    catch (err: any) {
      toast.error(`OneDrive 连接测试失败: ${err}`);
    }
    finally {
      setTestingOneDrive(false);
    }
  };

  const handleOneDriveAuth = async () => {
    setAuthorizingOneDrive(true);
    try {
      const authURL = await GetOneDriveAuthURL();
      BrowserOpenURL(authURL);
      const refreshToken = await StartOneDriveAuth();
      onChange({ ...formData, onedrive_refresh_token: refreshToken } as appconf.AppConfig);
      toast.success("OneDrive 授权成功");
    }
    catch (err: any) {
      toast.error(`授权失败: ${err}`);
    }
    finally {
      setAuthorizingOneDrive(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="glass-card flex items-center justify-between p-4 bg-brand-50 dark:bg-brand-800/50 rounded-lg">
        <div className="flex flex-col">
          <label htmlFor="cloud_backup_enabled" className="text-sm font-medium text-brand-700 dark:text-brand-300 cursor-pointer">
            启用云备份
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">将您的数据同步到云端存储</p>
        </div>
        <BetterSwitch
          id="cloud_backup_enabled"
          checked={formData.cloud_backup_enabled || false}
          onCheckedChange={checked => onChange({ ...formData, cloud_backup_enabled: checked } as appconf.AppConfig)}
        />
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">云存储提供商</label>
        <select
          value={formData.cloud_backup_provider || "s3"}
          onChange={e => onChange({ ...formData, cloud_backup_provider: e.target.value } as appconf.AppConfig)}
          className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
        >
          <option value="s3">S3 兼容存储</option>
          <option value="onedrive">OneDrive</option>
        </select>
      </div>

      {/* S3 配置 */}
      {formData.cloud_backup_provider === "s3" && (
        <div className="glass-card space-y-4 p-4 bg-brand-100 dark:bg-brand-800 rounded-lg">
          <h3 className="text-sm font-medium text-brand-800 dark:text-brand-200">S3 配置</h3>
          <div className="space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">备份密码</label>
            {formData.backup_password
              ? (
                  <div className="space-y-2">
                    <div className="px-3 py-2 bg-brand-100 dark:bg-brand-700 border border-brand-300 dark:border-brand-600 rounded-md text-brand-600 dark:text-brand-300">
                      ********
                    </div>
                    <p className="text-xs text-brand-500 dark:text-brand-400">
                      <span className="text-success-600 dark:text-success-400">
                        ✓ 密码已设置，用户ID:
                        {" "}
                        {formData.backup_user_id?.substring(0, 8)}
                        ...
                      </span>
                    </p>
                  </div>
                )
              : (
                  <div className="space-y-2">
                    <button
                      type="button"
                      onClick={() => setShowPasswordModal(true)}
                      className="glass-btn-neutral w-full px-4 py-2 bg-brand-600 hover:bg-brand-700 text-white rounded-md transition-colors flex items-center justify-center gap-2"
                    >
                      <span className="i-mdi-lock-plus text-lg" />
                      设置备份密码
                    </button>
                    <p className="text-xs text-brand-500 dark:text-brand-400">
                      用于生成用户标识
                    </p>
                    <p className="text-xs text-warning-600 dark:text-warning-400">
                      重要：密码只能设置一次，设置后无法修改，请牢记
                    </p>
                  </div>
                )}
          </div>
          <div className="space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">S3 端点 (Endpoint)</label>
            <input type="text" name="s3_endpoint" value={formData.s3_endpoint || ""} onChange={handleChange} placeholder="https://s3.example.com" className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white" />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">区域 (Region)</label>
              <input type="text" name="s3_region" value={formData.s3_region || ""} onChange={handleChange} placeholder="us-east-1" className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white" />
            </div>
            <div className="space-y-2">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">存储桶 (Bucket)</label>
              <input type="text" name="s3_bucket" value={formData.s3_bucket || ""} onChange={handleChange} placeholder="lunabox-backup" className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white" />
            </div>
          </div>
          <div className="space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">Access Key</label>
            <input type="text" name="s3_access_key" value={formData.s3_access_key || ""} onChange={handleChange} className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white" />
          </div>
          <div className="space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">Secret Key</label>
            <input type="password" name="s3_secret_key" value={formData.s3_secret_key || ""} onChange={handleChange} className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white" />
          </div>
          <div className="flex justify-end">
            <button type="button" onClick={handleTestS3} disabled={testingS3} className="glass-btn-neutral px-3 py-1.5 text-sm bg-brand-100 text-brand-700 rounded-md hover:bg-brand-200 dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600 disabled:opacity-50">
              {testingS3 ? "测试中..." : "测试连接"}
            </button>
          </div>
        </div>
      )}

      {/* OneDrive 配置 */}
      {formData.cloud_backup_provider === "onedrive" && (
        <div className="space-y-4 p-4 bg-brand-100 dark:bg-brand-800 rounded-lg">
          <h3 className="text-sm font-medium text-brand-800 dark:text-brand-200">OneDrive 配置</h3>

          <div className="p-3 bg-brand-100 dark:bg-brand-700 rounded-md border border-brand-300 dark:border-brand-600">
            <div className="flex items-start gap-2">
              <span className="i-mdi-information-outline text-lg text-warning-500 dark:text-brand-400 mt-0.5 flex-shrink-0" />
              <div className="text-xs text-brand-600 dark:text-brand-400 space-y-1">
                <p className="font-medium">注意：</p>
                <ul className="list-disc list-inside space-y-0.5 pl-2">
                  <li>默认使用项目提供的 Client ID，方便开箱即用</li>
                  <li>您的数据完全保存在您自己的 OneDrive 账户的应用文件夹中，其他人无法访问</li>
                  <li>所有敏感数据均保存在本地(如Refresh Token)，不会上传到云端</li>
                  <li>使用默认 Client ID 产生的任何问题，作者不承担责任</li>
                  <li>如需更高安全性，可在下方自定义 Client ID</li>
                </ul>
              </div>
            </div>
          </div>

          {/* Client ID 配置 */}
          <div className="space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              Client ID
              {(!formData.onedrive_client_id || formData.onedrive_client_id === "26fcab6e-41ea-49ff-8ec9-063983cae3ef")
                && <span className="ml-2 text-xs text-brand-500 dark:text-brand-400">(使用默认)</span>}
            </label>
            <input
              type="text"
              name="onedrive_client_id"
              value={formData.onedrive_client_id || ""}
              onChange={handleChange}
              placeholder="26fcab6e-41ea-49ff-8ec9-063983cae3ef (默认)"
              className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white font-mono text-sm"
            />
            <p className="text-xs text-brand-500 dark:text-brand-400">
              如需自定义，请在
              {" "}
              <a href="https://portal.azure.com/#view/Microsoft_AAD_RegisteredApps/ApplicationsListBlade" target="_blank" rel="noopener noreferrer" className="underline hover:text-brand-600 dark:hover:text-brand-300">Microsoft Entra</a>
              {" "}
              中注册应用
            </p>
          </div>

          <div className="space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">授权状态</label>
            {formData.onedrive_refresh_token
              ? (
                  <div className="flex items-center gap-2">
                    <span className="text-success-600 dark:text-success-400 flex items-center gap-1">
                      <span className="i-mdi-check-circle text-lg" />
                      已授权
                    </span>
                    <button type="button" onClick={() => onChange({ ...formData, onedrive_refresh_token: "" } as appconf.AppConfig)} className="px-2 py-1 text-xs text-error-600 hover:bg-error-100 dark:hover:bg-error-900 rounded">
                      取消授权
                    </button>
                  </div>
                )
              : (
                  <div className="space-y-3">
                    <button type="button" onClick={handleOneDriveAuth} disabled={authorizingOneDrive} className="glass-btn-neutral px-3 py-1.5 text-sm bg-neutral-600 text-white rounded-md hover:bg-neutral-700 disabled:opacity-50 flex items-center gap-2">
                      {authorizingOneDrive
                        ? (
                            <>
                              <span className="i-mdi-loading animate-spin" />
                              等待授权中...
                            </>
                          )
                        : (
                            <>
                              <span className="i-mdi-microsoft" />
                              授权 OneDrive
                            </>
                          )}
                    </button>
                    {authorizingOneDrive && <p className="text-xs text-brand-500 dark:text-brand-400">请在浏览器中完成授权，授权成功后会自动返回</p>}
                  </div>
                )}
          </div>
          {formData.onedrive_refresh_token && (
            <div className="flex justify-end">
              <button type="button" onClick={handleTestOneDrive} disabled={testingOneDrive} className="glass-btn-neutral px-3 py-1.5 text-sm bg-brand-100 text-brand-700 rounded-md hover:bg-brand-200 dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600 disabled:opacity-50">
                {testingOneDrive ? "测试中..." : "测试连接"}
              </button>
            </div>
          )}
        </div>
      )}

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">保留备份数量</label>
        <input
          type="number"
          value={formData.cloud_backup_retention || 5}
          onChange={e => onChange({ ...formData, cloud_backup_retention: Number.parseInt(e.target.value) || 20 } as appconf.AppConfig)}
          min={1}
          max={100}
          className="glass-input w-32 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">云端每个游戏保留的最大备份数量</p>
      </div>

      <PasswordInputModal
        isOpen={showPasswordModal}
        onClose={() => setShowPasswordModal(false)}
        onConfirm={handleSetupBackupPassword}
      />
    </div>
  );
}
