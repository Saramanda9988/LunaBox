import { useCallback, useEffect, useState } from "react";

import type { vo } from "../bindings/models";

import { onWailsEvent } from "../bindings/runtime";

type SyncAllStatuses = () => Promise<vo.RemoteStatusSyncProgress>;

function createInitialProgress(provider: string): vo.RemoteStatusSyncProgress {
  return {
    provider,
    status: "started",
    current: 0,
    total: 0,
    game_name: "",
    succeeded_games: 0,
    failed_games: 0,
    failed_game_names: [],
    last_error: "",
  } as vo.RemoteStatusSyncProgress;
}

export function useRemoteStatusSync(
  provider: string,
  eventName: string,
  syncAllStatuses: SyncAllStatuses,
) {
  const [progress, setProgress] = useState<vo.RemoteStatusSyncProgress>(() =>
    createInitialProgress(provider),
  );
  const [isSyncing, setIsSyncing] = useState(false);
  const [isProgressOpen, setIsProgressOpen] = useState(false);

  useEffect(
    () =>
      onWailsEvent<vo.RemoteStatusSyncProgress>(eventName, (nextProgress) => {
        setProgress(nextProgress);
        setIsProgressOpen(true);
        if (
          nextProgress.status === "done"
          || nextProgress.status === "failed"
        ) {
          setIsSyncing(false);
        }
      }),
    [eventName],
  );

  const startSync = useCallback(async () => {
    setProgress(createInitialProgress(provider));
    setIsProgressOpen(true);
    setIsSyncing(true);
    try {
      const result = await syncAllStatuses();
      setProgress(result);
    }
    catch (error) {
      setProgress(current => ({
        ...current,
        status: "failed",
        last_error: error instanceof Error ? error.message : String(error),
      }));
    }
    finally {
      setIsSyncing(false);
    }
  }, [provider, syncAllStatuses]);

  const closeProgress = useCallback(() => {
    if (!isSyncing) {
      setIsProgressOpen(false);
    }
  }, [isSyncing]);

  return {
    closeProgress,
    isProgressOpen,
    isSyncing,
    progress,
    startSync,
  };
}
