export function StatsSkeleton() {
  return (
    <div className="space-y-6 max-w-8xl mx-auto p-8 animate-pulse bg-brand-100 dark:bg-brand-900 min-h-screen">
      <div
        className="data-glass:bg-white/2 data-glass:dark:bg-black/2 h-12 w-32 bg-brand-200 dark:bg-brand-800 rounded-lg mb-8"
      />

      <div className="flex justify-between items-center mb-6">
        <div
          className="data-glass:bg-white/2 data-glass:dark:bg-black/2 h-10 w-48 bg-brand-200 dark:bg-brand-800 rounded-lg"
        />
        <div className="flex space-x-2">
          <div
            className="data-glass:bg-white/2 data-glass:dark:bg-black/2 h-8 w-8 bg-brand-200 dark:bg-brand-800 rounded-full"
          />
          <div
            className="data-glass:bg-white/2 data-glass:dark:bg-black/2 h-8 w-8 bg-brand-200 dark:bg-brand-800 rounded-full"
          />
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div
          className="data-glass:bg-white/2 data-glass:dark:bg-black/2 h-32 bg-brand-200 dark:bg-brand-800 rounded-xl"
        />
        <div
          className="data-glass:bg-white/2 data-glass:dark:bg-black/2 h-32 bg-brand-200 dark:bg-brand-800 rounded-xl"
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div
          className="lg:col-span-1 data-glass:bg-white/2 data-glass:dark:bg-black/2 h-[450px] bg-brand-200 dark:bg-brand-800 rounded-xl"
        />
        <div
          className="lg:col-span-2 data-glass:bg-white/2 data-glass:dark:bg-black/2 h-[450px] bg-brand-200 dark:bg-brand-800 rounded-xl"
        />
      </div>

      <div className="space-y-6">
        <div
          className="data-glass:bg-white/2 data-glass:dark:bg-black/2 h-96 bg-brand-200 dark:bg-brand-800 rounded-xl"
        />
        <div
          className="data-glass:bg-white/2 data-glass:dark:bg-black/2 h-96 bg-brand-200 dark:bg-brand-800 rounded-xl"
        />
      </div>
    </div>
  );
}
