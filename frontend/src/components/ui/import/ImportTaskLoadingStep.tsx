interface ImportTaskLoadingStepProps {
  iconClassName: string;
  title: string;
  subtitle?: string;
}

export function ImportTaskLoadingStep({
  iconClassName,
  title,
  subtitle,
}: ImportTaskLoadingStepProps) {
  return (
    <div className="py-12 text-center">
      <div className={`i-mdi-loading mx-auto mb-4 animate-spin text-5xl ${iconClassName}`} />
      <p className="text-lg text-brand-600 dark:text-brand-300">
        {title}
      </p>
      {subtitle && (
        <p className="mt-2 text-sm text-brand-400 dark:text-brand-500">
          {subtitle}
        </p>
      )}
    </div>
  );
}
