import type { enums, models } from "../../../src/bindings/models";
import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  GetGameReview,
  SaveGameReview,
  SyncGameReview,
} from "../../../bindings/lunabox/internal/service/gamereviewservice";
import {
  enums as modelEnums,
  models as modelTypes,
} from "../../../src/bindings/models";
import { fetchBangumiAuthStatus } from "../../utils/bangumiAuth";
import { fetchHikarinagiAuthStatus } from "../../utils/hikarinagiAuth";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterSwitch } from "../ui/better/BetterSwitch";

interface GameReviewPanelProps {
  game: models.Game;
}

type AutoSaveStatus = "idle" | "pending" | "saving" | "saved" | "error";

const REVIEW_CONTENT_MAX_LENGTH = 100;

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function gameHasProvider(
  game: models.Game,
  provider: enums.SourceType,
): boolean {
  const linked = (game.metadata_sources ?? []).some(
    source =>
      source.source_type === provider && Boolean(source.source_id?.trim()),
  );
  return (
    linked || (game.source_type === provider && Boolean(game.source_id?.trim()))
  );
}

interface StarRatingProps {
  rating: number | null;
  onChange: (rating: number) => void;
}

function StarRating({ rating, onChange }: StarRatingProps) {
  const { t } = useTranslation();
  const [previewRating, setPreviewRating] = useState<number | null>(null);
  const displayedRating = previewRating ?? rating ?? 0;

  return (
    <div
      className="flex w-fit items-center gap-2"
      role="radiogroup"
      aria-label={t("gameReview.ratingLabel")}
      onMouseLeave={() => setPreviewRating(null)}
    >
      {Array.from({ length: 5 }, (_, starIndex) => {
        const leftValue = starIndex * 2 + 1;
        const rightValue = leftValue + 1;
        const fillPercent
          = displayedRating >= rightValue
            ? 100
            : displayedRating === leftValue
              ? 50
              : 0;

        return (
          <span key={leftValue} className="relative block h-8 w-8 shrink-0">
            <span
              aria-hidden="true"
              className="i-mdi-star-outline absolute inset-0 h-8 w-8 text-3xl text-brand-300 dark:text-brand-500"
            />
            <span
              aria-hidden="true"
              className="pointer-events-none absolute inset-y-0 left-0 overflow-hidden transition-[width] duration-150"
              style={{ width: `${fillPercent}%` }}
            >
              <span className="i-mdi-star absolute inset-y-0 left-0 h-8 w-8 text-3xl text-amber-400 dark:text-amber-300" />
            </span>
            <button
              type="button"
              role="radio"
              aria-checked={rating === leftValue}
              aria-label={t("gameReview.ratingValue", { rating: leftValue })}
              className="absolute inset-y-0 left-0 w-1/2 appearance-none border-0 bg-transparent p-0 outline-none"
              onMouseEnter={() => setPreviewRating(leftValue)}
              onFocus={() => setPreviewRating(leftValue)}
              onBlur={() => setPreviewRating(null)}
              onClick={() => onChange(leftValue)}
            />
            <button
              type="button"
              role="radio"
              aria-checked={rating === rightValue}
              aria-label={t("gameReview.ratingValue", { rating: rightValue })}
              className="absolute inset-y-0 right-0 w-1/2 appearance-none border-0 bg-transparent p-0 outline-none"
              onMouseEnter={() => setPreviewRating(rightValue)}
              onFocus={() => setPreviewRating(rightValue)}
              onBlur={() => setPreviewRating(null)}
              onClick={() => onChange(rightValue)}
            />
          </span>
        );
      })}
    </div>
  );
}

export function GameReviewPanel({ game }: GameReviewPanelProps) {
  const { t } = useTranslation();
  const [rating, setRating] = useState<number | null>(null);
  const [content, setContent] = useState("");
  const [isSpoiler, setIsSpoiler] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [autoSaveStatus, setAutoSaveStatus] = useState<AutoSaveStatus>("idle");
  const [isAutoSaveInFlight, setIsAutoSaveInFlight] = useState(false);
  const [syncingProvider, setSyncingProvider]
    = useState<enums.SourceType | null>(null);
  const [auth, setAuth] = useState<Record<string, boolean>>({});
  const editRevision = useRef(0);

  useEffect(() => {
    let cancelled = false;

    async function loadReview() {
      setIsLoading(true);
      const [reviewResult, bangumiResult, hikarinagiResult]
        = await Promise.allSettled([
          GetGameReview(game.id),
          fetchBangumiAuthStatus(),
          fetchHikarinagiAuthStatus(),
        ]);
      if (cancelled) {
        return;
      }

      if (reviewResult.status === "fulfilled" && reviewResult.value) {
        setRating(reviewResult.value.rating);
        setContent(reviewResult.value.content);
        setIsSpoiler(reviewResult.value.is_spoiler);
        setAutoSaveStatus("saved");
      }
      else if (reviewResult.status === "rejected") {
        toast.error(
          t("gameReview.toast.loadFailed", {
            error: errorMessage(reviewResult.reason),
          }),
        );
      }

      const nextAuth: Record<string, boolean> = {
        [modelEnums.SourceType.Bangumi]:
          bangumiResult.status === "fulfilled"
          && bangumiResult.value.authorized
          && !bangumiResult.value.needs_reauthorization,
        [modelEnums.SourceType.Hikarinagi]:
          hikarinagiResult.status === "fulfilled"
          && hikarinagiResult.value.authorized
          && !hikarinagiResult.value.needs_reauthorization,
      };
      setAuth(nextAuth);
      setIsLoading(false);
    }

    loadReview();
    return () => {
      cancelled = true;
    };
  }, [game, t]);

  const saveReview = useCallback(async (): Promise<models.GameReview> => {
    const saved = await SaveGameReview(
      modelTypes.GameReview.createFrom({
        game_id: game.id,
        rating,
        content,
        is_spoiler: isSpoiler,
      }),
    );
    if (!saved) {
      throw new Error(t("gameReview.toast.emptySaveResult"));
    }
    return saved;
  }, [content, game.id, isSpoiler, rating, t]);

  useEffect(() => {
    if (
      autoSaveStatus !== "pending"
      || isLoading
      || isAutoSaveInFlight
      || syncingProvider !== null
    ) {
      return;
    }

    const revision = editRevision.current;
    const timer = window.setTimeout(async () => {
      setIsAutoSaveInFlight(true);
      setAutoSaveStatus("saving");
      try {
        await saveReview();
        setAutoSaveStatus(
          editRevision.current === revision ? "saved" : "pending",
        );
      }
      catch {
        if (editRevision.current === revision) {
          setAutoSaveStatus("error");
        }
      }
      finally {
        setIsAutoSaveInFlight(false);
      }
    }, 800);

    return () => window.clearTimeout(timer);
  }, [
    autoSaveStatus,
    isAutoSaveInFlight,
    isLoading,
    saveReview,
    syncingProvider,
  ]);

  const markEdited = () => {
    editRevision.current += 1;
    setAutoSaveStatus("pending");
  };

  const handleRatingChange = (value: number | null) => {
    setRating(value);
    markEdited();
  };

  const handleContentChange = (value: string) => {
    setContent(value);
    markEdited();
  };

  const handleSpoilerChange = (value: boolean) => {
    setIsSpoiler(value);
    markEdited();
  };

  const handleProviderSync = async (provider: enums.SourceType) => {
    const revision = editRevision.current;
    setSyncingProvider(provider);
    setAutoSaveStatus("saving");
    try {
      await saveReview();
      setAutoSaveStatus(
        editRevision.current === revision ? "saved" : "pending",
      );
      const result = await SyncGameReview(game.id, [provider]);
      if (result.failed > 0) {
        const providerError = result.results?.[0]?.error;
        toast.error(
          providerError
          || t("gameReview.toast.syncPartial", {
            succeeded: result.succeeded,
            failed: result.failed,
          }),
        );
      }
      else {
        toast.success(
          t("gameReview.toast.synced", { count: result.succeeded }),
        );
      }
    }
    catch (error) {
      if (editRevision.current === revision) {
        setAutoSaveStatus("error");
      }
      toast.error(
        t("gameReview.toast.syncFailed", {
          error: errorMessage(error),
        }),
      );
    }
    finally {
      setSyncingProvider(null);
    }
  };

  const canSyncBangumi
    = Boolean(auth[modelEnums.SourceType.Bangumi])
      && gameHasProvider(game, modelEnums.SourceType.Bangumi);
  const canSyncHikarinagi
    = Boolean(auth[modelEnums.SourceType.Hikarinagi])
      && gameHasProvider(game, modelEnums.SourceType.Hikarinagi);
  const hasReviewContent = content.trim().length > 0;
  const autoSaveLabel
    = autoSaveStatus === "pending"
      ? t("gameReview.autoSave.pending")
      : autoSaveStatus === "saving"
        ? t("gameReview.autoSave.saving")
        : autoSaveStatus === "saved"
          ? t("gameReview.autoSave.saved")
          : autoSaveStatus === "error"
            ? t("gameReview.autoSave.failed")
            : t("gameReview.autoSave.hint");
  const autoSaveIcon
    = autoSaveStatus === "saving"
      ? "i-mdi-loading animate-spin"
      : autoSaveStatus === "saved"
        ? "i-mdi-check-circle-outline text-success-600 dark:text-success-400"
        : autoSaveStatus === "error"
          ? "i-mdi-alert-circle-outline text-error-600 dark:text-error-400"
          : "i-mdi-content-save-clock-outline";

  return (
    <div>
      <section
        className="glass-card relative rounded-lg bg-white p-6 shadow-sm dark:bg-brand-800"
        aria-busy={isLoading}
      >
        <div className={isLoading ? "invisible" : undefined}>
          <header>
            <div className="min-w-0">
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-brand-500 dark:text-brand-400">
                {t("gameReview.title")}
              </p>
              <h3 className="mt-2 break-words text-2xl font-bold leading-tight text-brand-900 dark:text-white">
                {game.name}
              </h3>
              <p className="mt-5 text-sm text-brand-600 dark:text-brand-300">
                {t("gameReview.ratingPrompt")}
              </p>
              <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-2">
                <StarRating rating={rating} onChange={handleRatingChange} />
                <span className="font-mono text-xs text-brand-500 dark:text-brand-400">
                  {rating === null
                    ? t("gameReview.ratingEmpty")
                    : t("gameReview.ratingValue", { rating })}
                </span>
                {rating !== null && (
                  <button
                    type="button"
                    className="border-0 bg-transparent p-0 text-xs text-brand-500 outline-none underline-offset-2 hover:text-brand-800 hover:underline dark:text-brand-400 dark:hover:text-brand-200"
                    onClick={() => handleRatingChange(null)}
                  >
                    {t("gameReview.clearRating")}
                  </button>
                )}
              </div>
            </div>
          </header>

          <div className="mt-6">
            <label htmlFor="game-review-content" className="sr-only">
              {t("gameReview.contentLabel")}
            </label>
            <div className="relative">
              <textarea
                id="game-review-content"
                value={content}
                maxLength={REVIEW_CONTENT_MAX_LENGTH}
                onChange={event => handleContentChange(event.target.value)}
                placeholder={t("gameReview.contentPlaceholder")}
                className="min-h-60 w-full resize-y rounded-xl border border-brand-250 bg-brand-50 px-5 py-4 pb-10 text-sm leading-6 text-brand-900 outline-none transition-[border-color,box-shadow] placeholder:text-brand-400 focus:border-neutral-500 focus:ring-2 focus:ring-neutral-500/20 dark:border-brand-700 dark:bg-brand-750 dark:text-white dark:placeholder:text-brand-500 dark:focus:border-neutral-300 dark:focus:ring-neutral-300/20"
              />
              <span className="pointer-events-none absolute bottom-4 right-4 font-mono text-xs text-brand-400 dark:text-brand-500">
                {content.length}
                /
                {REVIEW_CONTENT_MAX_LENGTH}
              </span>
            </div>
            <div className="mt-3 flex flex-col gap-3 rounded-lg bg-brand-50 px-3 py-2.5 dark:bg-brand-750 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <label
                  htmlFor="game-review-spoiler"
                  className="text-sm font-medium text-brand-800 dark:text-brand-100"
                >
                  {t("gameReview.spoilerLabel")}
                </label>
                <p className="text-xs text-brand-500 dark:text-brand-400">
                  {t("gameReview.spoilerHint")}
                </p>
              </div>
              <BetterSwitch
                id="game-review-spoiler"
                checked={isSpoiler}
                onCheckedChange={handleSpoilerChange}
              />
            </div>
          </div>

          <div className="mt-6 flex flex-col gap-3 border-t border-brand-200 pt-5 dark:border-brand-700 sm:flex-row sm:items-center sm:justify-between">
            <span
              className={`flex items-center gap-2 text-xs ${
                autoSaveStatus === "error"
                  ? "text-error-600 dark:text-error-400"
                  : "text-brand-500 dark:text-brand-400"
              }`}
            >
              <span className={`${autoSaveIcon} text-base`} />
              {autoSaveLabel}
            </span>
            <div className="flex flex-col gap-3 sm:flex-row">
              <BetterButton
                variant="primary"
                icon="i-mdi-cloud-upload-outline"
                isLoading={syncingProvider === modelEnums.SourceType.Bangumi}
                disabled={
                  autoSaveStatus === "saving"
                  || syncingProvider !== null
                  || !canSyncBangumi
                  || !hasReviewContent
                }
                onClick={() =>
                  handleProviderSync(modelEnums.SourceType.Bangumi)}
              >
                {t("gameReview.syncTo", { provider: "Bangumi" })}
              </BetterButton>
              <BetterButton
                variant="primary"
                icon="i-mdi-cloud-upload-outline"
                isLoading={syncingProvider === modelEnums.SourceType.Hikarinagi}
                disabled={
                  autoSaveStatus === "saving"
                  || syncingProvider !== null
                  || !canSyncHikarinagi
                  || !hasReviewContent
                }
                onClick={() =>
                  handleProviderSync(modelEnums.SourceType.Hikarinagi)}
              >
                {t("gameReview.syncTo", { provider: "Hikarinagi" })}
              </BetterButton>
            </div>
          </div>
        </div>

        {isLoading && (
          <div
            className="absolute inset-0 animate-pulse p-6"
            aria-hidden="true"
          >
            <div className="h-4 w-24 rounded bg-brand-200 dark:bg-brand-700" />
            <div className="mt-2 h-8 w-1/2 rounded bg-brand-200 dark:bg-brand-700" />
            <div className="mt-5 h-4 w-28 rounded bg-brand-100 dark:bg-brand-750" />
            <div className="mt-2 h-8 w-56 rounded bg-brand-100 dark:bg-brand-750" />
            <div className="mt-6 h-60 rounded-xl bg-brand-100 dark:bg-brand-750" />
            <div className="mt-3 h-16 rounded-lg bg-brand-100 dark:bg-brand-750" />
            <div className="mt-6 flex items-center justify-between border-t border-brand-200 pt-5 dark:border-brand-700">
              <div className="h-4 w-28 rounded bg-brand-100 dark:bg-brand-750" />
              <div className="h-10 w-56 rounded-lg bg-brand-200 dark:bg-brand-700" />
            </div>
          </div>
        )}
      </section>
    </div>
  );
}
