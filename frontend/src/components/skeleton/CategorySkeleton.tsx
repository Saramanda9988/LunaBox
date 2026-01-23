export function CategorySkeleton() {
  return (
    <div className="h-full w-full overflow-y-auto p-8 animate-pulse">
      <div
        className="data-glass:bg-white/2 data-glass:dark:bg-black/2 h-8 w-24 bg-brand-200 dark:bg-brand-800 rounded-md mb-6"
      />

      <div className="flex flex-col gap-6">
        <div className="flex justify-between items-center">
          <div>
            <div
              className="data-glass:bg-white/2 data-glass:dark:bg-black/2 h-12 w-64 bg-brand-200 dark:bg-brand-800 rounded-lg mb-2"
            />
            <div
              className="data-glass:bg-white/2 data-glass:dark:bg-black/2 h-6 w-32 bg-brand-200 dark:bg-brand-800 rounded-md"
            />
          </div>
        </div>

        <div
          className="data-glass:bg-white/2 data-glass:dark:bg-black/2 h-12 w-full bg-brand-200 dark:bg-brand-800 rounded-lg"
        />
      </div>

      <div className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-8">
        {[...Array.from({ length: 12 })].map((_, i) => (
          <div
            key={i}
            className="data-glass:bg-white/2 data-glass:dark:bg-black/2 aspect-[3/4] bg-brand-200 dark:bg-brand-800 rounded-lg"
          />
        ))}
      </div>
    </div>
  );
}
