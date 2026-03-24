import type { enums, vo } from "../../../../wailsjs/go/models";
import { BetterSelect } from "../better/BetterSelect";
import { ImportTaskLoadingStep } from "./ImportTaskLoadingStep";

interface ManualSourceOption {
  value: enums.SourceType;
  label: string;
}

interface ImportManualSelectTheme {
  loadingSpinnerClassName: string;
  cardHoverClassName: string;
  searchButtonClassName: string;
}

interface ImportManualSelectLabels {
  searching: string;
  noMatchResult: string;
  searchById: string;
  search: string;
  skipMetadata: string;
}

interface ImportManualSelectModalProps {
  isOpen: boolean;
  title: string;
  candidateName: string;
  isSearching: boolean;
  matches: vo.GameMetadataFromWebVO[];
  manualSource: enums.SourceType;
  manualId: string;
  sourceOptions: ManualSourceOption[];
  idPlaceholder: string;
  theme: ImportManualSelectTheme;
  labels: ImportManualSelectLabels;
  searchDisabled: boolean;
  onClose: () => void;
  onSelectMatch: (match: vo.GameMetadataFromWebVO) => void;
  onSourceChange: (source: enums.SourceType) => void;
  onManualIdChange: (id: string) => void;
  onSearchById: () => void;
  onSkipMetadata: () => void;
}

export function ImportManualSelectModal({
  isOpen,
  title,
  candidateName,
  isSearching,
  matches,
  manualSource,
  manualId,
  sourceOptions,
  idPlaceholder,
  theme,
  labels,
  searchDisabled,
  onClose,
  onSelectMatch,
  onSourceChange,
  onManualIdChange,
  onSearchById,
  onSkipMetadata,
}: ImportManualSelectModalProps) {
  if (!isOpen) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-60 flex items-center justify-center bg-black/50">
      <div className="flex max-h-[80vh] w-full max-w-2xl flex-col rounded-xl bg-white shadow-2xl dark:bg-brand-800">
        <div className="flex items-center justify-between border-b border-brand-200 p-4 dark:border-brand-700">
          <h3 className="text-lg font-bold text-brand-900 dark:text-white">
            {title}
            :
            {" "}
            {candidateName}
          </h3>
          <button
            type="button"
            onClick={onClose}
            className="i-mdi-close text-xl text-brand-500 hover:text-brand-700"
          />
        </div>

        <div className="flex-1 space-y-4 overflow-y-auto p-4">
          {isSearching
            ? (
                <ImportTaskLoadingStep
                  iconClassName={theme.loadingSpinnerClassName}
                  title={labels.searching}
                />
              )
            : (
                <>
                  <div className="flex flex-wrap gap-3">
                    {matches.filter(match => match.Game).map(match => (
                      <div
                        key={`${match.Source}-${match.Game?.source_id || match.Game?.name}`}
                        onClick={() => onSelectMatch(match)}
                        className={`w-36 cursor-pointer rounded-lg border border-brand-200 p-2 transition hover:shadow-md dark:border-brand-700 ${theme.cardHoverClassName}`}
                      >
                        <div className="aspect-[3/4] w-full overflow-hidden rounded-md bg-brand-200 dark:bg-brand-700">
                          {match.Game?.cover_url
                            ? (
                                <img
                                  src={match.Game.cover_url}
                                  alt={match.Game.name}
                                  className="h-full w-full object-cover"
                                  referrerPolicy="no-referrer"
                                  draggable="false"
                                  onDragStart={e => e.preventDefault()}
                                />
                              )
                            : (
                                <div className="flex h-full items-center justify-center text-brand-400">
                                  <div className="i-mdi-image-off text-3xl" />
                                </div>
                              )}
                        </div>
                        <h4
                          className="mt-1 truncate text-xs font-bold text-brand-900 dark:text-white"
                          title={match.Game?.name}
                        >
                          {match.Game?.name}
                        </h4>
                        <p className="text-xs text-brand-400">{match.Source}</p>
                      </div>
                    ))}
                  </div>

                  {matches.length === 0 && (
                    <p className="py-4 text-center text-brand-400">
                      {labels.noMatchResult}
                    </p>
                  )}

                  <div className="mt-4 border-t border-brand-200 pt-4 dark:border-brand-700">
                    <p className="mb-3 text-sm text-brand-500">
                      {labels.searchById}
                    </p>
                    <div className="flex gap-2">
                      <BetterSelect
                        value={manualSource}
                        onChange={value => onSourceChange(value as enums.SourceType)}
                        options={sourceOptions}
                        className="w-32"
                      />
                      <input
                        type="text"
                        value={manualId}
                        onChange={e => onManualIdChange(e.target.value)}
                        placeholder={idPlaceholder}
                        className="flex-1 rounded border border-brand-300 bg-brand-50 px-3 py-1.5 text-sm dark:border-brand-600 dark:bg-brand-700"
                      />
                      <button
                        type="button"
                        onClick={onSearchById}
                        disabled={searchDisabled}
                        className={`rounded px-4 py-1.5 text-sm text-white disabled:opacity-50 ${theme.searchButtonClassName}`}
                      >
                        {labels.search}
                      </button>
                    </div>
                  </div>

                  <button
                    type="button"
                    onClick={onSkipMetadata}
                    className="w-full py-2 text-center text-sm text-brand-400 hover:text-brand-600"
                  >
                    {labels.skipMetadata}
                  </button>
                </>
              )}
        </div>
      </div>
    </div>
  );
}
