import type { ImportCandidate } from "./types";

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
  leftAction: string;
  statusPending: string;
  statusMatched: string;
  statusNotFound: string;
  statusError: string;
  manualSelect: string;
}

interface ImportPreviewStepProps {
  candidates: ImportCandidate[];
  matchedCount: number;
  notFoundCount: number;
  pendingCount: number;
  labels: ImportPreviewLabels;
  theme: ImportPreviewTheme;
  onLeftAction: () => void;
  onStartMatch: () => void;
  onImport: () => void;
  onToggleAll: (checked: boolean) => void;
  onToggleCandidate: (index: number) => void;
  onUpdateSearchName: (index: number, name: string) => void;
  onUpdateSelectedExe: (index: number, exe: string) => void;
  onManualSelect: (index: number) => void;
}

export function ImportPreviewStep({
  candidates,
  matchedCount,
  notFoundCount,
  pendingCount,
  labels,
  theme,
  onLeftAction,
  onStartMatch,
  onImport,
  onToggleAll,
  onToggleCandidate,
  onUpdateSearchName,
  onUpdateSelectedExe,
  onManualSelect,
}: ImportPreviewStepProps) {
  const selectedCount = candidates.filter(c => c.isSelected).length;

  return (
    <div className="space-y-4">
      <div className="flex gap-4">
        <div className={`flex-1 rounded-lg p-4 text-center ${theme.detectedCardClassName}`}>
          <div className={`text-3xl font-bold ${theme.detectedValueClassName}`}>
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

      <div className="max-h-[400px] overflow-y-auto rounded-lg border border-brand-200 dark:border-brand-700">
        {candidates.length === 0
          ? (
              <div className="p-8 text-center text-brand-400">
                {labels.empty}
              </div>
            )
          : (
              <table className="w-full">
                <thead className="sticky top-0 bg-brand-50 dark:bg-brand-700">
                  <tr>
                    <th className="w-10 px-3 py-2 text-left text-sm font-medium text-brand-600 dark:text-brand-300">
                      <input
                        type="checkbox"
                        checked={candidates.length > 0 && candidates.every(c => c.isSelected)}
                        onChange={e => onToggleAll(e.target.checked)}
                      />
                    </th>
                    <th className="px-3 py-2 text-left text-sm font-medium text-brand-600 dark:text-brand-300">
                      {labels.searchName}
                    </th>
                    <th className="px-3 py-2 text-left text-sm font-medium text-brand-600 dark:text-brand-300">
                      {labels.executable}
                    </th>
                    <th className="w-32 px-3 py-2 text-center text-sm font-medium text-brand-600 dark:text-brand-300">
                      {labels.matchStatus}
                    </th>
                    <th className="w-20 px-3 py-2 text-center text-sm font-medium text-brand-600 dark:text-brand-300">
                      {labels.action}
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-brand-100 dark:divide-brand-700">
                  {candidates.map((candidate, index) => (
                    <tr
                      key={`${candidate.folderPath}-${candidate.selectedExe}-${index}`}
                      className={candidate.isSelected ? "hover:bg-brand-50 dark:hover:bg-brand-750" : "opacity-50"}
                    >
                      <td className="px-3 py-2">
                        <input
                          type="checkbox"
                          checked={candidate.isSelected}
                          onChange={() => onToggleCandidate(index)}
                        />
                      </td>
                      <td className="px-3 py-2">
                        <input
                          type="text"
                          value={candidate.searchName}
                          onChange={e => onUpdateSearchName(index, e.target.value)}
                          className={`w-full border-b border-transparent bg-transparent text-sm text-brand-900 hover:border-brand-300 focus:outline-none dark:text-white ${theme.searchInputFocusClassName}`}
                        />
                        {candidate.matchedGame && (
                          <div className="mt-1 flex items-center gap-1 text-xs text-success-600 dark:text-success-400">
                            <span>
                              →
                              {candidate.matchedGame.name}
                            </span>
                            <span className="text-brand-400">
                              (
                              {candidate.matchSource}
                              )
                            </span>
                          </div>
                        )}
                      </td>
                      <td className="px-3 py-2">
                        {candidate.executables.length > 1
                          ? (
                              <select
                                value={candidate.selectedExe}
                                onChange={e => onUpdateSelectedExe(index, e.target.value)}
                                className="w-full rounded border border-brand-200 bg-transparent px-2 py-1 text-sm text-brand-700 dark:border-brand-600 dark:text-brand-300"
                              >
                                {candidate.executables.map(exe => (
                                  <option key={exe} value={exe}>
                                    {exe.split(/[/\\]/).pop()}
                                  </option>
                                ))}
                              </select>
                            )
                          : (
                              <span className="text-sm text-brand-500 dark:text-brand-400">
                                {candidate.selectedExe.split(/[/\\]/).pop()}
                              </span>
                            )}
                      </td>
                      <td className="px-3 py-2 text-center">
                        {candidate.matchStatus === "pending" && (
                          <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-700 dark:bg-gray-900/30 dark:text-gray-400">
                            <div className="i-mdi-clock-outline mr-1" />
                            {" "}
                            {labels.statusPending}
                          </span>
                        )}
                        {(candidate.matchStatus === "matched" || candidate.matchStatus === "manual") && (
                          <span className="inline-flex items-center rounded-full bg-success-100 px-2 py-1 text-xs text-success-700 dark:bg-success-900/30 dark:text-success-400">
                            <div className="i-mdi-check-circle mr-1" />
                            {" "}
                            {labels.statusMatched}
                          </span>
                        )}
                        {candidate.matchStatus === "not_found" && (
                          <span className="inline-flex items-center rounded-full bg-orange-100 px-2 py-1 text-xs text-orange-700 dark:bg-orange-900/30 dark:text-orange-400">
                            <div className="i-mdi-alert-circle mr-1" />
                            {" "}
                            {labels.statusNotFound}
                          </span>
                        )}
                        {candidate.matchStatus === "error" && (
                          <span className="inline-flex items-center rounded-full bg-error-100 px-2 py-1 text-xs text-error-700 dark:bg-error-900/30 dark:text-error-400">
                            <div className="i-mdi-close-circle mr-1" />
                            {" "}
                            {labels.statusError}
                          </span>
                        )}
                      </td>
                      <td className="px-3 py-2 text-center">
                        <button
                          type="button"
                          onClick={() => onManualSelect(index)}
                          className={`text-sm ${theme.manualButtonClassName}`}
                          title={labels.manualSelect}
                        >
                          <div className="i-mdi-pencil text-lg" />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
      </div>

      <div className="flex justify-between">
        <button
          type="button"
          onClick={onLeftAction}
          className="rounded-lg border border-brand-300 px-5 py-2.5 text-sm font-medium text-brand-700 hover:bg-brand-100 dark:border-brand-600 dark:text-brand-300 dark:hover:bg-brand-700"
        >
          {labels.leftAction}
        </button>
        <div className="flex gap-3">
          {pendingCount > 0 && (
            <button
              type="button"
              onClick={onStartMatch}
              className={`rounded-lg px-5 py-2.5 text-sm font-medium text-white ${theme.startMatchButtonClassName}`}
            >
              {labels.startMatching}
            </button>
          )}
          <button
            type="button"
            onClick={onImport}
            disabled={selectedCount === 0}
            className={`rounded-lg px-5 py-2.5 text-sm font-medium text-white disabled:opacity-50 ${theme.importButtonClassName}`}
          >
            {labels.importCount(selectedCount)}
          </button>
        </div>
      </div>
    </div>
  );
}
