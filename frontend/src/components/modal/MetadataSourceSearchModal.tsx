import type { vo } from "../../../src/bindings/models";
import { useTranslation } from "react-i18next";
import { MetadataSearchResultsStep } from "../ui/import/MetadataSearchResultsStep";
import { ModalPortal } from "../ui/ModalPortal";

interface MetadataSourceSearchModalProps {
  isOpen: boolean;
  results: vo.GameMetadataFromWebVO[];
  isApplying: boolean;
  onClose: () => void;
  onSelect: (result: vo.GameMetadataFromWebVO) => void;
  onRemove: (index: number) => void;
}

export function MetadataSourceSearchModal({
  isOpen,
  results,
  isApplying,
  onClose,
  onSelect,
  onRemove,
}: MetadataSourceSearchModalProps) {
  const { t } = useTranslation();

  if (!isOpen)
    return null;

  return (
    <ModalPortal>
      <div className="absolute inset-0 z-[10000] flex items-center justify-center bg-black/50 p-6 backdrop-blur-sm">
        <div
          className="w-full max-w-2xl rounded-xl bg-white p-6 shadow-2xl dark:bg-brand-800"
          role="dialog"
          aria-modal="true"
          aria-labelledby="metadata-source-search-title"
        >
          <div className="mb-2 flex items-center justify-between gap-4">
            <h2
              id="metadata-source-search-title"
              className="text-4xl font-bold text-brand-900 dark:text-white"
            >
              {t("gameEdit.metadataSearchTitle")}
            </h2>
            <button
              type="button"
              disabled={isApplying}
              onClick={onClose}
              className="i-mdi-close rounded-lg p-1 text-2xl text-brand-500 hover:bg-brand-100 hover:text-brand-700 focus:outline-none disabled:cursor-wait disabled:opacity-60 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-200"
              aria-label={t("common.cancel")}
            />
          </div>

          <MetadataSearchResultsStep
            results={results}
            disabled={isApplying}
            onSelect={onSelect}
            onRemove={onRemove}
            empty={(
              <div className="flex min-h-64 flex-col items-center justify-center gap-3 text-brand-500 dark:text-brand-400">
                <span
                  className="i-mdi-database-search-outline text-4xl"
                  aria-hidden="true"
                />
                <span className="text-sm">
                  {t("gameEdit.metadataSearchEmpty")}
                </span>
              </div>
            )}
            footer={(
              <div className="flex items-center justify-between border-t border-brand-200 pt-4 dark:border-brand-700">
                <button
                  type="button"
                  disabled={isApplying}
                  onClick={onClose}
                  className="text-sm text-brand-500 hover:text-brand-700 disabled:cursor-wait disabled:opacity-60 dark:text-brand-400 dark:hover:text-brand-200"
                >
                  &larr;
                  {t("addGameModal.goBack")}
                </button>
              </div>
            )}
          />
        </div>
      </div>
    </ModalPortal>
  );
}
