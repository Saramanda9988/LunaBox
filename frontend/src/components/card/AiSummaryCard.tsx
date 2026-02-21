import { useTranslation } from "react-i18next";

interface AiSummaryCardProps {
  aiSummary?: string;
  aiLoading?: boolean;
}

export function AiSummaryCard({ aiSummary, aiLoading }: AiSummaryCardProps) {
  const { t } = useTranslation();

  return (
    <div className="relative overflow-hidden bg-white dark:bg-brand-800 p-6 rounded-xl shadow-sm border border-brand-200 dark:border-brand-700 transition-all duration-300">
      {/* Background decoration */}
      <div className="absolute -top-12 -right-12 w-40 h-40 bg-primary-500/10 dark:bg-primary-500/20 rounded-full blur-3xl pointer-events-none" />
      <div className="absolute -bottom-12 -left-12 w-40 h-40 bg-accent-500/10 dark:bg-accent-500/20 rounded-full blur-3xl pointer-events-none" />

      <div className="relative z-10">
        <div className="flex items-center gap-3 mb-4">
          <div className="p-2 bg-primary-50 dark:bg-primary-900/30 rounded-lg text-primary-600 dark:text-primary-400 shadow-sm">
            <span className="i-mdi-robot-happy text-2xl block" />
          </div>
          <h3 className="text-lg font-semibold text-brand-900 dark:text-white">{t("stats.ai.title")}</h3>
          {aiLoading && (
            <span className="text-xs px-2.5 py-0.5 rounded-full bg-primary-50 dark:bg-primary-900/30 text-primary-600 dark:text-primary-400 font-medium animate-pulse border border-primary-100 dark:border-primary-800">
              {t("stats.ai.thinking")}
            </span>
          )}
        </div>

        {aiLoading
          ? (
              <div className="space-y-3 py-1">
                <div className="h-4 bg-brand-50 dark:bg-brand-700/50 rounded w-3/4 animate-pulse" />
                <div className="h-4 bg-brand-50 dark:bg-brand-700/50 rounded w-full animate-pulse delay-75" />
                <div className="h-4 bg-brand-50 dark:bg-brand-700/50 rounded w-5/6 animate-pulse delay-150" />
              </div>
            )
          : (
              <div className="text-neutral-600 dark:text-neutral-300 leading-relaxed whitespace-pre-wrap text-sm md:text-base">
                {aiSummary}
              </div>
            )}
      </div>
    </div>
  );
}
