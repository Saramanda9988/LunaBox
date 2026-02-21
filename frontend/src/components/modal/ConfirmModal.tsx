import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";

interface ConfirmModalProps {
  isOpen: boolean;
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  type?: "danger" | "info";
  onClose: () => void;
  onConfirm: () => void;
}

export function ConfirmModal({
  isOpen,
  title,
  message,
  confirmText,
  cancelText,
  type = "info",
  onClose,
  onConfirm,
}: ConfirmModalProps) {
  const { t } = useTranslation();

  if (!isOpen)
    return null;

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl dark:bg-brand-800 border border-brand-200 dark:border-brand-700">
        <div className="flex items-start gap-4">
          <div className={`p-2 rounded-full ${type === "danger"
            ? "bg-error-100 text-error-600 dark:bg-error-900/30 dark:text-error-400"
            : "bg-neutral-100 text-neutral-600 dark:bg-neutral-900/30 dark:text-neutral-400"
          }`}
          >
            <div className={type === "danger" ? "i-mdi-alert-circle text-2xl" : "i-mdi-information text-2xl"} />
          </div>
          <div className="flex-1">
            <h3 className="text-xl font-bold text-brand-900 dark:text-white mb-2">{title}</h3>
            <p className="text-brand-600 dark:text-brand-400 text-sm leading-relaxed">
              {message}
            </p>
          </div>
        </div>

        <div className="flex justify-end gap-3 mt-8">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm font-medium text-brand-700 hover:bg-brand-100 rounded-lg dark:text-brand-300 dark:hover:bg-brand-700 transition-colors"
          >
            {cancelText || t("common.cancel")}
          </button>
          <button
            onClick={() => {
              onConfirm();
              onClose();
            }}
            className={`px-4 py-2 text-sm font-medium text-white rounded-lg transition-colors ${type === "danger"
              ? "bg-error-600 hover:bg-error-700 shadow-sm shadow-error-200 dark:shadow-none"
              : "bg-neutral-600 hover:bg-neutral-700 shadow-sm shadow-neutral-200 dark:shadow-none"
            }`}
          >
            {confirmText || t("common.confirm")}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  );
}
