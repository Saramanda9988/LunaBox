import type { saveprobe } from "../../bindings/models";
import {
  Dialog,
  DialogBackdrop,
  DialogDescription,
  DialogPanel,
  DialogTitle,
} from "@headlessui/react";
import { useEffect, useRef, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  CancelSavePathProbe,
  StartSavePathProbe,
  StopSavePathProbe,
} from "../../../bindings/lunabox/internal/service/startservice";
import { useElapsedSeconds } from "../../hooks/useElapsedSeconds";
import { formatFileSize } from "../../utils/size";
import { BetterButton } from "../ui/better/BetterButton";

type ProbePhase = "intro" | "starting" | "probing" | "stopping" | "results";

interface SavePathProbeModalProps {
  gameID: string;
  gameName: string;
  isOpen: boolean;
  onClose: () => void;
  onSelect: (path: string) => void;
}

const confidenceClasses: Record<string, string> = {
  high: "bg-success-50 text-success-700 dark:bg-success-900/30 dark:text-success-300",
  medium:
    "bg-warning-50 text-warning-700 dark:bg-warning-900/30 dark:text-warning-300",
  low: "bg-brand-100 text-brand-600 dark:bg-brand-700 dark:text-brand-300",
};

export function SavePathProbeModal({
  gameID,
  gameName,
  isOpen,
  onClose,
  onSelect,
}: SavePathProbeModalProps) {
  const { t } = useTranslation();
  const [phase, setPhase] = useState<ProbePhase>("intro");
  const [startedAt, setStartedAt] = useState("");
  const [result, setResult] = useState<saveprobe.Result | null>(null);
  const [selectedPath, setSelectedPath] = useState("");
  const probingRef = useRef(false);
  const elapsedSeconds = useElapsedSeconds(startedAt, phase === "probing");
  const isBusy = phase === "starting" || phase === "stopping";
  const confidenceLabels: Record<string, string> = {
    high: t("savePathProbe.confidence.high"),
    medium: t("savePathProbe.confidence.medium"),
    low: t("savePathProbe.confidence.low"),
  };
  const signalLabels: Record<string, string> = {
    frequent_writes: t("savePathProbe.signals.frequent_writes"),
    recent_change: t("savePathProbe.signals.recent_change"),
    related_files: t("savePathProbe.signals.related_files"),
    save_like_extension: t("savePathProbe.signals.save_like_extension"),
    user_data_location: t("savePathProbe.signals.user_data_location"),
    game_directory: t("savePathProbe.signals.game_directory"),
    directory_change: t("savePathProbe.signals.directory_change"),
    incidental_activity: t("savePathProbe.signals.incidental_activity"),
  };

  useEffect(() => {
    return () => {
      if (probingRef.current) {
        probingRef.current = false;
        void CancelSavePathProbe(gameID);
      }
    };
  }, [gameID]);

  const handleStart = async () => {
    setPhase("starting");
    try {
      const status = await StartSavePathProbe(gameID);
      probingRef.current = true;
      setStartedAt(status.started_at);
      setPhase("probing");
    }
    catch (error) {
      console.error("Failed to start save path probe:", error);
      toast.error(t("savePathProbe.toast.startFailed"));
      setPhase("intro");
    }
  };

  const handleStop = async () => {
    setPhase("stopping");
    try {
      const probeResult = await StopSavePathProbe(gameID);
      probingRef.current = false;
      setResult(probeResult);
      setSelectedPath(probeResult.candidates[0]?.path ?? "");
      setPhase("results");
    }
    catch (error) {
      console.error("Failed to stop save path probe:", error);
      toast.error(t("savePathProbe.toast.stopFailed"));
      setPhase("probing");
    }
  };

  const handleClose = async () => {
    if (isBusy) {
      return;
    }
    if (probingRef.current) {
      probingRef.current = false;
      try {
        await CancelSavePathProbe(gameID);
      }
      catch (error) {
        console.error("Failed to cancel save path probe:", error);
      }
    }
    onClose();
  };

  const handleUsePath = () => {
    if (!selectedPath) {
      return;
    }
    onSelect(selectedPath);
    toast.success(t("savePathProbe.toast.pathSelected"));
    onClose();
  };

  return (
    <Dialog
      open={isOpen}
      onClose={() => void handleClose()}
      transition
      className="relative z-[9999]"
    >
      <DialogBackdrop
        transition
        className="fixed inset-0 bg-black/50 backdrop-blur-[3px] transition-opacity duration-250 data-closed:opacity-0 motion-reduce:duration-0"
      />
      <div className="fixed inset-0 overflow-y-auto p-4 sm:p-6">
        <div className="flex min-h-full items-center justify-center">
          <DialogPanel
            transition
            className="w-full max-w-2xl overflow-hidden rounded-2xl border border-brand-200 bg-white/98 shadow-2xl shadow-black/25 backdrop-blur-20 transition-[transform,opacity] duration-250 data-closed:scale-97 data-closed:opacity-0 dark:border-brand-700 dark:bg-brand-800/98 motion-reduce:duration-0"
          >
            <header className="border-b border-brand-200 px-5 py-5 dark:border-brand-700 sm:px-6">
              <div className="flex items-start gap-4">
                <div className="relative flex h-12 w-12 shrink-0 items-center justify-center rounded-xl border border-primary-200 bg-primary-50 text-primary-600 dark:border-primary-800 dark:bg-primary-900/35 dark:text-primary-300">
                  <span
                    className="i-mdi-file-eye-outline text-2xl"
                    aria-hidden="true"
                  />
                  {phase === "probing" && (
                    <span className="absolute -right-1 -top-1 h-3 w-3 rounded-full border-2 border-white bg-success-500 dark:border-brand-800">
                      <span className="absolute inset-0 animate-ping rounded-full bg-success-400 opacity-70" />
                    </span>
                  )}
                </div>
                <div className="min-w-0 flex-1">
                  <DialogTitle className="text-lg font-semibold text-brand-900 dark:text-white">
                    {t("savePathProbe.title")}
                  </DialogTitle>
                  <DialogDescription className="mt-1 text-sm leading-6 text-brand-500 dark:text-brand-400">
                    {t("savePathProbe.description", { name: gameName })}
                  </DialogDescription>
                </div>
                <button
                  type="button"
                  aria-label={t("common.close")}
                  disabled={isBusy}
                  onClick={() => void handleClose()}
                  className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-500 disabled:opacity-40 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white"
                >
                  <span className="i-mdi-close text-xl" aria-hidden="true" />
                </button>
              </div>
            </header>

            <div className="max-h-[62dvh] overflow-y-auto p-5 sm:p-6">
              {(phase === "intro" || phase === "starting") && (
                <div className="space-y-5">
                  <div className="rounded-xl border border-brand-200 bg-brand-50/80 p-4 dark:border-brand-700 dark:bg-brand-900/45">
                    <div className="grid gap-4 sm:grid-cols-[auto_1fr]">
                      <div className="flex h-9 w-9 items-center justify-center rounded-full bg-primary-100 text-sm font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">
                        1
                      </div>
                      <div>
                        <h4 className="text-sm font-semibold text-brand-800 dark:text-brand-200">
                          {t("savePathProbe.steps.startTitle")}
                        </h4>
                        <p className="mt-1 text-sm leading-6 text-brand-500 dark:text-brand-400">
                          {t("savePathProbe.steps.startDescription")}
                        </p>
                      </div>
                      <div className="flex h-9 w-9 items-center justify-center rounded-full bg-brand-200 text-sm font-bold text-brand-700 dark:bg-brand-700 dark:text-brand-200">
                        2
                      </div>
                      <div>
                        <h4 className="text-sm font-semibold text-brand-800 dark:text-brand-200">
                          {t("savePathProbe.steps.saveTitle")}
                        </h4>
                        <p className="mt-1 text-sm leading-6 text-brand-500 dark:text-brand-400">
                          {t("savePathProbe.steps.saveDescription")}
                        </p>
                      </div>
                      <div className="flex h-9 w-9 items-center justify-center rounded-full bg-brand-200 text-sm font-bold text-brand-700 dark:bg-brand-700 dark:text-brand-200">
                        3
                      </div>
                      <div>
                        <h4 className="text-sm font-semibold text-brand-800 dark:text-brand-200">
                          {t("savePathProbe.steps.finishTitle")}
                        </h4>
                        <p className="mt-1 text-sm leading-6 text-brand-500 dark:text-brand-400">
                          {t("savePathProbe.steps.finishDescription")}
                        </p>
                      </div>
                    </div>
                  </div>
                </div>
              )}

              {(phase === "probing" || phase === "stopping") && (
                <div className="flex flex-col items-center py-7 text-center">
                  <div className="relative flex h-28 w-28 items-center justify-center">
                    <span className="absolute inset-0 animate-pulse rounded-full bg-primary-100/80 dark:bg-primary-900/25" />
                    <span className="absolute inset-3 rounded-full border border-primary-200 dark:border-primary-800" />
                    <span
                      className="i-mdi-content-save-search-outline relative text-5xl text-primary-600 dark:text-primary-300"
                      aria-hidden="true"
                    />
                  </div>
                  <h4 className="mt-5 text-base font-semibold text-brand-900 dark:text-white">
                    {phase === "stopping"
                      ? t("savePathProbe.analyzing")
                      : t("savePathProbe.listening")}
                  </h4>
                  <p className="mt-2 max-w-md text-sm leading-6 text-brand-500 dark:text-brand-400">
                    {t("savePathProbe.listeningHint")}
                  </p>
                  <span className="mt-4 rounded-full bg-brand-100 px-3 py-1 font-mono text-xs text-brand-600 dark:bg-brand-700 dark:text-brand-300">
                    {t("savePathProbe.elapsed", { seconds: elapsedSeconds })}
                  </span>
                </div>
              )}

              {phase === "results" && result && (
                <div className="space-y-4">
                  <div className="flex items-center justify-between gap-4">
                    <div>
                      <h4 className="text-sm font-semibold text-brand-900 dark:text-white">
                        {t("savePathProbe.resultsTitle", {
                          count: result.candidates.length,
                        })}
                      </h4>
                      <p className="mt-1 text-xs text-brand-500 dark:text-brand-400">
                        {t("savePathProbe.eventSummary", {
                          observed: result.observed_events,
                        })}
                      </p>
                    </div>
                    <span className="shrink-0 rounded-full bg-primary-50 px-3 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
                      {t("savePathProbe.sources")}
                    </span>
                  </div>

                  {result.candidates.length === 0 ? (
                    <div className="rounded-xl border border-dashed border-brand-300 px-5 py-8 text-center dark:border-brand-600">
                      <span
                        className="i-mdi-file-question-outline text-4xl text-brand-400"
                        aria-hidden="true"
                      />
                      <p className="mt-3 text-sm font-medium text-brand-700 dark:text-brand-300">
                        {t("savePathProbe.noCandidates")}
                      </p>
                      <p className="mt-1 text-xs leading-5 text-brand-500 dark:text-brand-400">
                        {result.observed_events === 0
                          ? t("savePathProbe.noEventsHint")
                          : t("savePathProbe.noCandidatesHint")}
                      </p>
                    </div>
                  ) : (
                    <div className="space-y-2">
                      {result.candidates.map(candidate => (
                        <button
                          type="button"
                          key={`${candidate.kind}:${candidate.path}`}
                          onClick={() => setSelectedPath(candidate.path)}
                          className={`w-full rounded-xl border p-3.5 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-500 ${selectedPath === candidate.path ? "border-primary-400 bg-primary-50/70 dark:border-primary-600 dark:bg-primary-900/20" : "border-brand-200 hover:bg-brand-50 dark:border-brand-700 dark:hover:bg-brand-800"}`}
                        >
                          <div className="flex items-start gap-3">
                            <span
                              className={`${candidate.kind === "directory" ? "i-mdi-folder-star-outline" : "i-mdi-file-star-outline"} mt-0.5 shrink-0 text-xl text-primary-600 dark:text-primary-300`}
                              aria-hidden="true"
                            />
                            <div className="min-w-0 flex-1">
                              <div className="flex flex-wrap items-center gap-2">
                                <span
                                  className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${confidenceClasses[candidate.confidence] ?? confidenceClasses.low}`}
                                >
                                  {confidenceLabels[candidate.confidence]
                                    ?? confidenceLabels.low}
                                </span>
                                <span className="text-[11px] text-brand-500 dark:text-brand-400">
                                  {candidate.kind === "directory"
                                    ? t("savePathProbe.fileCount", {
                                        count: candidate.modified_files,
                                      })
                                    : formatFileSize(candidate.size ?? 0)}
                                </span>
                                <span className="text-[11px] text-brand-500 dark:text-brand-400">
                                  {t("savePathProbe.writeCount", {
                                    count: candidate.write_count,
                                  })}
                                </span>
                              </div>
                              <p className="mt-1.5 break-all font-mono text-xs leading-5 text-brand-800 dark:text-brand-200">
                                {candidate.path}
                              </p>
                              <div className="mt-2 flex flex-wrap gap-1.5">
                                {candidate.signals
                                  .filter(
                                    signal =>
                                      signal !== "incidental_activity",
                                  )
                                  .slice(0, 3)
                                  .map(signal => (
                                    <span
                                      key={signal}
                                      className="rounded-md bg-brand-100 px-1.5 py-0.5 text-[10px] text-brand-600 dark:bg-brand-700 dark:text-brand-300"
                                    >
                                      {signalLabels[signal] ?? signal}
                                    </span>
                                  ))}
                              </div>
                            </div>
                            <span
                              className={`${selectedPath === candidate.path ? "i-mdi-radiobox-marked text-primary-600 dark:text-primary-300" : "i-mdi-radiobox-blank text-brand-400"} shrink-0 text-xl`}
                              aria-hidden="true"
                            />
                          </div>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>

            <footer className="flex justify-end gap-2 border-t border-brand-200 px-5 py-4 dark:border-brand-700 sm:px-6">
              <BetterButton
                variant="secondary"
                disabled={isBusy}
                onClick={() => void handleClose()}
              >
                {phase === "results" ? t("common.close") : t("common.cancel")}
              </BetterButton>
              {(phase === "intro" || phase === "starting") && (
                <BetterButton
                  variant="primary"
                  icon="i-mdi-radar"
                  isLoading={phase === "starting"}
                  onClick={() => void handleStart()}
                >
                  {t("savePathProbe.start")}
                </BetterButton>
              )}
              {(phase === "probing" || phase === "stopping") && (
                <BetterButton
                  variant="primary"
                  icon="i-mdi-stop-circle-outline"
                  isLoading={phase === "stopping"}
                  onClick={() => void handleStop()}
                >
                  {t("savePathProbe.finish")}
                </BetterButton>
              )}
              {phase === "results"
                && result
                && result.candidates.length > 0 && (
                <BetterButton
                  variant="primary"
                  icon="i-mdi-check"
                  disabled={!selectedPath}
                  onClick={handleUsePath}
                >
                  {t("savePathProbe.useSelected")}
                </BetterButton>
              )}
            </footer>
          </DialogPanel>
        </div>
      </div>
    </Dialog>
  );
}
