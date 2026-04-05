import { useEffect, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { service } from "../../../wailsjs/go/models";
import {
  GetGameProgress,
  ListGameProgresses,
  UpsertGameProgress,
} from "../../../wailsjs/go/service/GameProgressService";
import { useAppStore } from "../../store";
import { formatLocalDateTime } from "../../utils/time";
import { BetterSelect } from "../ui/better/BetterSelect";

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
  const [isSaving, setIsSaving] = useState(false);
  const [progressHistory, setProgressHistory] = useState<
    service.GameProgress[]
  >([]);

  const [chapter, setChapter] = useState("");
  const [route, setRoute] = useState("");
  const [progressNote, setProgressNote] = useState("");
  const [spoilerBoundary, setSpoilerBoundary] = useState("none");

  useEffect(() => {
    const load = async () => {
      setIsLoading(true);
      try {
        const [latest, history] = await Promise.all([
          GetGameProgress(gameId),
          ListGameProgresses(gameId),
        ]);

        if (latest?.game_id) {
          setChapter(latest.chapter || "");
          setRoute(latest.route || "");
          setProgressNote(latest.progress_note || "");
          setSpoilerBoundary(latest.spoiler_boundary || "none");
        }
        else {
          setChapter("");
          setRoute("");
          setProgressNote("");
          setSpoilerBoundary("none");
        }

        setProgressHistory(history || []);
      }
      catch (e) {
        console.error("Failed to load game progress:", e);
      }
      finally {
        setIsLoading(false);
      }
    };
    load();
  }, [gameId]);

  const handleSave = async () => {
    setIsSaving(true);
    try {
      await UpsertGameProgress(
        new service.GameProgress({
          id: "",
          game_id: gameId,
          chapter,
          route,
          progress_note: progressNote,
          spoiler_boundary: spoilerBoundary,
        }),
      );
      const [latest, history] = await Promise.all([
        GetGameProgress(gameId),
        ListGameProgresses(gameId),
      ]);
      if (latest?.game_id) {
        setChapter(latest.chapter || "");
        setRoute(latest.route || "");
        setProgressNote(latest.progress_note || "");
        setSpoilerBoundary(latest.spoiler_boundary || "none");
      }
      setProgressHistory(history || []);
      toast.success(t("gameProgress.toast.saved"));
    }
    catch (e) {
      console.error("Failed to save game progress:", e);
      toast.error(t("gameProgress.toast.saveFailed"));
    }
    finally {
      setIsSaving(false);
    }
  };

  const spoilerOptions = SPOILER_OPTIONS.map(o => ({
    value: o.value,
    label: t(o.labelKey),
  }));
  const spoilerLabelMap = Object.fromEntries(
    spoilerOptions.map(option => [option.value, option.label]),
  ) as Record<string, string>;

  // Spoiler boundary helper text
  const boundaryHints: Record<string, string> = {
    none: t("gameProgress.spoilerBoundaryHints.none"),
    chapter_end: t("gameProgress.spoilerBoundaryHints.chapterEnd"),
    route_end: t("gameProgress.spoilerBoundaryHints.routeEnd"),
    full: t("gameProgress.spoilerBoundaryHints.full"),
  };
  const latestProgress = progressHistory[0];

  return (
    <div className="glass-panel mx-auto bg-white dark:bg-brand-800 p-8 rounded-lg shadow-sm">
      <div className="space-y-8 w-full">
        <section className="space-y-6">
          <div className="space-y-2">
            <div className="flex items-center gap-2">
              <div className="i-mdi-playlist-plus text-xl text-brand-600 dark:text-brand-400" />
              <h3 className="text-sm font-semibold text-brand-900 dark:text-white">
                {t("gameProgress.newRecord")}
              </h3>
            </div>
            <p className="text-xs leading-relaxed text-brand-500 dark:text-brand-400">
              {t("gameProgress.appendHint")}
            </p>
            {latestProgress && (
              <p className="text-xs text-brand-500 dark:text-brand-400">
                {t("gameProgress.latestRecordedAt", {
                  time: formatLocalDateTime(
                    latestProgress.updated_at,
                    config?.time_zone,
                  ),
                })}
              </p>
            )}
          </div>

          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              {t("gameProgress.chapter")}
            </label>
            {isLoading ? (
              <div className="h-9 bg-brand-100 dark:bg-brand-700 rounded-md animate-pulse" />
            ) : (
              <input
                type="text"
                value={chapter}
                onChange={e => setChapter(e.target.value)}
                placeholder={t("gameProgress.chapterPlaceholder")}
                className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white text-sm"
              />
            )}
          </div>

          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              {t("gameProgress.route")}
            </label>
            {isLoading ? (
              <div className="h-9 bg-brand-100 dark:bg-brand-700 rounded-md animate-pulse" />
            ) : (
              <input
                type="text"
                value={route}
                onChange={e => setRoute(e.target.value)}
                placeholder={t("gameProgress.routePlaceholder")}
                className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white text-sm"
              />
            )}
          </div>

          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              {t("gameProgress.progressNote")}
            </label>
            {isLoading ? (
              <div className="h-20 bg-brand-100 dark:bg-brand-700 rounded-md animate-pulse" />
            ) : (
              <textarea
                value={progressNote}
                onChange={e => setProgressNote(e.target.value)}
                rows={3}
                placeholder={t("gameProgress.progressNotePlaceholder")}
                className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white text-sm resize-none"
              />
            )}
          </div>

          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              {t("gameProgress.spoilerBoundary")}
            </label>
            {isLoading ? (
              <div className="h-9 bg-brand-100 dark:bg-brand-700 rounded-md animate-pulse" />
            ) : (
              <>
                <BetterSelect
                  value={spoilerBoundary}
                  onChange={setSpoilerBoundary}
                  options={spoilerOptions}
                />
                <p className="text-xs text-brand-500 dark:text-brand-400 mt-1">
                  {boundaryHints[spoilerBoundary]}
                </p>
              </>
            )}
          </div>

          <div className="flex justify-end pt-2">
            <button
              type="button"
              onClick={handleSave}
              disabled={isSaving}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-neutral-600 text-white text-sm font-medium hover:bg-neutral-700 dark:bg-white dark:text-neutral-900 dark:hover:bg-neutral-200 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <span
                className={
                  isSaving
                    ? "i-mdi-loading animate-spin"
                    : "i-mdi-playlist-plus"
                }
              />
              {isSaving ? t("common.saving") : t("gameProgress.addRecord")}
            </button>
          </div>
        </section>

        <section className="space-y-4 border-t border-brand-200 pt-6 dark:border-brand-700">
          <div className="flex items-center gap-2">
            <div className="i-mdi-timeline-text text-xl text-brand-600 dark:text-brand-400" />
            <h3 className="text-sm font-semibold text-brand-900 dark:text-white">
              {t("gameProgress.historyTitle")}
            </h3>
          </div>

          {isLoading ? (
            <div className="space-y-3">
              <div className="h-24 rounded-lg bg-brand-100 dark:bg-brand-700 animate-pulse" />
              <div className="h-24 rounded-lg bg-brand-100 dark:bg-brand-700 animate-pulse" />
            </div>
          ) : progressHistory.length === 0 ? (
            <div className="rounded-lg border border-dashed border-brand-300 px-4 py-6 text-sm text-brand-500 dark:border-brand-600 dark:text-brand-400">
              {t("gameProgress.historyEmpty")}
            </div>
          ) : (
            <div className="space-y-3">
              {progressHistory.map(progress => (
                <article
                  key={progress.id}
                  className="rounded-lg border border-brand-200 bg-brand-50/70 p-4 shadow-sm dark:border-brand-700 dark:bg-brand-900/30"
                >
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div className="flex items-center gap-2 text-sm font-medium text-brand-900 dark:text-white">
                      <span className="i-mdi-circle-medium text-brand-500 dark:text-brand-400" />
                      {formatLocalDateTime(
                        progress.updated_at,
                        config?.time_zone,
                      )}
                    </div>
                    <span className="rounded-full bg-brand-200 px-2.5 py-1 text-xs font-medium text-brand-700 dark:bg-brand-700 dark:text-brand-200">
                      {spoilerLabelMap[progress.spoiler_boundary]
                        || progress.spoiler_boundary}
                    </span>
                  </div>

                  <div className="mt-3 grid gap-3 md:grid-cols-2">
                    <div className="space-y-1">
                      <p className="text-xs font-medium uppercase tracking-wide text-brand-500 dark:text-brand-400">
                        {t("gameProgress.chapter")}
                      </p>
                      <p className="text-sm text-brand-900 dark:text-white">
                        {progress.chapter || "-"}
                      </p>
                    </div>
                    <div className="space-y-1">
                      <p className="text-xs font-medium uppercase tracking-wide text-brand-500 dark:text-brand-400">
                        {t("gameProgress.route")}
                      </p>
                      <p className="text-sm text-brand-900 dark:text-white">
                        {progress.route || "-"}
                      </p>
                    </div>
                  </div>

                  <div className="mt-3 space-y-1">
                    <p className="text-xs font-medium uppercase tracking-wide text-brand-500 dark:text-brand-400">
                      {t("gameProgress.progressNote")}
                    </p>
                    <p className="whitespace-pre-wrap text-sm leading-relaxed text-brand-700 dark:text-brand-200">
                      {progress.progress_note || t("gameProgress.noNote")}
                    </p>
                  </div>
                </article>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
