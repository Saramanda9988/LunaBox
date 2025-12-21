export function GameDetailSkeleton() {
    return (
        <div className="space-y-8 max-w-8xl mx-auto p-8 animate-pulse">
            <div className="h-8 w-24 bg-brand-200 dark:bg-brand-800 rounded-md" />

            <div className="flex gap-6 items-center">
                <div className="w-60 h-80 flex-shrink-0 bg-brand-200 dark:bg-brand-800 rounded-lg" />
                <div className="flex-1 space-y-4">
                    <div className="h-12 w-3/4 bg-brand-200 dark:bg-brand-800 rounded-lg" />
                    <div className="grid grid-cols-4 gap-4">
                        {[...Array(4)].map((_, i) => (
                            <div key={i} className="h-12 bg-brand-200 dark:bg-brand-800 rounded-md" />
                        ))}
                    </div>
                    <div className="h-32 w-full bg-brand-200 dark:bg-brand-800 rounded-lg" />
                </div>
            </div>

            <div className="border-b border-brand-200 dark:border-brand-700 flex space-x-8">
                {[...Array(3)].map((_, i) => (
                    <div key={i} className="h-10 w-20 bg-brand-200 dark:bg-brand-800 rounded-t-md" />
                ))}
            </div>

            <div className="grid grid-cols-3 gap-6">
                {[...Array(3)].map((_, i) => (
                    <div key={i} className="h-24 bg-brand-200 dark:bg-brand-800 rounded-lg" />
                ))}
            </div>

            <div className="h-80 bg-brand-200 dark:bg-brand-800 rounded-lg" />
        </div>
    )
}