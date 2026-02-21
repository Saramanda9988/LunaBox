import type { service } from "../../../wailsjs/go/models";
import { useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  ImportFromPlaynite,
  ImportFromPotatoVN,
  PreviewImport,
  PreviewPlayniteImport,
  SelectJSONFile,
  SelectZipFile,
} from "../../../wailsjs/go/service/ImportService";

export type ImportSource = "playnite" | "potatovn";

interface GameImportModalProps {
  isOpen: boolean;
  source: ImportSource;
  onClose: () => void;
  onImportComplete: () => void;
}

type Step = "select" | "preview" | "importing" | "result";

// 配置类型
interface ImportConfig {
  title: string;
  icon: string;
  fileType: string;
  fileDescription: string;
  fileHint: string;
  buttonText: string;
  primaryColor: string;
  hoverColor: string;
  selectFile: () => Promise<string>;
  previewImport: (path: string) => Promise<service.PreviewGame[]>;
  doImport: (path: string, skipNoPath: boolean) => Promise<service.ImportResult>;
}

function getImportConfigs(t: any): Record<ImportSource, ImportConfig> {
  return {
    playnite: {
      title: t("gameImportModal.playnite.title"),
      icon: "i-mdi-application-import",
      fileType: "JSON",
      fileDescription: t("gameImportModal.playnite.desc"),
      fileHint: t("gameImportModal.playnite.hint"),
      buttonText: t("gameImportModal.playnite.btn"),
      primaryColor: "bg-purple-500",
      hoverColor: "hover:bg-purple-600",
      selectFile: SelectJSONFile,
      previewImport: PreviewPlayniteImport,
      doImport: ImportFromPlaynite,
    },
    potatovn: {
      title: t("gameImportModal.potatovn.title"),
      icon: "i-mdi-database-import",
      fileType: "ZIP",
      fileDescription: t("gameImportModal.potatovn.desc"),
      fileHint: t("gameImportModal.potatovn.hint"),
      buttonText: t("gameImportModal.potatovn.btn"),
      primaryColor: "bg-neutral-500",
      hoverColor: "hover:bg-neutral-600",
      selectFile: SelectZipFile,
      previewImport: PreviewImport,
      doImport: ImportFromPotatoVN,
    },
  };
}

export function GameImportModal({ isOpen, source, onClose, onImportComplete }: GameImportModalProps) {
  const [step, setStep] = useState<Step>("select");
  const [filePath, setFilePath] = useState("");
  const [previewGames, setPreviewGames] = useState<service.PreviewGame[]>([]);
  const [importResult, setImportResult] = useState<service.ImportResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [skipNoPath, setSkipNoPath] = useState(true);
  const { t } = useTranslation();

  const config = getImportConfigs(t)[source];

  if (!isOpen)
    return null;

  const handleSelectFile = async () => {
    try {
      const path = await config.selectFile();
      if (path) {
        setFilePath(path);
        setIsLoading(true);
        try {
          const games = await config.previewImport(path);
          setPreviewGames(games || []);
          setStep("preview");
        }
        catch (error) {
          console.error("Failed to preview import:", error);
          toast.error(t("gameImportModal.toast.previewFailed"));
        }
        finally {
          setIsLoading(false);
        }
      }
    }
    catch (error) {
      console.error("Failed to select file:", error);
      toast.error(t("gameImportModal.toast.selectFileFailed"));
    }
  };

  const handleImport = async () => {
    if (!filePath)
      return;

    setStep("importing");
    setIsLoading(true);

    try {
      const result = await config.doImport(filePath, skipNoPath);
      setImportResult(result);
      setStep("result");

      if (result.success > 0) {
        toast.success(t("gameImportModal.toast.importSuccess", { count: result.success }));
        onImportComplete();
      }
    }
    catch (error) {
      console.error("Failed to import:", error);
      toast.error(t("gameImportModal.toast.importFailed"));
      setStep("preview");
    }
    finally {
      setIsLoading(false);
    }
  };

  const resetAndClose = () => {
    setStep("select");
    setFilePath("");
    setPreviewGames([]);
    setImportResult(null);
    setSkipNoPath(true);
    onClose();
  };

  const newGamesCount = previewGames.filter(g => !g.exists && (skipNoPath ? g.has_path : true)).length;
  const existingGamesCount = previewGames.filter(g => g.exists).length;
  const noPathGamesCount = previewGames.filter(g => !g.has_path && !g.exists).length;

  // 动态颜色类
  const buttonPrimaryClass = `${config.primaryColor} ${config.hoverColor}`;
  const iconColorClass = source === "playnite" ? "text-purple-500" : "text-neutral-500";
  const spinnerColorClass = source === "playnite" ? "text-purple-500" : "text-neutral-500";
  const resultButtonClass = source === "playnite"
    ? "bg-purple-600 hover:bg-purple-700"
    : "bg-neutral-600 hover:bg-neutral-700";
  const importButtonClass = source === "playnite"
    ? "bg-purple-600 hover:bg-purple-700"
    : "bg-neutral-600 hover:bg-neutral-700";

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
      <div className="w-full max-w-3xl max-h-[90vh] rounded-xl bg-white shadow-2xl dark:bg-brand-800 flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-brand-200 dark:border-brand-700">
          <div className="flex items-center gap-3">
            <div className={`${config.icon} text-3xl ${iconColorClass}`} />
            <h2 className="text-2xl font-bold text-brand-900 dark:text-white">
              {config.title}
            </h2>
          </div>
          <button
            type="button"
            onClick={resetAndClose}
            className="i-mdi-close text-2xl text-brand-500 p-1 rounded-lg
              hover:bg-brand-100 hover:text-brand-700 focus:outline-none
              dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-200"
          />
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6">
          {/* Step: Select File */}
          {step === "select" && (
            <div className="space-y-6">
              <div className="text-center py-8">
                <div className={`${source === "playnite" ? "i-mdi-file-document" : "i-mdi-folder-zip"} text-6xl text-brand-400 mx-auto mb-4`} />
                <p className="text-brand-600 dark:text-brand-300 mb-2">
                  {config.fileDescription}
                </p>
                <p className="text-sm text-brand-400 dark:text-brand-500">
                  {config.fileHint}
                </p>
              </div>

              <button
                type="button"
                onClick={handleSelectFile}
                disabled={isLoading}
                className={`flex w-full items-center justify-center rounded-lg py-4 text-white transition disabled:opacity-50 ${buttonPrimaryClass}`}
              >
                {isLoading
                  ? (
                      <>
                        <div className="i-mdi-loading animate-spin mr-2 text-xl" />
                        {t("common.loading")}
                      </>
                    )
                  : (
                      <>
                        <div className="i-mdi-file-find mr-2 text-xl" />
                        {config.buttonText}
                      </>
                    )}
              </button>
            </div>
          )}

          {/* Step: Preview */}
          {step === "preview" && (
            <div className="space-y-4">
              {/* Summary */}
              <div className="flex gap-4">
                <div className="flex-1 rounded-lg bg-success-50 dark:bg-success-900/20 p-4 text-center">
                  <div className="text-3xl font-bold text-success-600 dark:text-success-400">
                    {newGamesCount}
                  </div>
                  <div className="text-sm text-success-700 dark:text-success-300">
                    {t("gameImportModal.willImport")}
                  </div>
                </div>
                <div className="flex-1 rounded-lg bg-yellow-50 dark:bg-yellow-900/20 p-4 text-center">
                  <div className="text-3xl font-bold text-yellow-600 dark:text-yellow-400">
                    {existingGamesCount}
                  </div>
                  <div className="text-sm text-yellow-700 dark:text-yellow-300">
                    {t("gameImportModal.exists")}
                  </div>
                </div>
                {noPathGamesCount > 0 && (
                  <div className="flex-1 rounded-lg bg-orange-50 dark:bg-orange-900/20 p-4 text-center">
                    <div className="text-3xl font-bold text-orange-600 dark:text-orange-400">
                      {noPathGamesCount}
                    </div>
                    <div className="text-sm text-orange-700 dark:text-orange-300">
                      {t("gameImportModal.noPath")}
                    </div>
                  </div>
                )}
              </div>

              {/* Skip no-path option */}
              {noPathGamesCount > 0 && (
                <div className="rounded-lg bg-orange-50 dark:bg-orange-900/20 p-4">
                  <label className="flex items-start cursor-pointer">
                    <input
                      type="checkbox"
                      checked={skipNoPath}
                      onChange={e => setSkipNoPath(e.target.checked)}
                      className="mt-1 mr-3"
                    />
                    <div>
                      <div className="text-sm font-medium text-orange-700 dark:text-orange-300">
                        {t("gameImportModal.skipNoPath")}
                      </div>
                      <div className="text-xs text-orange-600 dark:text-orange-400 mt-1">
                        {t("gameImportModal.skipNoPathHint1", { count: noPathGamesCount })}
                        <br />
                        {skipNoPath
                          ? t("gameImportModal.skipNoPathHintUncheck")
                          : t("gameImportModal.skipNoPathHintCheck")}
                      </div>
                    </div>
                  </label>
                </div>
              )}

              {/* Game List */}
              <div className="max-h-[300px] overflow-y-auto rounded-lg border border-brand-200 dark:border-brand-700">
                {previewGames.length === 0
                  ? (
                      <div className="p-8 text-center text-brand-400">
                        {t("gameImportModal.noGameData")}
                      </div>
                    )
                  : (
                      <table className="w-full">
                        <thead className="top-0 bg-brand-50 dark:bg-brand-700">
                          <tr>
                            <th className="px-4 py-2 text-left text-sm font-medium text-brand-600 dark:text-brand-300">
                              {t("gameImportModal.gameName")}
                            </th>
                            <th className="px-4 py-2 text-left text-sm font-medium text-brand-600 dark:text-brand-300">
                              {t("gameImportModal.developer")}
                            </th>
                            <th className="px-4 py-2 text-center text-sm font-medium text-brand-600 dark:text-brand-300">
                              {t("common.status")}
                            </th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-brand-100 dark:divide-brand-700">
                          {previewGames.map((game, index) => {
                            const willBeSkipped = game.exists || (skipNoPath && !game.has_path);
                            return (
                              <tr
                                key={game.name + index}
                                className={`${willBeSkipped
                                  ? "opacity-50"
                                  : "hover:bg-brand-50 dark:hover:bg-brand-750"
                                }`}
                              >
                                <td className="px-4 py-3 text-sm text-brand-900 dark:text-white">
                                  {game.name}
                                  {!game.has_path && (
                                    <span className="ml-2 text-xs text-orange-500">
                                      {t("gameImportModal.noPathLabel")}
                                    </span>
                                  )}
                                </td>
                                <td className="px-4 py-3 text-sm text-brand-500 dark:text-brand-400">
                                  {game.developer || "-"}
                                </td>
                                <td className="px-4 py-3 text-center">
                                  {game.exists
                                    ? (
                                        <span className="inline-flex items-center rounded-full bg-yellow-100 px-2 py-1 text-xs text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400">
                                          <div className="i-mdi-check-circle mr-1" />
                                          已存在
                                        </span>
                                      )
                                    : !game.has_path && skipNoPath
                                        ? (
                                            <span className="inline-flex items-center rounded-full bg-orange-100 px-2 py-1 text-xs text-orange-700 dark:bg-orange-900/30 dark:text-orange-400">
                                              <div className="i-mdi-close-circle mr-1" />
                                              {t("gameImportModal.status.skipped")}
                                            </span>
                                          )
                                        : (
                                            <span className="inline-flex items-center rounded-full bg-success-100 px-2 py-1 text-xs text-success-700 dark:bg-success-900/30 dark:text-success-400">
                                              <div className="i-mdi-plus-circle mr-1" />
                                              新增
                                            </span>
                                          )}
                                </td>
                              </tr>
                            );
                          })}
                        </tbody>
                      </table>
                    )}
              </div>

              {/* Actions */}
              <div className="flex justify-between">
                <button
                  type="button"
                  onClick={() => setStep("select")}
                  className="rounded-lg border border-brand-300 px-5 py-2.5 text-sm font-medium text-brand-700 hover:bg-brand-100 dark:border-brand-600 dark:text-brand-300 dark:hover:bg-brand-700"
                >
                  &larr;
                  {" "}
                  {t("gameImportModal.reselect")}
                </button>
                <button
                  type="button"
                  onClick={handleImport}
                  disabled={newGamesCount === 0}
                  className={`rounded-lg px-5 py-2.5 text-sm font-medium text-white disabled:opacity-50 ${importButtonClass}`}
                >
                  {t("gameImportModal.importCount", { count: newGamesCount })}
                </button>
              </div>
            </div>
          )}

          {/* Step: Importing */}
          {step === "importing" && (
            <div className="py-12 text-center">
              <div className={`i-mdi-loading animate-spin text-5xl mx-auto mb-4 ${spinnerColorClass}`} />
              <p className="text-lg text-brand-600 dark:text-brand-300">
                {t("gameImportModal.importing")}
              </p>
              <p className="text-sm text-brand-400 dark:text-brand-500 mt-2">
                {t("gameImportModal.importHint")}
              </p>
            </div>
          )}

          {/* Step: Result */}
          {step === "result" && importResult && (
            <div className="space-y-6">
              {/* Result Summary */}
              <div className="flex gap-4">
                <div className="flex-1 rounded-lg bg-success-50 dark:bg-success-900/20 p-4 text-center">
                  <div className="i-mdi-check-circle text-3xl text-success-500 mx-auto mb-2" />
                  <div className="text-2xl font-bold text-success-600 dark:text-success-400">
                    {importResult.success}
                  </div>
                  <div className="text-sm text-success-700 dark:text-success-300">{t("gameImportModal.result.success")}</div>
                </div>
                {importResult.skipped > 0 && (
                  <div className="flex-1 rounded-lg bg-yellow-50 dark:bg-yellow-900/20 p-4 text-center">
                    <div className="i-mdi-skip-next-circle text-3xl text-yellow-500 mx-auto mb-2" />
                    <div className="text-2xl font-bold text-yellow-600 dark:text-yellow-400">
                      {importResult.skipped}
                    </div>
                    <div className="text-sm text-yellow-700 dark:text-yellow-300">{t("gameImportModal.result.skipped")}</div>
                  </div>
                )}
                {importResult.failed > 0 && (
                  <div className="flex-1 rounded-lg bg-error-50 dark:bg-error-900/20 p-4 text-center">
                    <div className="i-mdi-close-circle text-3xl text-error-500 mx-auto mb-2" />
                    <div className="text-2xl font-bold text-error-600 dark:text-error-400">
                      {importResult.failed}
                    </div>
                    <div className="text-sm text-error-700 dark:text-error-300">{t("gameImportModal.result.failed")}</div>
                  </div>
                )}
              </div>

              {/* Skipped Names */}
              {importResult.skipped_names && importResult.skipped_names.length > 0 && (
                <div className="rounded-lg border border-yellow-200 dark:border-yellow-800 p-4">
                  <h4 className="font-medium text-yellow-700 dark:text-yellow-400 mb-2">
                    {t("gameImportModal.skippedGames")}
                    :
                  </h4>
                  <div className="max-h-[150px] overflow-y-auto">
                    <ul className="text-sm text-yellow-600 dark:text-yellow-300 space-y-1">
                      {importResult.skipped_names.map((name, i) => (
                        <li key={name + i}>
                          •
                          {name}
                        </li>
                      ))}
                    </ul>
                  </div>
                </div>
              )}

              {/* Failed Names */}
              {importResult.failed_names && importResult.failed_names.length > 0 && (
                <div className="rounded-lg border border-error-200 dark:border-error-800 p-4">
                  <h4 className="font-medium text-error-700 dark:text-error-400 mb-2">
                    {t("gameImportModal.failedGames")}
                    :
                  </h4>
                  <ul className="text-sm text-error-600 dark:text-error-300 space-y-1">
                    {importResult.failed_names.map((name, i) => (
                      <li key={name + i}>
                        •
                        {name}
                      </li>
                    ))}
                  </ul>
                </div>
              )}

              {/* Close Button */}
              <div className="flex justify-center">
                <button
                  type="button"
                  onClick={resetAndClose}
                  className={`rounded-lg px-8 py-2.5 text-sm font-medium text-white ${resultButtonClass}`}
                >
                  {t("common.complete")}
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
