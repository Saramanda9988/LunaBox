import type { service } from "../../../wailsjs/go/models";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ListGameProgresses } from "../../../wailsjs/go/service/GameProgressService";
import { useAppStore } from "../../store";
import { formatLocalDateTime } from "../../utils/time";
import { AddGameProgressModal } from "../modal/AddGameProgressModal";
import { GameProgressSkeleton } from "../skeleton/GameProgressSkeleton";
import { BetterButton } from "../ui/better/BetterButton";

interface GameProgressPanelProps {
  gameId: string;
}

const SPOILER_OPTIONS = [
  { value: "none", labelKey: "gameProgress.spoilerBoundaryOpts.none" },
  {
    value: "chapter_end",
    labelKey: "gameProgress.spoilerBoundaryOpts.chapterEnd",
  },
  { value: "route_end", labelKey: "gameProgress.spoilerBoundaryOpts.routeEnd" },
  { value: "full", labelKey: "gameProgress.spoilerBoundaryOpts.full" },
];

export function GameProgressPanel({ gameId }: GameProgressPanelProps) {
  const { t } = useTranslation();
  const config = useAppStore(state => state.config);

  const [isLoading, setIsLoading] = useState(true);
  const [showLoadingSkeleton, setShowLoadingSkeleton] = useState(false);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [progressHistory, setProgressHistory] = useState<
    service.GameProgress[]
  >([]);

  const loadHistory = async () => {
    setIsLoading(true);
    try {
      const history = await ListGameProgresses(gameId);
      setProgressHistory(history || []);
    }
    catch (e) {
      console.error("Failed to load game progress history:", e);
    }
    finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadHistory();
  }, [gameId]);

  useEffect(() => {
    let timer: number | undefined;

    if (isLoading && progressHistory.length === 0) {
      timer = window.setTimeout(() => {
        setShowLoadingSkeleton(true);
      }, 300);
    }
    else {
      setShowLoadingSkeleton(false);
    }

    return () => {
      if (timer !== undefined) {
        window.clearTimeout(timer);
      }
    };
  }, [isLoading, progressHistory.length]);

  const spoilerOptions = SPOILER_OPTIONS.map(o => ({
    value: o.value,
    label: t(o.labelKey),
  }));
  const spoilerLabelMap = Object.fromEntries(
    spoilerOptions.map(option => [option.value, option.label]),
  ) as Record<string, string>;
  const latestRecord = progressHistory[0];

  return (
    <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm min-h-[22rem]">
      <div className="flex h-full min-h-0 flex-col">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <h3 className="text-lg font-semibold text-brand-900 dark:text-white">
              {t("gameProgress.title")}
            </h3>
            <p className="text-sm text-brand-500 dark:text-brand-400">
              {latestRecord
                ? t("gameProgress.latestRecordedAt", {
                    time: formatLocalDateTime(
                      latestRecord.updated_at,
                      config?.time_zone,
                    ),
                  })
                : t("gameProgress.hint")}
            </p>
          </div>
          <BetterButton
            onClick={() => setIsModalOpen(true)}
            icon="i-mdi-playlist-plus"
            variant="primary"
            className="w-full sm:w-auto"
          >
            {t("gameProgress.addRecord")}
          </BetterButton>
        </div>

        <div className="mt-4 flex-1 min-h-[14rem]">
          {progressHistory.length > 0 ? (
            <div className="space-y-3">
              {progressHistory.map(progress => (
                <article
                  key={progress.id}
                  className="data-glass:bg-white/1 data-glass:dark:bg-black/1 rounded-lg bg-brand-50 p-4 dark:bg-brand-700"
                >
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div className="flex min-w-0 items-center gap-2 text-sm font-medium text-brand-900 dark:text-white">
                      <span className="i-mdi-circle-medium shrink-0 text-brand-500 dark:text-brand-400" />
                      <span className="min-w-0 break-words">
                        {formatLocalDateTime(
                          progress.updated_at,
                          config?.time_zone,
                        )}
                      </span>
                    </div>
                    <span className="w-fit rounded-full bg-brand-200 px-2.5 py-1 text-xs font-medium text-brand-700 dark:bg-brand-600 dark:text-brand-100">
                      {spoilerLabelMap[progress.spoiler_boundary]
                        || progress.spoiler_boundary}
                    </span>
                  </div>

                  <div className="mt-3 grid gap-3 md:grid-cols-2">
                    <div className="space-y-1 min-w-0">
                      <p className="text-xs font-medium uppercase tracking-wide text-brand-500 dark:text-brand-400">
                        {t("gameProgress.chapter")}
                      </p>
                      <p className="break-words text-sm text-brand-900 dark:text-white">
                        {progress.chapter || "-"}
                      </p>
                    </div>
                    <div className="space-y-1 min-w-0">
                      <p className="text-xs font-medium uppercase tracking-wide text-brand-500 dark:text-brand-400">
                        {t("gameProgress.route")}
                      </p>
                      <p className="break-words text-sm text-brand-900 dark:text-white">
                        {progress.route || "-"}
                      </p>
                    </div>
                  </div>

                  <div className="mt-3 space-y-1 min-w-0">
                    <p className="text-xs font-medium uppercase tracking-wide text-brand-500 dark:text-brand-400">
                      {t("gameProgress.progressNote")}
                    </p>
                    <p className="whitespace-pre-wrap break-words text-sm leading-relaxed text-brand-700 dark:text-brand-200">
                      {progress.progress_note || t("gameProgress.noNote")}
                    </p>
                  </div>
                </article>
              ))}
            </div>
          ) : isLoading ? (
            showLoadingSkeleton ? (
              <GameProgressSkeleton />
            ) : (
              <div className="min-h-[14rem]" />
            )
          ) : (
            <div className="flex h-full min-h-[14rem] items-center justify-center rounded-lg border border-dashed border-brand-300 px-4 py-6 text-center text-sm text-brand-500 dark:border-brand-600 dark:text-brand-400">
              {t("gameProgress.historyEmpty")}
            </div>
          )}
        </div>
      </div>

      <AddGameProgressModal
        isOpen={isModalOpen}
        gameId={gameId}
        onClose={() => setIsModalOpen(false)}
        onSuccess={loadHistory}
      />
    </div>
  );
}
