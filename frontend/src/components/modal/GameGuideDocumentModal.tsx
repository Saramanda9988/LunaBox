import type { GameGuideDocument } from "../../../bindings/lunabox/internal/common/vo/models";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ModalPortal } from "../ui/ModalPortal";

interface GameGuideDocumentModalProps {
  documents: GameGuideDocument[];
  isOpen: boolean;
  onClose: () => void;
  onOpen: (document: GameGuideDocument) => Promise<void>;
}

export function GameGuideDocumentModal({
  documents,
  isOpen,
  onClose,
  onOpen,
}: GameGuideDocumentModalProps) {
  const { t } = useTranslation();
  const [openingPath, setOpeningPath] = useState("");

  if (!isOpen)
    return null;

  const handleOpen = async (document: GameGuideDocument) => {
    setOpeningPath(document.relative_path);
    try {
      await onOpen(document);
      onClose();
    }
    finally {
      setOpeningPath("");
    }
  };

  return (
    <ModalPortal>
      <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
        <section
          aria-labelledby="game-guide-documents-title"
          aria-modal="true"
          role="dialog"
          className="w-full max-w-lg rounded-xl border border-brand-200 bg-white p-6 shadow-xl dark:border-brand-700 dark:bg-brand-800"
        >
          <div className="mb-5 flex items-start gap-4">
            <div className="min-w-0 flex-1">
              <h3
                id="game-guide-documents-title"
                className="mb-1 text-xl font-bold text-brand-900 dark:text-white"
              >
                {t("gameEdit.guideDocumentsTitle")}
              </h3>
              <p className="text-sm leading-relaxed text-brand-600 dark:text-brand-400">
                {t("gameEdit.guideDocumentsDescription", {
                  count: documents.length,
                })}
              </p>
            </div>
          </div>

          <div className="max-h-80 overflow-y-auto rounded-lg border border-brand-200 bg-brand-50 dark:border-brand-600 dark:bg-brand-900/40">
            <div className="divide-y divide-brand-200 dark:divide-brand-700">
              {documents.map((document) => {
                const isOpening = openingPath === document.relative_path;
                return (
                  <button
                    key={document.relative_path}
                    type="button"
                    disabled={Boolean(openingPath)}
                    onClick={() => void handleOpen(document)}
                    className="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-brand-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 disabled:cursor-wait disabled:opacity-60 dark:hover:bg-brand-800"
                  >
                    <span
                      className="i-mdi-file-document-outline shrink-0 text-xl text-primary-600 dark:text-primary-400"
                      aria-hidden="true"
                    />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium text-brand-800 dark:text-brand-100">
                        {document.name}
                      </span>
                      <span className="mt-0.5 block truncate text-xs text-brand-500 dark:text-brand-400">
                        {document.relative_path}
                      </span>
                    </span>
                    {isOpening ? (
                      <span
                        className="i-mdi-loading shrink-0 animate-spin text-lg text-primary-600 dark:text-primary-400"
                        aria-label={t("gameEdit.openingGuideDocument")}
                      />
                    ) : (
                      <span
                        className="i-mdi-open-in-new shrink-0 text-lg text-brand-400"
                        aria-hidden="true"
                      />
                    )}
                  </button>
                );
              })}
            </div>
          </div>

          <div className="mt-6 flex justify-end">
            <button
              type="button"
              disabled={Boolean(openingPath)}
              onClick={onClose}
              className="rounded-lg px-4 py-2 text-sm font-medium text-brand-700 transition-colors hover:bg-brand-100 disabled:opacity-50 dark:text-brand-300 dark:hover:bg-brand-700"
            >
              {t("common.cancel")}
            </button>
          </div>
        </section>
      </div>
    </ModalPortal>
  );
}
