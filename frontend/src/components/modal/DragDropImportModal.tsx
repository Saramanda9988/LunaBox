import type { appconf, enums } from "../../../src/bindings/models";
import type { PreferredSourceValue } from "../ui/import/importFlow";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";

import { OpenLocalPath } from "../../../bindings/lunabox/internal/service/gameservice";
import { ProcessDroppedPathsWithOptions } from "../../../bindings/lunabox/internal/service/importservice";
import { useAppStore } from "../../store";
import { BetterSelect } from "../ui/better/BetterSelect";
import {
  getImportScanConfig,
  getPreferredSource,
  preferredSourceOptions,
  scanResultToCandidates,
} from "../ui/import/importFlow";
import { ImportManualSelectModal } from "../ui/import/ImportManualSelectModal";
import { ImportMatchProgressStep } from "../ui/import/ImportMatchProgressStep";
import { ImportModalContainer } from "../ui/import/ImportModalContainer";
import { ImportPreviewStep } from "../ui/import/ImportPreviewStep";
import { ImportResultStep } from "../ui/import/ImportResultStep";
import { ImportTaskLoadingStep } from "../ui/import/ImportTaskLoadingStep";
import { useImportFlow } from "../ui/import/useImportFlow";

interface DragDropImportModalProps {
  isOpen: boolean;
  droppedPaths: string[];
  onClose: () => void;
  onImportComplete: () => void;
}

type Step = "processing" | "preview" | "match" | "importing" | "result";

export function DragDropImportModal({
  isOpen,
  droppedPaths,
  onClose,
  onImportComplete,
}: DragDropImportModalProps) {
  const [step, setStep] = useState<Step>("processing");
  const activeRequestRef = useRef(0);
  const processingPathsRef = useRef<string[] | null>(null);
  const { t } = useTranslation();
  const config = useAppStore(state => state.config);
  const enabledMetadataSources = useAppStore(
    state => state.enabledMetadataSources,
  );
  const patchLiveConfig = useAppStore(state => state.patchLiveConfig);
  const saveBatchImportPreferences = useCallback(
    (patch: Partial<appconf.AppConfig>) => {
      void patchLiveConfig(patch);
    },
    [patchLiveConfig],
  );
  const sourceOptions = useMemo(
    () => preferredSourceOptions(enabledMetadataSources, t),
    [enabledMetadataSources, t],
  );
  const { scanPreset, scanOptions } = getImportScanConfig(
    config as appconf.AppConfig | null,
  );
  const preferredSource = getPreferredSource(config, enabledMetadataSources);
  const preferredSourceLabel
    = sourceOptions.find(option => option.value === preferredSource)?.label
      || t("batchImportModal.preferredSource.none");
  const importFlow = useImportFlow({
    t,
    preferredSource,
    preferredSourceLabel,
    enabledMetadataSources,
    onImportComplete,
  });
  const {
    candidates,
    skippedCandidates,
    importResult,
    matchProgress,
    matchPauseMessage,
    showManualSelect,
    manualSelectIndex,
    manualMatches,
    isSearching,
    manualId,
    manualSource,
    matchedCount,
    notFoundCount,
    pendingCount,
    errorCount,
    matchableCount,
    setCandidates,
    setSkippedCandidates,
    resetImportFlow,
    handleStartMatch: runStartMatch,
    handleImport: runImport,
    toggleCandidate,
    toggleAllCandidates,
    updateSearchName,
    updateSelectedExe,
    openManualSelect,
    selectManualMatch,
    handleSearchById,
    handleSkipMetadata,
    closeManualSelect,
    setManualId,
    setManualSource,
    stopMatching,
  } = importFlow;

  const resetAndClose = useCallback(() => {
    activeRequestRef.current += 1;
    processingPathsRef.current = null;
    resetImportFlow();
    setStep("processing");
    onClose();
  }, [onClose, resetImportFlow]);

  useEffect(() => {
    if (
      !isOpen
      || droppedPaths.length === 0
      || processingPathsRef.current === droppedPaths
    ) {
      return;
    }

    processingPathsRef.current = droppedPaths;
    const requestId = activeRequestRef.current + 1;
    activeRequestRef.current = requestId;

    const processDroppedPaths = async () => {
      try {
        const processed = await ProcessDroppedPathsWithOptions(
          droppedPaths,
          scanOptions,
        );
        if (activeRequestRef.current !== requestId) {
          return;
        }
        if (
          !processed
          || ((processed.candidates || []).length === 0
            && (processed.skipped_candidates || []).length === 0)
        ) {
          toast.error(t("dragDropImportModal.toast.noValidGames"));
          resetAndClose();
          return;
        }

        const converted = scanResultToCandidates(processed);
        setCandidates(converted.candidates);
        setSkippedCandidates(converted.skippedCandidates);
        setStep("preview");
      }
      catch (error) {
        if (activeRequestRef.current !== requestId) {
          return;
        }
        console.error("Failed to process dropped paths:", error);
        toast.error(t("dragDropImportModal.toast.processFailed"));
        resetAndClose();
      }
    };

    void processDroppedPaths();
  }, [
    droppedPaths,
    isOpen,
    resetAndClose,
    scanOptions,
    setCandidates,
    setSkippedCandidates,
    t,
  ]);

  if (!isOpen) {
    return null;
  }

  const handleStartMatch = async () => {
    setStep("match");
    await runStartMatch(() => setStep("preview"));
  };

  const handleOpenFolder = async (path: string) => {
    try {
      await OpenLocalPath(path);
    }
    catch (error) {
      console.error("Failed to open import candidate folder:", error);
      toast.error(t("batchImportModal.toast.openFolderFailed"));
    }
  };

  const handleImport = async (matchedOnly = false) => {
    setStep("importing");
    await runImport(
      () => setStep("preview"),
      () => setStep("preview"),
      { matchedOnly },
    );
  };

  const handlePreferredSourceChange = (source: PreferredSourceValue) => {
    saveBatchImportPreferences({ batch_import_preferred_source: source });
  };

  return (
    <>
      <ImportModalContainer
        title={t("batchImportModal.title")}
        iconClassName="i-mdi-folder-multiple text-3xl text-success-500"
        onClose={resetAndClose}
      >
        {step === "processing" && (
          <ImportTaskLoadingStep
            iconClassName="text-success-500"
            title={`${t("dragDropImportModal.processing")}...`}
            subtitle={t("dragDropImportModal.fileCount", {
              count: droppedPaths.length,
            })}
          />
        )}

        {step === "preview" && (
          <ImportPreviewStep
            candidates={candidates}
            skippedCandidates={skippedCandidates}
            matchedCount={matchedCount}
            notFoundCount={notFoundCount}
            pendingCount={pendingCount}
            canStartMatch={matchableCount > 0}
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
              startMatching:
                errorCount > 0
                  ? t("batchImportModal.continueMatching")
                  : t("batchImportModal.startMatching"),
              importCount: count =>
                t("batchImportModal.importCount", { count }),
              importMatchedCount: count =>
                t("batchImportModal.importMatchedCount", { count }),
              leftAction: t("common.cancel"),
              statusPending: t("batchImportModal.status.pending"),
              statusMatched: t("batchImportModal.status.matched"),
              statusNotFound: t("batchImportModal.status.notFound"),
              statusError: t("batchImportModal.status.error"),
              manualSelect: t("batchImportModal.manualSelect"),
              openFolder: t("batchImportModal.openFolder"),
              metadataExists: name =>
                t("batchImportModal.metadataExists", { name }),
              skippedSummary: count =>
                t("batchImportModal.skippedExistingSummary", { count }),
              skippedDetails: t("batchImportModal.skippedExistingDetails"),
              skippedViewDetails: t("batchImportModal.skippedExistingView"),
              skippedModalTitle: t("batchImportModal.skippedExistingTitle"),
              skippedModalHint: t("batchImportModal.skippedExistingHint"),
              skippedReason: t("batchImportModal.skippedExistingReason"),
              skippedPath: t("batchImportModal.skippedExistingPath"),
              closeSkippedModal: t("common.confirm"),
              filterAll: t("batchImportModal.filter.all"),
              filterByStatus: t("batchImportModal.filter.byStatus"),
              filterBySelection: t("batchImportModal.filter.bySelection"),
              filterSelected: t("batchImportModal.filter.selected"),
              filterUnselected: t("batchImportModal.filter.unselected"),
              filteredCount: (visible, total) =>
                t("batchImportModal.filter.count", { visible, total }),
              clearFilters: t("batchImportModal.filter.clear"),
              emptyFiltered: t("batchImportModal.filter.emptyFiltered"),
            }}
            toolbar={
              matchPauseMessage ? (
                <div className="flex items-start gap-2 rounded-lg border border-warning-300 bg-warning-50 px-4 py-3 text-sm text-warning-800 dark:border-warning-700 dark:bg-warning-900/25 dark:text-warning-200">
                  <div className="i-mdi-pause-circle-outline mt-0.5 shrink-0 text-lg" />
                  <span>{matchPauseMessage}</span>
                </div>
              ) : undefined
            }
            actionToolbar={
              matchableCount > 0 ? (
                <div className="flex w-full flex-col gap-2 sm:h-11 sm:w-auto sm:flex-row sm:items-stretch sm:gap-0">
                  <div className="inline-flex h-11 shrink-0 items-center justify-center gap-1.5 whitespace-nowrap rounded-lg border border-brand-200 bg-white/80 px-3 text-xs font-medium text-brand-600 shadow-sm dark:border-brand-700 dark:bg-brand-800/60 dark:text-brand-300 sm:rounded-r-none sm:border-r-0">
                    <div className="i-mdi-database-search-outline text-base text-brand-400 dark:text-brand-400" />
                    <span>{t("batchImportModal.preferredSource.label")}</span>
                  </div>
                  <BetterSelect
                    value={preferredSource}
                    onChange={source =>
                      handlePreferredSourceChange(
                        source as PreferredSourceValue,
                      )}
                    options={sourceOptions}
                    className="w-full sm:h-11 sm:w-40"
                    buttonClassName="h-11 rounded-lg py-0 text-sm shadow-sm sm:rounded-l-none"
                  />
                </div>
              ) : undefined
            }
            theme={{
              detectedCardClassName: "bg-neutral-50 dark:bg-neutral-900/20",
              detectedValueClassName: "text-neutral-600 dark:text-neutral-400",
              detectedLabelClassName: "text-neutral-700 dark:text-neutral-300",
              searchInputFocusClassName: "focus:border-neutral-500",
              manualButtonClassName: "text-neutral-500 hover:text-neutral-700",
              startMatchButtonClassName: "bg-neutral-600 hover:bg-neutral-700",
              importButtonClassName: "bg-success-600 hover:bg-success-700",
            }}
            onLeftAction={resetAndClose}
            onStartMatch={handleStartMatch}
            onImport={() => handleImport()}
            onImportMatched={() => handleImport(true)}
            onToggleAll={toggleAllCandidates}
            onToggleCandidate={toggleCandidate}
            onUpdateSearchName={updateSearchName}
            onUpdateSelectedExe={updateSelectedExe}
            onManualSelect={openManualSelect}
            onOpenFolder={handleOpenFolder}
          />
        )}

        {step === "match" && (
          <ImportMatchProgressStep
            title={t("batchImportModal.matching")}
            hint={t("batchImportModal.matchHint")}
            progress={matchProgress}
            spinnerClassName="text-neutral-500"
            progressClassName="bg-neutral-500"
            onStop={() => {
              stopMatching();
              setStep("preview");
            }}
            stopLabel={t("common.stop")}
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
        sourceOptions={
          sourceOptions.filter(option => option.value !== "") as Array<{
            value: enums.SourceType;
            label: string;
          }>
        }
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
