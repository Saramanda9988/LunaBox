import { useEffect, useState } from "react";
import { toast } from "react-hot-toast";
import { CheckForUpdatesOnStartup, SkipVersion } from "../../wailsjs/go/service/UpdateService";

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
    }
    catch (err) {
      toast.error(`Failed to skip version:${err}`);
    }
  };

  return {
    updateInfo,
    showUpdateDialog,
    setShowUpdateDialog,
    handleSkipVersion,
  };
}
