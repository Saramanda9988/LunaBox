import type { TFunction } from "i18next";

import { useCallback, useRef, useState } from "react";
import toast from "react-hot-toast";

import type { service, vo as voTypes } from "../../../../src/bindings/models";
import type { ImportRequestOptions, PreferredSourceValue } from "./importFlow";
import type { ImportCandidate, MatchProgressState } from "./types";

import { FetchMetadataFromWeb } from "../../../../bindings/lunabox/internal/service/gameservice";
import {
  BatchImportGames,
  FetchMetadataForCandidateWithPreference,
} from "../../../../bindings/lunabox/internal/service/importservice";
import { enums, vo } from "../../../../src/bindings/models";
import {
  candidatesToImportRequest,
  errorText,
  NO_PREFERRED_SOURCE,
  pickBestMatch,
  PREFERRED_SOURCE_FAILURE_PAUSE_THRESHOLD,
  shouldImportCandidate,
} from "./importFlow";
import { applyMetadataDuplicateHints } from "./metadataDuplicate";

export type UseImportFlowOptions = {
  t: TFunction;
  preferredSource: PreferredSourceValue;
  preferredSourceLabel: string;
  enabledMetadataSources: readonly enums.SourceType[];
  onImportComplete: () => void;
};

function importCandidateResultNames(candidate: ImportCandidate) {
  return [candidate.searchName, candidate.matchedGame?.name].filter(
    (name): name is string => Boolean(name),
  );
}

function findImportResultMessage(
  candidate: ImportCandidate,
  resultNames: string[] | undefined,
) {
  if (!resultNames || resultNames.length === 0) {
    return "";
  }

  const candidateNames = importCandidateResultNames(candidate);
  return (
    resultNames.find(resultName =>
      candidateNames.some(
        name => resultName === name || resultName.startsWith(`${name} (`),
      ),
    ) || ""
  );
}

export function useImportFlow({
  t,
  preferredSource,
  preferredSourceLabel,
  enabledMetadataSources,
  onImportComplete,
}: UseImportFlowOptions) {
  const [candidates, setCandidates] = useState<ImportCandidate[]>([]);
  const [skippedCandidates, setSkippedCandidates] = useState<ImportCandidate[]>(
    [],
  );
  const [importResult, setImportResult] = useState<service.ImportResult | null>(
    null,
  );
  const [matchProgress, setMatchProgress] = useState<MatchProgressState>({
    current: 0,
    total: 0,
    gameName: "",
  });
  const [matchPauseMessage, setMatchPauseMessage] = useState("");

  const abortMatchRef = useRef(false);
  const [showManualSelect, setShowManualSelect] = useState(false);
  const [manualSelectIndex, setManualSelectIndex] = useState<number | null>(
    null,
  );
  const [manualMatches, setManualMatches] = useState<
    voTypes.GameMetadataFromWebVO[]
  >([]);
  const [isSearching, setIsSearching] = useState(false);
  const [manualId, setManualId] = useState("");
  const [manualSource, setManualSource] = useState<enums.SourceType>(
    enabledMetadataSources[0] ?? enums.SourceType.Bangumi,
  );
  const selectedManualSource = enabledMetadataSources.includes(manualSource)
    ? manualSource
    : (enabledMetadataSources[0] ?? enums.SourceType.Bangumi);

  const shouldMatchCandidate = useCallback((candidate: ImportCandidate) => {
    if (!candidate.isSelected) {
      return false;
    }
    return (
      candidate.matchStatus === "pending" || candidate.matchStatus === "error"
    );
  }, []);

  const closeManualSelect = useCallback(() => {
    setShowManualSelect(false);
    setManualSelectIndex(null);
  }, []);

  const resetImportFlow = useCallback(() => {
    abortMatchRef.current = true;
    setCandidates([]);
    setSkippedCandidates([]);
    setImportResult(null);
    setMatchPauseMessage("");
    setMatchProgress({ current: 0, total: 0, gameName: "" });
    closeManualSelect();
  }, [closeManualSelect]);

  const handleStartMatch = useCallback(
    async (onDone?: () => void) => {
      setMatchPauseMessage("");
      abortMatchRef.current = false;

      const toMatchCandidates = candidates.filter(c =>
        shouldMatchCandidate(c),
      );
      setMatchProgress({
        current: 0,
        total: toMatchCandidates.length,
        gameName: "",
      });

      const updatedCandidates = [...candidates];
      let matchedCount = 0;
      let consecutiveFetchFailures = 0;
      let pauseReason = "";

      for (let i = 0; i < candidates.length; i++) {
        if (abortMatchRef.current) {
          break;
        }

        if (!shouldMatchCandidate(candidates[i])) {
          continue;
        }

        matchedCount++;
        setMatchProgress(prev => ({
          ...prev,
          current: matchedCount,
          gameName: candidates[i].searchName,
        }));

        try {
          const matchResult = await FetchMetadataForCandidateWithPreference(
            candidates[i].searchName,
            preferredSource as enums.SourceType,
          );
          const results = matchResult?.matches || [];

          if (
            preferredSource !== NO_PREFERRED_SOURCE
            && !matchResult?.preferred_matched
          ) {
            const reason
              = matchResult?.preferred_error
                || t("batchImportModal.noMatchResult");
            const isNoResult = Boolean(matchResult?.preferred_no_result);

            updatedCandidates[i] = {
              ...updatedCandidates[i],
              matchedGame: null,
              matchedTags: [],
              matchSource: null,
              matchStatus: isNoResult ? "not_found" : "error",
              matchError: reason,
              allMatches: results,
              metadataDuplicateExistingId: undefined,
              metadataDuplicateExistingName: undefined,
            };

            if (isNoResult) {
              consecutiveFetchFailures = 0;
            }
            else {
              consecutiveFetchFailures++;
              if (matchResult?.preferred_rate_limited) {
                pauseReason = t(
                  "batchImportModal.preferredSource.rateLimitedPause",
                  {
                    source: preferredSourceLabel,
                    error: reason,
                  },
                );
              }
              else if (
                consecutiveFetchFailures
                >= PREFERRED_SOURCE_FAILURE_PAUSE_THRESHOLD
              ) {
                pauseReason = t(
                  "batchImportModal.preferredSource.consecutiveFailurePause",
                  {
                    source: preferredSourceLabel,
                    count: PREFERRED_SOURCE_FAILURE_PAUSE_THRESHOLD,
                    error: reason,
                  },
                );
              }
            }

            setCandidates([...updatedCandidates]);
            if (pauseReason) {
              abortMatchRef.current = true;
              break;
            }
          }
          else {
            consecutiveFetchFailures = 0;
            const bestMatch = pickBestMatch(results, preferredSource);

            if (bestMatch && bestMatch.Game) {
              updatedCandidates[i] = {
                ...updatedCandidates[i],
                matchedGame: bestMatch.Game,
                matchedTags: bestMatch.Tags || [],
                matchSource: bestMatch.Source,
                matchStatus: "matched",
                matchError: "",
                allMatches: results,
                metadataDuplicateExistingId: undefined,
                metadataDuplicateExistingName: undefined,
              };
            }
            else {
              updatedCandidates[i] = {
                ...updatedCandidates[i],
                matchedGame: null,
                matchedTags: [],
                matchSource: null,
                matchStatus: "not_found",
                matchError: t("batchImportModal.noMatchResult"),
                allMatches: results,
                metadataDuplicateExistingId: undefined,
                metadataDuplicateExistingName: undefined,
              };
            }
          }
        }
        catch (error) {
          console.error(`Failed to match ${candidates[i].searchName}:`, error);
          const reason = errorText(error);
          consecutiveFetchFailures++;
          updatedCandidates[i] = {
            ...updatedCandidates[i],
            matchedGame: null,
            matchedTags: [],
            matchSource: null,
            matchStatus: "error",
            matchError: reason,
            metadataDuplicateExistingId: undefined,
            metadataDuplicateExistingName: undefined,
          };

          if (
            consecutiveFetchFailures >= PREFERRED_SOURCE_FAILURE_PAUSE_THRESHOLD
          ) {
            pauseReason = t(
              "batchImportModal.preferredSource.consecutiveFailurePause",
              {
                source: preferredSourceLabel,
                count: PREFERRED_SOURCE_FAILURE_PAUSE_THRESHOLD,
                error: reason,
              },
            );
            abortMatchRef.current = true;
          }
        }

        setCandidates([...updatedCandidates]);
        if (pauseReason) {
          break;
        }

        if (!abortMatchRef.current) {
          await new Promise(resolve => setTimeout(resolve, 1500));
        }
      }

      if (pauseReason) {
        setMatchPauseMessage(pauseReason);
        toast.error(pauseReason);
        onDone?.();
        return;
      }

      if (!abortMatchRef.current) {
        try {
          setCandidates(await applyMetadataDuplicateHints(updatedCandidates));
        }
        catch (error) {
          console.error("Failed to check metadata duplicates:", error);
        }
        onDone?.();
      }
    },
    [
      candidates,
      preferredSource,
      preferredSourceLabel,
      shouldMatchCandidate,
      t,
    ],
  );

  const handleImport = useCallback(
    async (
      onDone: () => void,
      onFailed: () => void,
      options: ImportRequestOptions = {},
    ) => {
      try {
        const submittedCandidates = candidates
          .map((candidate, index) => ({ candidate, index }))
          .filter(({ candidate }) => shouldImportCandidate(candidate, options));
        const submittedIndexes = new Set(
          submittedCandidates.map(({ index }) => index),
        );
        const result = await BatchImportGames(
          candidatesToImportRequest(candidates, options),
        );
        setImportResult(result);
        setCandidates((current) => {
          const skippedIndexes = new Map<number, string>();
          const failedIndexes = new Map<number, string>();

          for (const { candidate, index } of submittedCandidates) {
            const skippedMessage = findImportResultMessage(
              candidate,
              result.skipped_names,
            );
            if (skippedMessage) {
              skippedIndexes.set(index, skippedMessage);
              continue;
            }

            const failedMessage = findImportResultMessage(
              candidate,
              result.failed_names,
            );
            if (failedMessage) {
              failedIndexes.set(index, failedMessage);
            }
          }

          let remainingImported = result.success;
          return current.flatMap((candidate, index) => {
            if (!submittedIndexes.has(index)) {
              return [candidate];
            }

            const skippedMessage = skippedIndexes.get(index);
            if (skippedMessage) {
              return [
                {
                  ...candidate,
                  isSelected: false,
                  matchError: skippedMessage,
                },
              ];
            }

            const failedMessage = failedIndexes.get(index);
            if (failedMessage) {
              return [
                {
                  ...candidate,
                  isSelected: false,
                  matchStatus: "error",
                  matchError: failedMessage,
                },
              ];
            }

            if (remainingImported > 0) {
              remainingImported--;
              return [];
            }

            return [
              {
                ...candidate,
                isSelected: false,
              },
            ];
          });
        });
        onDone();

        if (result.success > 0) {
          toast.success(
            t("batchImportModal.toast.importSuccess", {
              count: result.success,
            }),
          );
          onImportComplete();
        }
      }
      catch (error) {
        console.error("Failed to import:", error);
        toast.error(t("batchImportModal.toast.importFailed"));
        onFailed();
      }
    },
    [candidates, onImportComplete, t],
  );

  const toggleCandidate = useCallback((index: number) => {
    setCandidates((current) => {
      const updated = [...current];
      updated[index].isSelected = !updated[index].isSelected;
      return updated;
    });
  }, []);

  const toggleAllCandidates = useCallback(
    (checked: boolean, indexes?: number[]) => {
      setCandidates((current) => {
        const targetIndexes = indexes ? new Set(indexes) : null;
        return current.map((candidate, index) => {
          if (targetIndexes && !targetIndexes.has(index)) {
            return candidate;
          }
          return {
            ...candidate,
            isSelected: checked,
          };
        });
      });
    },
    [],
  );

  const updateSearchName = useCallback((index: number, name: string) => {
    setCandidates((current) => {
      const updated = [...current];
      updated[index] = {
        ...updated[index],
        searchName: name,
        matchStatus: "pending",
        matchedGame: null,
        matchedTags: [],
        matchSource: null,
        matchError: "",
        metadataDuplicateExistingId: undefined,
        metadataDuplicateExistingName: undefined,
      };
      return updated;
    });
    setMatchPauseMessage("");
  }, []);

  const updateSelectedExe = useCallback((index: number, exe: string) => {
    setCandidates((current) => {
      const updated = [...current];
      updated[index] = {
        ...updated[index],
        selectedExe: exe,
      };
      return updated;
    });
  }, []);

  const openManualSelect = useCallback(
    async (index: number) => {
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
          const matchResult = await FetchMetadataForCandidateWithPreference(
            candidates[index].searchName,
            preferredSource as enums.SourceType,
          );
          setManualMatches(matchResult?.matches || []);
        }
        catch (error) {
          console.error("Failed to search:", error);
        }
        finally {
          setIsSearching(false);
        }
      }
    },
    [candidates, preferredSource],
  );

  const selectManualMatch = useCallback(
    async (match: voTypes.GameMetadataFromWebVO) => {
      if (!match.Game || manualSelectIndex === null) {
        return;
      }

      const updated = [...candidates];
      updated[manualSelectIndex] = {
        ...updated[manualSelectIndex],
        matchedGame: match.Game,
        matchedTags: match.Tags || [],
        matchSource: match.Source,
        matchStatus: "manual",
        matchError: "",
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
      closeManualSelect();
    },
    [candidates, closeManualSelect, manualSelectIndex],
  );

  const handleSearchById = useCallback(async () => {
    if (!manualId || manualSelectIndex === null) {
      return;
    }
    setIsSearching(true);
    try {
      const request = new vo.MetadataRequest({
        source: selectedManualSource,
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
  }, [manualId, manualSelectIndex, selectedManualSource, selectManualMatch, t]);

  const handleSkipMetadata = useCallback(() => {
    if (manualSelectIndex === null) {
      return;
    }
    setCandidates((current) => {
      const updated = [...current];
      updated[manualSelectIndex] = {
        ...updated[manualSelectIndex],
        matchedGame: null,
        matchedTags: [],
        matchSource: null,
        matchStatus: "not_found",
        matchError: "",
        metadataDuplicateExistingId: undefined,
        metadataDuplicateExistingName: undefined,
      };
      return updated;
    });
    closeManualSelect();
  }, [closeManualSelect, manualSelectIndex]);

  const stopMatching = useCallback(() => {
    abortMatchRef.current = true;
  }, []);

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
  const errorCount = candidates.filter(
    c => c.isSelected && c.matchStatus === "error",
  ).length;
  const matchableCount = pendingCount + errorCount;

  return {
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
    manualSource: selectedManualSource,
    matchedCount,
    notFoundCount,
    pendingCount,
    errorCount,
    matchableCount,
    setCandidates,
    setSkippedCandidates,
    resetImportFlow,
    handleStartMatch,
    handleImport,
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
  };
}
