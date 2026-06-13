import { useTranslation } from "react-i18next";
import { enums } from "../../../wailsjs/go/models";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterDateRangePicker } from "../ui/better/BetterDateRangePicker";
import { SlideButton } from "../ui/SlideButton";

interface StatsToolbarProps {
  period: enums.Period;
  customRangeActive: boolean;
  startDate: string;
  endDate: string;
  loading?: boolean;
  aiLoading?: boolean;
  onPeriodChange: (period: enums.Period) => void;
  onStartDateChange: (value: string) => void;
  onEndDateChange: (value: string) => void;
  onApplyDateRange: () => void;
  onResetDateRange: () => void;
  onExportReport: () => void;
  onAISummarize: () => void;
}

export function StatsToolbar({
  period,
  customRangeActive,
  startDate,
  endDate,
  loading = false,
  aiLoading = false,
  onPeriodChange,
  onStartDateChange,
  onEndDateChange,
  onApplyDateRange,
  onResetDateRange,
  onExportReport,
  onAISummarize,
}: StatsToolbarProps) {
  const { t } = useTranslation();

  return (
    <div className="no-export my-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
      <div className="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center">
        <SlideButton
          options={[
            { label: t("stats.period.week"), value: enums.Period.WEEK },
            { label: t("stats.period.month"), value: enums.Period.MONTH },
            { label: t("stats.period.year"), value: enums.Period.YEAR },
          ]}
          value={customRangeActive ? "" : period}
          onChange={(value) => {
            if (value) {
              onPeriodChange(value);
            }
          }}
          disabled={loading}
          className="shrink-0"
        />

        <BetterDateRangePicker
          startDate={startDate}
          endDate={endDate}
          triggerLabel={t("stats.customRange")}
          applyLabel={t("stats.applyBtn")}
          resetLabel={t("stats.resetBtn")}
          active={customRangeActive}
          disabled={loading}
          onStartDateChange={onStartDateChange}
          onEndDateChange={onEndDateChange}
          onApply={onApplyDateRange}
          onReset={onResetDateRange}
        />
      </div>

      <div className="flex shrink-0 items-center gap-2">
        <BetterButton
          size="md"
          variant="secondary"
          icon="i-mdi-image-filter-hdr"
          disabled={loading}
          onClick={onExportReport}
          title={t("stats.exportTitle")}
          className="min-w-[9.25rem]"
        >
          {t("stats.exportTitle")}
        </BetterButton>
        <BetterButton
          size="md"
          variant="primary"
          icon="i-mdi-robot-happy"
          disabled={loading}
          isLoading={aiLoading}
          onClick={onAISummarize}
          title={t("stats.aiSummarizeTitle")}
          className="min-w-[9.25rem]"
        >
          {t("stats.aiSummarizeTitle")}
        </BetterButton>
      </div>
    </div>
  );
}
