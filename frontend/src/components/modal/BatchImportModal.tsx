import type { service } from "../../../wailsjs/go/models";
import type { ImportCandidate, MatchProgressState } from "../ui/import/types";
import { useRef, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";

import { enums, vo } from "../../../wailsjs/go/models";
import { FetchMetadataByName, FetchMetadataFromWeb } from "../../../wailsjs/go/service/GameService";
import {
  BatchImportGames,
  ScanLibraryDirectory,
  SelectLibraryDirectory,
} from "../../../wailsjs/go/service/ImportService";
import { ImportManualSelectModal } from "../ui/import/ImportManualSelectModal";
import { ImportMatchProgressStep } from "../ui/import/ImportMatchProgressStep";
import { ImportModalContainer } from "../ui/import/ImportModalContainer";
import { ImportPreviewStep } from "../ui/import/ImportPreviewStep";
import { ImportResultStep } from "../ui/import/ImportResultStep";
import { ImportTaskLoadingStep } from "../ui/import/ImportTaskLoadingStep";

interface BatchImportModalProps {
  isOpen: boolean;
  onClose: () => void;
  onImportComplete: () => void;
}

type Step = "select" | "scan" | "preview" | "match" | "importing" | "result";

export function BatchImportModal({ isOpen, onClose, onImportComplete }: BatchImportModalProps) {
  const [step, setStep] = useState<Step>("select");
  const [libraryPath, setLibraryPath] = useState("");
  const [candidates, setCandidates] = useState<ImportCandidate[]>([]);
  const [importResult, setImportResult] = useState<service.ImportResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [matchProgress, setMatchProgress] = useState<MatchProgressState>({
    current: 0,
    total: 0,
    gameName: "",
  });

  const { t } = useTranslation();

  const abortMatchRef = useRef(false);

  const [showManualSelect, setShowManualSelect] = useState(false);
  const [manualSelectIndex, setManualSelectIndex] = useState<number | null>(null);
  const [manualMatches, setManualMatches] = useState<vo.GameMetadataFromWebVO[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [manualId, setManualId] = useState("");
  const [manualSource, setManualSource] = useState<enums.SourceType>(enums.SourceType.BANGUMI);

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
          const scanned = await ScanLibraryDirectory(path);
          const localCandidates: ImportCandidate[] = (scanned || []).map(c => ({
            folderPath: c.folder_path,
            folderName: c.folder_name,
            executables: c.executables || [],
            selectedExe: c.selected_exe,
            searchName: c.search_name,
            isSelected: true,
            matchedGame: null,
            matchedTags: [],
            matchSource: null,
            matchStatus: "pending",
          }));
          setCandidates(localCandidates);
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

    const toMatchCandidates = candidates.filter(c => c.isSelected && c.matchStatus === "pending");
    setMatchProgress({ current: 0, total: toMatchCandidates.length, gameName: "" });

    const updatedCandidates = [...candidates];
    let matchedCount = 0;

    for (let i = 0; i < candidates.length; i++) {
      if (abortMatchRef.current) {
        break;
      }

      if (!candidates[i].isSelected || candidates[i].matchStatus === "matched" || candidates[i].matchStatus === "manual") {
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
          const priorityOrder = [enums.SourceType.BANGUMI, enums.SourceType.VNDB, enums.SourceType.YMGAL];
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
        toast.success(t("batchImportModal.toast.importSuccess", { count: result.success }));
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
    setCandidates(candidates.map(c => ({
      ...c,
      isSelected: checked,
    })));
  };

  const updateSearchName = (index: number, name: string) => {
    const updated = [...candidates];
    updated[index].searchName = name;
    updated[index].matchStatus = "pending";
    updated[index].matchedGame = null;
    updated[index].matchedTags = [];
    updated[index].matchSource = null;
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

    if (!candidates[index].allMatches || candidates[index].allMatches.length === 0) {
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

  const selectManualMatch = (match: vo.GameMetadataFromWebVO) => {
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
        selectManualMatch(metadata);
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
    };
    setCandidates(updated);
    closeManualSelect();
  };

  const resetAndClose = () => {
    abortMatchRef.current = true;

    setStep("select");
    setLibraryPath("");
    setCandidates([]);
    setImportResult(null);
    setMatchProgress({ current: 0, total: 0, gameName: "" });
    closeManualSelect();
    onClose();
  };

  const matchedCount = candidates.filter(c => c.isSelected && (c.matchStatus === "matched" || c.matchStatus === "manual")).length;
  const notFoundCount = candidates.filter(c => c.isSelected && c.matchStatus === "not_found").length;
  const pendingCount = candidates.filter(c => c.isSelected && c.matchStatus === "pending").length;

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
              <p className="mb-2 text-brand-600 dark:text-brand-300">
                {t("batchImportModal.selectDir")}
              </p>
              <p className="text-sm text-brand-400 dark:text-brand-500">
                {t("batchImportModal.scanHint")}
              </p>
            </div>

            <button
              type="button"
              onClick={handleSelectDirectory}
              disabled={isLoading}
              className="flex w-full items-center justify-center rounded-lg bg-success-500 py-4 text-white transition hover:bg-success-600 disabled:opacity-50"
            >
              <div className="i-mdi-folder-search mr-2 text-xl" />
              {t("batchImportModal.btn.selectDir")}
            </button>
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
            matchedCount={matchedCount}
            notFoundCount={notFoundCount}
            pendingCount={pendingCount}
            labels={{
              detected: t("batchImportModal.detected"),
              matched: t("batchImportModal.matched"),
              notMatched: t("batchImportModal.notMatched"),
              pending: t("batchImportModal.pending"),
              searchName: t("batchImportModal.searchName"),
              executable: t("batchImportModal.executable"),
              matchStatus: t("batchImportModal.matchStatus"),
              action: t("common.action"),
              empty: t("batchImportModal.noFolderDetected"),
              startMatching: t("batchImportModal.startMatching"),
              importCount: count => t("batchImportModal.importCount", { count }),
              leftAction: `← ${t("batchImportModal.reselect")}`,
              statusPending: t("batchImportModal.status.pending"),
              statusMatched: t("batchImportModal.status.matched"),
              statusNotFound: t("batchImportModal.status.notFound"),
              statusError: t("batchImportModal.status.error"),
              manualSelect: t("batchImportModal.manualSelect"),
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
        candidateName={manualSelectIndex !== null ? (candidates[manualSelectIndex]?.searchName || "") : ""}
        isSearching={isSearching}
        matches={manualMatches}
        manualSource={manualSource}
        manualId={manualId}
        sourceOptions={[
          { value: enums.SourceType.BANGUMI, label: "Bangumi" },
          { value: enums.SourceType.VNDB, label: "VNDB" },
          { value: enums.SourceType.YMGAL, label: t("gameEdit.sourceYmgal") },
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
