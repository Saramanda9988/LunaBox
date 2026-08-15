import { useTranslation } from "react-i18next";
import { CATEGORY_NAME_MAX_LENGTH } from "../../consts/category";
import { ModalPortal } from "../ui/ModalPortal";

interface CategoryModalProps {
  isOpen: boolean;
  value: string;
  onChange: (value: string) => void;
  onClose: () => void;
  onSubmit: () => void;
  mode?: "add" | "edit";
}

export function CategoryModal({
  isOpen,
  value,
  onChange,
  onClose,
  onSubmit,
  mode = "add",
}: CategoryModalProps) {
  const { t } = useTranslation();

  if (!isOpen)
    return null;

  const title
    = mode === "add"
      ? t("categories.modal.addTitle")
      : t("categories.modal.editTitle");
  const submitText
    = mode === "add" ? t("categories.modal.create") : t("common.save");
  const characterCountText = `${value.length}/${CATEGORY_NAME_MAX_LENGTH}`;

  return (
    <ModalPortal>
      <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
        <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl dark:bg-brand-800">
          <h3 className="text-xl font-bold text-brand-900 dark:text-white mb-4">
            {title}
          </h3>
          <div className="relative mb-4">
            <input
              type="text"
              value={value}
              onChange={e => onChange(e.target.value)}
              maxLength={CATEGORY_NAME_MAX_LENGTH}
              placeholder={t("categories.modal.namePlaceholder")}
              className="w-full rounded-lg border border-brand-300 p-2 pr-16 focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
              autoFocus
            />
            <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs tabular-nums text-brand-400 dark:text-brand-500">
              {characterCountText}
            </span>
          </div>
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-brand-700 hover:bg-brand-100 rounded-lg dark:text-brand-300 dark:hover:bg-brand-700"
            >
              {t("common.cancel")}
            </button>
            <button
              type="button"
              onClick={onSubmit}
              disabled={!value.trim()}
              className="px-4 py-2 bg-neutral-600 text-white rounded-lg hover:bg-neutral-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {submitText}
            </button>
          </div>
        </div>
      </div>
    </ModalPortal>
  );
}
