import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { enums } from "../../../wailsjs/go/models";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterSwitch } from "../ui/better/BetterSwitch";
import { ModalPortal } from "../ui/ModalPortal";

export const DEFAULT_METADATA_UPDATE_FIELDS: enums.MetadataUpdateField[] = [
  enums.MetadataUpdateField.NAME,
  enums.MetadataUpdateField.COVER,
  enums.MetadataUpdateField.COMPANY,
  enums.MetadataUpdateField.SUMMARY,
  enums.MetadataUpdateField.RATING,
  enums.MetadataUpdateField.RELEASE_DATE,
  enums.MetadataUpdateField.TAGS,
];

interface MetadataFieldSelectModalProps {
  isOpen: boolean;
  title: string;
  description: string;
  confirmText: string;
  initialFields?: enums.MetadataUpdateField[];
  isSubmitting?: boolean;
  onClose: () => void;
  onConfirm: (fields: enums.MetadataUpdateField[]) => void;
}

export function MetadataFieldSelectModal({
  isOpen,
  title,
  description,
  confirmText,
  initialFields,
  isSubmitting = false,
  onClose,
  onConfirm,
}: MetadataFieldSelectModalProps) {
  const { t } = useTranslation();
  const initialSelectedFields
    = initialFields && initialFields.length > 0
      ? initialFields
      : DEFAULT_METADATA_UPDATE_FIELDS;
  const [selectedFields, setSelectedFields] = useState<
    enums.MetadataUpdateField[]
  >(initialSelectedFields);
  const selectedFieldSet = useMemo(
    () => new Set(selectedFields),
    [selectedFields],
  );
  const isAllSelected
    = selectedFields.length === DEFAULT_METADATA_UPDATE_FIELDS.length;
  const canConfirm = selectedFields.length > 0 && !isSubmitting;

  useEffect(() => {
    if (!isOpen)
      return;
    setSelectedFields(initialSelectedFields);
  }, [initialSelectedFields, isOpen]);

  if (!isOpen)
    return null;

  const fieldItems = DEFAULT_METADATA_UPDATE_FIELDS.map(field => ({
    value: field,
    label: t(`metadataUpdateFields.${field}.label`),
    hint: t(`metadataUpdateFields.${field}.hint`),
  }));

  const handleToggleField = (
    field: enums.MetadataUpdateField,
    checked: boolean,
  ) => {
    setSelectedFields((current) => {
      if (checked) {
        if (current.includes(field))
          return current;
        return [...current, field];
      }
      return current.filter(item => item !== field);
    });
  };

  const handleToggleAll = (checked: boolean) => {
    setSelectedFields(checked ? [...DEFAULT_METADATA_UPDATE_FIELDS] : []);
  };

  const handleConfirm = () => {
    if (!canConfirm)
      return;
    onConfirm(selectedFields);
  };

  return (
    <ModalPortal>
      <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
        <div className="w-full max-w-2xl rounded-xl border border-brand-200 bg-white p-6 shadow-xl dark:border-brand-700 dark:bg-brand-800">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0 space-y-2">
              <h3 className="text-xl font-bold text-brand-900 dark:text-white">
                {title}
              </h3>
              <p className="text-sm leading-relaxed text-brand-600 dark:text-brand-400">
                {description}
              </p>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg p-2 text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-800 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-100"
              aria-label={t("common.close", "关闭")}
            >
              <div className="i-mdi-close text-xl" />
            </button>
          </div>

          <div className="mt-5 flex items-center justify-between gap-4 rounded-lg border border-brand-200 bg-brand-50 p-3 dark:border-brand-700 dark:bg-brand-700/40">
            <div className="space-y-1">
              <div className="text-sm font-semibold text-brand-800 dark:text-brand-200">
                {t("metadataUpdateFields.selectAll")}
              </div>
              <div className="text-xs text-brand-500 dark:text-brand-400">
                {t("metadataUpdateFields.selectedCount", {
                  count: selectedFields.length,
                  total: DEFAULT_METADATA_UPDATE_FIELDS.length,
                })}
              </div>
            </div>
            <BetterSwitch
              id="metadata-update-fields-all"
              checked={isAllSelected}
              onCheckedChange={handleToggleAll}
            />
          </div>

          <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
            {fieldItems.map(item => (
              <div
                key={item.value}
                className="flex min-w-0 items-center justify-between gap-3 rounded-lg border border-brand-200 p-3 dark:border-brand-700"
              >
                <label
                  htmlFor={`metadata-update-field-${item.value}`}
                  className="min-w-0 flex-1 cursor-pointer space-y-1"
                >
                  <span className="block text-sm font-medium text-brand-800 dark:text-brand-200">
                    {item.label}
                  </span>
                  <span className="block text-xs leading-relaxed text-brand-500 dark:text-brand-400">
                    {item.hint}
                  </span>
                </label>
                <BetterSwitch
                  id={`metadata-update-field-${item.value}`}
                  checked={selectedFieldSet.has(item.value)}
                  onCheckedChange={checked =>
                    handleToggleField(item.value, checked)}
                />
              </div>
            ))}
          </div>

          {selectedFields.length === 0 && (
            <p className="mt-3 text-xs text-error-600 dark:text-error-400">
              {t("metadataUpdateFields.emptyWarning")}
            </p>
          )}

          <div className="mt-8 flex justify-end gap-3">
            <BetterButton variant="secondary" onClick={onClose}>
              {t("common.cancel")}
            </BetterButton>
            <BetterButton
              variant="primary"
              icon="i-mdi-cloud-sync"
              disabled={!canConfirm}
              isLoading={isSubmitting}
              onClick={handleConfirm}
            >
              {confirmText}
            </BetterButton>
          </div>
        </div>
      </div>
    </ModalPortal>
  );
}
