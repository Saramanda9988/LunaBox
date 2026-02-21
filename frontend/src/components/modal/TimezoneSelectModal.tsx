import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { BetterSelect } from "../ui/BetterSelect";

interface TimezoneSelectModalProps {
  isOpen: boolean;
  onConfirm: (timezone: string) => void;
}

const COMMON_TIMEZONES = [
  { value: "Asia/Shanghai", label: "China Standard Time (UTC+8)" },
  { value: "Asia/Tokyo", label: "Japan Standard Time (UTC+9)" },
  { value: "Asia/Seoul", label: "Korea Standard Time (UTC+9)" },
  { value: "Asia/Singapore", label: "Singapore Time (UTC+8)" },
  { value: "Asia/Bangkok", label: "Bangkok Time (UTC+7)" },
  { value: "Asia/Dubai", label: "Dubai Time (UTC+4)" },
  { value: "Europe/London", label: "London Time (UTC+0)" },
  { value: "Europe/Paris", label: "Paris Time (UTC+1)" },
  { value: "Europe/Berlin", label: "Berlin Time (UTC+1)" },
  { value: "Europe/Moscow", label: "Moscow Time (UTC+3)" },
  { value: "America/New_York", label: "New York Time (UTC-5)" },
  { value: "America/Chicago", label: "Chicago Time (UTC-6)" },
  { value: "America/Denver", label: "Denver Time (UTC-7)" },
  { value: "America/Los_Angeles", label: "Los Angeles Time (UTC-8)" },
  { value: "America/Sao_Paulo", label: "São Paulo Time (UTC-3)" },
  { value: "Australia/Sydney", label: "Sydney Time (UTC+10)" },
  { value: "Pacific/Auckland", label: "Auckland Time (UTC+12)" },
  { value: "UTC", label: "Coordinated Universal Time (UTC)" },
];

export function TimezoneSelectModal({ isOpen, onConfirm }: TimezoneSelectModalProps) {
  const { t } = useTranslation();
  const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const [selectedTimezone, setSelectedTimezone] = useState(browserTimezone || "Asia/Shanghai");

  // Keep timezone list labels consistent with current language
  useEffect(() => {
    setSelectedTimezone(browserTimezone || "Asia/Shanghai");
  }, [browserTimezone]);

  if (!isOpen)
    return null;

  const handleConfirm = () => {
    onConfirm(selectedTimezone);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className="w-full max-w-lg rounded-xl bg-white p-6 shadow-xl dark:bg-brand-800 border border-brand-200 dark:border-brand-700">
        <div className="flex items-start gap-4 mb-6">
          <div className="p-2 rounded-full bg-primary-100 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400">
            <div className="i-mdi-earth text-2xl" />
          </div>
          <div className="flex-1">
            <h3 className="text-xl font-bold text-brand-900 dark:text-white mb-2">{t("timezoneModal.title")}</h3>
            <p className="text-brand-600 dark:text-brand-400 text-sm leading-relaxed">
              {t("timezoneModal.desc")}
            </p>
          </div>
        </div>

        <div className="mb-6">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-2">
            {t("timezoneModal.detectedLabel")}
            {" "}
            <span className="font-semibold text-primary-600 dark:text-primary-400">{browserTimezone || t("timezoneModal.unknown")}</span>
          </label>
          <BetterSelect
            value={selectedTimezone}
            onChange={setSelectedTimezone}
            options={COMMON_TIMEZONES}
            placeholder={t("timezoneModal.placeholder")}
            className="w-full"
          />
        </div>

        <div className="flex justify-end gap-3">
          <button
            type="button"
            onClick={handleConfirm}
            className="px-6 py-2.5 text-sm font-medium text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors shadow-sm shadow-primary-200 dark:shadow-none"
          >
            {t("timezoneModal.confirmBtn")}
          </button>
        </div>
      </div>
    </div>
  );
}
