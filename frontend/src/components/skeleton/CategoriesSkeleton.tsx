export function CategoriesSkeleton() {
  return (
    <div className="h-full w-full overflow-y-auto p-8 animate-pulse">
      <div className="h-12 w-32 bg-brand-200 dark:bg-brand-800 rounded-lg mb-8" />
      <div className="h-12 w-full bg-brand-200 dark:bg-brand-800 rounded-lg mb-8" />
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
        {[...Array.from({ length: 8 })].map((_, i) => (
          <div key={i} className="h-32 bg-brand-200 dark:bg-brand-800 rounded-xl" />
        ))}
      </div>
    </div>
  );
}
