import type { FormEvent } from "react";
import { useMemo, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { AddPlaySession } from "../../../wailsjs/go/service/SessionService";
import {
  formatDateToYYYYMMDD,
  formatDuration,
  formatLocalDateTime,
  toLocalISOString,
} from "../../utils/time";
import { BetterDateRangePicker } from "../ui/better/BetterDateRangePicker";
import {
  BetterDurationWheelPicker,
  BetterTimeWheelPicker,
} from "../ui/better/BetterWheelPicker";
import { ModalPortal } from "../ui/ModalPortal";

interface AddPlaySessionModalProps {
  isOpen: boolean;
  gameId: string;
  onClose: () => void;
  onSuccess: () => void;
}

function getClockMinutes(date: Date) {
  return date.getHours() * 60 + date.getMinutes();
}

function formatClockValueFromMinutes(value: number) {
  const hours = Math.floor(value / 60);
  const minutes = value % 60;
  return `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}`;
}

function createDefaultSessionDraft() {
  const now = new Date();
  const oneHourAgo = new Date(now);
  oneHourAgo.setHours(now.getHours() - 1);

  return {
    date: formatDateToYYYYMMDD(oneHourAgo),
    durationMinutes: 60,
    startMinutes: getClockMinutes(oneHourAgo),
  };
}

function parseLocalDateTime(dateValue: string, clockValue: string) {
  const dateMatch = /^(\d{4})-(\d{2})-(\d{2})$/.exec(dateValue);
  const clockMatch = /^(\d{2}):(\d{2})$/.exec(clockValue);

  if (!dateMatch || !clockMatch) {
    return null;
  }

  const year = Number(dateMatch[1]);
  const month = Number(dateMatch[2]);
  const day = Number(dateMatch[3]);
  const hours = Number(clockMatch[1]);
  const minutes = Number(clockMatch[2]);
  const date = new Date(year, month - 1, day, hours, minutes, 0, 0);

  if (
    date.getFullYear() !== year
    || date.getMonth() !== month - 1
    || date.getDate() !== day
    || date.getHours() !== hours
    || date.getMinutes() !== minutes
  ) {
    return null;
  }

  return date;
}

export function AddPlaySessionModal({
  isOpen,
  gameId,
  onClose,
  onSuccess,
}: AddPlaySessionModalProps) {
  const { t } = useTranslation();
  const [selectedDate, setSelectedDate] = useState(
    () => createDefaultSessionDraft().date,
  );
  const [startMinutes, setStartMinutes] = useState(
    () => createDefaultSessionDraft().startMinutes,
  );
  const [durationMinutes, setDurationMinutes] = useState(
    () => createDefaultSessionDraft().durationMinutes,
  );
  const [isSubmitting, setIsSubmitting] = useState(false);

  const today = new Date();
  const todayValue = formatDateToYYYYMMDD(today);
  const currentClockMinutes = getClockMinutes(today);
  const startDateTime = useMemo(
    () =>
      parseLocalDateTime(
        selectedDate,
        formatClockValueFromMinutes(startMinutes),
      ),
    [selectedDate, startMinutes],
  );
  const endDateTime = useMemo(() => {
    if (!startDateTime) {
      return null;
    }

    return new Date(startDateTime.getTime() + durationMinutes * 60 * 1000);
  }, [durationMinutes, startDateTime]);
  const totalSeconds = durationMinutes * 60;

  const resetDraft = () => {
    const draft = createDefaultSessionDraft();
    setSelectedDate(draft.date);
    setStartMinutes(draft.startMinutes);
    setDurationMinutes(draft.durationMinutes);
  };

  const handleDateChange = (value: string) => {
    setSelectedDate(value);

    if (value === todayValue && startMinutes > currentClockMinutes) {
      setStartMinutes(currentClockMinutes);
    }
  };

  if (!isOpen)
    return null;

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();

    if (!startDateTime || !endDateTime) {
      toast.error(t("addPlaySession.toast.invalidStartTime"));
      return;
    }

    if (totalSeconds <= 0) {
      toast.error(t("addPlaySession.toast.zeroDuration"));
      return;
    }

    if (endDateTime > new Date()) {
      toast.error(t("addPlaySession.toast.futureEnd"));
      return;
    }

    setIsSubmitting(true);
    try {
      await AddPlaySession(
        gameId,
        toLocalISOString(startDateTime),
        durationMinutes,
      );
      toast.success(t("addPlaySession.toast.success"));
      onSuccess();
      onClose();
      resetDraft();
    }
    catch (error) {
      console.error("Failed to add play session:", error);
      toast.error(t("addPlaySession.toast.failed"));
    }
    finally {
      setIsSubmitting(false);
    }
  };

  return (
    <ModalPortal>
      <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
        <div className="relative mx-4 flex max-h-[86vh] w-full max-w-lg flex-col overflow-hidden rounded-lg bg-white shadow-xl dark:bg-brand-800">
          <div className="flex shrink-0 items-center justify-between px-6 pb-3 pt-6">
            <h2 className="text-xl font-semibold text-brand-900 dark:text-white">
              {t("addPlaySession.title")}
            </h2>
            <button
              type="button"
              onClick={onClose}
              className="text-brand-500 hover:text-brand-700 dark:text-brand-400 dark:hover:text-white transition-colors"
            >
              <div className="i-mdi-close text-xl" />
            </button>
          </div>

          <div className="shrink-0 px-6 pb-4 text-sm text-brand-600 dark:text-brand-400">
            {t("addPlaySession.desc")}
          </div>

          <form
            onSubmit={handleSubmit}
            className="flex-1 space-y-4 overflow-y-auto px-6 pb-6"
          >
            <div>
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
                {t("addPlaySession.date")}
              </label>
              <BetterDateRangePicker
                startDate={selectedDate}
                endDate={selectedDate}
                triggerLabel={t("addPlaySession.selectDate")}
                applyLabel={t("common.confirm")}
                resetLabel={t("stats.resetBtn")}
                selectionMode="single"
                active={Boolean(selectedDate)}
                disabled={isSubmitting}
                className="w-full"
                triggerClassName="w-full"
                onStartDateChange={handleDateChange}
                onEndDateChange={handleDateChange}
                onApply={() => {}}
                onReset={() => handleDateChange(todayValue)}
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
                {t("addPlaySession.startTime")}
              </label>
              <BetterTimeWheelPicker
                valueMinutes={startMinutes}
                onChange={setStartMinutes}
                disabled={isSubmitting}
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-2">
                {t("addPlaySession.duration")}
              </label>
              <BetterDurationWheelPicker
                valueMinutes={durationMinutes}
                onChange={setDurationMinutes}
                hourLabel={t("common.duration.hoursShort")}
                minuteLabel={t("common.duration.minutesShort")}
                disabled={isSubmitting}
              />
            </div>

            <div className="rounded-lg bg-brand-50 p-3 dark:bg-brand-700/50">
              <div className="text-xs text-brand-500 dark:text-brand-400">
                {t("addPlaySession.endPreview")}
              </div>
              <div className="mt-1 flex flex-wrap items-baseline justify-between gap-2">
                <div className="text-sm font-medium text-brand-700 dark:text-brand-200">
                  {endDateTime ? formatLocalDateTime(endDateTime) : "--"}
                </div>
                <div className="text-lg font-semibold text-brand-900 dark:text-white">
                  {formatDuration(totalSeconds, t)}
                </div>
              </div>
            </div>

            <div className="flex justify-end gap-3 pt-4">
              <button
                type="button"
                onClick={onClose}
                className="px-4 py-2 text-brand-600 dark:text-brand-400 hover:bg-brand-100 dark:hover:bg-brand-700 rounded-md transition-colors"
              >
                {t("common.cancel")}
              </button>
              <button
                type="submit"
                disabled={isSubmitting}
                className="px-4 py-2 bg-neutral-600 text-white rounded-md hover:bg-neutral-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {isSubmitting
                  ? t("addPlaySession.submitting")
                  : t("common.add")}
              </button>
            </div>
          </form>
        </div>
      </div>
    </ModalPortal>
  );
}
