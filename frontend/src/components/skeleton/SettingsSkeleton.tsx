export function SettingsSkeleton() {
    return (
        <div className="space-y-8 max-w-8xl mx-auto p-8 animate-pulse">
            <div className="h-12 w-32 bg-brand-200 dark:bg-brand-800 rounded-lg mb-8" />

            <div className="space-y-6">
                {[...Array(4)].map((_, i) => (
                    <div key={i} className="space-y-2">
                        <div className="h-4 w-32 bg-brand-200 dark:bg-brand-800 rounded" />
                        <div className="h-10 w-full bg-brand-200 dark:bg-brand-800 rounded-md" />
                    </div>
                ))}
            </div>

            <div className="pt-6 border-t border-brand-200 dark:border-brand-700 space-y-4">
                <div className="h-6 w-48 bg-brand-200 dark:bg-brand-800 rounded" />
                <div className="h-10 w-full bg-brand-200 dark:bg-brand-800 rounded-md" />
            </div>
        </div>
    )
}