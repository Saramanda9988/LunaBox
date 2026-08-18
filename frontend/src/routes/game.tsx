import type { models, service, vo } from "../../src/bindings/models";
import { createRoute, useNavigate } from "@tanstack/react-router";
import { Browser } from "@wailsio/runtime";
import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  AddGameToCategory,
  GetCategories,
  GetCategoriesByGame,
  RemoveGameFromCategory,
} from "../../bindings/lunabox/internal/service/categoryservice";
import {
  DeleteGame,
  DeleteGameMetadataSource,
  ExportLaunchShortcut,
  FetchMetadataByName,
  GetGameByID,
  OpenLocalPath,
  SelectCoverImage,
  SelectGameDirectory,
  SelectGameExecutable,
  SelectSaveDirectory,
  SelectSaveFile,
  SetDefaultMetadataSource,
  UpdateGame,
  UpdateGameFromRemoteBySource,
  UpdateGameFromRemoteWithFields,
  UpsertGameMetadataSource,
} from "../../bindings/lunabox/internal/service/gameservice";
import {
  GetGameSteamStatus,
  ImportGameToSteam,
} from "../../bindings/lunabox/internal/service/integrationservice";
import { GetTagsByGame } from "../../bindings/lunabox/internal/service/tagservice";
import { enums } from "../../src/bindings/models";
import { SetGameSteamLaunchOptions } from "../bindings/integration";
import {
  cacheGameUpdate,
  invalidateAllGameLists,
  invalidateCategoryGameLists,
  removeGamesFromCache,
} from "../cache/gameCache";
import { AddToCategoryModal } from "../components/modal/AddToCategoryModal";
import { ConfirmModal } from "../components/modal/ConfirmModal";
import {
  DEFAULT_METADATA_UPDATE_FIELDS,
  MetadataFieldSelectModal,
} from "../components/modal/MetadataFieldSelectModal";
import { MetadataSourceSearchModal } from "../components/modal/MetadataSourceSearchModal";
import { ProcessSelectModal } from "../components/modal/ProcessSelectModal";
import { SteamImportModal } from "../components/modal/SteamImportModal";
import { GameBackupPanel } from "../components/panel/GameBackupPanel";
import { GameEditPanel } from "../components/panel/GameEditPanel";
import { GameLaunchPanel } from "../components/panel/GameLaunchPanel";
import { GameProgressPanel } from "../components/panel/GameProgressPanel";
import { GameReviewPanel } from "../components/panel/GameReviewPanel";
import { GameStatsPanel } from "../components/panel/GameStatsPanel";
import { GameDetailSkeleton } from "../components/skeleton/GameDetailSkeleton";
import { BetterDropdownMenu } from "../components/ui/better/BetterDropdownMenu";
import { BetterSplitButton } from "../components/ui/better/BetterSplitButton";
import { GameCoverImage } from "../components/ui/GameCoverImage";
import { GameTags } from "../components/ui/GameTags";
import { sourceLabel } from "../components/ui/import/importFlow";
import { useAppStore } from "../store";
import {
  getMetadataSourceIcon,
  getMetadataSourceURL,
} from "../utils/metadataSources";
import { formatLocalDate } from "../utils/time";
import { Route as rootRoute } from "./__root";

type LaunchMode = enums.LaunchMode | "admin";
type SteamPendingAction = "save-default" | "launch";

function defaultLaunchModeForGame(game: models.Game): enums.LaunchMode {
  if (game.launch_mode === enums.LaunchMode.LaunchModeSteam) {
    return enums.LaunchMode.LaunchModeSteam;
  }
  if (game.launch_mode === enums.LaunchMode.LaunchModeCompatibility) {
    return enums.LaunchMode.LaunchModeCompatibility;
  }
  return enums.LaunchMode.LaunchModeNormal;
}

function gameWithSteamStatus(
  game: models.Game,
  status: service.SteamLaunchStatus,
): models.Game {
  const protonPrefix = (
    status as service.SteamLaunchStatus & {
      proton_prefix?: string;
    }
  ).proton_prefix;
  return {
    ...game,
    steam_launch_id: status.launch_id,
    steam_launch_kind: status.launch_kind,
    steam_user_id: status.user_id,
    wine_prefix: game.wine_prefix || protonPrefix || "",
  } as models.Game;
}

type GameWithSteamSettings = models.Game & {
  steam_launch_options?: string;
};

function mergeSteamSettings(
  game: models.Game,
  refreshedGame: models.Game,
): models.Game {
  const refreshed = refreshedGame as GameWithSteamSettings;
  return {
    ...game,
    steam_launch_id: refreshedGame.steam_launch_id,
    steam_launch_kind: refreshedGame.steam_launch_kind,
    steam_launch_options: refreshed.steam_launch_options || "",
    steam_user_id: refreshedGame.steam_user_id,
    use_locale_emulator: refreshedGame.use_locale_emulator,
    wine_prefix: refreshedGame.wine_prefix,
  } as GameWithSteamSettings as models.Game;
}

function gameWithUpdatedMetadataSourceID(
  game: models.Game,
  source: enums.SourceType,
  sourceID: string,
): models.Game {
  const shouldBecomeDefault
    = !game.source_type
      || game.source_type === enums.SourceType.Local
      || game.source_type === source;

  return {
    ...game,
    source_type: shouldBecomeDefault ? source : game.source_type,
    source_id: shouldBecomeDefault ? sourceID : game.source_id,
    metadata_sources: (game.metadata_sources ?? []).map(metadataSource =>
      metadataSource.source_type === source
        ? { ...metadataSource, source_id: sourceID }
        : metadataSource,
    ),
  } as models.Game;
}

function isManagedLocalCoverURL(coverURL: string): boolean {
  return (
    coverURL.startsWith("/local/covers/")
    || /^https?:\/\/wails\.localhost(?::\d+)?\/local\/covers\//.test(coverURL)
  );
}

function buildCoverImageSrc(coverURL: string, refreshKey: string): string {
  if (!isManagedLocalCoverURL(coverURL)) {
    return coverURL;
  }

  const separator = coverURL.includes("?") ? "&" : "?";
  return `${coverURL}${separator}v=${encodeURIComponent(refreshKey)}`;
}

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/game/$gameId",
  component: GameDetailPage,
});

function GameDetailPage() {
  const navigate = useNavigate();
  const { gameId } = Route.useParams();
  const config = useAppStore(state => state.config);
  const platformGOOS = useAppStore(state => state.platformGOOS);
  const startGame = useAppStore(state => state.startGame);
  const fetchHomeData = useAppStore(state => state.fetchHomeData);
  const gameRuntime = useAppStore(state => state.gameRuntimes[gameId]);
  const { t } = useTranslation();
  const [game, setGame] = useState<models.Game | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [showSkeleton, setShowSkeleton] = useState(false);
  const [activeTab, setActiveTab] = useState(() =>
    window.location.hash === "#launch" ? "launch" : "stats",
  );
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [isCategoryModalOpen, setIsCategoryModalOpen] = useState(false);
  const [isProcessSelectModalOpen, setIsProcessSelectModalOpen]
    = useState(false);
  const [isMetadataFieldModalOpen, setIsMetadataFieldModalOpen]
    = useState(false);
  const [isMetadataSearchModalOpen, setIsMetadataSearchModalOpen]
    = useState(false);
  const [isApplyingMetadataResult, setIsApplyingMetadataResult]
    = useState(false);
  const [metadataSearchResults, setMetadataSearchResults] = useState<
    vo.GameMetadataFromWebVO[]
  >([]);
  const [isUpdatingFromRemote, setIsUpdatingFromRemote] = useState(false);
  const [isSteamModalOpen, setIsSteamModalOpen] = useState(false);
  const [isCheckingSteam, setIsCheckingSteam] = useState(false);
  const [isImportingSteam, setIsImportingSteam] = useState(false);
  const [steamStatus, setSteamStatus]
    = useState<service.SteamLaunchStatus | null>(null);
  const [selectedMetadataFields, setSelectedMetadataFields] = useState<
    enums.MetadataUpdateField[]
  >(DEFAULT_METADATA_UPDATE_FIELDS);
  const [allCategories, setAllCategories] = useState<vo.CategoryVO[]>([]);
  const [selectedCategoryIds, setSelectedCategoryIds] = useState<string[]>([]);
  const [initialTags, setInitialTags] = useState<models.GameTag[]>([]);
  const [tagRefreshToken, setTagRefreshToken] = useState(0);
  const [launchMode, setLaunchMode] = useState<LaunchMode>(
    enums.LaunchMode.LaunchModeNormal,
  );
  const [coverImageRefreshToken, setCoverImageRefreshToken] = useState(() =>
    Date.now(),
  );
  const isInitialMount = useRef(true);
  const pendingSteamAction = useRef<SteamPendingAction | null>(null);
  const originalGameData = useRef<models.Game | null>(null);
  const latestGameData = useRef<models.Game | null>(null);
  latestGameData.current = game;
  const supportsAdminLaunch = platformGOOS === "windows";
  const supportsSteamLaunch
    = platformGOOS === "windows"
      || platformGOOS === "linux"
      || (platformGOOS === "darwin"
        && game?.steam_launch_kind === "native"
        && Boolean(game?.steam_launch_id));

  const updateGameState = useCallback(
    (
      updatedGame: models.Game,
      options: { forceListInvalidation?: boolean } = {},
    ) => {
      const previousGame = latestGameData.current;
      latestGameData.current = updatedGame;
      setGame(updatedGame);
      cacheGameUpdate(previousGame, updatedGame, options);
    },
    [],
  );

  const navigateToLibrary = () => {
    navigate({ to: "/library" });
  };

  const searchLibraryByCompany = (company: string) => {
    navigate({
      to: "/library",
      search: { searchQuery: company },
    });
  };

  useEffect(() => {
    let isCurrent = true;

    const loadData = async () => {
      try {
        const gameDataPromise = GetGameByID(gameId);
        const gameTagsPromise = GetTagsByGame(gameId).catch(() => []);
        const [gameData, gameTags] = await Promise.all([
          gameDataPromise,
          gameTagsPromise,
        ]);
        if (!isCurrent) {
          return;
        }

        updateGameState(gameData);
        setInitialTags(gameTags ?? []);
        setLaunchMode(defaultLaunchModeForGame(gameData));
        originalGameData.current = gameData;
        isInitialMount.current = false;
      }
      catch (error) {
        if (!isCurrent) {
          return;
        }
        console.error("Failed to load game data:", error);
        toast.error(t("game.toast.loadDataFailed"));
      }
      finally {
        if (isCurrent) {
          setIsLoading(false);
        }
      }
    };
    void loadData();

    return () => {
      isCurrent = false;
    };
  }, [gameId, t, updateGameState]);

  useEffect(() => {
    const syncTabFromHash = () => {
      if (window.location.hash === "#launch") {
        setActiveTab("launch");
      }
    };
    syncTabFromHash();
    window.addEventListener("hashchange", syncTabFromHash);
    return () => window.removeEventListener("hashchange", syncTabFromHash);
  }, []);

  useEffect(() => {
    if (!game) {
      return;
    }
    setLaunchMode(defaultLaunchModeForGame(game));
  }, [game?.id, game?.launch_mode]);

  // 延迟显示骨架屏
  useEffect(() => {
    let timer: number;
    if (isLoading) {
      timer = window.setTimeout(() => {
        setShowSkeleton(true);
      }, 300);
    }
    else {
      setShowSkeleton(false);
    }
    return () => clearTimeout(timer);
  }, [isLoading]);

  // 自动保存
  useEffect(() => {
    if (!game || isInitialMount.current)
      return;

    const hasChanges
      = JSON.stringify(game) !== JSON.stringify(originalGameData.current);
    if (!hasChanges)
      return;

    const timer = setTimeout(async () => {
      try {
        await UpdateGame(game);
        originalGameData.current = game;
      }
      catch (error) {
        invalidateAllGameLists();
        console.error("Failed to auto-save game:", error);
        toast.error(
          t("game.toast.saveFailed", { error: (error as Error).message }),
        );
      }
    }, 500);

    return () => clearTimeout(timer);
  }, [game, t]);

  // 路由卸载会取消上面的防抖定时器，离开前补交尚未保存的修改。
  useEffect(() => {
    return () => {
      const latestGame = latestGameData.current;
      if (
        !latestGame
        || isInitialMount.current
        || JSON.stringify(latestGame) === JSON.stringify(originalGameData.current)
      ) {
        return;
      }

      void UpdateGame(latestGame).catch((error) => {
        invalidateAllGameLists();
        console.error("Failed to flush game changes before leaving:", error);
      });
    };
  }, []);

  if (isLoading && !game) {
    if (!showSkeleton) {
      return null;
    }
    return <GameDetailSkeleton />;
  }

  if (!game) {
    return (
      <div className="flex flex-col items-center justify-center h-full space-y-4 text-brand-500">
        <div className="i-mdi-gamepad-variant-outline text-6xl" />
        <p className="text-xl">{t("game.notFound")}</p>
        <button
          type="button"
          onClick={navigateToLibrary}
          className="text-neutral-600 hover:underline"
        >
          {t("game.backToLibrary")}
        </button>
      </div>
    );
  }

  const handleSelectExecutable = async () => {
    try {
      const path = await SelectGameExecutable(game.path || "");
      if (path && game) {
        updateGameState({ ...game, path } as models.Game);
      }
    }
    catch (error) {
      console.error("Failed to select executable:", error);
      toast.error(t("game.toast.selectExecutableFailed"));
    }
  };

  const handleSelectGameDirectory = async () => {
    try {
      const path = await SelectGameDirectory(
        game.game_directory || game.path || "",
      );
      if (path && game) {
        updateGameState({ ...game, game_directory: path } as models.Game);
      }
    }
    catch (error) {
      console.error("Failed to select game directory:", error);
      toast.error(t("game.toast.selectGameDirFailed"));
    }
  };

  const handleDeleteGame = async () => {
    if (!game)
      return;
    setIsDeleteModalOpen(true);
  };

  const confirmDeleteGame = async () => {
    if (!game)
      return;
    try {
      await DeleteGame(game.id);
      removeGamesFromCache([game.id]);
      void fetchHomeData({ showLoading: false, syncRuntime: false });
      toast.success(t("game.toast.deleteSuccess"));
      navigateToLibrary();
    }
    catch (error) {
      console.error("Failed to delete game:", error);
      toast.error(t("game.toast.deleteFailed"));
    }
  };

  const handleSelectSaveDirectory = async () => {
    try {
      const path = await SelectSaveDirectory();
      if (path && game) {
        updateGameState({ ...game, save_path: path } as models.Game);
      }
    }
    catch (error) {
      console.error("Failed to select save directory:", error);
      toast.error(t("game.toast.selectSaveDirFailed"));
    }
  };

  const handleSelectSaveFile = async () => {
    try {
      const path = await SelectSaveFile();
      if (path && game) {
        updateGameState({ ...game, save_path: path } as models.Game);
      }
    }
    catch (error) {
      console.error("Failed to select save file:", error);
      toast.error(t("game.toast.selectSaveFileFailed"));
    }
  };

  const handleSelectCoverImage = async () => {
    if (!game)
      return;
    try {
      const coverUrl = await SelectCoverImage(game.id);
      if (coverUrl) {
        updateGameState({ ...game, cover_url: coverUrl } as models.Game);
        setCoverImageRefreshToken(prev => prev + 1);
      }
    }
    catch (error) {
      console.error("Failed to select cover image:", error);
      toast.error(t("game.toast.selectCoverFailed"));
    }
  };

  const handleOpenUpdateFromRemote = () => {
    if (!game || game.metadata_locked)
      return;
    setIsMetadataFieldModalOpen(true);
  };

  const handleUpdateFromRemote = async (
    fields: enums.MetadataUpdateField[],
  ) => {
    if (!game)
      return;
    const updateFields
      = fields.length > 0 ? fields : DEFAULT_METADATA_UPDATE_FIELDS;
    setSelectedMetadataFields(updateFields);
    setIsMetadataFieldModalOpen(false);
    setIsUpdatingFromRemote(true);
    try {
      await UpdateGameFromRemoteWithFields(game.id, updateFields);
      const updatedGame = await GetGameByID(game.id);
      updateGameState(updatedGame, { forceListInvalidation: true });
      originalGameData.current = updatedGame;
      setTagRefreshToken(prev => prev + 1);
      setCoverImageRefreshToken(prev => prev + 1);
      toast.success(t("game.toast.updateRemoteSuccess"));
    }
    catch (error) {
      console.error("Failed to update from remote:", error);
      toast.error(t("game.toast.updateRemoteFailed", { error }));
    }
    finally {
      setIsUpdatingFromRemote(false);
    }
  };

  const refreshGameAfterMetadataSourceChange = async () => {
    if (!game)
      return;
    const updatedGame = await GetGameByID(game.id);
    updateGameState(updatedGame, { forceListInvalidation: true });
    originalGameData.current = updatedGame;
  };

  const handleUpsertMetadataSource = async (
    source: enums.SourceType,
    sourceID: string,
  ) => {
    if (!game)
      return;
    await UpsertGameMetadataSource(game.id, source, sourceID);
    await refreshGameAfterMetadataSourceChange();
    toast.success(t("gameEdit.sourceSaved"));
  };

  const handleDeleteMetadataSource = async (source: enums.SourceType) => {
    if (!game)
      return;
    await DeleteGameMetadataSource(game.id, source);
    await refreshGameAfterMetadataSourceChange();
    toast.success(t("gameEdit.sourceDeleted"));
  };

  const handleSetDefaultMetadataSource = async (source: enums.SourceType) => {
    if (!game)
      return;
    await SetDefaultMetadataSource(game.id, source);
    await refreshGameAfterMetadataSourceChange();
    toast.success(t("gameEdit.defaultSourceSaved"));
  };

  const handleAutoSaveMetadataSource = async (
    source: enums.SourceType,
    sourceID: string,
  ) => {
    if (!game)
      return;

    await UpsertGameMetadataSource(game.id, source, sourceID);
    const latestGame = latestGameData.current;
    if (!latestGame)
      return;

    const updatedGame = gameWithUpdatedMetadataSourceID(
      latestGame,
      source,
      sourceID,
    );
    if (originalGameData.current) {
      originalGameData.current = gameWithUpdatedMetadataSourceID(
        originalGameData.current,
        source,
        sourceID,
      );
    }
    updateGameState(updatedGame);
  };

  const handleSearchMetadataByName = async () => {
    if (!game)
      return false;

    setMetadataSearchResults([]);
    try {
      const results = await FetchMetadataByName(game.name.trim());
      setMetadataSearchResults(results || []);
      setIsMetadataSearchModalOpen(true);
      return true;
    }
    catch (error) {
      console.error("Failed to search metadata by game name:", error);
      toast.error(t("gameEdit.metadataSearchFailed", { error }));
      return false;
    }
  };

  const handleSelectMetadataSearchResult = async (
    result: vo.GameMetadataFromWebVO,
  ) => {
    const sourceID = result.Game?.source_id?.trim();
    if (!game || !sourceID)
      return;

    const seenSources = new Set<enums.SourceType>();
    const sourcesToLink: Array<{
      source: enums.SourceType;
      sourceID: string;
    }> = [];
    for (const item of metadataSearchResults) {
      const itemSourceID = item.Game?.source_id?.trim();
      if (!item.Game || !itemSourceID)
        continue;

      if (seenSources.has(item.Source)) {
        toast.error(
          t("addGameModal.toast.duplicateSource", {
            source: sourceLabel(item.Source, t),
          }),
        );
        return;
      }
      seenSources.add(item.Source);
      sourcesToLink.push({ source: item.Source, sourceID: itemSourceID });
    }
    if (sourcesToLink.length === 0)
      return;

    setIsApplyingMetadataResult(true);
    try {
      for (const source of sourcesToLink) {
        await UpsertGameMetadataSource(game.id, source.source, source.sourceID);
      }
      await UpdateGameFromRemoteBySource(game.id, result.Source);
      await SetDefaultMetadataSource(game.id, result.Source);
      await refreshGameAfterMetadataSourceChange();
      setTagRefreshToken(prev => prev + 1);
      setCoverImageRefreshToken(prev => prev + 1);
      setIsMetadataSearchModalOpen(false);
      toast.success(t("gameEdit.metadataSearchApplySuccess"));
    }
    catch (error) {
      console.error("Failed to apply metadata search result:", error);
      toast.error(t("gameEdit.metadataSearchApplyFailed", { error }));
    }
    finally {
      setIsApplyingMetadataResult(false);
    }
  };

  const statusConfig = {
    [enums.GameStatus.StatusNotStarted]: {
      label: t("common.notStarted"),
      icon: "i-mdi-clock-outline",
      color: "bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300",
    },
    [enums.GameStatus.StatusWantToPlay]: {
      label: t("common.wantToPlay"),
      icon: "i-mdi-bookmark-outline",
      color: "bg-info-100 text-info-700 dark:bg-info-900 dark:text-info-300",
    },
    [enums.GameStatus.StatusPlaying]: {
      label: t("common.playing"),
      icon: "i-mdi-gamepad-variant",
      color:
        "bg-neutral-100 text-neutral-700 dark:bg-neutral-900 dark:text-neutral-300",
    },
    [enums.GameStatus.StatusCompleted]: {
      label: t("common.completed"),
      icon: "i-mdi-trophy",
      color:
        "bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300",
    },
    [enums.GameStatus.StatusOnHold]: {
      label: t("common.onHold"),
      icon: "i-mdi-pause-circle-outline",
      color:
        "bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300",
    },
  };

  const performStartGame = async (
    targetGame: models.Game,
    mode: LaunchMode,
  ) => {
    const effectiveMode
      = mode === "admin" && !supportsAdminLaunch
        ? enums.LaunchMode.LaunchModeNormal
        : mode;
    try {
      const started
        = effectiveMode === "admin"
          ? await startGame(targetGame, { RunAsAdmin: true, UseSteam: false })
          : effectiveMode === enums.LaunchMode.LaunchModeSteam
            ? await startGame(targetGame, { UseSteam: true })
            : mode === enums.LaunchMode.LaunchModeCompatibility
              ? await startGame(targetGame, {
                  UseSteam: false,
                  UseCompatibility: true,
                })
              : await startGame(targetGame, {
                  UseSteam: false,
                  UseCompatibility: false,
                });
      if (started) {
        try {
          const updatedGame = await GetGameByID(targetGame.id);
          updateGameState(updatedGame);
          originalGameData.current = updatedGame;
        }
        catch (refreshError) {
          console.error("Failed to refresh game after start:", refreshError);
        }
        // toast.success(t("gameCard.startSuccess", { name: game.name }));
      }
      else {
        toast.error(
          t("gameCard.startFailedNotLaunched", { name: targetGame.name }),
        );
      }
    }
    catch (error) {
      console.error("Failed to start game:", error);
      toast.error(t("gameCard.startFailedLog", { name: targetGame.name }));
    }
  };

  const resolveSteamGame = async (
    action: SteamPendingAction,
    targetGame: models.Game = game,
  ): Promise<models.Game | null> => {
    pendingSteamAction.current = action;
    setIsCheckingSteam(true);
    try {
      const status = await GetGameSteamStatus(targetGame.id);
      setSteamStatus(status);
      if (!status.ready) {
        setIsSteamModalOpen(true);
        return null;
      }
      return gameWithSteamStatus(targetGame, status);
    }
    catch (error) {
      console.error("Failed to inspect Steam launch status:", error);
      pendingSteamAction.current = null;
      toast.error(t("steamImport.checkFailed", { error }));
      return null;
    }
    finally {
      setIsCheckingSteam(false);
    }
  };

  const saveSteamAsDefault = async (targetGame: models.Game) => {
    const updatedGame = {
      ...targetGame,
      launch_mode: enums.LaunchMode.LaunchModeSteam,
    } as models.Game;
    try {
      await UpdateGame(updatedGame);
      updateGameState(updatedGame);
      originalGameData.current = updatedGame;
      setLaunchMode(enums.LaunchMode.LaunchModeSteam);
    }
    catch (error) {
      console.error("Failed to save Steam as default launch mode:", error);
      toast.error(
        t("game.toast.saveFailed", { error: (error as Error).message }),
      );
    }
  };

  const steamLaunchOptionsStatusError = (status: service.SteamLaunchStatus) => {
    switch (status.state) {
      case "steam_running":
        return t("gameLaunch.steamLaunchOptionsSteamRunning");
      case "steam_not_installed":
        return t("gameLaunch.steamProtonNotInstalled");
      case "executable_required":
        return t("steamImport.executableRequired");
      case "user_unavailable":
        return t("steamImport.userUnavailable");
      default:
        return t("gameLaunch.steamLaunchOptionsNotReady", {
          state: status.state || "unknown",
        });
    }
  };

  const handleRefreshSteamSettings = async () => {
    if (!game) {
      return;
    }

    const refreshedGame = await GetGameByID(game.id);
    const mergedGame = mergeSteamSettings(game, refreshedGame);
    updateGameState(mergedGame);
    originalGameData.current = originalGameData.current
      ? mergeSteamSettings(originalGameData.current, refreshedGame)
      : refreshedGame;
  };

  const handleSaveSteamLaunchOptions = async (launchOptions: string) => {
    if (!game) {
      return;
    }

    const status = await SetGameSteamLaunchOptions(game.id, launchOptions);
    setSteamStatus(status);
    if (!status.ready) {
      throw new Error(steamLaunchOptionsStatusError(status));
    }

    const refreshedGame = await GetGameByID(game.id);
    const mergedGame = mergeSteamSettings(game, refreshedGame);
    updateGameState(mergedGame);
    originalGameData.current = originalGameData.current
      ? mergeSteamSettings(originalGameData.current, refreshedGame)
      : refreshedGame;
    return status;
  };

  const handleDefaultLaunchModeChange = async (mode: enums.LaunchMode) => {
    if (mode !== enums.LaunchMode.LaunchModeSteam) {
      updateGameState({
        ...game,
        launch_mode: mode,
        wine_runner:
          mode === enums.LaunchMode.LaunchModeCompatibility
            ? game.wine_runner === "crossover"
              ? "crossover"
              : "system"
            : game.wine_runner,
      } as models.Game);
      return;
    }

    const steamGame = await resolveSteamGame("save-default");
    if (!steamGame)
      return;
    pendingSteamAction.current = null;
    await saveSteamAsDefault(steamGame);
  };

  const handleStartGame = async (mode: LaunchMode = launchMode) => {
    if (!game || !game.id || gameRuntime)
      return;

    if (mode === enums.LaunchMode.LaunchModeSteam) {
      const steamGame = await resolveSteamGame("launch");
      if (!steamGame)
        return;
      pendingSteamAction.current = null;
      await performStartGame(steamGame, mode);
      return;
    }

    await performStartGame(game, mode);
  };

  const handleRetrySteamStatus = async () => {
    const action = pendingSteamAction.current;
    if (!action)
      return;
    const steamGame = await resolveSteamGame(action);
    if (!steamGame)
      return;

    setIsSteamModalOpen(false);
    pendingSteamAction.current = null;
    if (action === "save-default") {
      await saveSteamAsDefault(steamGame);
    }
    else {
      await performStartGame(steamGame, enums.LaunchMode.LaunchModeSteam);
    }
  };

  const handleImportGameToSteam = async () => {
    const action = pendingSteamAction.current;
    if (!action)
      return;

    setIsImportingSteam(true);
    try {
      const result = await ImportGameToSteam(game.id);
      setSteamStatus(result.status);
      if (!result.status.ready) {
        return;
      }

      const steamGame = gameWithSteamStatus(game, result.status);
      setIsSteamModalOpen(false);
      pendingSteamAction.current = null;
      if (action === "save-default") {
        await saveSteamAsDefault(steamGame);
      }
      else {
        setLaunchMode(enums.LaunchMode.LaunchModeSteam);
      }

      if (result.imported) {
        toast.success(t("steamImport.importSuccess"), { duration: 6000 });
      }
      else if (action === "launch") {
        await performStartGame(steamGame, enums.LaunchMode.LaunchModeSteam);
      }
    }
    catch (error) {
      console.error("Failed to import game into Steam:", error);
      toast.error(t("steamImport.importFailed", { error }));
    }
    finally {
      setIsImportingSteam(false);
    }
  };

  const handleSteamSelectExecutable = async () => {
    try {
      const path = await SelectGameExecutable(game.path || "");
      if (!path)
        return;
      const updatedGame = { ...game, path } as models.Game;
      await UpdateGame(updatedGame);
      updateGameState(updatedGame);
      originalGameData.current = updatedGame;
      const action = pendingSteamAction.current;
      if (action) {
        const steamGame = await resolveSteamGame(action, updatedGame);
        if (!steamGame)
          return;
        setIsSteamModalOpen(false);
        pendingSteamAction.current = null;
        if (action === "save-default") {
          await saveSteamAsDefault(steamGame);
        }
        else {
          await performStartGame(steamGame, enums.LaunchMode.LaunchModeSteam);
        }
      }
    }
    catch (error) {
      console.error("Failed to select executable for Steam:", error);
      toast.error(t("game.toast.selectExecutableFailed"));
    }
  };

  const handleCloseSteamModal = () => {
    setIsSteamModalOpen(false);
    pendingSteamAction.current = null;
  };

  const handleStatusChange = async (newStatus: string) => {
    if (
      !game
      || (game.status || enums.GameStatus.StatusNotStarted) === newStatus
    ) {
      return;
    }
    const updatedGame = { ...game, status: newStatus } as models.Game;
    updateGameState(updatedGame);
    try {
      await UpdateGame(updatedGame);
      toast.success(t("game.toast.statusUpdated"));
    }
    catch (error) {
      console.error("Failed to update status:", error);
      toast.error(t("game.toast.statusUpdateFailed"));
    }
  };

  const openCategoryModal = async () => {
    try {
      const [categories, gameCategories] = await Promise.all([
        GetCategories(),
        GetCategoriesByGame(gameId),
      ]);
      setAllCategories(categories || []);
      setSelectedCategoryIds(gameCategories?.map(c => c.id) || []);
      setIsCategoryModalOpen(true);
    }
    catch (error) {
      console.error("Failed to load categories:", error);
      toast.error(t("game.toast.loadFavFailed"));
    }
  };

  const handleSaveCategories = async (newSelectedIds: string[]) => {
    const currentIds = selectedCategoryIds;

    // 计算需要添加的和移除的
    const toAdd = newSelectedIds.filter(id => !currentIds.includes(id));
    const toRemove = currentIds.filter(id => !newSelectedIds.includes(id));

    try {
      // 执行添加操作
      for (const categoryId of toAdd) {
        await AddGameToCategory(gameId, categoryId);
      }
      // 执行移除操作
      for (const categoryId of toRemove) {
        await RemoveGameFromCategory(gameId, categoryId);
      }

      setSelectedCategoryIds(newSelectedIds);

      // 刷新所有分类的game_count
      const categories = await GetCategories();
      setAllCategories(categories || []);

      if (toAdd.length > 0 || toRemove.length > 0) {
        invalidateCategoryGameLists();
        toast.success(t("game.toast.favUpdated"));
      }
    }
    catch (error) {
      console.error("Failed to update categories:", error);
      toast.error(t("game.toast.updateFavFailed"));
    }
  };

  const handleSelectProcessExecutable = async () => {
    try {
      const path = await SelectGameExecutable(game.path || "");
      if (path && game) {
        // 从路径中提取文件名
        const filename = path.split(/[\\/]/).pop();
        if (filename) {
          updateGameState({ ...game, process_name: filename } as models.Game);
        }
      }
    }
    catch (error) {
      console.error("Failed to select executable:", error);
      toast.error(t("game.toast.selectFileFailed"));
    }
  };

  const handleRunningProcessSelected = (processName: string) => {
    if (!game)
      return;
    updateGameState({ ...game, process_name: processName } as models.Game);
  };

  const handleExportLaunchShortcut = async () => {
    if (!game)
      return;
    try {
      const savePath = await ExportLaunchShortcut(game.id);
      if (!savePath) {
        return;
      }
      toast.success(
        t("gameLaunch.toast.shortcutExportSuccess", { path: savePath }),
      );
    }
    catch (error) {
      console.error("Failed to export launch shortcut:", error);
      toast.error(t("gameLaunch.toast.shortcutExportFailed", { error }));
    }
  };

  const ratingText = game.rating > 0 ? `${game.rating.toFixed(1)} / 10` : "-";
  const createdAtText = formatLocalDate(
    game.created_at,
    config?.time_zone,
  ).replaceAll("/", "-");
  const releaseDateText = game.release_date?.trim() || "-";
  const metadataSources = game.metadata_sources?.length
    ? game.metadata_sources
    : game.source_type && game.source_id
      ? [{ source_type: game.source_type, source_id: game.source_id }]
      : [];
  const metadataSourceText = game.source_type
    ? `${game.source_type}${
      metadataSources.length > 1
        ? ` ${t("gameEdit.multipleSources", { count: metadataSources.length })}`
        : ""
    }`
    : "-";
  const defaultMetadataSource = game.source_type;
  const metadataSourceLinks = metadataSources
    .map((source) => {
      const sourceID = source.source_id?.trim() || "";
      return {
        source: source.source_type,
        sourceID,
        url: getMetadataSourceURL(source.source_type, sourceID),
      };
    })
    .filter(source => Boolean(source.url))
    .sort(
      (left, right) =>
        Number(right.source === defaultMetadataSource)
        - Number(left.source === defaultMetadataSource),
    );
  const defaultMetadataSourceURL
    = metadataSourceLinks.find(
      source => source.source === defaultMetadataSource,
    )?.url
    || metadataSourceLinks[0]?.url
    || "";
  const hasMultipleMetadataSourceLinks = metadataSourceLinks.length > 1;
  const coverImageSrc
    = game.cover_url || game.cover_source_url
      ? buildCoverImageSrc(
          game.cover_url || game.cover_source_url,
          String(coverImageRefreshToken),
        )
      : "";
  const launchOptions: Array<{
    key: LaunchMode;
    label: string;
    description: string;
    icon: string;
  }> = [
    {
      key: enums.LaunchMode.LaunchModeNormal,
      label: t("gameCard.startGame"),
      description: t("gameCard.normalLaunchDesc"),
      icon: "i-mdi-play",
    },
  ];
  if (supportsAdminLaunch) {
    launchOptions.push({
      key: "admin",
      label: t("gameCard.startAsAdmin"),
      description: t("gameCard.adminLaunchDesc"),
      icon: "i-mdi-shield-account",
    });
  }
  if (supportsSteamLaunch) {
    launchOptions.splice(1, 0, {
      key: enums.LaunchMode.LaunchModeSteam,
      label: t("gameCard.startWithSteam"),
      description: t("gameCard.steamLaunchDesc"),
      icon: "i-mdi-steam",
    });
  }
  if (platformGOOS === "darwin") {
    launchOptions.splice(1, 0, {
      key: enums.LaunchMode.LaunchModeCompatibility,
      label: t("gameCard.startWithCompatibility"),
      description: t("gameCard.compatibilityLaunchDesc"),
      icon: "i-mdi-application-brackets-outline",
    });
  }
  const selectedLaunchMode
    = (launchMode === enums.LaunchMode.LaunchModeSteam && !supportsSteamLaunch)
      || (launchMode === enums.LaunchMode.LaunchModeCompatibility
        && platformGOOS !== "darwin")
      ? enums.LaunchMode.LaunchModeNormal
      : launchMode;
  const selectedLaunchOption
    = launchOptions.find(option => option.key === selectedLaunchMode)
      ?? launchOptions[0];
  const isCurrentGameRunning = Boolean(gameRuntime);
  const isCurrentGameEnding = gameRuntime?.state === "ending";

  return (
    <div
      className={`space-y-8 max-w-8xl mx-auto p-8 transition-opacity duration-300 ${isLoading ? "opacity-50 pointer-events-none" : "opacity-100"}`}
    >
      {/* Back Button */}
      <button
        type="button"
        onClick={() => window.history.back()}
        className="flex rounded-md items-center text-brand-750 hover:text-brand-900 dark:text-brand-400 dark:hover:text-brand-200 transition-colors"
      >
        <div className="i-mdi-arrow-left text-2xl mr-1" />
        <span>{t("common.back")}</span>
      </button>

      {/* Header Section */}
      <div className="grid min-w-0 grid-cols-[15rem_minmax(0,1fr)] items-stretch gap-6">
        <div className="relative min-h-64 w-60">
          {coverImageSrc ? (
            <GameCoverImage
              src={coverImageSrc}
              fallbackSrc={game.cover_source_url}
              alt={game.name}
              loading="eager"
              fetchPriority="high"
              isNSFW={game.is_nsfw}
              revealNSFWOnHover
              className="absolute left-0 top-1/2 w-full -translate-y-1/2 rounded-lg shadow-lg"
              imageClassName="block h-auto w-full"
            />
          ) : (
            <div className="flex h-full min-h-64 w-full items-center justify-center text-brand-400">
              {t("game.noCover")}
            </div>
          )}
        </div>

        <div className="min-w-0 flex-1 space-y-4">
          <div className="flex flex-col gap-3">
            <h1 className="break-words text-4xl font-bold text-brand-900 dark:text-white">
              {game.name}
            </h1>
            {/* 操作和状态标签组 */}
            <div className="flex flex-wrap items-center gap-4">
              <BetterSplitButton
                label={
                  isCurrentGameEnding
                    ? t("playingIsland.ending")
                    : isCurrentGameRunning
                      ? t("home.gaming")
                      : selectedLaunchOption.label
                }
                icon={
                  isCurrentGameRunning
                    ? "i-mdi-gamepad-variant"
                    : selectedLaunchOption.icon
                }
                selectedKey={selectedLaunchMode}
                options={launchOptions}
                onClick={() => handleStartGame(selectedLaunchMode)}
                onSelect={setLaunchMode}
                size="sm"
                variant="primary"
                menuTitle={t("gameCard.launchMode")}
                disabled={isCurrentGameRunning}
                isLoading={isCurrentGameEnding}
              />
              <div className="h-6 w-px bg-brand-200 dark:bg-brand-700" />
              {" "}
              {/* 分隔线 */}
              <div className="flex flex-wrap gap-1.5">
                {Object.entries(statusConfig).map(([key, config]) => {
                  const isActive
                    = (game.status || enums.GameStatus.StatusNotStarted) === key;
                  return (
                    <button
                      type="button"
                      key={key}
                      onClick={() => handleStatusChange(key)}
                      className={`flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium transition-all ${
                        isActive
                          ? `${config.color} ring-2 ring-offset-1 ring-brand-400 dark:ring-offset-brand-900`
                          : "bg-brand-150 text-brand-500 dark:bg-brand-700 dark:text-brand-400 hover:bg-brand-200 dark:hover:bg-brand-600"
                      }`}
                    >
                      <div className={`${config.icon} text-base`} />
                      {isActive && <span>{config.label}</span>}
                    </button>
                  );
                })}
                <div className="ml-2 flex items-center gap-4">
                  <div className="h-6 w-px bg-brand-200 dark:bg-brand-700" />
                  <div className="flex items-center gap-1.5">
                    {hasMultipleMetadataSourceLinks ? (
                      <BetterDropdownMenu
                        align="start"
                        menuWidth="min-w-[240px]"
                        title={t("gameEdit.openSourcePage")}
                        ariaLabel={t("gameEdit.chooseSourcePage")}
                        trigger={(
                          <div className="flex h-8 items-center justify-center gap-0.5 rounded-full bg-brand-150 px-2 text-brand-500 transition-colors hover:bg-brand-200 hover:text-brand-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400/70 dark:bg-brand-700 dark:text-brand-400 dark:hover:bg-brand-600 dark:hover:text-brand-100">
                            <span
                              className="i-mdi-open-in-new text-base"
                              aria-hidden="true"
                            />
                            <span
                              className="i-mdi-chevron-down text-sm"
                              aria-hidden="true"
                            />
                          </div>
                        )}
                        items={metadataSourceLinks.map(source => ({
                          key: source.source,
                          label: sourceLabel(source.source, t),
                          description:
                            source.source === defaultMetadataSource
                              ? t("gameEdit.defaultSourceEntry", {
                                  id: source.sourceID,
                                })
                              : t("gameEdit.sourceEntry", {
                                  id: source.sourceID,
                                }),
                          iconSrc: getMetadataSourceIcon(
                            source.source,
                            "compact",
                          ),
                          onClick: () => void Browser.OpenURL(source.url),
                        }))}
                      />
                    ) : (
                      <button
                        type="button"
                        onClick={() =>
                          void Browser.OpenURL(defaultMetadataSourceURL)}
                        disabled={!defaultMetadataSourceURL}
                        aria-label={t("gameEdit.openSourcePage")}
                        className="flex h-8 w-8 items-center justify-center rounded-full bg-brand-150 text-brand-500 transition-colors hover:bg-brand-200 hover:text-brand-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400/70 disabled:cursor-not-allowed disabled:opacity-45 dark:bg-brand-700 dark:text-brand-400 dark:hover:bg-brand-600 dark:hover:text-brand-100"
                      >
                        <span
                          className="i-mdi-open-in-new text-base"
                          aria-hidden="true"
                        />
                      </button>
                    )}
                    <button
                      type="button"
                      onClick={async () => {
                        const path = game.game_directory || game.path;
                        if (!path)
                          return;
                        try {
                          await OpenLocalPath(path);
                        }
                        catch {
                          toast.error(t("gameEdit.openPathFailed"));
                        }
                      }}
                      disabled={!game.game_directory && !game.path}
                      aria-label={t("gameEdit.openInExplorer")}
                      className="flex h-8 w-8 items-center justify-center rounded-full bg-brand-150 text-brand-500 transition-colors hover:bg-brand-200 hover:text-brand-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400/70 disabled:cursor-not-allowed disabled:opacity-45 dark:bg-brand-700 dark:text-brand-400 dark:hover:bg-brand-600 dark:hover:text-brand-100"
                    >
                      <span
                        className="i-mdi-folder-open-outline text-base"
                        aria-hidden="true"
                      />
                    </button>
                    <button
                      type="button"
                      onClick={openCategoryModal}
                      aria-label={t("addToCategory.title")}
                      className="flex h-8 w-8 items-center justify-center rounded-full bg-brand-150 text-brand-500 transition-colors hover:bg-brand-200 hover:text-brand-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400/70 dark:bg-brand-700 dark:text-brand-400 dark:hover:bg-brand-600 dark:hover:text-brand-100"
                    >
                      <span
                        className="i-mdi-folder-plus-outline text-base"
                        aria-hidden="true"
                      />
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div className="grid min-w-0 grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4 text-sm text-brand-750 dark:text-brand-400">
            <div className="min-w-0">
              <div className="font-semibold mb-1">{t("game.dataSource")}</div>
              <div className="break-words">{metadataSourceText}</div>
            </div>
            <div className="min-w-0">
              <div className="font-semibold mb-1">{t("game.developer")}</div>
              {game.company?.trim() ? (
                <button
                  type="button"
                  onClick={() => searchLibraryByCompany(game.company.trim())}
                  className="max-w-full break-all text-left text-brand-750 dark:text-brand-400"
                >
                  {game.company}
                </button>
              ) : (
                <div>-</div>
              )}
            </div>
            <div>
              <div className="font-semibold mb-1">{t("common.createdAt")}</div>
              <div>{createdAtText}</div>
            </div>
            <div>
              <div className="font-semibold mb-1">{t("game.rating")}</div>
              <div>{ratingText}</div>
            </div>
            <div className="min-w-0">
              <div className="font-semibold mb-1">{t("game.releaseDate")}</div>
              <div className="break-words">{releaseDateText}</div>
            </div>
          </div>

          <div className="mt-4 min-w-0">
            <div className="font-semibold mb-2 text-brand-900 dark:text-white">
              {t("game.summary")}
            </div>
            <p className="max-w-full break-words text-brand-750 dark:text-brand-400 text-sm leading-relaxed whitespace-pre-wrap max-h-60 overflow-y-auto overflow-x-hidden scrollbar-hide pr-2 [overflow-wrap:anywhere]">
              {game.summary || t("game.noSummary")}
            </p>
          </div>

          <div className="mt-3 min-w-0">
            <GameTags
              key={gameId}
              gameId={gameId}
              initialTags={initialTags}
              showNSFW={config?.show_nsfw_tags}
              refreshToken={tagRefreshToken}
            />
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-brand-200 dark:border-brand-700">
        <div className="flex items-center">
          <nav className="-mb-px flex space-x-8">
            {["stats", "edit", "launch", "backup", "progress", "review"].map(
              tab => (
                <button
                  type="button"
                  key={tab}
                  onClick={() => setActiveTab(tab)}
                  className={`
                  whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm
                  ${
                activeTab === tab
                  ? "border-neutral-500 text-brand-700 dark:text-neutral-400"
                  : "border-transparent text-brand-700 hover:text-brand-750 hover:border-brand-300 dark:text-brand-400 dark:hover:text-brand-300"
                }
                `}
                >
                  {tab === "stats" && t("game.tabs.stats")}
                  {tab === "edit" && t("common.edit")}
                  {tab === "launch" && t("game.tabs.launch")}
                  {tab === "backup" && t("game.tabs.backup")}
                  {tab === "progress" && t("game.tabs.progress")}
                  {tab === "review" && t("game.tabs.review")}
                </button>
              ),
            )}
          </nav>
        </div>
      </div>

      {/* Content */}
      {activeTab === "stats" && <GameStatsPanel gameId={gameId} />}

      {activeTab === "edit" && game && (
        <GameEditPanel
          game={game}
          onGameChange={updateGameState}
          onDelete={handleDeleteGame}
          onSelectExecutable={handleSelectExecutable}
          onSelectGameDirectory={handleSelectGameDirectory}
          onSelectSaveDirectory={handleSelectSaveDirectory}
          onSelectSaveFile={handleSelectSaveFile}
          onSelectCoverImage={handleSelectCoverImage}
          onCoverImageChanged={() =>
            setCoverImageRefreshToken(prev => prev + 1)}
          onUpdateFromRemote={handleOpenUpdateFromRemote}
          onUpsertMetadataSource={handleUpsertMetadataSource}
          onDeleteMetadataSource={handleDeleteMetadataSource}
          onSetDefaultMetadataSource={handleSetDefaultMetadataSource}
          onAutoSaveMetadataSource={handleAutoSaveMetadataSource}
          onSearchMetadataByName={handleSearchMetadataByName}
        />
      )}

      {activeTab === "launch" && game && (
        <GameLaunchPanel
          game={game}
          config={config || undefined}
          goos={platformGOOS}
          onGameChange={updateGameState}
          onLaunchModeChange={handleDefaultLaunchModeChange}
          onRefreshSteamSettings={handleRefreshSteamSettings}
          onSaveSteamLaunchOptions={handleSaveSteamLaunchOptions}
          onSelectProcessExecutable={handleSelectProcessExecutable}
          onSelectRunningProcess={() => setIsProcessSelectModalOpen(true)}
          onExportShortcut={handleExportLaunchShortcut}
        />
      )}

      {activeTab === "backup" && (
        <GameBackupPanel gameId={gameId} savePath={game?.save_path} />
      )}

      {activeTab === "progress" && <GameProgressPanel gameId={gameId} />}

      {activeTab === "review" && game && <GameReviewPanel game={game} />}

      <ConfirmModal
        isOpen={isDeleteModalOpen}
        title={t("game.deleteGame")}
        message={t("game.deleteConfirmMsg", { name: game.name })}
        confirmText={t("game.confirmDelete")}
        type="danger"
        onClose={() => setIsDeleteModalOpen(false)}
        onConfirm={confirmDeleteGame}
      />

      <AddToCategoryModal
        isOpen={isCategoryModalOpen}
        allCategories={allCategories}
        initialSelectedIds={selectedCategoryIds}
        onClose={() => setIsCategoryModalOpen(false)}
        onSave={handleSaveCategories}
      />

      <ProcessSelectModal
        isOpen={isProcessSelectModalOpen}
        gameID={gameId}
        onClose={() => setIsProcessSelectModalOpen(false)}
        onSelected={handleRunningProcessSelected}
      />

      <MetadataFieldSelectModal
        isOpen={isMetadataFieldModalOpen}
        title={t("metadataUpdateFields.modal.singleTitle")}
        description={t("metadataUpdateFields.modal.singleDescription")}
        confirmText={t("metadataUpdateFields.modal.update")}
        initialFields={selectedMetadataFields}
        isSubmitting={isUpdatingFromRemote}
        onClose={() => setIsMetadataFieldModalOpen(false)}
        onConfirm={handleUpdateFromRemote}
      />

      <MetadataSourceSearchModal
        isOpen={isMetadataSearchModalOpen}
        results={metadataSearchResults}
        isApplying={isApplyingMetadataResult}
        onClose={() => setIsMetadataSearchModalOpen(false)}
        onSelect={result => void handleSelectMetadataSearchResult(result)}
        onRemove={index =>
          setMetadataSearchResults(current =>
            current.filter((_, resultIndex) => resultIndex !== index),
          )}
      />

      <SteamImportModal
        isOpen={isSteamModalOpen}
        gameName={game.name}
        status={steamStatus}
        isChecking={isCheckingSteam}
        isImporting={isImportingSteam}
        canRestartSteam={platformGOOS === "linux"}
        onClose={handleCloseSteamModal}
        onImport={handleImportGameToSteam}
        onRetry={handleRetrySteamStatus}
        onSelectExecutable={handleSteamSelectExecutable}
      />
    </div>
  );
}
