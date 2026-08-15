import type { enums, service } from "../../../src/bindings/models";
import type { BetterDataTableColumn } from "../ui/better/BetterDataTable";
import { useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  ImportFromPlayniteWithSelection,
  ImportFromPotatoVNWithSelection,
  ImportFromReinaManagerWithSelection,
  ImportFromSteamLocalWithSelection,
  ImportFromVniteWithSelection,
  ImportFromYukiHubWithSelection,
  PreviewImport,
  PreviewPlayniteImport,
  PreviewReinaManagerImport,
  PreviewSteamLocalImport,
  PreviewVniteImport,
  PreviewYukiHubImport,
  SelectJSONFile,
  SelectReinaManagerDatabase,
  SelectVniteDirectory,
  SelectYukiHubBackup,
  SelectZipFile,
} from "../../../bindings/lunabox/internal/service/importservice";
import { vo } from "../../../src/bindings/models";
import playniteIconUrl from "../../assets/importers/playnite.png";
import potatovnIconUrl from "../../assets/importers/potatovn.png";
import reinaManagerIconUrl from "../../assets/importers/reinamanager.png";
import vniteIconUrl from "../../assets/importers/vnite.png";
import yukihubIconUrl from "../../assets/importers/yukihub.png";
import { BetterDataTable } from "../ui/better/BetterDataTable";
import { ModalPortal } from "../ui/ModalPortal";

export type ImportSource
  = | "playnite"
    | "potatovn"
    | "reinamanager"
    | "steam"
    | "vnite"
    | "yukihub";

interface GameImportModalProps {
  isOpen: boolean;
  source: ImportSource;
  onClose: () => void;
  onImportComplete: () => void;
}

type Step = "select" | "preview" | "importing" | "result";
type SamePathAction = "skip" | "merge_sessions" | "merge";

// 配置类型
interface ImportConfig {
  title: string;
  icon?: string;
  iconSrc?: string;
  fileType: string;
  fileDescription: string;
  fileHint: string;
  buttonText: string;
  primaryColor: string;
  hoverColor: string;
  skipNoPathByDefault?: boolean;
  selectFile: () => Promise<string>;
  previewImport: (path: string) => Promise<service.PreviewGame[]>;
  doImport: (
    path: string,
    skipNoPath: boolean,
    samePathAction: SamePathAction,
    selections: vo.ImportSelection[],
  ) => Promise<service.ImportResult>;
}

function getImportConfigs(t: any): Record<ImportSource, ImportConfig> {
  return {
    playnite: {
      title: t("gameImportModal.playnite.title"),
      icon: "i-mdi-application-import",
      iconSrc: playniteIconUrl,
      fileType: "JSON",
      fileDescription: t("gameImportModal.playnite.desc"),
      fileHint: t("gameImportModal.playnite.hint"),
      buttonText: t("gameImportModal.playnite.btn"),
      primaryColor: "bg-purple-500",
      hoverColor: "hover:bg-purple-600",
      selectFile: SelectJSONFile,
      previewImport: PreviewPlayniteImport,
      doImport: ImportFromPlayniteWithSelection,
    },
    potatovn: {
      title: t("gameImportModal.potatovn.title"),
      icon: "i-mdi-database-import",
      iconSrc: potatovnIconUrl,
      fileType: "ZIP",
      fileDescription: t("gameImportModal.potatovn.desc"),
      fileHint: t("gameImportModal.potatovn.hint"),
      buttonText: t("gameImportModal.potatovn.btn"),
      primaryColor: "bg-neutral-500",
      hoverColor: "hover:bg-neutral-600",
      selectFile: SelectZipFile,
      previewImport: PreviewImport,
      doImport: ImportFromPotatoVNWithSelection,
    },
    yukihub: {
      title: t("gameImportModal.yukihub.title"),
      iconSrc: yukihubIconUrl,
      fileType: "YKBAK",
      fileDescription: t("gameImportModal.yukihub.desc"),
      fileHint: t("gameImportModal.yukihub.hint"),
      buttonText: t("gameImportModal.yukihub.btn"),
      primaryColor: "bg-brand-600",
      hoverColor: "hover:bg-brand-700",
      skipNoPathByDefault: false,
      selectFile: SelectYukiHubBackup,
      previewImport: PreviewYukiHubImport,
      doImport: ImportFromYukiHubWithSelection,
    },
    vnite: {
      title: t("gameImportModal.vnite.title"),
      icon: "i-mdi-folder-cog-outline",
      iconSrc: vniteIconUrl,
      fileType: "DIR",
      fileDescription: t("gameImportModal.vnite.desc"),
      fileHint: t("gameImportModal.vnite.hint"),
      buttonText: t("gameImportModal.vnite.btn"),
      primaryColor: "bg-sky-500",
      hoverColor: "hover:bg-sky-600",
      selectFile: SelectVniteDirectory,
      previewImport: PreviewVniteImport,
      doImport: ImportFromVniteWithSelection,
    },
    reinamanager: {
      title: t("gameImportModal.reinamanager.title"),
      icon: "i-mdi-chess-queen",
      iconSrc: reinaManagerIconUrl,
      fileType: "DB",
      fileDescription: t("gameImportModal.reinamanager.desc"),
      fileHint: t("gameImportModal.reinamanager.hint"),
      buttonText: t("gameImportModal.reinamanager.btn"),
      primaryColor: "bg-rose-500",
      hoverColor: "hover:bg-rose-600",
      selectFile: SelectReinaManagerDatabase,
      previewImport: PreviewReinaManagerImport,
      doImport: ImportFromReinaManagerWithSelection,
    },
    steam: {
      title: t("gameImportModal.steam.title"),
      icon: "i-mdi-steam",
      fileType: "STEAM",
      fileDescription: t("gameImportModal.steam.desc"),
      fileHint: t("gameImportModal.steam.hint"),
      buttonText: t("gameImportModal.steam.btn"),
      primaryColor: "bg-slate-700",
      hoverColor: "hover:bg-slate-800",
      selectFile: async () => "steam-local",
      previewImport: () => PreviewSteamLocalImport(),
      doImport: (_path, skipNoPath, samePathAction, selections) =>
        ImportFromSteamLocalWithSelection(
          skipNoPath,
          samePathAction,
          selections,
        ),
    },
  };
}

function previewGameKey(game: service.PreviewGame, index: number) {
  return [
    game.source_type || "",
    game.source_id || "",
    game.path || "",
    game.name || "",
    String(index),
  ].join("\0");
}

function isPreviewGameActionable(
  game: service.PreviewGame,
  skipNoPath: boolean,
  samePathAction: SamePathAction,
) {
  if (game.conflict_type === "same_path") {
    return samePathAction !== "skip";
  }
  if (game.exists) {
    return false;
  }
  return !skipNoPath || game.has_path;
}

function toImportSelections(games: service.PreviewGame[]) {
  return games.map(
    game =>
      new vo.ImportSelection({
        name: game.name,
        path: game.path,
        source_type: game.source_type as enums.SourceType,
        source_id: game.source_id,
      }),
  );
}

export function GameImportModal({
  isOpen,
  source,
  onClose,
  onImportComplete,
}: GameImportModalProps) {
  const [step, setStep] = useState<Step>("select");
  const [filePath, setFilePath] = useState("");
  const [previewGames, setPreviewGames] = useState<service.PreviewGame[]>([]);
  const [selectedPreviewKeys, setSelectedPreviewKeys] = useState<Set<string>>(
    () => new Set(),
  );
  const [importResult, setImportResult] = useState<service.ImportResult | null>(
    null,
  );
  const [isLoading, setIsLoading] = useState(false);
  const [skipNoPath, setSkipNoPath] = useState(true);
  const [samePathAction, setSamePathAction] = useState<SamePathAction>("skip");
  const { t } = useTranslation();

  const config = getImportConfigs(t)[source];

  if (!isOpen)
    return null;

  const handleSelectFile = async () => {
    try {
      const path = await config.selectFile();
      if (path) {
        const nextSkipNoPath = config.skipNoPathByDefault ?? true;
        setFilePath(path);
        setSkipNoPath(nextSkipNoPath);
        setIsLoading(true);
        try {
          const games = await config.previewImport(path);
          const nextPreviewGames = games || [];
          setPreviewGames(nextPreviewGames);
          setSelectedPreviewKeys(
            new Set(
              nextPreviewGames
                .map((game, index) => ({ game, index }))
                .filter(({ game }) =>
                  isPreviewGameActionable(game, nextSkipNoPath, samePathAction),
                )
                .map(({ game, index }) => previewGameKey(game, index)),
            ),
          );
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
      const result = await config.doImport(
        filePath,
        skipNoPath,
        samePathAction,
        toImportSelections(
          previewGames.filter(
            (game, index) =>
              selectedPreviewKeys.has(previewGameKey(game, index))
              && isPreviewGameActionable(game, skipNoPath, samePathAction),
          ),
        ),
      );
      setImportResult(result);
      setStep("result");

      if (result.success > 0) {
        toast.success(
          t("gameImportModal.toast.importSuccess", { count: result.success }),
        );
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
    setSelectedPreviewKeys(new Set());
    setImportResult(null);
    setSkipNoPath(true);
    setSamePathAction("skip");
    onClose();
  };

  const samePathGamesCount = previewGames.filter(
    g => g.conflict_type === "same_path",
  ).length;
  const shouldMergeSamePath = samePathAction !== "skip";
  const isRowActionable = (game: service.PreviewGame) =>
    isPreviewGameActionable(game, skipNoPath, samePathAction);
  const isRowSelected = (game: service.PreviewGame, index: number) =>
    selectedPreviewKeys.has(previewGameKey(game, index));
  const newGamesCount = previewGames.filter(
    (g, index) => !g.exists && isRowActionable(g) && isRowSelected(g, index),
  ).length;
  const updateGamesCount = previewGames.filter(
    (g, index) =>
      shouldMergeSamePath
      && g.conflict_type === "same_path"
      && isRowSelected(g, index),
  ).length;
  const actionableGamesCount = newGamesCount + updateGamesCount;
  const existingGamesCount = previewGames.filter(
    g => g.exists && g.conflict_type !== "same_path",
  ).length;
  const noPathGamesCount = previewGames.filter(
    g => !g.has_path && !g.exists,
  ).length;
  const togglePreviewGame = (
    game: service.PreviewGame,
    index: number,
    checked: boolean,
  ) => {
    if (!isRowActionable(game)) {
      return;
    }
    const key = previewGameKey(game, index);
    setSelectedPreviewKeys((current) => {
      const next = new Set(current);
      if (checked) {
        next.add(key);
      }
      else {
        next.delete(key);
      }
      return next;
    });
  };
  const toggleAllPreviewGames = (checked: boolean) => {
    setSelectedPreviewKeys(() => {
      if (!checked) {
        return new Set();
      }
      return new Set(
        previewGames
          .map((game, index) => ({ game, index }))
          .filter(({ game }) => isRowActionable(game))
          .map(({ game, index }) => previewGameKey(game, index)),
      );
    });
  };

  // 动态颜色类
  const buttonPrimaryClass = `${config.primaryColor} ${config.hoverColor}`;
  const iconColorClass
    = source === "playnite"
      ? "text-purple-500"
      : source === "reinamanager"
        ? "text-rose-500"
        : source === "vnite"
          ? "text-sky-500"
          : source === "yukihub"
            ? "text-brand-600 dark:text-brand-300"
            : source === "steam"
              ? "text-slate-600 dark:text-slate-300"
              : "text-neutral-500";
  const spinnerColorClass = iconColorClass;
  const resultButtonClass
    = source === "playnite"
      ? "bg-purple-600 hover:bg-purple-700"
      : source === "reinamanager"
        ? "bg-rose-600 hover:bg-rose-700"
        : source === "vnite"
          ? "bg-sky-600 hover:bg-sky-700"
          : source === "yukihub"
            ? "bg-brand-600 hover:bg-brand-700"
            : source === "steam"
              ? "bg-slate-700 hover:bg-slate-800"
              : "bg-neutral-600 hover:bg-neutral-700";
  const importButtonClass = resultButtonClass;
  const statusBadgeClass
    = "inline-flex min-w-[4.5rem] items-center justify-center gap-1.5 whitespace-nowrap rounded-full px-3 py-1.5 text-xs font-medium leading-none";
  const statusIconClass = "flex-shrink-0 text-sm";
  const previewColumns: BetterDataTableColumn<service.PreviewGame>[] = [
    {
      key: "name",
      header: t("gameImportModal.gameName"),
      className: "w-[46%]",
      render: game => (
        <div className="min-w-0">
          <span className="block truncate font-medium text-brand-900 dark:text-white">
            {game.name}
          </span>
          {!game.has_path && (
            <span className="mt-1 inline-flex text-xs text-orange-500">
              {t("gameImportModal.noPathLabel")}
            </span>
          )}
        </div>
      ),
    },
    {
      key: "developer",
      header: t("gameImportModal.developer"),
      className: "w-[30%]",
      render: game => (
        <span className="block truncate text-brand-500 dark:text-brand-400">
          {game.developer || "-"}
        </span>
      ),
    },
    {
      key: "status",
      header: t("common.status"),
      className: "w-36",
      headerClassName: "text-center",
      cellClassName: "text-center",
      render: (game) => {
        const willBeUpdated
          = game.conflict_type === "same_path" && shouldMergeSamePath;
        if (game.exists) {
          return (
            <span
              className={
                willBeUpdated
                  ? `${statusBadgeClass} bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-400`
                  : `${statusBadgeClass} bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400`
              }
            >
              <div
                className={`${willBeUpdated ? "i-mdi-database-sync-outline" : "i-mdi-check-circle"} ${statusIconClass}`}
              />
              {willBeUpdated
                ? t("gameImportModal.status.update")
                : t("gameImportModal.exists")}
            </span>
          );
        }
        if (!game.has_path && skipNoPath) {
          return (
            <span
              className={`${statusBadgeClass} bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400`}
            >
              <div className={`i-mdi-close-circle ${statusIconClass}`} />
              {t("gameImportModal.status.skipped")}
            </span>
          );
        }
        return (
          <span
            className={`${statusBadgeClass} bg-success-100 text-success-700 dark:bg-success-900/30 dark:text-success-400`}
          >
            <div className={`i-mdi-plus-circle ${statusIconClass}`} />
            {t("gameImportModal.newAdd")}
          </span>
        );
      },
    },
  ];

  return (
    <ModalPortal>
      <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
        <div className="w-full max-w-3xl max-h-[90vh] rounded-xl bg-white shadow-2xl dark:bg-brand-800 flex flex-col">
          {/* Header */}
          <div className="flex items-center justify-between p-6 border-b border-brand-200 dark:border-brand-700">
            <div className="flex items-center gap-3">
              {config.iconSrc ? (
                <img
                  src={config.iconSrc}
                  alt=""
                  className="h-9 w-9 shrink-0 object-contain"
                />
              ) : (
                <div className={`${config.icon} text-3xl ${iconColorClass}`} />
              )}
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
                  <div className="i-mdi-application-import mx-auto mb-4 text-6xl text-brand-400" />
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
                  {isLoading ? (
                    <>
                      <div className="i-mdi-loading animate-spin mr-2 text-xl" />
                      {t("common.loading")}
                    </>
                  ) : (
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
                  {samePathGamesCount > 0 && (
                    <div className="flex-1 rounded-lg bg-sky-50 dark:bg-sky-900/20 p-4 text-center">
                      <div className="text-3xl font-bold text-sky-600 dark:text-sky-400">
                        {samePathGamesCount}
                      </div>
                      <div className="text-sm text-sky-700 dark:text-sky-300">
                        {t("gameImportModal.samePath")}
                      </div>
                    </div>
                  )}
                </div>

                {samePathGamesCount > 0 && (
                  <div className="rounded-lg border border-sky-200 bg-sky-50 p-4 dark:border-sky-800 dark:bg-sky-900/20">
                    <div className="mb-3 text-sm font-medium text-sky-800 dark:text-sky-200">
                      {t("gameImportModal.samePathActionTitle", {
                        count: samePathGamesCount,
                      })}
                    </div>
                    <div className="grid gap-2 sm:grid-cols-3">
                      <button
                        type="button"
                        onClick={() => setSamePathAction("skip")}
                        className={`rounded-lg border px-3 py-2 text-left text-sm transition ${
                          samePathAction === "skip"
                            ? "border-sky-500 bg-white text-sky-800 shadow-sm dark:bg-sky-950/40 dark:text-sky-100"
                            : "border-sky-200 bg-white/60 text-sky-700 hover:bg-white dark:border-sky-800 dark:bg-sky-950/20 dark:text-sky-300"
                        }`}
                      >
                        <div className="flex items-center gap-2 font-medium">
                          <div className="i-mdi-skip-next-circle text-base" />
                          {t("gameImportModal.samePathSkip")}
                        </div>
                        <div className="mt-1 text-xs opacity-80">
                          {t("gameImportModal.samePathSkipHint")}
                        </div>
                      </button>
                      <button
                        type="button"
                        onClick={() => setSamePathAction("merge_sessions")}
                        className={`rounded-lg border px-3 py-2 text-left text-sm transition ${
                          samePathAction === "merge_sessions"
                            ? "border-sky-500 bg-white text-sky-800 shadow-sm dark:bg-sky-950/40 dark:text-sky-100"
                            : "border-sky-200 bg-white/60 text-sky-700 hover:bg-white dark:border-sky-800 dark:bg-sky-950/20 dark:text-sky-300"
                        }`}
                      >
                        <div className="flex items-center gap-2 font-medium">
                          <div className="i-mdi-history text-base" />
                          {t("gameImportModal.samePathMergeSessions")}
                        </div>
                        <div className="mt-1 text-xs opacity-80">
                          {t("gameImportModal.samePathMergeSessionsHint")}
                        </div>
                      </button>
                      <button
                        type="button"
                        onClick={() => setSamePathAction("merge")}
                        className={`rounded-lg border px-3 py-2 text-left text-sm transition ${
                          samePathAction === "merge"
                            ? "border-sky-500 bg-white text-sky-800 shadow-sm dark:bg-sky-950/40 dark:text-sky-100"
                            : "border-sky-200 bg-white/60 text-sky-700 hover:bg-white dark:border-sky-800 dark:bg-sky-950/20 dark:text-sky-300"
                        }`}
                      >
                        <div className="flex items-center gap-2 font-medium">
                          <div className="i-mdi-database-sync-outline text-base" />
                          {t("gameImportModal.samePathMerge")}
                        </div>
                        <div className="mt-1 text-xs opacity-80">
                          {t("gameImportModal.samePathMergeHint")}
                        </div>
                      </button>
                    </div>
                  </div>
                )}

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
                          {t("gameImportModal.skipNoPathHint1", {
                            count: noPathGamesCount,
                          })}
                          <br />
                          {skipNoPath
                            ? t("gameImportModal.skipNoPathHintUncheck")
                            : t("gameImportModal.skipNoPathHintCheck")}
                        </div>
                      </div>
                    </label>
                  </div>
                )}

                <BetterDataTable
                  rows={previewGames}
                  columns={previewColumns}
                  rowKey={previewGameKey}
                  empty={t("gameImportModal.noGameData")}
                  maxHeightClassName="max-h-[300px]"
                  rowClassName={(game, index) =>
                    !isRowActionable(game)
                      ? "opacity-45"
                      : isRowSelected(game, index)
                        ? ""
                        : "opacity-65"}
                  selection={{
                    selectedKeys: selectedPreviewKeys,
                    isRowSelectable: isRowActionable,
                    onToggleRow: togglePreviewGame,
                    onToggleAll: toggleAllPreviewGames,
                    allLabel: t("common.selectAll"),
                    rowLabel: game => game.name,
                  }}
                />

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
                    disabled={actionableGamesCount === 0}
                    className={`rounded-lg px-5 py-2.5 text-sm font-medium text-white disabled:opacity-50 ${importButtonClass}`}
                  >
                    {t("gameImportModal.importCount", {
                      count: actionableGamesCount,
                    })}
                  </button>
                </div>
              </div>
            )}

            {/* Step: Importing */}
            {step === "importing" && (
              <div className="py-12 text-center">
                <div
                  className={`i-mdi-loading animate-spin text-5xl mx-auto mb-4 ${spinnerColorClass}`}
                />
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
                    <div className="text-sm text-success-700 dark:text-success-300">
                      {t("gameImportModal.result.success")}
                    </div>
                  </div>
                  {importResult.skipped > 0 && (
                    <div className="flex-1 rounded-lg bg-yellow-50 dark:bg-yellow-900/20 p-4 text-center">
                      <div className="i-mdi-skip-next-circle text-3xl text-yellow-500 mx-auto mb-2" />
                      <div className="text-2xl font-bold text-yellow-600 dark:text-yellow-400">
                        {importResult.skipped}
                      </div>
                      <div className="text-sm text-yellow-700 dark:text-yellow-300">
                        {t("gameImportModal.result.skipped")}
                      </div>
                    </div>
                  )}
                  {importResult.failed > 0 && (
                    <div className="flex-1 rounded-lg bg-error-50 dark:bg-error-900/20 p-4 text-center">
                      <div className="i-mdi-close-circle text-3xl text-error-500 mx-auto mb-2" />
                      <div className="text-2xl font-bold text-error-600 dark:text-error-400">
                        {importResult.failed}
                      </div>
                      <div className="text-sm text-error-700 dark:text-error-300">
                        {t("gameImportModal.result.failed")}
                      </div>
                    </div>
                  )}
                </div>

                {/* Skipped Names */}
                {importResult.skipped_names
                  && importResult.skipped_names.length > 0 && (
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
                {importResult.failed_names
                  && importResult.failed_names.length > 0 && (
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
    </ModalPortal>
  );
}
