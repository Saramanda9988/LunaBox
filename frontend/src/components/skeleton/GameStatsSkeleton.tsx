export function GameStatsSkeleton() {
  return (
    <div className="space-y-8">
      <div className="grid grid-cols-3 gap-6">
        {[1, 2, 3].map(i => (
          <div
            key={i}
            className="data-glass:bg-white/2 data-glass:dark:bg-black/2 bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm"
          >
            <div className="h-4 bg-brand-200/50 dark:bg-brand-700/50 rounded w-24 mb-2" />
            <div className="h-8 bg-brand-200/50 dark:bg-brand-700/50 rounded w-16" />
          </div>
        ))}
      </div>
      <div
        className="data-glass:bg-white/2 data-glass:dark:bg-black/2 bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm"
      >
        <div className="h-80 bg-brand-100/50 dark:bg-brand-700/50 rounded" />
      </div>
    </div>
  );
}
