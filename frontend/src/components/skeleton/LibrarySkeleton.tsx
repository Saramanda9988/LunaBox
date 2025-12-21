export function LibrarySkeleton() {
    return (
        <div className="space-y-6 max-w-8xl mx-auto p-8 animate-pulse">
            <div className="h-12 w-48 bg-brand-200 dark:bg-brand-800 rounded-lg" />

            <div className="flex justify-between items-center">
                <div className="h-10 w-64 bg-brand-200 dark:bg-brand-800 rounded-lg" />
                <div className="h-10 w-32 bg-brand-200 dark:bg-brand-800 rounded-lg" />
            </div>

            <div className="grid grid-cols-[repeat(auto-fill,minmax(max(8rem,11%),1fr))] gap-3">
                {[...Array(16)].map((_, i) => (
                    <div key={i} className="aspect-[3/4] bg-brand-200 dark:bg-brand-800 rounded-lg" />
                ))}
            </div>
        </div>
    )
}