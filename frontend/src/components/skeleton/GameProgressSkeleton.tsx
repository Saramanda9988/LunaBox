export function GameProgressSkeleton() {
  return (
    <div className="space-y-3 animate-pulse">
      {[...Array.from({ length: 2 })].map((_, i) => (
        <div
          key={i}
          className="data-glass:bg-white/2 data-glass:dark:bg-black/2 rounded-lg bg-brand-100 p-4 dark:bg-brand-700"
        >
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="h-5 w-36 rounded bg-brand-200/60 dark:bg-brand-600/60" />
            <div className="h-6 w-24 rounded-full bg-brand-200/60 dark:bg-brand-600/60" />
          </div>

          <div className="mt-3 grid gap-3 md:grid-cols-2">
            <div className="space-y-2">
              <div className="h-3 w-16 rounded bg-brand-200/60 dark:bg-brand-600/60" />
              <div className="h-4 w-28 rounded bg-brand-200/60 dark:bg-brand-600/60" />
            </div>
            <div className="space-y-2">
              <div className="h-3 w-14 rounded bg-brand-200/60 dark:bg-brand-600/60" />
              <div className="h-4 w-24 rounded bg-brand-200/60 dark:bg-brand-600/60" />
            </div>
          </div>

          <div className="mt-3 space-y-2">
            <div className="h-3 w-20 rounded bg-brand-200/60 dark:bg-brand-600/60" />
            <div className="h-4 w-full rounded bg-brand-200/60 dark:bg-brand-600/60" />
            <div className="h-4 w-3/4 rounded bg-brand-200/60 dark:bg-brand-600/60" />
          </div>
        </div>
      ))}
    </div>
  );
}
