import { useEffect, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { GetRunningProcesses } from "../../../wailsjs/go/service/GameService";
import { CancelProcessSelection, NotifyProcessSelected } from "../../../wailsjs/go/service/StartService";

interface ProcessInfo {
  name: string;
  pid: number;
}

interface ProcessSelectModalProps {
  isOpen: boolean;
  gameID: string;
  launcherExeName: string;
  onClose: () => void;
  onSelected: (processName: string) => void;
}

export function ProcessSelectModal({
  isOpen,
  gameID,
  launcherExeName,
  onClose,
  onSelected,
}: ProcessSelectModalProps) {
  const { t } = useTranslation();
  const [processes, setProcesses] = useState<ProcessInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchTerm, setSearchTerm] = useState("");
  const [selectedProcess, setSelectedProcess] = useState<string | null>(null);

  const loadProcesses = async () => {
    setLoading(true);
    try {
      const result = await GetRunningProcesses();
      setProcesses(result || []);
    }
    catch (error) {
      console.error("Failed to load processes:", error);
      toast.error(t("processSelectModal.toast.loadFailed"));
    }
    finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen) {
      loadProcesses();
      setSelectedProcess(null);
      setSearchTerm("");
    }
  }, [isOpen]);

  const filteredProcesses = processes.filter(p =>
    p.name.toLowerCase().includes(searchTerm.toLowerCase()),
  );

  const handleClose = async () => {
    try {
      await CancelProcessSelection(gameID);
    }
    catch (error) {
      console.error("Failed to cancel process selection:", error);
    }
    finally {
      onClose();
    }
  };

  const handleConfirm = async () => {
    if (!selectedProcess) {
      toast.error(t("processSelectModal.toast.selectProcess"));
      return;
    }

    try {
      await NotifyProcessSelected(gameID, selectedProcess);
      toast.success(t("processSelectModal.toast.setSuccess", { name: selectedProcess }));
      onSelected(selectedProcess);
      onClose();
    }
    catch (error) {
      console.error("Failed to update process name:", error);
      toast.error(t("processSelectModal.toast.saveFailed"));
    }
  };

  if (!isOpen)
    return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className="w-full max-w-lg rounded-xl bg-white p-6 shadow-xl dark:bg-brand-800 border border-brand-200 dark:border-brand-700">
        {/* Title */}
        <div className="flex items-start gap-4 mb-4">
          <div className="p-2 rounded-full bg-warning-100 text-warning-600 dark:bg-warning-900/30 dark:text-warning-400">
            <div className="i-mdi-application-cog text-2xl" />
          </div>
          <div className="flex-1">
            <h3 className="text-xl font-bold text-brand-900 dark:text-white mb-1">
              {t("processSelectModal.title")}
            </h3>
            <p className="text-brand-600 dark:text-brand-400 text-sm leading-relaxed">
              {t("processSelectModal.desc", { launcher: launcherExeName })}
            </p>
          </div>
        </div>

        {/* Search */}
        <div className="mb-4">
          <div className="relative">
            <div className="absolute left-3 top-1/2 -translate-y-1/2 i-mdi-magnify text-brand-400" />
            <input
              type="text"
              value={searchTerm}
              onChange={e => setSearchTerm(e.target.value)}
              placeholder={t("processSelectModal.searchPlaceholder")}
              className="w-full pl-10 pr-4 py-2 rounded-lg border border-brand-200 dark:border-brand-600 bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-primary-500"
            />
          </div>
        </div>

        {/* Process List */}
        <div className="h-64 overflow-y-auto rounded-lg border border-brand-200 dark:border-brand-600 bg-brand-50 dark:bg-brand-900">
          {loading
            ? (
                <div className="flex items-center justify-center h-full">
                  <div className="i-mdi-loading animate-spin text-2xl text-primary-500" />
                  <span className="ml-2 text-brand-600 dark:text-brand-400">{t("processSelectModal.loading")}</span>
                </div>
              )
            : filteredProcesses.length === 0
              ? (
                  <div className="flex items-center justify-center h-full text-brand-500 dark:text-brand-400">
                    {searchTerm ? t("processSelectModal.noMatch") : t("processSelectModal.noProcesses")}
                  </div>
                )
              : (
                  <div className="divide-y divide-brand-200 dark:divide-brand-700">
                    {filteredProcesses.map(process => (
                      <button
                        type="button"
                        key={`${process.name}-${process.pid}`}
                        onClick={() => setSelectedProcess(process.name)}
                        className={`w-full flex items-center justify-between px-4 py-3 text-left transition-colors ${selectedProcess === process.name
                          ? "bg-primary-100 dark:bg-primary-900/30"
                          : "hover:bg-brand-100 dark:hover:bg-brand-800"
                        }`}
                      >
                        <div className="flex items-center gap-3">
                          <div className={`i-mdi-application text-lg ${selectedProcess === process.name
                            ? "text-primary-600 dark:text-primary-400"
                            : "text-brand-500 dark:text-brand-400"
                          }`}
                          />
                          <span className={`font-mono text-sm ${selectedProcess === process.name
                            ? "text-primary-700 dark:text-primary-300 font-medium"
                            : "text-brand-700 dark:text-brand-300"
                          }`}
                          >
                            {process.name}
                          </span>
                        </div>
                        <span className="text-xs text-brand-400 dark:text-brand-500">
                          PID:
                          {process.pid}
                        </span>
                      </button>
                    ))}
                  </div>
                )}
        </div>

        {/* Refresh */}
        <div className="mt-2 flex justify-start">
          <button
            type="button"
            onClick={loadProcesses}
            disabled={loading}
            className="flex items-center gap-1 text-sm text-brand-600 hover:text-primary-600 dark:text-brand-400 dark:hover:text-primary-400 transition-colors disabled:opacity-50"
          >
            <div className={`i-mdi-refresh ${loading ? "animate-spin" : ""}`} />
            {t("processSelectModal.refreshBtn")}
          </button>
        </div>

        {/* Buttons */}
        <div className="flex justify-end gap-3 mt-6">
          <button
            type="button"
            onClick={handleClose}
            className="px-4 py-2 text-sm font-medium text-brand-700 hover:bg-brand-100 rounded-lg dark:text-brand-300 dark:hover:bg-brand-700 transition-colors"
          >
            {t("processSelectModal.cancelBtn")}
          </button>
          <button
            type="button"
            onClick={handleConfirm}
            disabled={!selectedProcess}
            className="px-4 py-2 text-sm font-medium text-white bg-primary-600 hover:bg-primary-700 rounded-lg shadow-sm shadow-primary-200 dark:shadow-none transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {t("processSelectModal.confirmBtn")}
          </button>
        </div>
      </div>
    </div>
  );
}
