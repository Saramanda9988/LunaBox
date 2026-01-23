import { useEffect, useState } from "react";
import { toast } from "react-hot-toast";
import { CheckForUpdatesOnStartup, SkipVersion } from "../../bindings/lunabox/internal/service/UpdateService";

interface UpdateInfo {
  has_update: boolean;
  current_ver: string;
  latest_ver: string;
  release_date: string;
  changelog: string[];
  downloads: Record<string, string>;
}

export function useUpdateCheck() {
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null);
  const [showUpdateDialog, setShowUpdateDialog] = useState(false);

  useEffect(() => {
    const checkUpdate = async () => {
      try {
        const result = await CheckForUpdatesOnStartup();
        if (result?.has_update) {
          setUpdateInfo(result);
          setShowUpdateDialog(true);
        }
      }
      catch (err) {
        toast.error(`Failed to check updates on startup:${err}`);
      }
    };

    checkUpdate();
  }, []);

  const handleSkipVersion = async (version: string) => {
    try {
      await SkipVersion(version);
      // 更新 UI 状态：关闭对话框并标记为已处理
      setShowUpdateDialog(false);
      if (updateInfo) {
        setUpdateInfo({ ...updateInfo, has_update: false });
      }
    }
    catch (err) {
      toast.error(`Failed to skip version:${err}`);
    }
  };

  return {
    updateInfo,
    setUpdateInfo,
    showUpdateDialog,
    setShowUpdateDialog,
    handleSkipVersion,
  };
}
