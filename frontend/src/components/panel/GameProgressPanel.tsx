import { useEffect, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { service } from "../../../wailsjs/go/models";
import { GetGameProgress, UpsertGameProgress } from "../../../wailsjs/go/service/GameProgressService";
import { BetterSelect } from "../ui/better/BetterSelect";

interface GameProgressPanelProps {
  gameId: string;
}

const SPOILER_OPTIONS = [
  { value: "none", labelKey: "gameProgress.spoilerBoundaryOpts.none" },
  { value: "chapter_end", labelKey: "gameProgress.spoilerBoundaryOpts.chapterEnd" },
  { value: "route_end", labelKey: "gameProgress.spoilerBoundaryOpts.routeEnd" },
  { value: "full", labelKey: "gameProgress.spoilerBoundaryOpts.full" },
];

export function GameProgressPanel({ gameId }: GameProgressPanelProps) {
  const { t } = useTranslation();

  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);

  const [chapter, setChapter] = useState("");
  const [route, setRoute] = useState("");
  const [progressNote, setProgressNote] = useState("");
  const [spoilerBoundary, setSpoilerBoundary] = useState("none");

  useEffect(() => {
    const load = async () => {
      try {
        const data: service.GameProgress = await GetGameProgress(gameId);
        if (data && data.game_id) {
          setChapter(data.chapter || "");
          setRoute(data.route || "");
          setProgressNote(data.progress_note || "");
          setSpoilerBoundary(data.spoiler_boundary || "none");
        }
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
      await UpsertGameProgress(new service.GameProgress({
        id: "",
        game_id: gameId,
        chapter,
        route,
        progress_note: progressNote,
        spoiler_boundary: spoilerBoundary,
      }));
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

  const spoilerOptions = SPOILER_OPTIONS.map(o => ({ value: o.value, label: t(o.labelKey) }));

  // Spoiler boundary helper text
  const boundaryHints: Record<string, string> = {
    none: t("gameProgress.spoilerBoundaryHints.none"),
    chapter_end: t("gameProgress.spoilerBoundaryHints.chapterEnd"),
    route_end: t("gameProgress.spoilerBoundaryHints.routeEnd"),
    full: t("gameProgress.spoilerBoundaryHints.full"),
  };

  return (
    <div className="glass-panel mx-auto bg-white dark:bg-brand-800 p-8 rounded-lg shadow-sm">
      <div className="space-y-6 w-full">

        {/* Chapter */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("gameProgress.chapter")}
          </label>
          {isLoading
            ? <div className="h-9 bg-brand-100 dark:bg-brand-700 rounded-md animate-pulse" />
            : (
                <input
                  type="text"
                  value={chapter}
                  onChange={e => setChapter(e.target.value)}
                  placeholder={t("gameProgress.chapterPlaceholder")}
                  className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white text-sm"
                />
              )}
        </div>

        {/* Route */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("gameProgress.route")}
          </label>
          {isLoading
            ? <div className="h-9 bg-brand-100 dark:bg-brand-700 rounded-md animate-pulse" />
            : (
                <input
                  type="text"
                  value={route}
                  onChange={e => setRoute(e.target.value)}
                  placeholder={t("gameProgress.routePlaceholder")}
                  className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white text-sm"
                />
              )}
        </div>

        {/* Progress note */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("gameProgress.progressNote")}
          </label>
          {isLoading
            ? <div className="h-20 bg-brand-100 dark:bg-brand-700 rounded-md animate-pulse" />
            : (
                <textarea
                  value={progressNote}
                  onChange={e => setProgressNote(e.target.value)}
                  rows={3}
                  placeholder={t("gameProgress.progressNotePlaceholder")}
                  className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white text-sm resize-none"
                />
              )}
        </div>

        {/* Spoiler boundary */}
        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("gameProgress.spoilerBoundary")}
          </label>
          {isLoading
            ? <div className="h-9 bg-brand-100 dark:bg-brand-700 rounded-md animate-pulse" />
            : (
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

        {/* Save button */}
        <div className="flex justify-end pt-2">
          <button
            type="button"
            onClick={handleSave}
            disabled={isSaving}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-neutral-600 text-white text-sm font-medium hover:bg-neutral-700 dark:bg-white dark:text-neutral-900 dark:hover:bg-neutral-200 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <span className={isSaving ? "i-mdi-loading animate-spin" : "i-mdi-content-save"} />
            {isSaving ? t("common.saving") : t("common.save")}
          </button>
        </div>
      </div>
    </div>
  );
}
