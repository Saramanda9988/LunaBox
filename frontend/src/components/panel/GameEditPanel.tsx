import type {
  ClipboardEvent,
  KeyboardEvent as ReactKeyboardEvent,
} from "react";
import type { models } from "../../../src/bindings/models";
import type { BetterDataTableColumn } from "../ui/better/BetterDataTable";
import { Browser } from "@wailsio/runtime";
import { useEffect, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  DownloadCoverImage,
  OpenLocalPath,
  SaveCoverImageDataURL,
} from "../../../bindings/lunabox/internal/service/gameservice";
import { enums } from "../../../src/bindings/models";
import {
  getMetadataSourceIcon,
  getMetadataSourceURL,
} from "../../utils/metadataSources";
import { formatDateInputValue, formatDateToYYYYMMDD } from "../../utils/time";
import { TOPBAR_HEIGHT } from "../bar/TopBar";
import { BetterActionInput } from "../ui/better/BetterActionInput";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterDataTable } from "../ui/better/BetterDataTable";
import { BetterDrawer } from "../ui/better/BetterDrawer";
import { BetterSelect } from "../ui/better/BetterSelect";
import { BetterSwitch } from "../ui/better/BetterSwitch";

interface GameEditFormProps {
  game: models.Game;
  onGameChange: (game: models.Game) => void;
  onDelete: () => void;
  onSelectExecutable: () => void;
  onSelectGameDirectory: () => void;
  onSelectSaveDirectory: () => void;
  onSelectSaveFile: () => void;
  onSelectCoverImage: () => void;
  onCoverImageChanged?: () => void;
  onUpdateFromRemote?: () => void;
  onUpsertMetadataSource: (
    source: enums.SourceType,
    sourceID: string,
  ) => Promise<void>;
  onDeleteMetadataSource: (source: enums.SourceType) => Promise<void>;
  onSetDefaultMetadataSource: (source: enums.SourceType) => Promise<void>;
  onAutoSaveMetadataSource: (
    source: enums.SourceType,
    sourceID: string,
  ) => Promise<void>;
  onSearchMetadataByName: () => Promise<boolean>;
}

const metadataSourceTypes: enums.SourceType[] = [
  enums.SourceType.Bangumi,
  enums.SourceType.VNDB,
  enums.SourceType.Ymgal,
  enums.SourceType.Steam,
  enums.SourceType.DLsite,
  enums.SourceType.TouchGal,
  enums.SourceType.Hikarinagi,
  enums.SourceType.ErogameScape,
];

interface ReleaseDateRow {
  week: string;
  dates: Date[];
}

const weekdayLabels = ["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"];
const METADATA_SOURCE_AUTO_SAVE_DELAY = 500;

function parseDateInputValue(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match)
    return null;

  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(year, month - 1, day);

  if (
    date.getFullYear() !== year
    || date.getMonth() !== month - 1
    || date.getDate() !== day
  ) {
    return null;
  }

  return date;
}

function startOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function addMonths(date: Date, amount: number): Date {
  return new Date(date.getFullYear(), date.getMonth() + amount, 1);
}

function formatMonthLabel(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  return `${year}/${month}`;
}

function getCalendarRows(monthDate: Date): ReleaseDateRow[] {
  const firstDay = new Date(monthDate.getFullYear(), monthDate.getMonth(), 1);
  const start = new Date(firstDay);
  start.setDate(firstDay.getDate() - firstDay.getDay());

  return Array.from({ length: 6 }, (_, weekIndex) => ({
    week: `week-${weekIndex}`,
    dates: Array.from({ length: 7 }, (__, dayIndex) => {
      const date = new Date(start);
      date.setDate(start.getDate() + weekIndex * 7 + dayIndex);
      return date;
    }),
  }));
}

function ReleaseDatePicker({
  value,
  label,
  onChange,
}: {
  value: string;
  label: string;
  onChange: (value: string) => void;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const selectedDate = parseDateInputValue(value);
  const [monthDate, setMonthDate] = useState(() =>
    startOfMonth(selectedDate ?? new Date()),
  );
  const [isOpen, setIsOpen] = useState(false);
  const rows = getCalendarRows(monthDate);
  const displayValue = value ? value.replaceAll("-", "/") : "";
  const columns: BetterDataTableColumn<ReleaseDateRow>[] = weekdayLabels.map(
    (weekday, dayIndex) => ({
      key: weekday,
      header: weekday,
      className: "w-[14.285%]",
      headerClassName: "text-center",
      cellClassName: "p-1 text-center",
      render: (row) => {
        const date = row.dates[dayIndex];
        const dateValue = formatDateToYYYYMMDD(date);
        const isOutside = date.getMonth() !== monthDate.getMonth();
        const isSelected = dateValue === value;

        return (
          <button
            type="button"
            onClick={() => {
              onChange(dateValue);
              setIsOpen(false);
            }}
            className={[
              "inline-flex h-8 w-8 items-center justify-center rounded-md text-sm transition-colors",
              "focus:outline-none focus:ring-2 focus:ring-neutral-500/30",
              isOutside
                ? "text-brand-300 hover:text-brand-600 dark:text-brand-600 dark:hover:text-brand-300"
                : "text-brand-700 hover:bg-brand-100 dark:text-brand-200 dark:hover:bg-brand-700",
              isSelected
                ? "bg-neutral-800 text-white hover:bg-neutral-800 dark:bg-white dark:text-neutral-950 dark:hover:bg-white"
                : "",
            ].join(" ")}
          >
            {date.getDate()}
          </button>
        );
      },
    }),
  );

  useEffect(() => {
    if (!isOpen)
      return;

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Node && !containerRef.current?.contains(target)) {
        setIsOpen(false);
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape")
        setIsOpen(false);
    };

    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [isOpen]);

  return (
    <div ref={containerRef} className="relative">
      <button
        type="button"
        aria-label={label}
        aria-expanded={isOpen}
        onClick={() => setIsOpen(open => !open)}
        className={[
          "glass-input flex min-h-10 w-full min-w-0 items-center justify-between gap-3",
          "rounded-md border border-brand-300 bg-white px-3 py-2 text-left",
          "text-brand-900 outline-none transition-colors",
          "focus:ring-2 focus:ring-neutral-500",
          "dark:border-brand-600 dark:bg-brand-700 dark:text-white",
        ].join(" ")}
      >
        <span
          className={[
            "min-w-0 flex-1 truncate",
            displayValue ? "" : "text-brand-400 dark:text-brand-500",
          ].join(" ")}
        >
          {displayValue || label}
        </span>
        <span
          className="i-mdi-calendar-month-outline shrink-0 text-lg text-brand-500 dark:text-brand-300"
          aria-hidden="true"
        />
      </button>

      {isOpen && (
        <div className="absolute left-0 top-full z-[9999] mt-2 w-[22rem] max-w-[calc(100vw-2rem)] rounded-xl border border-brand-200 bg-white p-3 shadow-xl focus:outline-none dark:border-brand-700 dark:bg-brand-800 data-glass:bg-white/90 data-glass:backdrop-blur-20 data-glass:dark:bg-brand-900/90">
          <div className="space-y-3">
            <div className="grid h-9 grid-cols-[4rem_1fr_4rem] items-center">
              <div className="flex items-center gap-1">
                <button
                  type="button"
                  aria-label="Previous year"
                  onClick={() => setMonthDate(date => addMonths(date, -12))}
                  className="flex h-8 w-8 items-center justify-center rounded-lg text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-900 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white"
                >
                  <span className="i-mdi-chevron-double-left text-lg" />
                </button>
                <button
                  type="button"
                  aria-label="Previous month"
                  onClick={() => setMonthDate(date => addMonths(date, -1))}
                  className="flex h-8 w-8 items-center justify-center rounded-lg text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-900 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white"
                >
                  <span className="i-mdi-chevron-left text-lg" />
                </button>
              </div>
              <div className="text-center text-sm font-semibold text-brand-900 dark:text-white">
                {formatMonthLabel(monthDate)}
              </div>
              <div className="flex items-center justify-end gap-1">
                <button
                  type="button"
                  aria-label="Next month"
                  onClick={() => setMonthDate(date => addMonths(date, 1))}
                  className="flex h-8 w-8 items-center justify-center rounded-lg text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-900 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white"
                >
                  <span className="i-mdi-chevron-right text-lg" />
                </button>
                <button
                  type="button"
                  aria-label="Next year"
                  onClick={() => setMonthDate(date => addMonths(date, 12))}
                  className="flex h-8 w-8 items-center justify-center rounded-lg text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-900 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white"
                >
                  <span className="i-mdi-chevron-double-right text-lg" />
                </button>
              </div>
            </div>

            <BetterDataTable
              rows={rows}
              columns={columns}
              rowKey={row => row.week}
              maxHeightClassName="max-h-none"
            />
          </div>
        </div>
      )}
    </div>
  );
}

function readBlobAsDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.addEventListener("load", () => {
      if (typeof reader.result === "string") {
        resolve(reader.result);
        return;
      }
      reject(new Error("clipboard-image-read-failed"));
    });
    reader.addEventListener("error", () => {
      reject(reader.error ?? new Error("clipboard-image-read-failed"));
    });
    reader.readAsDataURL(blob);
  });
}

function getClipboardImageBlob(items: DataTransferItemList): Blob | null {
  for (const item of Array.from(items)) {
    if (item.kind !== "file" || !item.type.startsWith("image/"))
      continue;

    const file = item.getAsFile();
    if (file)
      return file;
  }
  return null;
}

function isRemoteCoverURL(coverURL?: string): boolean {
  const trimmedURL = coverURL?.trim() ?? "";
  const normalizedURL = trimmedURL.toLowerCase();
  return (
    (normalizedURL.startsWith("http://")
      || normalizedURL.startsWith("https://"))
    && !normalizedURL.includes("wails.localhost")
  );
}

function getExecutableDisplayPath(
  executablePath: string,
  gameDirectory?: string,
): string {
  const normalizedDirectory = (gameDirectory || "")
    .replaceAll("/", "\\")
    .replace(/\\+$/, "");
  const normalizedExecutablePath = executablePath.replaceAll("/", "\\");
  const directoryPrefix = `${normalizedDirectory}\\`;

  if (
    normalizedDirectory
    && normalizedExecutablePath
      .toLowerCase()
      .startsWith(directoryPrefix.toLowerCase())
  ) {
    return executablePath.slice(directoryPrefix.length);
  }

  return executablePath;
}

function resolveExecutablePath(
  executablePath: string,
  gameDirectory?: string,
): string {
  if (
    !executablePath
    || !gameDirectory
    || /^(?:[a-z]:[\\/]|[\\/]{2})/i.test(executablePath)
  ) {
    return executablePath;
  }

  const directory = gameDirectory.replace(/[\\/]+$/, "");
  const separator = directory.includes("\\") ? "\\" : "/";
  return `${directory}${separator}${executablePath.replace(/^[\\/]+/, "")}`;
}

export function GameEditPanel({
  game,
  onGameChange,
  onDelete,
  onSelectExecutable,
  onSelectGameDirectory,
  onSelectSaveDirectory,
  onSelectSaveFile,
  onSelectCoverImage,
  onCoverImageChanged,
  onUpdateFromRemote,
  onUpsertMetadataSource,
  onDeleteMetadataSource,
  onSetDefaultMetadataSource,
  onAutoSaveMetadataSource,
  onSearchMetadataByName,
}: GameEditFormProps) {
  const { t } = useTranslation();
  const [isDownloadingCover, setIsDownloadingCover] = useState(false);
  const [sourceDraftType, setSourceDraftType] = useState<enums.SourceType>(
    enums.SourceType.Bangumi,
  );
  const [sourceDraftID, setSourceDraftID] = useState("");
  const [sourceIDEditState, setSourceIDEditState] = useState<{
    gameID: string;
    values: Record<string, string>;
  }>({ gameID: "", values: {} });
  const [busySource, setBusySource] = useState("");
  const [isMetadataDrawerOpen, setIsMetadataDrawerOpen] = useState(false);
  const [isSearchingMetadataByName, setIsSearchingMetadataByName]
    = useState(false);
  const [aliasDraftState, setAliasDraftState] = useState({
    gameId: game.id,
    value: "",
  });
  const [aliasAddingGameId, setAliasAddingGameId] = useState<string | null>(
    null,
  );
  const aliasInputRef = useRef<HTMLInputElement>(null);
  const submitAliasAfterComposition = useRef(false);
  const metadataSourceSaveTimersRef = useRef(new Map<string, number>());
  const pendingMetadataSourceSavesRef = useRef(
    new Map<enums.SourceType, string>(),
  );
  const autoSaveMetadataSourceRef = useRef(onAutoSaveMetadataSource);
  autoSaveMetadataSourceRef.current = onAutoSaveMetadataSource;
  const aliasDraft
    = aliasDraftState.gameId === game.id ? aliasDraftState.value : "";
  const setAliasDraft = (value: string) => {
    setAliasDraftState({ gameId: game.id, value });
  };
  const aliases = game.aliases ?? [];
  const isAddingAlias = aliasAddingGameId === game.id;
  const releaseDateInputValue = formatDateInputValue(game.release_date);
  const hasUnsupportedReleaseDate
    = Boolean(game.release_date) && releaseDateInputValue === "";
  const remoteCoverURL = isRemoteCoverURL(game.cover_url)
    ? game.cover_url
    : game.cover_source_url || "";
  const canDownloadCover
    = isRemoteCoverURL(remoteCoverURL) && !isDownloadingCover;
  const executableDisplayPath = getExecutableDisplayPath(
    game.path,
    game.game_directory,
  );
  const metadataSources = game.metadata_sources?.length
    ? game.metadata_sources
    : [];
  const sourceIDEdits
    = sourceIDEditState.gameID === game.id ? sourceIDEditState.values : {};
  const setSourceIDEdits = (
    update: (current: Record<string, string>) => Record<string, string>,
  ) => {
    setSourceIDEditState(current => ({
      gameID: game.id,
      values: update(current.gameID === game.id ? current.values : {}),
    }));
  };
  const configuredSourceTypes = new Set(
    metadataSources.map(source => source.source_type),
  );
  const sourceOptions = metadataSourceTypes.map(source => ({
    value: source,
    label:
      source === enums.SourceType.Ymgal
        ? t("gameEdit.sourceYmgal")
        : source === enums.SourceType.DLsite
          ? t("gameEdit.sourceDlsite")
          : source === enums.SourceType.TouchGal
            ? t("gameEdit.sourceTouchGal")
            : source === enums.SourceType.Hikarinagi
              ? t("gameEdit.sourceHikarinagi")
              : source === enums.SourceType.ErogameScape
                ? t("gameEdit.sourceErogameScape")
                : source === enums.SourceType.VNDB
                  ? "VNDB"
                  : source === enums.SourceType.Steam
                    ? "Steam"
                    : "Bangumi",
  }));
  const sourceLabels = new Map(
    sourceOptions.map(option => [option.value, option.label]),
  );
  const defaultMetadataSource = metadataSources.find(
    source => source.source_type === game.source_type,
  );
  const defaultMetadataSourceType
    = defaultMetadataSource?.source_type || game.source_type;
  const defaultMetadataSourceID
    = defaultMetadataSource?.source_id || game.source_id || "";
  const defaultMetadataSourceLabel = defaultMetadataSourceType
    ? (sourceLabels.get(defaultMetadataSourceType) ?? defaultMetadataSourceType)
    : "-";
  const defaultMetadataSourceIcon = defaultMetadataSourceType
    ? getMetadataSourceIcon(defaultMetadataSourceType, "compact")
    : undefined;
  const defaultMetadataSourceUsesSquareIcon
    = defaultMetadataSourceType === enums.SourceType.Bangumi
      || defaultMetadataSourceType === enums.SourceType.Hikarinagi;
  useEffect(() => {
    if (isAddingAlias)
      aliasInputRef.current?.focus();
  }, [isAddingAlias]);

  const runSourceAction = async (key: string, action: () => Promise<void>) => {
    setBusySource(key);
    try {
      await action();
    }
    catch (error) {
      console.error("Failed to update metadata source:", error);
      toast.error(t("gameEdit.sourceOperationFailed", { error }));
    }
    finally {
      setBusySource("");
    }
  };

  const persistMetadataSourceID = async (
    source: enums.SourceType,
    sourceID: string,
  ) => {
    try {
      await autoSaveMetadataSourceRef.current(source, sourceID);
      return true;
    }
    catch (error) {
      console.error("Failed to auto-save metadata source:", error);
      toast.error(t("gameEdit.sourceOperationFailed", { error }));
      return false;
    }
  };

  const cancelMetadataSourceAutoSave = (source: enums.SourceType) => {
    const sourceKey = String(source);
    const timer = metadataSourceSaveTimersRef.current.get(sourceKey);
    if (timer !== undefined)
      window.clearTimeout(timer);
    metadataSourceSaveTimersRef.current.delete(sourceKey);
    pendingMetadataSourceSavesRef.current.delete(source);
  };

  const scheduleMetadataSourceAutoSave = (
    source: enums.SourceType,
    sourceID: string,
  ) => {
    cancelMetadataSourceAutoSave(source);
    const normalizedSourceID = sourceID.trim();
    if (!normalizedSourceID)
      return;

    pendingMetadataSourceSavesRef.current.set(source, normalizedSourceID);
    const sourceKey = String(source);
    const timer = window.setTimeout(() => {
      metadataSourceSaveTimersRef.current.delete(sourceKey);
      pendingMetadataSourceSavesRef.current.delete(source);
      void persistMetadataSourceID(source, normalizedSourceID);
    }, METADATA_SOURCE_AUTO_SAVE_DELAY);
    metadataSourceSaveTimersRef.current.set(sourceKey, timer);
  };

  const flushMetadataSourceAutoSaves = async () => {
    const pendingSaves = Array.from(
      pendingMetadataSourceSavesRef.current.entries(),
    );
    for (const timer of metadataSourceSaveTimersRef.current.values())
      window.clearTimeout(timer);
    metadataSourceSaveTimersRef.current.clear();
    pendingMetadataSourceSavesRef.current.clear();

    const results = await Promise.all(
      pendingSaves.map(([source, sourceID]) =>
        persistMetadataSourceID(source, sourceID),
      ),
    );
    return results.every(Boolean);
  };

  useEffect(() => {
    const saveTimers = metadataSourceSaveTimersRef.current;
    const pendingSavesBySource = pendingMetadataSourceSavesRef.current;
    const autoSaveMetadataSource = autoSaveMetadataSourceRef.current;
    return () => {
      const pendingSaves = Array.from(pendingSavesBySource.entries());
      for (const timer of saveTimers.values()) window.clearTimeout(timer);
      saveTimers.clear();
      pendingSavesBySource.clear();
      for (const [source, sourceID] of pendingSaves)
        void autoSaveMetadataSource(source, sourceID);
    };
  }, [game.id]);

  const searchMetadataByName = async () => {
    setIsSearchingMetadataByName(true);
    try {
      const didSavePendingSources = await flushMetadataSourceAutoSaves();
      if (!didSavePendingSources)
        return;
      const didOpenResults = await onSearchMetadataByName();
      if (didOpenResults)
        setIsMetadataDrawerOpen(false);
    }
    finally {
      setIsSearchingMetadataByName(false);
    }
  };

  const addAlias = (value = aliasDraft) => {
    const alias = value.trim();
    if (!alias) {
      setAliasDraft("");
      setAliasAddingGameId(null);
      return;
    }

    const duplicate = aliases.some(
      existingAlias =>
        existingAlias.toLocaleLowerCase() === alias.toLocaleLowerCase(),
    );
    if (!duplicate) {
      onGameChange({
        ...game,
        aliases: [...aliases, alias],
      } as models.Game);
    }
    setAliasDraft("");
    setAliasAddingGameId(null);
  };

  const removeAlias = (index: number) => {
    onGameChange({
      ...game,
      aliases: aliases.filter((_, aliasIndex) => aliasIndex !== index),
    } as models.Game);
  };

  const handleAliasKeyDown = (event: ReactKeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter" && event.nativeEvent.isComposing) {
      submitAliasAfterComposition.current = true;
      return;
    }

    if (event.key === "Enter") {
      event.preventDefault();
      addAlias();
      return;
    }

    if (event.key === "Escape") {
      event.preventDefault();
      setAliasDraft("");
      setAliasAddingGameId(null);
      return;
    }

    if (event.key === "Backspace" && !aliasDraft && aliases.length > 0)
      removeAlias(aliases.length - 1);
  };

  const handleAliasKeyUp = (event: ReactKeyboardEvent<HTMLInputElement>) => {
    if (event.key !== "Enter" || !submitAliasAfterComposition.current)
      return;

    submitAliasAfterComposition.current = false;
    event.preventDefault();
    addAlias(event.currentTarget.value);
  };

  const importCoverDataURL = async (dataURL: string) => {
    const coverUrl = await SaveCoverImageDataURL(game.id, dataURL);
    if (coverUrl) {
      onGameChange({
        ...game,
        cover_url: coverUrl,
      } as models.Game);
      onCoverImageChanged?.();
    }
    toast.success(t("gameEdit.importFromClipboardSuccess"));
  };

  const handleCoverPaste = async (event: ClipboardEvent<HTMLInputElement>) => {
    const imageBlob = getClipboardImageBlob(event.clipboardData.items);
    if (!imageBlob)
      return;

    event.preventDefault();
    try {
      await importCoverDataURL(await readBlobAsDataURL(imageBlob));
    }
    catch (error) {
      console.error("Failed to import pasted cover image:", error);
      toast.error(t("gameEdit.importFromClipboardFailed"));
    }
  };

  const handleDownloadCover = async () => {
    if (!isRemoteCoverURL(remoteCoverURL))
      return;

    setIsDownloadingCover(true);
    try {
      const coverUrl = await DownloadCoverImage(game.id, remoteCoverURL);
      if (coverUrl) {
        onGameChange({
          ...game,
          cover_url: coverUrl,
          cover_source_url: remoteCoverURL,
        } as models.Game);
        onCoverImageChanged?.();
      }
      toast.success(t("gameEdit.downloadCoverSuccess"));
    }
    catch (error) {
      console.error("Failed to download cover image:", error);
      toast.error(t("gameEdit.downloadCoverFailed"));
    }
    finally {
      setIsDownloadingCover(false);
    }
  };

  return (
    <div className="glass-panel w-full min-w-0 bg-white dark:bg-brand-800 p-8 rounded-lg shadow-sm">
      <div className="space-y-6">
        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.name")}
          </label>
          <input
            type="text"
            value={game.name}
            onChange={e =>
              onGameChange({ ...game, name: e.target.value } as models.Game)}
            className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.aliases")}
          </label>
          <div className="glass-input flex min-h-10 w-full flex-wrap items-center gap-1.5 rounded-md border border-brand-300 bg-white px-2.5 py-[5px] text-brand-900 outline-none focus-within:ring-2 focus-within:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white">
            {aliases.map((alias, index) => (
              <span
                key={alias}
                className="inline-flex max-w-full items-center gap-1 rounded-full border border-dashed border-brand-400/80 bg-white/70 py-1.5 pl-3 pr-2 text-xs text-brand-700 transition-colors duration-200 dark:border-brand-500/70 dark:bg-brand-900/45 dark:text-brand-200"
              >
                <span className="min-w-0 truncate px-0.5">{alias}</span>
                <button
                  type="button"
                  aria-label={t("gameEdit.removeAlias", { alias })}
                  onClick={() => removeAlias(index)}
                  className="inline-flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-brand-400 transition-colors duration-200 hover:bg-red-50 hover:text-red-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400/70 dark:text-brand-500 dark:hover:bg-red-500/12 dark:hover:text-red-300"
                >
                  <span className="i-mdi-close block text-xs" />
                </button>
              </span>
            ))}
            {isAddingAlias ? (
              <input
                ref={aliasInputRef}
                type="text"
                value={aliasDraft}
                onChange={event => setAliasDraft(event.target.value)}
                onKeyDown={handleAliasKeyDown}
                onKeyUp={handleAliasKeyUp}
                onBlur={() => addAlias()}
                placeholder={t("gameEdit.aliasPlaceholder")}
                className="w-40 flex-none rounded-full border border-brand-300 bg-white/90 px-3 py-1.5 text-xs text-brand-900 outline-none transition-[border-color,box-shadow,background-color] duration-200 placeholder:text-brand-400 focus:w-56 focus:border-brand-500 focus:bg-white focus:ring-2 focus:ring-brand-200 dark:border-brand-600 dark:bg-brand-900/80 dark:text-white dark:placeholder:text-brand-500 dark:focus:border-brand-400 dark:focus:bg-brand-900 dark:focus:ring-brand-700"
              />
            ) : (
              <button
                type="button"
                aria-label={t("gameEdit.addAlias")}
                onClick={() => setAliasAddingGameId(game.id)}
                className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-dashed border-brand-300 text-brand-500 transition-all duration-200 hover:border-brand-500 hover:bg-brand-50 hover:text-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400/70 dark:border-brand-600 dark:text-brand-400 dark:hover:border-brand-400 dark:hover:bg-brand-800/50 dark:hover:text-brand-200"
              >
                <span className="i-mdi-plus block text-sm" />
              </button>
            )}
          </div>
          <p className="mt-1 text-xs text-brand-500">
            {t("gameEdit.aliasHint")}
          </p>
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.cover")}
          </label>
          <BetterActionInput
            value={game.cover_url}
            onChange={e =>
              onGameChange({
                ...game,
                cover_url: e.target.value,
              } as models.Game)}
            onPaste={handleCoverPaste}
            placeholder={t("gameEdit.coverPlaceholder")}
            actions={[
              {
                ariaLabel: t("gameEdit.selectImage"),
                icon: "i-mdi-image-search-outline",
                onClick: onSelectCoverImage,
              },
              {
                ariaLabel: t("gameEdit.downloadCover"),
                disabled: !canDownloadCover,
                icon: isDownloadingCover
                  ? "i-mdi-loading animate-spin"
                  : "i-mdi-download",
                onClick: handleDownloadCover,
              },
            ]}
          />
          <p className="mt-1 text-xs text-brand-500">
            {t("gameEdit.coverHint")}
          </p>
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.coverSource")}
          </label>
          <input
            type="text"
            value={game.cover_source_url || ""}
            onChange={e =>
              onGameChange({
                ...game,
                cover_source_url: e.target.value,
              } as models.Game)}
            placeholder={t("gameEdit.coverSourcePlaceholder")}
            className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
          />
          <p className="mt-1 text-xs text-brand-500">
            {t("gameEdit.coverSourceHint")}
          </p>
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.developer")}
          </label>
          <input
            type="text"
            value={game.company}
            onChange={e =>
              onGameChange({
                ...game,
                company: e.target.value,
              } as models.Game)}
            className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
          />
        </div>

        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              {t("gameEdit.rating")}
            </label>
            <div className="flex items-center gap-2">
              <input
                type="number"
                min={0}
                max={10}
                step={0.1}
                inputMode="decimal"
                value={game.rating > 0 ? game.rating : ""}
                onChange={(e) => {
                  const rawValue = e.target.value;
                  const nextRating
                    = rawValue === ""
                      ? 0
                      : Math.min(10, Math.max(0, Number(rawValue)));
                  onGameChange({
                    ...game,
                    rating: Number.isFinite(nextRating) ? nextRating : 0,
                  } as models.Game);
                }}
                placeholder={t("gameEdit.ratingPlaceholder")}
                className="glass-input min-w-0 flex-1 px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none"
              />
              <span className="shrink-0 text-sm text-brand-500 dark:text-brand-400">
                / 10
              </span>
            </div>
            <p className="mt-1 text-xs text-brand-500 dark:text-brand-400">
              {t("gameEdit.ratingHint")}
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              {t("gameEdit.releaseDate")}
            </label>
            <ReleaseDatePicker
              value={releaseDateInputValue}
              label={t("gameEdit.releaseDate")}
              onChange={value =>
                onGameChange({
                  ...game,
                  release_date: value,
                } as models.Game)}
            />
            {hasUnsupportedReleaseDate && (
              <p className="mt-1 text-xs text-brand-500 dark:text-brand-400">
                {t("gameEdit.releaseDateRawHint", {
                  value: game.release_date,
                })}
              </p>
            )}
          </div>
        </div>

        <div className="space-y-3">
          {/* <h3 className="text-lg font-semibold text-brand-800 dark:text-brand-100">
            {t("gameEdit.paths")}
          </h3> */}
          <div className="grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,2fr)_minmax(18rem,1fr)]">
            <div className="min-w-0">
              <label className="mb-1 block text-sm font-medium text-brand-700 dark:text-brand-300">
                {t("gameEdit.gameDirectory")}
              </label>
              <BetterActionInput
                value={game.game_directory || ""}
                onChange={e =>
                  onGameChange({
                    ...game,
                    game_directory: e.target.value,
                  } as models.Game)}
                placeholder={t("gameEdit.gameDirectoryPlaceholder")}
                actions={[
                  {
                    ariaLabel: t("gameEdit.selectFolder"),
                    icon: "i-mdi-folder-search-outline",
                    onClick: onSelectGameDirectory,
                  },
                  {
                    ariaLabel: t("gameEdit.openInExplorer"),
                    disabled: !game.game_directory && !game.path,
                    icon: "i-mdi-folder-open-outline",
                    onClick: async () => {
                      const path = game.game_directory || game.path;
                      if (!path)
                        return;
                      try {
                        await OpenLocalPath(path);
                      }
                      catch {
                        toast.error(t("gameEdit.openPathFailed"));
                      }
                    },
                  },
                ]}
              />
            </div>

            <div className="min-w-0">
              <label className="mb-1 block text-sm font-medium text-brand-700 dark:text-brand-300">
                {t("gameEdit.executable")}
              </label>
              <BetterActionInput
                value={executableDisplayPath}
                onChange={e =>
                  onGameChange({
                    ...game,
                    path: resolveExecutablePath(
                      e.target.value,
                      game.game_directory,
                    ),
                  } as models.Game)}
                actions={[
                  {
                    ariaLabel: t("gameEdit.selectFile"),
                    icon: "i-mdi-file-search-outline",
                    onClick: onSelectExecutable,
                  },
                  {
                    ariaLabel: t("gameEdit.openInExplorer"),
                    disabled: !game.path,
                    icon: "i-mdi-folder-open-outline",
                    onClick: async () => {
                      try {
                        await OpenLocalPath(game.path);
                      }
                      catch {
                        toast.error(t("gameEdit.openPathFailed"));
                      }
                    },
                  },
                ]}
              />
            </div>
          </div>
          <p className="text-xs text-brand-500 dark:text-brand-400">
            {t("gameEdit.gameDirectoryHint")}
          </p>
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.savePath")}
          </label>
          <BetterActionInput
            value={game.save_path || ""}
            onChange={e =>
              onGameChange({
                ...game,
                save_path: e.target.value,
              } as models.Game)}
            placeholder={t("gameEdit.savePathPlaceholder")}
            actions={[
              {
                ariaLabel: t("gameEdit.selectFolder"),
                icon: "i-mdi-folder-search-outline",
                onClick: onSelectSaveDirectory,
              },
              {
                ariaLabel: t("gameEdit.selectFile"),
                icon: "i-mdi-file-search-outline",
                onClick: onSelectSaveFile,
              },
              {
                ariaLabel: t("gameEdit.openInExplorer"),
                disabled: !game.save_path,
                icon: "i-mdi-folder-open-outline",
                onClick: async () => {
                  if (!game.save_path)
                    return;
                  try {
                    await OpenLocalPath(game.save_path);
                  }
                  catch {
                    toast.error(t("gameEdit.openPathFailed"));
                  }
                },
              },
            ]}
          />
          <p className="mt-1 text-xs text-brand-500">
            {t("gameEdit.savePathHint")}
          </p>
        </div>

        <div>
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.summary")}
          </label>
          <textarea
            value={game.summary}
            onChange={e =>
              onGameChange({ ...game, summary: e.target.value } as models.Game)}
            rows={6}
            className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-white dark:bg-brand-700 text-brand-900 dark:text-white focus:ring-2 focus:ring-neutral-500 outline-none resize-none"
          />
        </div>

        <section className="space-y-2">
          <h3 className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
            {t("gameEdit.metadataSources")}
          </h3>
          <div className="glass-panel flex items-center justify-between gap-4 rounded-xl border border-brand-200 bg-brand-50/60 p-3 dark:border-brand-700 dark:bg-brand-900/25">
            <div className="flex min-w-0 items-center gap-3">
              <span
                className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-brand-200 text-brand-700 dark:bg-brand-700 dark:text-brand-200"
                aria-hidden="true"
              >
                {defaultMetadataSourceIcon ? (
                  <img
                    src={defaultMetadataSourceIcon}
                    alt=""
                    className={
                      defaultMetadataSourceUsesSquareIcon
                        ? "h-full w-full object-cover"
                        : "max-h-6 max-w-8 object-contain brightness-0 opacity-80 dark:invert dark:opacity-90"
                    }
                  />
                ) : (
                  <span className="i-mdi-database-star-outline text-xl" />
                )}
              </span>
              <div className="min-w-0">
                <div className="truncate text-sm font-semibold text-brand-800 dark:text-brand-100">
                  {defaultMetadataSourceLabel}
                </div>
                <div className="mt-0.5 flex min-w-0 items-center gap-1.5 text-xs text-brand-500 dark:text-brand-400">
                  <span className="shrink-0 font-medium">ID</span>
                  <span className="truncate font-mono">
                    {defaultMetadataSourceID || "-"}
                  </span>
                </div>
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-3">
              {metadataSources.length > 1 ? (
                <span className="whitespace-nowrap text-xs text-brand-500 dark:text-brand-400">
                  {t("gameEdit.multipleSources", {
                    count: metadataSources.length,
                  })}
                </span>
              ) : null}
              <BetterButton
                size="sm"
                variant="secondary"
                icon="i-mdi-pencil-outline"
                onClick={() => setIsMetadataDrawerOpen(true)}
              >
                {t("common.edit")}
              </BetterButton>
            </div>
          </div>
        </section>

        <BetterDrawer
          isOpen={isMetadataDrawerOpen}
          onOpenChange={setIsMetadataDrawerOpen}
          title={t("gameEdit.metadataSources")}
          closeLabel={t("common.cancel")}
          className="!w-[min(92vw,42rem)]"
          topOffset={TOPBAR_HEIGHT}
        >
          <div className="space-y-4">
            {metadataSources.length > 0 ? (
              <div className="divide-y divide-brand-200 dark:divide-brand-700">
                {metadataSources.map((source) => {
                  const sourceID
                    = sourceIDEdits[source.source_type] ?? source.source_id;
                  const sourceURL = getMetadataSourceURL(
                    source.source_type,
                    sourceID,
                  );
                  const isDefault = game.source_type === source.source_type;
                  const isBusy = busySource.startsWith(
                    `${source.source_type}:`,
                  );
                  const label
                    = sourceLabels.get(source.source_type) ?? source.source_type;
                  const sourceIcon = getMetadataSourceIcon(source.source_type);

                  return (
                    <div
                      key={source.source_type}
                      className="space-y-3 py-4 first:pt-0 last:pb-0"
                    >
                      <div className="flex items-center justify-between gap-3">
                        <div className="flex min-w-0 items-center gap-2">
                          {sourceIcon ? (
                            <img
                              src={sourceIcon}
                              alt=""
                              aria-hidden="true"
                              className="h-[22px] w-auto max-w-24 shrink-0 object-contain brightness-0 opacity-80 transition-all dark:invert dark:opacity-90"
                            />
                          ) : null}
                          <span className="truncate text-sm font-semibold text-brand-800 dark:text-brand-100">
                            {label}
                          </span>
                        </div>
                        <div className="flex shrink-0 items-center gap-1.5">
                          <BetterButton
                            size="sm"
                            variant="ghost"
                            icon={
                              isDefault ? "i-mdi-star" : "i-mdi-star-outline"
                            }
                            className={
                              isDefault
                                ? "!text-warning-500 dark:!text-warning-400"
                                : ""
                            }
                            isLoading={
                              busySource === `${source.source_type}:default`
                            }
                            disabled={isBusy}
                            aria-pressed={isDefault}
                            aria-label={
                              isDefault
                                ? t("gameEdit.defaultSource")
                                : t("gameEdit.setDefaultSource")
                            }
                            onClick={() => {
                              if (isDefault)
                                return;
                              void runSourceAction(
                                `${source.source_type}:default`,
                                () =>
                                  onSetDefaultMetadataSource(
                                    source.source_type,
                                  ),
                              );
                            }}
                          />
                          <BetterButton
                            size="sm"
                            variant="ghost"
                            icon="i-mdi-delete-outline"
                            isLoading={
                              busySource === `${source.source_type}:delete`
                            }
                            disabled={isBusy}
                            aria-label={t("gameEdit.deleteSource")}
                            onClick={() =>
                              void runSourceAction(
                                `${source.source_type}:delete`,
                                async () => {
                                  cancelMetadataSourceAutoSave(
                                    source.source_type,
                                  );
                                  await onDeleteMetadataSource(
                                    source.source_type,
                                  );
                                  setSourceIDEdits((current) => {
                                    const next = { ...current };
                                    delete next[source.source_type];
                                    return next;
                                  });
                                },
                              )}
                          />
                        </div>
                      </div>

                      <div>
                        <BetterActionInput
                          value={sourceID}
                          onChange={(event) => {
                            const nextSourceID = event.target.value;
                            setSourceIDEdits(current => ({
                              ...current,
                              [source.source_type]: nextSourceID,
                            }));
                            scheduleMetadataSourceAutoSave(
                              source.source_type,
                              nextSourceID,
                            );
                          }}
                          onBlur={() => {
                            if (sourceID.trim())
                              return;
                            setSourceIDEdits(current => ({
                              ...current,
                              [source.source_type]: source.source_id,
                            }));
                          }}
                          placeholder={t("gameEdit.sourceIdPlaceholder")}
                          actions={[
                            {
                              ariaLabel: t("gameEdit.openSourcePage"),
                              icon: "i-mdi-open-in-new",
                              disabled: !sourceURL,
                              onClick: () => void Browser.OpenURL(sourceURL),
                            },
                          ]}
                        />
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="rounded-lg border border-dashed border-brand-300 px-3 py-5 text-center text-xs text-brand-500 dark:border-brand-600 dark:text-brand-400">
                {t("gameEdit.noMetadataSources")}
              </div>
            )}

            <div className="space-y-2 border-t border-brand-200 pt-4 dark:border-brand-700">
              <div className="text-sm font-semibold text-brand-800 dark:text-brand-100">
                {t("gameEdit.addSource")}
              </div>
              <div className="grid grid-cols-[9rem_minmax(0,1fr)_auto] gap-2">
                <BetterSelect
                  value={sourceDraftType}
                  onChange={value =>
                    setSourceDraftType(value as enums.SourceType)}
                  options={sourceOptions.filter(
                    option => !configuredSourceTypes.has(option.value),
                  )}
                />
                <input
                  type="text"
                  value={sourceDraftID}
                  onChange={event => setSourceDraftID(event.target.value)}
                  placeholder={t("gameEdit.sourceIdPlaceholder")}
                  className="glass-input min-w-0 rounded-md border border-brand-300 bg-white px-3 py-2 text-brand-900 outline-none focus:ring-2 focus:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700 dark:text-white"
                />
                <BetterButton
                  variant="secondary"
                  icon="i-mdi-plus"
                  isLoading={busySource === "add"}
                  disabled={
                    !sourceDraftID.trim()
                    || configuredSourceTypes.has(sourceDraftType)
                  }
                  onClick={() =>
                    void runSourceAction("add", async () => {
                      await onUpsertMetadataSource(
                        sourceDraftType,
                        sourceDraftID,
                      );
                      setSourceDraftID("");
                      const nextType = metadataSourceTypes.find(
                        source =>
                          source !== sourceDraftType
                          && !configuredSourceTypes.has(source),
                      );
                      if (nextType)
                        setSourceDraftType(nextType);
                    })}
                >
                  {t("gameEdit.addSource")}
                </BetterButton>
              </div>
            </div>

            <div className="flex items-center gap-3" aria-hidden="true">
              <div className="h-px flex-1 bg-brand-200 dark:bg-brand-700" />
              <span className="text-xs text-brand-500 dark:text-brand-400">
                {t("gameEdit.or")}
              </span>
              <div className="h-px flex-1 bg-brand-200 dark:bg-brand-700" />
            </div>

            <BetterButton
              className="w-full"
              variant="primary"
              icon="i-mdi-database-search-outline"
              isLoading={isSearchingMetadataByName}
              onClick={() => void searchMetadataByName()}
            >
              {isSearchingMetadataByName
                ? t("common.searching")
                : t("gameEdit.searchMetadataByCurrentName")}
            </BetterButton>
          </div>
        </BetterDrawer>

        <div className="data-glass:bg-white/2 data-glass:dark:bg-black/2 flex items-center justify-between gap-4 rounded-lg border border-brand-200 bg-brand-50 p-4 dark:border-brand-700 dark:bg-brand-700/50">
          <div className="flex-1 space-y-2">
            <label
              htmlFor="game-metadata-lock"
              className="block text-sm font-medium text-brand-700 dark:text-brand-300"
            >
              {t("gameEdit.metadataLock")}
            </label>
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("gameEdit.metadataLockHint")}
            </p>
          </div>
          <BetterSwitch
            id="game-metadata-lock"
            checked={Boolean(game.metadata_locked)}
            onCheckedChange={checked =>
              onGameChange({
                ...game,
                metadata_locked: checked,
              } as models.Game)}
          />
        </div>

        <div className="data-glass:bg-white/2 data-glass:dark:bg-black/2 flex items-center justify-between gap-4 rounded-lg border border-brand-200 bg-brand-50 p-4 dark:border-brand-700 dark:bg-brand-700/50">
          <div className="flex-1 space-y-2">
            <label
              htmlFor="game-is-nsfw"
              className="block text-sm font-medium text-brand-700 dark:text-brand-300"
            >
              {t("gameEdit.isNsfw")}
            </label>
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("gameEdit.isNsfwHint")}
            </p>
          </div>
          <BetterSwitch
            id="game-is-nsfw"
            checked={Boolean(game.is_nsfw)}
            onCheckedChange={checked =>
              onGameChange({
                ...game,
                is_nsfw: checked,
              } as models.Game)}
          />
        </div>

        <div className="flex justify-between pt-4">
          <div className="flex gap-4 justify-end w-full">
            {onUpdateFromRemote && (
              <BetterButton
                variant="primary"
                onClick={onUpdateFromRemote}
                disabled={Boolean(game.metadata_locked)}
                icon="i-mdi-cloud-sync"
              >
                {t("gameEdit.updateFromRemote")}
              </BetterButton>
            )}
            <BetterButton
              variant="danger"
              onClick={onDelete}
              icon="i-mdi-trash-can-outline"
            >
              {t("common.delete")}
            </BetterButton>
          </div>
        </div>
      </div>
    </div>
  );
}
