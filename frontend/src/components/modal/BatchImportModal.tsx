import type { service } from "../../../wailsjs/go/models";
import type { ImportCandidate, MatchProgressState } from "../ui/import/types";
import { useRef, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";

import { enums, vo } from "../../../wailsjs/go/models";
import {
  FetchMetadataByName,
  FetchMetadataFromWeb,
} from "../../../wailsjs/go/service/GameService";
import {
  BatchImportGames,
  ScanLibraryDirectoryWithOptions,
  SelectLibraryDirectory,
} from "../../../wailsjs/go/service/ImportService";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterDropdownMenu } from "../ui/better/BetterDropdownMenu";
import { ImportManualSelectModal } from "../ui/import/ImportManualSelectModal";
import { ImportMatchProgressStep } from "../ui/import/ImportMatchProgressStep";
import { ImportModalContainer } from "../ui/import/ImportModalContainer";
import { ImportPreviewStep } from "../ui/import/ImportPreviewStep";
import { ImportResultStep } from "../ui/import/ImportResultStep";
import { ImportTaskLoadingStep } from "../ui/import/ImportTaskLoadingStep";
import { applyMetadataDuplicateHints } from "../ui/import/metadataDuplicate";

interface BatchImportModalProps {
  isOpen: boolean;
  onClose: () => void;
  onImportComplete: () => void;
}

type Step = "select" | "scan" | "preview" | "match" | "importing" | "result";
type BatchScanPreset = "scan_parent" | "scan_library_child" | "hierarchy_child";
const MAX_HIERARCHY_DEPTH = 5;

export function BatchImportModal({
  isOpen,
  onClose,
  onImportComplete,
}: BatchImportModalProps) {
  const [step, setStep] = useState<Step>("select");
  const [libraryPath, setLibraryPath] = useState("");
  const [candidates, setCandidates] = useState<ImportCandidate[]>([]);
  const [skippedCandidates, setSkippedCandidates] = useState<ImportCandidate[]>(
    [],
  );
  const [importResult, setImportResult] = useState<service.ImportResult | null>(
    null,
  );
  const [isLoading, setIsLoading] = useState(false);
  const [matchProgress, setMatchProgress] = useState<MatchProgressState>({
    current: 0,
    total: 0,
    gameName: "",
  });
  const [scanPreset, setScanPreset] = useState<BatchScanPreset>("scan_parent");
  const [hierarchyDepth, setHierarchyDepth] = useState(0);

  const { t } = useTranslation();

  const abortMatchRef = useRef(false);

  const [showManualSelect, setShowManualSelect] = useState(false);
  const [manualSelectIndex, setManualSelectIndex] = useState<number | null>(
    null,
  );
  const [manualMatches, setManualMatches] = useState<
    vo.GameMetadataFromWebVO[]
  >([]);
  const [isSearching, setIsSearching] = useState(false);
  const [manualId, setManualId] = useState("");
  const [manualSource, setManualSource] = useState<enums.SourceType>(
    enums.SourceType.BANGUMI,
  );

  if (!isOpen) {
    return null;
  }

  const closeManualSelect = () => {
    setShowManualSelect(false);
    setManualSelectIndex(null);
  };

  const handleSelectDirectory = async () => {
    try {
      const path = await SelectLibraryDirectory();
      if (path) {
        setLibraryPath(path);
        setStep("scan");
        setIsLoading(true);

        try {
          const scanned = await ScanLibraryDirectoryWithOptions(
            path,
            new vo.BatchImportScanOptions({
              scan_mode:
                scanPreset === "hierarchy_child" ? "hierarchy" : "scan",
              scan_name_mode: scanPreset === "scan_parent" ? "parent" : "depth",
              name_depth: 0,
              hierarchy_depth: hierarchyDepth,
            }),
          );
          const toImportCandidate = (
            c: vo.BatchImportCandidate,
          ): ImportCandidate => ({
            folderPath: c.folder_path,
            folderName: c.folder_name,
            executables: c.executables || [],
            selectedExe: c.selected_exe,
            searchName: c.search_name,
            isSelected: true,
            importStatus: c.import_status || "new",
            skipReason: c.skip_reason || "",
            existingName: c.existing_name || "",
            matchedGame: null,
            matchedTags: [],
            matchSource: null,
            matchStatus: "pending",
            metadataDuplicateExistingId: undefined,
            metadataDuplicateExistingName: undefined,
          });
          const localCandidates = (scanned?.candidates || []).map(
            toImportCandidate,
          );
          const localSkippedCandidates = (
            scanned?.skipped_candidates || []
          ).map(c => ({
            ...toImportCandidate(c),
            isSelected: false,
          }));
          setCandidates(localCandidates);
          setSkippedCandidates(localSkippedCandidates);
          setStep("preview");
        }
        catch (error) {
          console.error("Failed to scan directory:", error);
          toast.error(t("batchImportModal.toast.scanFailed"));
          setStep("select");
        }
        finally {
          setIsLoading(false);
        }
      }
    }
    catch (error) {
      console.error("Failed to select directory:", error);
      toast.error(t("batchImportModal.toast.selectDirFailed"));
    }
  };

  const handleStartMatch = async () => {
    setStep("match");
    abortMatchRef.current = false;

    const toMatchCandidates = candidates.filter(
      c => c.isSelected && c.matchStatus === "pending",
    );
    setMatchProgress({
      current: 0,
      total: toMatchCandidates.length,
      gameName: "",
    });

    const updatedCandidates = [...candidates];
    let matchedCount = 0;

    for (let i = 0; i < candidates.length; i++) {
      if (abortMatchRef.current) {
        break;
      }

      if (
        !candidates[i].isSelected
        || candidates[i].matchStatus === "matched"
        || candidates[i].matchStatus === "manual"
      ) {
        continue;
      }

      matchedCount++;
      setMatchProgress(prev => ({
        ...prev,
        current: matchedCount,
        gameName: candidates[i].searchName,
      }));

      try {
        const results = await FetchMetadataByName(candidates[i].searchName);

        if (results && results.length > 0) {
          const priorityOrder = [
            enums.SourceType.BANGUMI,
            enums.SourceType.VNDB,
            enums.SourceType.YMGAL,
            enums.SourceType.DLSITE,
            enums.SourceType.EROGAMESCAPE,
            enums.SourceType.STEAM,
          ];
          let bestMatch: vo.GameMetadataFromWebVO | null = null;

          for (const source of priorityOrder) {
            const match = results.find(r => r.Source === source && r.Game);
            if (match) {
              bestMatch = match;
              break;
            }
          }

          if (bestMatch && bestMatch.Game) {
            updatedCandidates[i] = {
              ...updatedCandidates[i],
              matchedGame: bestMatch.Game,
              matchedTags: bestMatch.Tags || [],
              matchSource: bestMatch.Source,
              matchStatus: "matched",
              allMatches: results,
            };
          }
          else {
            updatedCandidates[i] = {
              ...updatedCandidates[i],
              matchedTags: [],
              matchStatus: "not_found",
              allMatches: results,
            };
          }
        }
        else {
          updatedCandidates[i] = {
            ...updatedCandidates[i],
            matchedTags: [],
            matchStatus: "not_found",
          };
        }
      }
      catch (error) {
        console.error(`Failed to match ${candidates[i].searchName}:`, error);
        updatedCandidates[i] = {
          ...updatedCandidates[i],
          matchedTags: [],
          matchStatus: "error",
        };
      }

      setCandidates([...updatedCandidates]);

      if (!abortMatchRef.current) {
        await new Promise(resolve => setTimeout(resolve, 1500));
      }
    }

    if (!abortMatchRef.current) {
      try {
        setCandidates(await applyMetadataDuplicateHints(updatedCandidates));
      }
      catch (error) {
        console.error("Failed to check metadata duplicates:", error);
      }
      setStep("preview");
    }
  };

  const handleImport = async () => {
    setStep("importing");
    setIsLoading(true);

    try {
      const importCandidates: vo.BatchImportCandidate[] = candidates
        .filter(c => c.isSelected)
        .map((c) => {
          const candidate = new vo.BatchImportCandidate({
            folder_path: c.folderPath,
            folder_name: c.folderName,
            executables: c.executables,
            selected_exe: c.selectedExe,
            search_name: c.searchName,
            is_selected: c.isSelected,
            match_status: c.matchStatus,
            import_status: c.importStatus,
            skip_reason: c.skipReason,
            existing_name: c.existingName,
          });
          if (c.matchedGame) {
            candidate.matched_game = c.matchedGame;
          }
          if (c.matchedTags.length > 0) {
            candidate.matched_tags = c.matchedTags;
          }
          if (c.matchSource) {
            candidate.match_source = c.matchSource;
          }
          return candidate;
        });

      const result = await BatchImportGames(importCandidates);
      setImportResult(result);
      setStep("result");

      if (result.success > 0) {
        toast.success(
          t("batchImportModal.toast.importSuccess", { count: result.success }),
        );
        onImportComplete();
      }
    }
    catch (error) {
      console.error("Failed to import:", error);
      toast.error(t("batchImportModal.toast.importFailed"));
      setStep("preview");
    }
    finally {
      setIsLoading(false);
    }
  };

  const toggleCandidate = (index: number) => {
    const updated = [...candidates];
    updated[index].isSelected = !updated[index].isSelected;
    setCandidates(updated);
  };

  const toggleAllCandidates = (checked: boolean) => {
    setCandidates(
      candidates.map(c => ({
        ...c,
        isSelected: checked,
      })),
    );
  };

  const updateSearchName = (index: number, name: string) => {
    const updated = [...candidates];
    updated[index].searchName = name;
    updated[index].matchStatus = "pending";
    updated[index].matchedGame = null;
    updated[index].matchedTags = [];
    updated[index].matchSource = null;
    updated[index].metadataDuplicateExistingId = undefined;
    updated[index].metadataDuplicateExistingName = undefined;
    setCandidates(updated);
  };

  const updateSelectedExe = (index: number, exe: string) => {
    const updated = [...candidates];
    updated[index].selectedExe = exe;
    setCandidates(updated);
  };

  const openManualSelect = async (index: number) => {
    setManualSelectIndex(index);
    setManualMatches(candidates[index].allMatches || []);
    setShowManualSelect(true);
    setManualId("");

    if (
      !candidates[index].allMatches
      || candidates[index].allMatches.length === 0
    ) {
      setIsSearching(true);
      try {
        const results = await FetchMetadataByName(candidates[index].searchName);
        setManualMatches(results || []);
      }
      catch (error) {
        console.error("Failed to search:", error);
      }
      finally {
        setIsSearching(false);
      }
    }
  };

  const selectManualMatch = async (match: vo.GameMetadataFromWebVO) => {
    if (!match.Game) {
      return;
    }
    if (manualSelectIndex !== null) {
      const updated = [...candidates];
      updated[manualSelectIndex] = {
        ...updated[manualSelectIndex],
        matchedGame: match.Game,
        matchedTags: match.Tags || [],
        matchSource: match.Source,
        matchStatus: "manual",
      };
      try {
        const [candidateWithHint] = await applyMetadataDuplicateHints([
          updated[manualSelectIndex],
        ]);
        updated[manualSelectIndex] = candidateWithHint;
      }
      catch (error) {
        console.error("Failed to check metadata duplicate:", error);
      }
      setCandidates(updated);
    }
    closeManualSelect();
  };

  const handleSearchById = async () => {
    if (!manualId || manualSelectIndex === null) {
      return;
    }
    setIsSearching(true);
    try {
      const request = new vo.MetadataRequest({
        source: manualSource,
        id: manualId,
      });
      const metadata = await FetchMetadataFromWeb(request);
      if (metadata && metadata.Game && metadata.Game.name) {
        await selectManualMatch(metadata);
      }
      else {
        toast.error(t("batchImportModal.toast.gameNotFound"));
      }
    }
    catch (error) {
      console.error("Failed to fetch by ID:", error);
      toast.error(t("batchImportModal.toast.fetchFailed"));
    }
    finally {
      setIsSearching(false);
    }
  };

  const handleSkipMetadata = () => {
    if (manualSelectIndex === null) {
      return;
    }
    const updated = [...candidates];
    updated[manualSelectIndex] = {
      ...updated[manualSelectIndex],
      matchedGame: null,
      matchedTags: [],
      matchSource: null,
      matchStatus: "not_found",
      metadataDuplicateExistingId: undefined,
      metadataDuplicateExistingName: undefined,
    };
    setCandidates(updated);
    closeManualSelect();
  };

  const resetAndClose = () => {
    abortMatchRef.current = true;

    setStep("select");
    setLibraryPath("");
    setCandidates([]);
    setSkippedCandidates([]);
    setImportResult(null);
    setMatchProgress({ current: 0, total: 0, gameName: "" });
    closeManualSelect();
    onClose();
  };

  const matchedCount = candidates.filter(
    c =>
      c.isSelected
      && (c.matchStatus === "matched" || c.matchStatus === "manual"),
  ).length;
  const notFoundCount = candidates.filter(
    c => c.isSelected && c.matchStatus === "not_found",
  ).length;
  const pendingCount = candidates.filter(
    c => c.isSelected && c.matchStatus === "pending",
  ).length;
  const hierarchyLevel = hierarchyDepth + 1;
  const scanPresetItems = [
    {
      key: "scan_parent",
      label: t("batchImportModal.scanMode.scanParent"),
      description: t("batchImportModal.scanMode.scanParentHint"),
      icon:
        scanPreset === "scan_parent"
          ? "i-mdi-check"
          : "i-mdi-file-search-outline",
      iconColor:
        scanPreset === "scan_parent" ? "text-success-500" : "text-brand-400",
      onClick: () => setScanPreset("scan_parent"),
    },
    {
      key: "scan_library_child",
      label: t("batchImportModal.scanMode.scanLibraryChild"),
      description: t("batchImportModal.scanMode.scanLibraryChildHint"),
      icon:
        scanPreset === "scan_library_child"
          ? "i-mdi-check"
          : "i-mdi-file-tree-outline",
      iconColor:
        scanPreset === "scan_library_child"
          ? "text-success-500"
          : "text-brand-400",
      onClick: () => setScanPreset("scan_library_child"),
    },
    {
      key: "hierarchy_child",
      label: t("batchImportModal.scanMode.hierarchyChild", {
        level: hierarchyLevel,
      }),
      description: t("batchImportModal.scanMode.hierarchyChildHint", {
        level: hierarchyLevel,
      }),
      icon:
        scanPreset === "hierarchy_child"
          ? "i-mdi-check"
          : "i-mdi-folder-table-outline",
      iconColor:
        scanPreset === "hierarchy_child"
          ? "text-success-500"
          : "text-brand-400",
      onClick: () => setScanPreset("hierarchy_child"),
    },
  ];
  const setHierarchyDepthWithinBounds = (depth: number) => {
    setHierarchyDepth(Math.min(MAX_HIERARCHY_DEPTH, Math.max(0, depth)));
  };

  return (
    <>
      <ImportModalContainer
        title={t("batchImportModal.title")}
        iconClassName="i-mdi-folder-multiple text-3xl text-success-500"
        onClose={resetAndClose}
      >
        {step === "select" && (
          <div className="space-y-6">
            <div className="py-8 text-center">
              <div className="i-mdi-folder-open mx-auto mb-4 text-6xl text-brand-400" />
              <p className="mb-2 text-brand-700 dark:text-brand-300">
                {t("batchImportModal.selectDir")}
              </p>
              <p className="text-sm text-brand-400 dark:text-brand-500">
                {t("batchImportModal.scanHint")}
              </p>
            </div>

            <div className="flex flex-col items-stretch gap-3 sm:flex-row sm:items-center">
              <BetterButton
                variant="primary"
                size="lg"
                icon="i-mdi-folder-search"
                onClick={handleSelectDirectory}
                disabled={isLoading}
                className="flex-1"
              >
                {t("batchImportModal.btn.selectDir")}
              </BetterButton>

              <BetterDropdownMenu
                title={t("batchImportModal.scanMode.label")}
                menuWidth="w-[22rem] max-w-[calc(100vw-3rem)]"
                items={scanPresetItems}
                footer={(
                  <div
                    className="mt-1 border-t border-brand-200 px-3 py-2.5 dark:border-brand-700"
                    onClick={event => event.stopPropagation()}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <div className="text-sm font-medium text-brand-700 dark:text-brand-200">
                          {t("batchImportModal.scanMode.hierarchyDepthLabel")}
                        </div>
                        <div className="mt-0.5 text-xs leading-tight text-brand-400 dark:text-brand-500">
                          {t("batchImportModal.scanMode.hierarchyDepthHint", {
                            level: hierarchyLevel,
                          })}
                        </div>
                      </div>

                      <div className="flex shrink-0 items-center rounded-lg border border-brand-200 bg-brand-50 p-0.5 dark:border-brand-700 dark:bg-brand-900/30">
                        <button
                          type="button"
                          aria-label={t(
                            "batchImportModal.scanMode.decreaseDepth",
                          )}
                          disabled={hierarchyDepth === 0}
                          onClick={() =>
                            setHierarchyDepthWithinBounds(hierarchyDepth - 1)}
                          className="flex h-8 w-8 items-center justify-center rounded-md text-brand-500 transition-colors hover:bg-white hover:text-brand-800 disabled:cursor-not-allowed disabled:opacity-40 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-100"
                        >
                          <div className="i-mdi-minus text-lg" />
                        </button>
                        <span className="min-w-12 px-2 text-center text-sm font-medium text-brand-800 dark:text-brand-100">
                          {t("batchImportModal.scanMode.hierarchyDepthValue", {
                            level: hierarchyLevel,
                          })}
                        </span>
                        <button
                          type="button"
                          aria-label={t(
                            "batchImportModal.scanMode.increaseDepth",
                          )}
                          disabled={hierarchyDepth === MAX_HIERARCHY_DEPTH}
                          onClick={() =>
                            setHierarchyDepthWithinBounds(hierarchyDepth + 1)}
                          className="flex h-8 w-8 items-center justify-center rounded-md text-brand-500 transition-colors hover:bg-white hover:text-brand-800 disabled:cursor-not-allowed disabled:opacity-40 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-100"
                        >
                          <div className="i-mdi-plus text-lg" />
                        </button>
                      </div>
                    </div>
                  </div>
                )}
                trigger={(
                  <BetterButton
                    variant="secondary"
                    size="lg"
                    icon="i-mdi-tune-variant"
                    className="w-full sm:w-auto"
                  >
                    {t("batchImportModal.scanMode.button")}
                  </BetterButton>
                )}
              />
            </div>
          </div>
        )}

        {step === "scan" && (
          <ImportTaskLoadingStep
            iconClassName="text-success-500"
            title={t("batchImportModal.scanning")}
            subtitle={libraryPath}
          />
        )}

        {step === "preview" && (
          <ImportPreviewStep
            candidates={candidates}
            skippedCandidates={skippedCandidates}
            matchedCount={matchedCount}
            notFoundCount={notFoundCount}
            pendingCount={pendingCount}
            labels={{
              detected: t("batchImportModal.detected"),
              matched: t("batchImportModal.matched"),
              notMatched: t("batchImportModal.notMatched"),
              pending: t("batchImportModal.pending"),
              searchName: t("batchImportModal.searchName"),
              executable:
                scanPreset === "hierarchy_child"
                  ? t("batchImportModal.gamePath")
                  : t("batchImportModal.executable"),
              matchStatus: t("batchImportModal.matchStatus"),
              action: t("common.action"),
              empty: t("batchImportModal.noFolderDetected"),
              startMatching: t("batchImportModal.startMatching"),
              importCount: count =>
                t("batchImportModal.importCount", { count }),
              leftAction: `← ${t("batchImportModal.reselect")}`,
              statusPending: t("batchImportModal.status.pending"),
              statusMatched: t("batchImportModal.status.matched"),
              statusNotFound: t("batchImportModal.status.notFound"),
              statusError: t("batchImportModal.status.error"),
              manualSelect: t("batchImportModal.manualSelect"),
              metadataExists: name =>
                t("batchImportModal.metadataExists", { name }),
              skippedSummary: count =>
                t("batchImportModal.skippedExistingSummary", { count }),
            }}
            theme={{
              detectedCardClassName: "bg-neutral-50 dark:bg-neutral-900/20",
              detectedValueClassName: "text-neutral-600 dark:text-neutral-400",
              detectedLabelClassName: "text-neutral-700 dark:text-neutral-300",
              searchInputFocusClassName: "focus:border-neutral-500",
              manualButtonClassName: "text-neutral-500 hover:text-neutral-700",
              startMatchButtonClassName: "bg-neutral-600 hover:bg-neutral-700",
              importButtonClassName: "bg-success-600 hover:bg-success-700",
            }}
            onLeftAction={() => setStep("select")}
            onStartMatch={handleStartMatch}
            onImport={handleImport}
            onToggleAll={toggleAllCandidates}
            onToggleCandidate={toggleCandidate}
            onUpdateSearchName={updateSearchName}
            onUpdateSelectedExe={updateSelectedExe}
            onManualSelect={openManualSelect}
          />
        )}

        {step === "match" && (
          <ImportMatchProgressStep
            title={t("batchImportModal.matching")}
            hint={t("batchImportModal.matchHint")}
            progress={matchProgress}
            spinnerClassName="text-neutral-500"
            progressClassName="bg-neutral-500"
          />
        )}

        {step === "importing" && (
          <ImportTaskLoadingStep
            iconClassName="text-success-500"
            title={t("batchImportModal.importing")}
          />
        )}

        {step === "result" && importResult && (
          <ImportResultStep
            result={importResult}
            labels={{
              success: t("batchImportModal.result.success"),
              skipped: t("batchImportModal.result.skipped"),
              failed: t("batchImportModal.result.failed"),
              skippedGames: t("batchImportModal.skippedGames"),
              failedGames: t("batchImportModal.failedGames"),
              complete: t("common.complete"),
            }}
            completeButtonClassName="bg-success-600 hover:bg-success-700"
            onComplete={resetAndClose}
          />
        )}
      </ImportModalContainer>

      <ImportManualSelectModal
        isOpen={showManualSelect && manualSelectIndex !== null}
        title={t("batchImportModal.manualSelect")}
        candidateName={
          manualSelectIndex !== null
            ? candidates[manualSelectIndex]?.searchName || ""
            : ""
        }
        isSearching={isSearching}
        matches={manualMatches}
        manualSource={manualSource}
        manualId={manualId}
        sourceOptions={[
          { value: enums.SourceType.BANGUMI, label: "Bangumi" },
          { value: enums.SourceType.VNDB, label: "VNDB" },
          { value: enums.SourceType.YMGAL, label: t("gameEdit.sourceYmgal") },
          { value: enums.SourceType.DLSITE, label: t("gameEdit.sourceDlsite") },
          {
            value: enums.SourceType.EROGAMESCAPE,
            label: t("gameEdit.sourceErogameScape"),
          },
          { value: enums.SourceType.STEAM, label: "Steam" },
        ]}
        idPlaceholder={t("batchImportModal.inputId")}
        theme={{
          loadingSpinnerClassName: "text-neutral-500",
          cardHoverClassName: "hover:border-neutral-500",
          searchButtonClassName: "bg-neutral-500 hover:bg-neutral-600",
        }}
        labels={{
          searching: t("common.searching"),
          noMatchResult: t("batchImportModal.noMatchResult"),
          searchById: t("batchImportModal.searchById"),
          search: t("common.search"),
          skipMetadata: t("batchImportModal.importWithoutMeta"),
        }}
        searchDisabled={!manualId || isSearching}
        onClose={closeManualSelect}
        onSelectMatch={selectManualMatch}
        onSourceChange={source => setManualSource(source)}
        onManualIdChange={setManualId}
        onSearchById={handleSearchById}
        onSkipMetadata={handleSkipMetadata}
      />
    </>
  );
}
