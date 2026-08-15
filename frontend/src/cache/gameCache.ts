import { create } from "zustand";

import type { models } from "../../src/bindings/models";

type LibraryGameListCacheEntry = {
  gamesByIndex: Map<number, models.Game>;
  revision: number;
  total: number;
};

type CategoryGameListMetaCacheEntry = {
  revision: number;
  total: number;
};

type GameCacheState = {
  categoryRevision: number;
  libraryRevision: number;
};

const libraryGameListCache = new Map<string, LibraryGameListCacheEntry>();
const categoryGameListMetaCache = new Map<
  string,
  CategoryGameListMetaCacheEntry
>();

export const useGameCacheStore = create<GameCacheState>(() => ({
  categoryRevision: 0,
  libraryRevision: 0,
}));

export function getLibraryGameListCache(queryKey: string) {
  return libraryGameListCache.get(queryKey);
}

export function setLibraryGameListCache(
  queryKey: string,
  gamesByIndex: ReadonlyMap<number, models.Game>,
  total: number,
) {
  libraryGameListCache.set(queryKey, {
    gamesByIndex: new Map(gamesByIndex),
    revision: useGameCacheStore.getState().libraryRevision,
    total,
  });
}

export function getCategoryGameListMetaCache(queryKey: string) {
  return categoryGameListMetaCache.get(queryKey);
}

export function setCategoryGameListMetaCache(queryKey: string, total: number) {
  categoryGameListMetaCache.set(queryKey, {
    revision: useGameCacheStore.getState().categoryRevision,
    total,
  });
}

export function invalidateLibraryGameLists() {
  libraryGameListCache.clear();
  useGameCacheStore.setState(state => ({
    libraryRevision: state.libraryRevision + 1,
  }));
}

export function invalidateCategoryGameLists() {
  categoryGameListMetaCache.clear();
  useGameCacheStore.setState(state => ({
    categoryRevision: state.categoryRevision + 1,
  }));
}

export function invalidateAllGameLists() {
  libraryGameListCache.clear();
  categoryGameListMetaCache.clear();
  useGameCacheStore.setState(state => ({
    categoryRevision: state.categoryRevision + 1,
    libraryRevision: state.libraryRevision + 1,
  }));
}

const GAME_LIST_STRUCTURE_FIELDS: Array<keyof models.Game> = [
  "name",
  "company",
  "status",
  "rating",
  "release_date",
  "created_at",
  "last_played_at",
];

function gameListStructureChanged(
  previousGame: models.Game,
  updatedGame: models.Game,
) {
  return GAME_LIST_STRUCTURE_FIELDS.some(
    field =>
      String(previousGame[field] ?? "") !== String(updatedGame[field] ?? ""),
  );
}

function patchCachedLibraryGame(updatedGame: models.Game) {
  if (!updatedGame.id) {
    return;
  }

  for (const entry of libraryGameListCache.values()) {
    for (const [index, cachedGame] of entry.gamesByIndex) {
      if (cachedGame.id === updatedGame.id) {
        entry.gamesByIndex.set(index, updatedGame);
      }
    }
  }
}

export function cacheGameUpdate(
  previousGame: models.Game | null,
  updatedGame: models.Game,
  options: { forceListInvalidation?: boolean } = {},
) {
  if (
    previousGame
    && (options.forceListInvalidation
      || gameListStructureChanged(previousGame, updatedGame))
  ) {
    invalidateAllGameLists();
    return;
  }

  patchCachedLibraryGame(updatedGame);
}

export function removeGamesFromCache(gameIds: string[]) {
  if (gameIds.length === 0) {
    return;
  }
  invalidateAllGameLists();
}
