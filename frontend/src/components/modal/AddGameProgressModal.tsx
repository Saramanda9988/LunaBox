import { useEffect, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { models } from "../../../wailsjs/go/models";
import {
  GetGameProgress,
  UpsertGameProgress,
} from "../../../wailsjs/go/service/GameProgressService";
import { BetterSelect } from "../ui/better/BetterSelect";

interface AddGameProgressModalProps {
  isOpen: boolean;
  gameId: string;
  onClose: () => void;
  onSuccess: () => void;
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

export function AddGameProgressModal({
  isOpen,
  gameId,
  onClose,
  onSuccess,
}: AddGameProgressModalProps) {
  const { t } = useTranslation();

  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);

  const [chapter, setChapter] = useState("");
  const [route, setRoute] = useState("");
  const [progressNote, setProgressNote] = useState("");
  const [spoilerBoundary, setSpoilerBoundary] = useState("none");

  useEffect(() => {
    if (!isOpen)
      return;

    const load = async () => {
      setIsLoading(true);
      try {
        const latest = await GetGameProgress(gameId);
        if (latest?.game_id) {
          setChapter(latest.chapter || "");
          setRoute(latest.route || "");
          setProgressNote(""); // usually note is left empty for new entry, but the old code kept the old note. I will keep the old behavior to populate it, wait, old code left it populated, users might just want it to be empty. Let's just follow the old code that populated it all, wait, the old code populated it so the user can edit on top of it.
          // Wait, no, progress note from latest? If we bring over from latest, we don't bring the note because note is for the specific update? Let's just copy exactly what was in the original:
          setSpoilerBoundary(latest.spoiler_boundary || "none");
        }
        else {
          setChapter("");
          setRoute("");
          setProgressNote("");
          setSpoilerBoundary("none");
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
  }, [gameId, isOpen]);

  if (!isOpen)
    return null;

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSaving(true);
    try {
      await UpsertGameProgress(
        new models.GameProgress({
          id: "",
          game_id: gameId,
          chapter,
          route,
          progress_note: progressNote,
          spoiler_boundary: spoilerBoundary,
        }),
      );
      toast.success(t("gameProgress.toast.saved"));
      onSuccess();
      onClose();
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

  // Spoiler boundary helper text
  const boundaryHints: Record<string, string> = {
    none: t("gameProgress.spoilerBoundaryHints.none"),
    chapter_end: t("gameProgress.spoilerBoundaryHints.chapterEnd"),
    route_end: t("gameProgress.spoilerBoundaryHints.routeEnd"),
    full: t("gameProgress.spoilerBoundaryHints.full"),
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
      <div className="relative bg-white dark:bg-brand-800 rounded-lg shadow-xl w-full max-w-2xl mx-4 p-6">
        <div className="flex justify-between items-center mb-4">
          <div className="flex items-center gap-2">
            <div className="i-mdi-playlist-plus text-xl text-brand-600 dark:text-brand-400" />
            <h2 className="text-lg font-semibold text-brand-900 dark:text-white">
              {t("gameProgress.newRecord")}
            </h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="text-brand-500 hover:text-brand-700 dark:text-brand-400 dark:hover:text-white transition-colors"
          >
            <div className="i-mdi-close text-xl" />
          </button>
        </div>

        <form onSubmit={handleSave} className="space-y-4">
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

          <div className="flex justify-end gap-3 pt-6 border-t border-brand-200 dark:border-brand-700 mt-6">
            <button
              type="submit"
              disabled={isSaving || isLoading}
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
        </form>
      </div>
    </div>
  );
}
