import type { service } from "../../../../wailsjs/go/models";

interface ImportResultStepLabels {
  success: string;
  skipped: string;
  failed: string;
  skippedGames: string;
  failedGames: string;
  complete: string;
}

interface ImportResultStepProps {
  result: service.ImportResult;
  labels: ImportResultStepLabels;
  completeButtonClassName: string;
  onComplete: () => void;
}

export function ImportResultStep({
  result,
  labels,
  completeButtonClassName,
  onComplete,
}: ImportResultStepProps) {
  return (
    <div className="space-y-6">
      <div className="flex gap-4">
        <div className="flex-1 rounded-lg bg-success-50 p-4 text-center dark:bg-success-900/20">
          <div className="i-mdi-check-circle mx-auto mb-2 text-3xl text-success-500" />
          <div className="text-2xl font-bold text-success-600 dark:text-success-400">
            {result.success}
          </div>
          <div className="text-sm text-success-700 dark:text-success-300">
            {labels.success}
          </div>
        </div>
        {result.skipped > 0 && (
          <div className="flex-1 rounded-lg bg-yellow-50 p-4 text-center dark:bg-yellow-900/20">
            <div className="i-mdi-skip-next-circle mx-auto mb-2 text-3xl text-yellow-500" />
            <div className="text-2xl font-bold text-yellow-600 dark:text-yellow-400">
              {result.skipped}
            </div>
            <div className="text-sm text-yellow-700 dark:text-yellow-300">
              {labels.skipped}
            </div>
          </div>
        )}
        {result.failed > 0 && (
          <div className="flex-1 rounded-lg bg-error-50 p-4 text-center dark:bg-error-900/20">
            <div className="i-mdi-close-circle mx-auto mb-2 text-3xl text-error-500" />
            <div className="text-2xl font-bold text-error-600 dark:text-error-400">
              {result.failed}
            </div>
            <div className="text-sm text-error-700 dark:text-error-300">
              {labels.failed}
            </div>
          </div>
        )}
      </div>

      {result.skipped_names && result.skipped_names.length > 0 && (
        <div className="rounded-lg border border-yellow-200 p-4 dark:border-yellow-800">
          <h4 className="mb-2 font-medium text-yellow-700 dark:text-yellow-400">
            {labels.skippedGames}
          </h4>
          <div className="max-h-[150px] overflow-y-auto">
            <ul className="space-y-1 text-sm text-yellow-600 dark:text-yellow-300">
              {result.skipped_names.map((name, index) => (
                <li key={`${name}-${index}`}>
                  •
                  {name}
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}

      {result.failed_names && result.failed_names.length > 0 && (
        <div className="rounded-lg border border-error-200 p-4 dark:border-error-800">
          <h4 className="mb-2 font-medium text-error-700 dark:text-error-400">
            {labels.failedGames}
          </h4>
          <ul className="space-y-1 text-sm text-error-600 dark:text-error-300">
            {result.failed_names.map((name, index) => (
              <li key={`${name}-${index}`}>
                •
                {name}
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="flex justify-center">
        <button
          type="button"
          onClick={onComplete}
          className={`rounded-lg px-8 py-2.5 text-sm font-medium text-white ${completeButtonClassName}`}
        >
          {labels.complete}
        </button>
      </div>
    </div>
  );
}
