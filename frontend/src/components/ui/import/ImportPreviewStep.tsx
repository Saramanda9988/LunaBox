import type { ReactNode } from "react";
import type { BetterDataTableColumn } from "../better/BetterDataTable";
import type { ImportCandidate, ImportMatchStatus } from "./types";
import { useMemo, useState } from "react";
import { SkippedImportCandidatesModal } from "../../modal/SkippedImportCandidatesModal";
import { BetterDataTable } from "../better/BetterDataTable";
import { BetterSelect } from "../better/BetterSelect";
import { isMatchedImportCandidate } from "./importFlow";

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
  emptyFiltered?: string;
  startMatching: string;
  importCount: (count: number) => string;
  importMatchedCount?: (count: number) => string;
  leftAction?: string;
  statusPending: string;
  statusMatched: string;
  statusNotFound: string;
  statusError: string;
  manualSelect: string;
  openFolder?: string;
  metadataExists?: (name: string) => string;
  skippedSummary?: (count: number) => string;
  skippedDetails?: string;
  skippedViewDetails?: string;
  skippedModalTitle?: string;
  skippedModalHint?: string;
  skippedReason?: string;
  skippedPath?: string;
  closeSkippedModal?: string;
  filterAll?: string;
  filterByStatus?: string;
  filterBySelection?: string;
  filterSelected?: string;
  filterUnselected?: string;
  filteredCount?: (visible: number, total: number) => string;
  clearFilters?: string;
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
  onImportMatched?: () => void;
  onToggleAll: (checked: boolean, indexes?: number[]) => void;
  onToggleCandidate: (index: number) => void;
  onUpdateSearchName: (index: number, name: string) => void;
  onUpdateSelectedExe: (index: number, exe: string) => void;
  onManualSelect: (index: number) => void;
  onOpenFolder?: (path: string) => void;
}

const EMPTY_SKIPPED_CANDIDATES: ImportCandidate[] = [];

type StatusFilterValue = "" | "pending" | "matched" | "not_found" | "error";
type SelectionFilterValue = "" | "selected" | "unselected";

interface IndexedCandidate {
  candidate: ImportCandidate;
  originalIndex: number;
}

function matchesStatusFilter(
  candidate: ImportCandidate,
  filter: StatusFilterValue,
): boolean {
  if (filter === "")
    return true;
  if (filter === "matched") {
    return (
      candidate.matchStatus === "matched" || candidate.matchStatus === "manual"
    );
  }
  return candidate.matchStatus === (filter as ImportMatchStatus);
}

function matchesSelectionFilter(
  candidate: ImportCandidate,
  filter: SelectionFilterValue,
): boolean {
  if (filter === "")
    return true;
  if (filter === "selected")
    return candidate.isSelected;
  return !candidate.isSelected;
}

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
  onImportMatched,
  onToggleAll,
  onToggleCandidate,
  onUpdateSearchName,
  onUpdateSelectedExe,
  onManualSelect,
  onOpenFolder,
}: ImportPreviewStepProps) {
  const [showSkippedModal, setShowSkippedModal] = useState(false);
  const [statusFilter, setStatusFilter] = useState<StatusFilterValue>("");
  const [selectionFilter, setSelectionFilter]
    = useState<SelectionFilterValue>("");

  const selectedCount = candidates.filter(c => c.isSelected).length;
  const matchedSelectedCount = candidates.filter(
    c => c.isSelected && isMatchedImportCandidate(c),
  ).length;
  const skippedCount = skippedCandidates.length;
  const hasDetectedItems = candidates.length > 0 || skippedCount > 0;
  const pathLabel = (filePath: string) =>
    filePath.split(/[/\\]/).pop() || filePath;

  const visibleRows = useMemo<IndexedCandidate[]>(
    () =>
      candidates
        .map((candidate, originalIndex) => ({ candidate, originalIndex }))
        .filter(
          ({ candidate }) =>
            matchesStatusFilter(candidate, statusFilter)
            && matchesSelectionFilter(candidate, selectionFilter),
        ),
    [candidates, statusFilter, selectionFilter],
  );

  const visibleSelectableCount = visibleRows.length;
  const visibleSelectedCount = visibleRows.filter(
    r => r.candidate.isSelected,
  ).length;
  const allVisibleSelected
    = visibleSelectableCount > 0
      && visibleSelectedCount === visibleSelectableCount;
  const someVisibleSelected
    = visibleSelectedCount > 0 && visibleSelectedCount < visibleSelectableCount;

  const isFiltering = statusFilter !== "" || selectionFilter !== "";
  const allLabel = labels.filterAll ?? "All";

  const statusFilterOptions = [
    {
      value: "pending" as const,
      label: labels.statusPending,
      icon: "i-mdi-clock-outline",
      pillColor:
        "bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400",
    },
    {
      value: "matched" as const,
      label: labels.statusMatched,
      icon: "i-mdi-check-circle",
      pillColor:
        "bg-success-100 text-success-700 dark:bg-success-900/30 dark:text-success-400",
    },
    {
      value: "not_found" as const,
      label: labels.statusNotFound,
      icon: "i-mdi-alert-circle",
      pillColor:
        "bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400",
    },
    {
      value: "error" as const,
      label: labels.statusError,
      icon: "i-mdi-close-circle",
      pillColor:
        "bg-error-100 text-error-700 dark:bg-error-900/30 dark:text-error-400",
    },
  ];

  const selectionFilterOptions = [
    {
      value: "selected" as const,
      label: labels.filterSelected ?? "Selected",
      icon: "i-mdi-checkbox-marked",
      iconColor: "text-success-500",
    },
    {
      value: "unselected" as const,
      label: labels.filterUnselected ?? "Unselected",
      icon: "i-mdi-checkbox-blank-outline",
      iconColor: "text-brand-400",
    },
  ];

  const columns: BetterDataTableColumn<IndexedCandidate>[] = [
    {
      key: "selected",
      header: (
        <input
          type="checkbox"
          checked={allVisibleSelected}
          disabled={visibleSelectableCount === 0}
          ref={(element) => {
            if (element) {
              element.indeterminate = someVisibleSelected;
            }
          }}
          onChange={e =>
            onToggleAll(
              e.target.checked,
              visibleRows.map(row => row.originalIndex),
            )}
          aria-label={labels.detected}
        />
      ),
      className: "w-16",
      filter: {
        value: selectionFilter,
        onChange: value => setSelectionFilter(value as SelectionFilterValue),
        title: labels.filterBySelection ?? "Filter by selection",
        allLabel,
        options: selectionFilterOptions,
      },
      render: ({ candidate, originalIndex }) => (
        <input
          type="checkbox"
          checked={candidate.isSelected}
          onChange={() => onToggleCandidate(originalIndex)}
          aria-label={candidate.searchName}
        />
      ),
    },
    {
      key: "searchName",
      header: labels.searchName,
      className: "w-[34%]",
      render: ({ candidate, originalIndex }) => (
        <div className="min-w-0">
          <input
            type="text"
            value={candidate.searchName}
            onChange={e => onUpdateSearchName(originalIndex, e.target.value)}
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
      render: ({ candidate, originalIndex }) =>
        candidate.executables.length > 1 ? (
          <BetterSelect
            value={candidate.selectedExe}
            onChange={value => onUpdateSelectedExe(originalIndex, value)}
            options={candidate.executables.map(exe => ({
              value: exe,
              label: pathLabel(exe),
            }))}
            className="w-full"
          />
        ) : (
          <span className="block truncate text-sm text-brand-500 dark:text-brand-400">
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
      filter: {
        value: statusFilter,
        onChange: value => setStatusFilter(value as StatusFilterValue),
        title: labels.filterByStatus ?? "Filter by status",
        allLabel,
        pill: true,
        align: "end",
        options: statusFilterOptions,
      },
      render: ({ candidate }) => (
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
      render: ({ candidate, originalIndex }) => (
        <div className="inline-flex items-center gap-1">
          {onOpenFolder && (
            <button
              type="button"
              onClick={() =>
                onOpenFolder(candidate.gameDirectory || candidate.folderPath)}
              aria-label={labels.openFolder || "Open current folder"}
              className={`inline-flex h-8 w-8 items-center justify-center rounded-md text-sm transition-colors ${theme.manualButtonClassName}`}
            >
              <div
                className="i-mdi-folder-open-outline text-lg"
                aria-hidden="true"
              />
            </button>
          )}
          <button
            type="button"
            onClick={() => onManualSelect(originalIndex)}
            aria-label={labels.manualSelect}
            className={`inline-flex h-8 w-8 items-center justify-center rounded-md text-sm transition-colors ${theme.manualButtonClassName}`}
          >
            <div className="i-mdi-pencil text-lg" aria-hidden="true" />
          </button>
        </div>
      ),
    },
  ];

  const clearAllFilters = () => {
    setStatusFilter("");
    setSelectionFilter("");
  };

  const filteredEmptyMessage
    = isFiltering && candidates.length > 0
      ? (labels.emptyFiltered ?? labels.empty)
      : labels.empty;

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

      {isFiltering && candidates.length > 0 && (
        <div className="flex items-center justify-between gap-3 rounded-lg border border-brand-200 bg-brand-50/70 px-3 py-2 text-xs text-brand-600 dark:border-brand-700 dark:bg-brand-800/50 dark:text-brand-300">
          <div className="flex min-w-0 items-center gap-2">
            <div className="i-mdi-filter shrink-0 text-success-500" />
            <span className="truncate">
              {labels.filteredCount
                ? labels.filteredCount(visibleRows.length, candidates.length)
                : `${visibleRows.length} / ${candidates.length}`}
            </span>
          </div>
          <button
            type="button"
            onClick={clearAllFilters}
            className="inline-flex shrink-0 items-center gap-1 rounded-md border border-brand-200 bg-white/70 px-2 py-1 font-medium text-brand-700 transition-colors hover:bg-brand-100 dark:border-brand-600 dark:bg-brand-900/40 dark:text-brand-200 dark:hover:bg-brand-700"
          >
            <div className="i-mdi-filter-off-outline text-sm" />
            {labels.clearFilters ?? "Clear filters"}
          </button>
        </div>
      )}

      <BetterDataTable
        rows={visibleRows}
        columns={columns}
        rowKey={(row, index) =>
          `${row.candidate.folderPath}-${row.candidate.selectedExe}-${row.originalIndex}-${index}`}
        empty={filteredEmptyMessage}
        rowClassName={row => (row.candidate.isSelected ? "" : "opacity-50")}
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
            {onImportMatched
              && matchedSelectedCount > 0
              && matchedSelectedCount < selectedCount && (
              <button
                type="button"
                onClick={onImportMatched}
                className="inline-flex h-11 w-full items-center justify-center gap-1.5 whitespace-nowrap rounded-lg border border-success-300 bg-success-50 px-3 text-sm font-medium text-success-700 transition-colors hover:bg-success-100 dark:border-success-700 dark:bg-success-900/25 dark:text-success-300 dark:hover:bg-success-900/40 sm:w-auto"
              >
                <div className="i-mdi-check-circle-outline text-base" />
                {labels.importMatchedCount
                  ? labels.importMatchedCount(matchedSelectedCount)
                  : labels.importCount(matchedSelectedCount)}
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
