import type { MatchProgressState } from "./types";

interface ImportMatchProgressStepProps {
  title: string;
  hint: string;
  progress: MatchProgressState;
  spinnerClassName: string;
  progressClassName: string;
  onStop?: () => void;
  stopLabel?: string;
}

export function ImportMatchProgressStep({
  title,
  hint,
  progress,
  spinnerClassName,
  progressClassName,
  onStop,
  stopLabel,
}: ImportMatchProgressStepProps) {
  const progressWidth = progress.total > 0
    ? `${(progress.current / progress.total) * 100}%`
    : "0%";

  return (
    <div className="py-12 text-center">
      <div className={`i-mdi-loading mx-auto mb-4 animate-spin text-5xl ${spinnerClassName}`} />
      <p className="text-lg text-brand-600 dark:text-brand-300">
        {title}
      </p>
      <p className="mt-2 text-sm text-brand-400 dark:text-brand-500">
        {progress.current}
        {" "}
        /
        {progress.total}
      </p>
      <p className="mt-2 text-sm text-neutral-500">
        {progress.gameName}
      </p>
      <div className="mx-auto mt-4 h-2 w-full max-w-md rounded-full bg-brand-200 dark:bg-brand-700">
        <div
          className={`h-2 rounded-full transition-all duration-300 ${progressClassName}`}
          style={{ width: progressWidth }}
        />
      </div>
      <p className="mt-4 text-xs text-brand-400">
        {hint}
      </p>
      {onStop && stopLabel && (
        <button
          type="button"
          onClick={onStop}
          className="mt-4 text-sm text-brand-500 hover:text-brand-700 dark:text-brand-400"
        >
          {stopLabel}
        </button>
      )}
    </div>
  );
}
