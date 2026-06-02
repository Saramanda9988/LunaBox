import type { ReactNode } from "react";
import type { BetterDataTableColumn } from "../better/BetterDataTable";
import type { ImportCandidate } from "./types";
import { useState } from "react";
import { SkippedImportCandidatesModal } from "../../modal/SkippedImportCandidatesModal";
import { BetterDataTable } from "../better/BetterDataTable";
import { BetterSelect } from "../better/BetterSelect";

interface ImportPreviewTheme {
  detectedCardClassName: string;
  detectedValueClassName: string;
  detectedLabelClassName: string;
  searchInputFocusClassName: string;
  manualButtonClassName: string;
  startMatchButtonClassName: string;
  importButtonClassName: string;
}

interface ImportPreviewLabels {
  detected: string;
  matched: string;
  notMatched: string;
  pending: string;
  searchName: string;
  executable: string;
  matchStatus: string;
  action: string;
  empty: string;
  startMatching: string;
  importCount: (count: number) => string;
  leftAction?: string;
  statusPending: string;
  statusMatched: string;
  statusNotFound: string;
  statusError: string;
  manualSelect: string;
  metadataExists?: (name: string) => string;
  skippedSummary?: (count: number) => string;
  skippedDetails?: string;
  skippedViewDetails?: string;
  skippedModalTitle?: string;
  skippedModalHint?: string;
  skippedReason?: string;
  skippedPath?: string;
  closeSkippedModal?: string;
}

interface ImportPreviewStepProps {
  candidates: ImportCandidate[];
  skippedCandidates?: ImportCandidate[];
  matchedCount: number;
  notFoundCount: number;
  pendingCount: number;
  canStartMatch?: boolean;
  labels: ImportPreviewLabels;
  theme: ImportPreviewTheme;
  toolbar?: ReactNode;
  actionToolbar?: ReactNode;
  onLeftAction?: () => void;
  onStartMatch: () => void;
  onImport: () => void;
  onToggleAll: (checked: boolean) => void;
  onToggleCandidate: (index: number) => void;
  onUpdateSearchName: (index: number, name: string) => void;
  onUpdateSelectedExe: (index: number, exe: string) => void;
  onManualSelect: (index: number) => void;
}

const EMPTY_SKIPPED_CANDIDATES: ImportCandidate[] = [];

export function ImportPreviewStep({
  candidates,
  skippedCandidates = EMPTY_SKIPPED_CANDIDATES,
  matchedCount,
  notFoundCount,
  pendingCount,
  canStartMatch = pendingCount > 0,
  labels,
  theme,
  toolbar,
  actionToolbar,
  onLeftAction,
  onStartMatch,
  onImport,
  onToggleAll,
  onToggleCandidate,
  onUpdateSearchName,
  onUpdateSelectedExe,
  onManualSelect,
}: ImportPreviewStepProps) {
  const [showSkippedModal, setShowSkippedModal] = useState(false);
  const selectedCount = candidates.filter(c => c.isSelected).length;
  const skippedCount = skippedCandidates.length;
  const hasDetectedItems = candidates.length > 0 || skippedCount > 0;
  const pathLabel = (filePath: string) =>
    filePath.split(/[/\\]/).pop() || filePath;

  const columns: BetterDataTableColumn<ImportCandidate>[] = [
    {
      key: "selected",
      header: (
        <input
          type="checkbox"
          checked={
            candidates.length > 0 && candidates.every(c => c.isSelected)
          }
          onChange={e => onToggleAll(e.target.checked)}
          aria-label={labels.detected}
        />
      ),
      className: "w-10",
      render: (_candidate, index) => (
        <input
          type="checkbox"
          checked={candidates[index].isSelected}
          onChange={() => onToggleCandidate(index)}
          aria-label={candidates[index].searchName}
        />
      ),
    },
    {
      key: "searchName",
      header: labels.searchName,
      className: "w-[34%]",
      render: (candidate, index) => (
        <div className="min-w-0">
          <input
            type="text"
            value={candidate.searchName}
            onChange={e => onUpdateSearchName(index, e.target.value)}
            className={`w-full border-b border-transparent bg-transparent text-sm text-brand-900 hover:border-brand-300 focus:outline-none dark:text-white ${theme.searchInputFocusClassName}`}
          />
          {candidate.matchedGame && (
            <>
              <div className="mt-1 flex min-w-0 items-center gap-1 text-xs text-success-600 dark:text-success-400">
                <span className="truncate">{candidate.matchedGame.name}</span>
                <span className="shrink-0 text-brand-400">
                  (
                  {candidate.matchSource}
                  )
                </span>
              </div>
              {candidate.metadataDuplicateExistingName && (
                <div className="mt-1 flex items-center gap-1 text-xs text-yellow-700 dark:text-yellow-300">
                  <div className="i-mdi-alert-circle-outline shrink-0" />
                  <span className="truncate">
                    {labels.metadataExists
                      ? labels.metadataExists(
                          candidate.metadataDuplicateExistingName,
                        )
                      : `Metadata already exists: ${candidate.metadataDuplicateExistingName}`}
                  </span>
                </div>
              )}
            </>
          )}
          {candidate.matchError && (
            <div
              className={`mt-1 flex min-w-0 items-center gap-1 text-xs ${
                candidate.matchStatus === "error"
                  ? "text-error-600 dark:text-error-300"
                  : "text-orange-600 dark:text-orange-300"
              }`}
              title={candidate.matchError}
            >
              <div className="i-mdi-alert-circle-outline shrink-0" />
              <span className="truncate">{candidate.matchError}</span>
            </div>
          )}
        </div>
      ),
    },
    {
      key: "executable",
      header: labels.executable,
      className: "w-[30%]",
      render: (candidate, index) =>
        candidate.executables.length > 1 ? (
          <BetterSelect
            value={candidate.selectedExe}
            onChange={value => onUpdateSelectedExe(index, value)}
            options={candidate.executables.map(exe => ({
              value: exe,
              label: pathLabel(exe),
            }))}
            className="w-full"
          />
        ) : (
          <span
            className="block truncate text-sm text-brand-500 dark:text-brand-400"
            title={candidate.selectedExe || candidate.folderPath}
          >
            {pathLabel(candidate.selectedExe || candidate.folderPath)}
          </span>
        ),
    },
    {
      key: "status",
      header: labels.matchStatus,
      className: "w-32",
      headerClassName: "text-center",
      cellClassName: "text-center",
      render: candidate => (
        <>
          {candidate.matchStatus === "pending" && (
            <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-700 dark:bg-gray-900/30 dark:text-gray-400">
              <div className="i-mdi-clock-outline mr-1" />
              {labels.statusPending}
            </span>
          )}
          {(candidate.matchStatus === "matched"
            || candidate.matchStatus === "manual") && (
            <span className="inline-flex items-center rounded-full bg-success-100 px-2 py-1 text-xs text-success-700 dark:bg-success-900/30 dark:text-success-400">
              <div className="i-mdi-check-circle mr-1" />
              {labels.statusMatched}
            </span>
          )}
          {candidate.matchStatus === "not_found" && (
            <span className="inline-flex items-center rounded-full bg-orange-100 px-2 py-1 text-xs text-orange-700 dark:bg-orange-900/30 dark:text-orange-400">
              <div className="i-mdi-alert-circle mr-1" />
              {labels.statusNotFound}
            </span>
          )}
          {candidate.matchStatus === "error" && (
            <span className="inline-flex items-center rounded-full bg-error-100 px-2 py-1 text-xs text-error-700 dark:bg-error-900/30 dark:text-error-400">
              <div className="i-mdi-close-circle mr-1" />
              {labels.statusError}
            </span>
          )}
        </>
      ),
    },
    {
      key: "action",
      header: labels.action,
      className: "w-20",
      headerClassName: "text-center",
      cellClassName: "text-center",
      render: (_candidate, index) => (
        <button
          type="button"
          onClick={() => onManualSelect(index)}
          className={`inline-flex h-8 w-8 items-center justify-center rounded-md text-sm transition-colors ${theme.manualButtonClassName}`}
          title={labels.manualSelect}
        >
          <div className="i-mdi-pencil text-lg" />
        </button>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      {hasDetectedItems && (
        <div className="flex gap-4">
          <div
            className={`flex-1 rounded-lg p-4 text-center ${theme.detectedCardClassName}`}
          >
            <div
              className={`text-3xl font-bold ${theme.detectedValueClassName}`}
            >
              {candidates.length}
            </div>
            <div className={`text-sm ${theme.detectedLabelClassName}`}>
              {labels.detected}
            </div>
          </div>
          <div className="flex-1 rounded-lg bg-success-50 p-4 text-center dark:bg-success-900/20">
            <div className="text-3xl font-bold text-success-600 dark:text-success-400">
              {matchedCount}
            </div>
            <div className="text-sm text-success-700 dark:text-success-300">
              {labels.matched}
            </div>
          </div>
          {notFoundCount > 0 && (
            <div className="flex-1 rounded-lg bg-orange-50 p-4 text-center dark:bg-orange-900/20">
              <div className="text-3xl font-bold text-orange-600 dark:text-orange-400">
                {notFoundCount}
              </div>
              <div className="text-sm text-orange-700 dark:text-orange-300">
                {labels.notMatched}
              </div>
            </div>
          )}
          {pendingCount > 0 && (
            <div className="flex-1 rounded-lg bg-gray-50 p-4 text-center dark:bg-gray-900/20">
              <div className="text-3xl font-bold text-gray-600 dark:text-gray-400">
                {pendingCount}
              </div>
              <div className="text-sm text-gray-700 dark:text-gray-300">
                {labels.pending}
              </div>
            </div>
          )}
        </div>
      )}

      {toolbar}

      {skippedCount > 0 && (
        <div className="flex items-center justify-between gap-3 rounded-lg border border-yellow-200 bg-yellow-50 px-4 py-3 dark:border-yellow-800/80 dark:bg-yellow-900/20">
          <div className="flex min-w-0 items-center gap-3">
            <div className="i-mdi-alert-circle-outline shrink-0 text-xl text-yellow-600 dark:text-yellow-300" />
            <div className="min-w-0">
              <div className="truncate text-sm font-medium text-yellow-800 dark:text-yellow-200">
                {labels.skippedSummary
                  ? labels.skippedSummary(skippedCount)
                  : `${skippedCount} skipped existing items`}
              </div>
              {labels.skippedDetails && (
                <div className="mt-0.5 truncate text-xs text-yellow-700/80 dark:text-yellow-300/80">
                  {labels.skippedDetails}
                </div>
              )}
            </div>
          </div>
          <button
            type="button"
            onClick={() => setShowSkippedModal(true)}
            className="inline-flex shrink-0 items-center gap-1.5 rounded-lg border border-yellow-300 bg-white/70 px-3 py-1.5 text-xs font-medium text-yellow-800 transition-colors hover:bg-yellow-100 dark:border-yellow-700 dark:bg-yellow-950/30 dark:text-yellow-200 dark:hover:bg-yellow-900/40"
          >
            <div className="i-mdi-format-list-bulleted text-base" />
            {labels.skippedViewDetails || labels.skippedDetails || "View list"}
          </button>
        </div>
      )}

      <BetterDataTable
        rows={candidates}
        columns={columns}
        rowKey={(candidate, index) =>
          `${candidate.folderPath}-${candidate.selectedExe}-${index}`}
        empty={labels.empty}
        rowClassName={candidate => (candidate.isSelected ? "" : "opacity-50")}
      />

      <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
        <div className="shrink-0">
          {onLeftAction && labels.leftAction && (
            <button
              type="button"
              onClick={onLeftAction}
              className="inline-flex h-11 w-full items-center justify-center rounded-lg border border-brand-300 px-4 text-sm font-medium text-brand-700 hover:bg-brand-100 dark:border-brand-600 dark:text-brand-300 dark:hover:bg-brand-700 sm:w-auto"
            >
              {labels.leftAction}
            </button>
          )}
        </div>
        <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-end">
          {actionToolbar}
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-end">
            {canStartMatch && (
              <button
                type="button"
                onClick={onStartMatch}
                className={`inline-flex h-11 w-full items-center justify-center whitespace-nowrap rounded-lg px-4 text-sm font-medium text-white sm:w-auto ${theme.startMatchButtonClassName}`}
              >
                {labels.startMatching}
              </button>
            )}
            <button
              type="button"
              onClick={onImport}
              disabled={selectedCount === 0}
              className={`inline-flex h-11 w-full items-center justify-center whitespace-nowrap rounded-lg px-4 text-sm font-medium text-white disabled:opacity-50 sm:w-auto ${theme.importButtonClassName}`}
            >
              {labels.importCount(selectedCount)}
            </button>
          </div>
        </div>
      </div>

      <SkippedImportCandidatesModal
        isOpen={showSkippedModal}
        candidates={skippedCandidates}
        labels={{
          title: labels.skippedModalTitle || "Skipped existing games",
          hint:
            labels.skippedModalHint
            || (labels.skippedSummary
              ? labels.skippedSummary(skippedCount)
              : `${skippedCount} skipped existing items`),
          path: labels.skippedPath || "Path",
          reason: labels.skippedReason || "Reason",
          close: labels.closeSkippedModal || "Close",
        }}
        onClose={() => setShowSkippedModal(false)}
      />
    </div>
  );
}
