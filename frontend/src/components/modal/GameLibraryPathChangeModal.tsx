import type { service } from "../../../src/bindings/models";
import {
  Dialog,
  DialogBackdrop,
  DialogDescription,
  DialogPanel,
  DialogTitle,
} from "@headlessui/react";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { BetterButton } from "../ui/better/BetterButton";

interface GameLibraryPathChangeModalProps {
  isApplying: boolean;
  onApply: (syncPaths: boolean) => void;
  onClose: () => void;
  preview: service.GameLibraryPathChangePreview | null;
}

const MAX_VISIBLE_GAMES = 200;

interface GamePathChangePreview {
  key: string;
  name: string;
  oldPath: string;
  newPath: string;
  targetExists: boolean;
}

interface RebasedPathParts {
  oldPrefix: string;
  newPrefix: string;
  suffix: string;
}

function fieldPriority(field: string) {
  switch (field) {
    case "game_directory":
      return 0;
    case "path":
      return 1;
    case "save_path":
      return 2;
    case "file_path":
      return 3;
    default:
      return 4;
  }
}

function hasDirectoryPrefix(path: string, prefix: string) {
  if (
    !prefix
    || !path.toLocaleLowerCase().startsWith(prefix.toLocaleLowerCase())
  ) {
    return false;
  }

  const boundary = path.slice(prefix.length, prefix.length + 1);
  return boundary === "" || boundary === "\\" || boundary === "/";
}

function splitRebasedPath(
  oldPath: string,
  newPath: string,
  oldLibraryPath: string,
  newLibraryPath: string,
): RebasedPathParts {
  if (
    hasDirectoryPrefix(oldPath, oldLibraryPath)
    && hasDirectoryPrefix(newPath, newLibraryPath)
  ) {
    const oldSuffix = oldPath.slice(oldLibraryPath.length);
    const newSuffix = newPath.slice(newLibraryPath.length);
    if (oldSuffix.toLocaleLowerCase() === newSuffix.toLocaleLowerCase()) {
      return {
        oldPrefix: oldLibraryPath,
        newPrefix: newLibraryPath,
        suffix: newSuffix,
      };
    }
  }

  return {
    oldPrefix: oldPath,
    newPrefix: newPath,
    suffix: "",
  };
}

function PathPrefixDiff({
  oldPath,
  newPath,
  oldLibraryPath,
  newLibraryPath,
}: {
  oldPath: string;
  newPath: string;
  oldLibraryPath: string;
  newLibraryPath: string;
}) {
  const parts = splitRebasedPath(
    oldPath,
    newPath,
    oldLibraryPath,
    newLibraryPath,
  );

  return (
    <p
      className="overflow-hidden text-ellipsis whitespace-nowrap font-mono text-xs text-brand-800 dark:text-brand-200"
      title={`${oldPath} → ${newPath}`}
      aria-label={`${oldPath} → ${newPath}`}
    >
      <span
        className="rounded bg-error-100 px-1 py-0.5 text-error-700 line-through decoration-error-500/70 dark:bg-error-900/35 dark:text-error-300"
        aria-hidden="true"
      >
        {parts.oldPrefix}
      </span>
      <span className="mx-1.5 text-brand-400" aria-hidden="true">
        →
      </span>
      <span
        className="rounded bg-success-100 px-1 py-0.5 text-success-700 dark:bg-success-900/35 dark:text-success-300"
        aria-hidden="true"
      >
        {parts.newPrefix}
      </span>
      <span aria-hidden="true">{parts.suffix}</span>
    </p>
  );
}

function groupChangesByGame(
  changes: service.GameLibraryPathChangeItem[],
): GamePathChangePreview[] {
  const groups = new Map<
    string,
    GamePathChangePreview & { representativePriority: number }
  >();

  for (const change of changes) {
    if (change.record_type !== "game")
      continue;

    const key = `game:${change.record_id}`;
    const priority = fieldPriority(change.field);
    const existing = groups.get(key);

    if (!existing) {
      groups.set(key, {
        key,
        name: change.record_name,
        oldPath: change.old_path,
        newPath: change.new_path,
        targetExists: change.target_exists,
        representativePriority: priority,
      });
      continue;
    }

    existing.targetExists = existing.targetExists && change.target_exists;
    if (priority < existing.representativePriority) {
      existing.oldPath = change.old_path;
      existing.newPath = change.new_path;
      existing.representativePriority = priority;
    }
  }

  return Array.from(
    groups.values(),
    ({ representativePriority: _, ...group }) => group,
  );
}

export function GameLibraryPathChangeModal({
  isApplying,
  onApply,
  onClose,
  preview,
}: GameLibraryPathChangeModalProps) {
  const { t } = useTranslation();
  const groupedChanges = useMemo(
    () => groupChangesByGame(preview?.changes ?? []),
    [preview?.changes],
  );
  const isBlocked = (preview?.blocking_download_task_count ?? 0) > 0;
  const hasChanges = groupedChanges.length > 0;
  const missingGameCount = groupedChanges.reduce(
    (count, change) => count + (change.targetExists ? 0 : 1),
    0,
  );
  const visibleChanges = groupedChanges.slice(0, MAX_VISIBLE_GAMES);
  const hiddenChangeCount = groupedChanges.length - visibleChanges.length;

  return (
    <Dialog
      open={preview !== null}
      onClose={() => {
        if (!isApplying)
          onClose();
      }}
      transition
      className="relative z-[9999]"
    >
      <DialogBackdrop
        transition
        className="fixed inset-0 bg-black/45 backdrop-blur-[3px] transition-opacity duration-250 ease-out data-closed:opacity-0 motion-reduce:duration-0"
      />

      <div className="fixed inset-0 overflow-y-auto p-4 sm:p-6">
        <div className="flex min-h-full items-center justify-center">
          <DialogPanel
            transition
            className="w-full max-w-3xl overflow-hidden rounded-2xl border border-brand-200 bg-white/98 shadow-2xl shadow-black/20 backdrop-blur-20 transition-[transform,opacity] duration-250 ease-out data-closed:scale-97 data-closed:opacity-0 dark:border-brand-700 dark:bg-brand-800/98 motion-reduce:duration-0"
          >
            {preview && (
              <>
                <header className="border-b border-brand-200 px-5 py-5 dark:border-brand-700 sm:px-6">
                  <div className="flex items-start gap-4">
                    <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-primary-200 bg-primary-50 text-primary-600 dark:border-primary-800 dark:bg-primary-900/35 dark:text-primary-300">
                      <span
                        className="i-mdi-folder-sync-outline text-2xl"
                        aria-hidden="true"
                      />
                    </div>
                    <div className="min-w-0 flex-1">
                      <DialogTitle className="text-lg font-semibold text-brand-900 dark:text-white">
                        {t("settings.basic.libraryChange.title")}
                      </DialogTitle>
                      <DialogDescription className="mt-1 text-sm text-brand-500 dark:text-brand-400">
                        {t(
                          "settings.basic.libraryChange.affectedGamesDescription",
                          { count: preview.affected_game_count },
                        )}
                      </DialogDescription>
                    </div>
                    <button
                      type="button"
                      aria-label={t("settings.basic.libraryChange.close")}
                      disabled={isApplying}
                      onClick={onClose}
                      className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-500 disabled:opacity-50 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white"
                    >
                      <span
                        className="i-mdi-close text-xl"
                        aria-hidden="true"
                      />
                    </button>
                  </div>
                </header>

                <div className="max-h-[58dvh] space-y-4 overflow-y-auto p-5 sm:p-6">
                  {isBlocked && (
                    <div className="flex items-start gap-3 rounded-xl border border-error-200 bg-error-50 px-4 py-3 text-error-700 dark:border-error-800/70 dark:bg-error-900/20 dark:text-error-300">
                      <span
                        className="i-mdi-download-lock mt-0.5 shrink-0 text-xl"
                        aria-hidden="true"
                      />
                      <p className="text-sm leading-6">
                        {t("settings.basic.libraryChange.downloadBlocked", {
                          count: preview.blocking_download_task_count,
                        })}
                      </p>
                    </div>
                  )}

                  {missingGameCount > 0 && (
                    <div className="flex items-start gap-3 rounded-xl border border-warning-200 bg-warning-50 px-4 py-3 text-warning-800 dark:border-warning-800/70 dark:bg-warning-900/20 dark:text-warning-300">
                      <span
                        className="i-mdi-folder-alert-outline mt-0.5 shrink-0 text-xl"
                        aria-hidden="true"
                      />
                      <p className="text-sm leading-6">
                        {t("settings.basic.libraryChange.missingWarning", {
                          count: missingGameCount,
                        })}
                      </p>
                    </div>
                  )}

                  {preview.steam_game_count > 0 && (
                    <div className="flex items-start gap-3 rounded-xl border border-info-200 bg-info-50 px-4 py-3 text-info-800 dark:border-info-800/70 dark:bg-info-900/20 dark:text-info-300">
                      <span
                        className="i-mdi-steam mt-0.5 shrink-0 text-xl"
                        aria-hidden="true"
                      />
                      <p className="text-sm leading-6">
                        {t("settings.basic.libraryChange.steamWarning", {
                          count: preview.steam_game_count,
                        })}
                      </p>
                    </div>
                  )}

                  {hasChanges ? (
                    <section className="space-y-2">
                      <h4 className="text-sm font-semibold text-brand-800 dark:text-brand-200">
                        {t("settings.basic.libraryChange.previewTitle")}
                      </h4>
                      <div className="overflow-hidden rounded-xl border border-brand-200 dark:border-brand-700">
                        {visibleChanges.map((change, index) => (
                          <div
                            key={change.key}
                            className={`grid gap-2 px-3.5 py-3 sm:grid-cols-[9rem_minmax(0,1fr)_auto] sm:items-center ${index > 0 ? "border-t border-brand-200 dark:border-brand-700" : ""}`}
                          >
                            <div className="min-w-0">
                              <p className="truncate text-sm font-medium text-brand-800 dark:text-brand-200">
                                {change.name}
                              </p>
                            </div>
                            <div className="min-w-0">
                              <PathPrefixDiff
                                oldPath={change.oldPath}
                                newPath={change.newPath}
                                oldLibraryPath={preview.old_library_path}
                                newLibraryPath={preview.new_library_path}
                              />
                            </div>
                            <span
                              className={`inline-flex w-fit items-center gap-1 rounded-full px-2 py-1 text-[11px] font-medium ${change.targetExists ? "bg-success-50 text-success-700 dark:bg-success-900/25 dark:text-success-300" : "bg-warning-50 text-warning-700 dark:bg-warning-900/25 dark:text-warning-300"}`}
                            >
                              <span
                                className={
                                  change.targetExists
                                    ? "i-mdi-check-circle"
                                    : "i-mdi-alert-circle-outline"
                                }
                                aria-hidden="true"
                              />
                            </span>
                          </div>
                        ))}
                      </div>
                      {hiddenChangeCount > 0 && (
                        <p className="text-xs text-brand-500 dark:text-brand-400">
                          {t("settings.basic.libraryChange.moreChanges", {
                            count: hiddenChangeCount,
                          })}
                        </p>
                      )}
                    </section>
                  ) : (
                    <div className="flex items-center gap-3 rounded-xl border border-brand-200 px-4 py-4 text-brand-600 dark:border-brand-700 dark:text-brand-300">
                      <span
                        className="i-mdi-check-circle-outline text-xl text-success-500"
                        aria-hidden="true"
                      />
                      <p className="text-sm">
                        {t("settings.basic.libraryChange.noAffectedRecords")}
                      </p>
                    </div>
                  )}
                </div>

                <footer className="flex justify-end gap-2 border-t border-brand-200 px-5 py-4 dark:border-brand-700 sm:px-6">
                  <BetterButton
                    variant="secondary"
                    disabled={isApplying}
                    onClick={() => onApply(false)}
                  >
                    {t("settings.basic.libraryChange.skipPathUpdate")}
                  </BetterButton>
                  <BetterButton
                    variant="primary"
                    disabled={isBlocked}
                    isLoading={isApplying}
                    icon="i-mdi-check"
                    onClick={() => onApply(true)}
                  >
                    {t("settings.basic.libraryChange.confirmChange")}
                  </BetterButton>
                </footer>
              </>
            )}
          </DialogPanel>
        </div>
      </div>
    </Dialog>
  );
}
