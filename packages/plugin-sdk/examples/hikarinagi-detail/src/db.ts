import type { Context } from 'lunabox';
import type { HikarinagiGalgameDetail } from './types';

export interface CachedDetailRow {
  detail: HikarinagiGalgameDetail;
  fetchedAt: number;
}

export interface BindingRow {
  hikarinagiId: string;
  manual: boolean;
  matchedTitle: string;
  updatedAt: number;
}

export function createCacheDB(ctx: Context) {
  let dbInitPromise: Promise<unknown> | null = null;

  function ensureDB(): Promise<unknown> {
    if (!dbInitPromise) {
      dbInitPromise = ctx.db
        .transaction([
          {
            sql: `CREATE TABLE IF NOT EXISTS galgame_cache (
              hikarinagi_id TEXT PRIMARY KEY,
              payload TEXT NOT NULL,
              fetched_at INTEGER NOT NULL,
              updated_at INTEGER NOT NULL
            )`,
          },
          {
            sql: `CREATE TABLE IF NOT EXISTS game_bindings (
              local_game_id TEXT PRIMARY KEY,
              hikarinagi_id TEXT NOT NULL,
              manual INTEGER NOT NULL DEFAULT 0,
              matched_title TEXT NOT NULL DEFAULT '',
              updated_at INTEGER NOT NULL
            )`,
          },
          {
            sql: `CREATE INDEX IF NOT EXISTS idx_galgame_cache_fetched ON galgame_cache(fetched_at)`,
          },
        ])
        .catch((err) => {
          dbInitPromise = null;
          ctx.logger.error('初始化 Hikarinagi 缓存数据库失败:', err);
          throw err;
        });
    }
    return dbInitPromise;
  }

  async function readCachedDetail(hikarinagiId: string): Promise<CachedDetailRow | null> {
    await ensureDB();
    const row = await ctx.db.queryOne<{
      hikarinagi_id: string;
      payload: string;
      fetched_at: number;
    }>('SELECT hikarinagi_id, payload, fetched_at FROM galgame_cache WHERE hikarinagi_id = ?', [
      hikarinagiId,
    ]);
    if (!row || !row.payload) return null;
    try {
      const detail = JSON.parse(String(row.payload)) as HikarinagiGalgameDetail;
      return {
        detail,
        fetchedAt: Number(row.fetched_at || 0),
      };
    } catch {
      return null;
    }
  }

  async function writeCachedDetail(
    hikarinagiId: string,
    detail: HikarinagiGalgameDetail
  ): Promise<number> {
    await ensureDB();
    const now = Date.now();
    await ctx.db.exec(
      `INSERT INTO galgame_cache (hikarinagi_id, payload, fetched_at, updated_at)
       VALUES (?, ?, ?, ?)
       ON CONFLICT(hikarinagi_id) DO UPDATE SET
         payload = excluded.payload,
         fetched_at = excluded.fetched_at,
         updated_at = excluded.updated_at`,
      [hikarinagiId, JSON.stringify(detail), now, now]
    );
    return now;
  }

  async function readBinding(localGameId: string): Promise<BindingRow | null> {
    await ensureDB();
    const row = await ctx.db.queryOne<{
      local_game_id: string;
      hikarinagi_id: string;
      manual: number;
      matched_title: string;
      updated_at: number;
    }>(
      'SELECT local_game_id, hikarinagi_id, manual, matched_title, updated_at FROM game_bindings WHERE local_game_id = ?',
      [localGameId]
    );
    if (!row) return null;
    return {
      hikarinagiId: String(row.hikarinagi_id || ''),
      manual: Number(row.manual || 0) === 1,
      matchedTitle: String(row.matched_title || ''),
      updatedAt: Number(row.updated_at || 0),
    };
  }

  async function writeBinding(
    localGameId: string,
    hikarinagiId: string,
    manual: boolean,
    matchedTitle: string
  ): Promise<void> {
    await ensureDB();
    const now = Date.now();
    await ctx.db.exec(
      `INSERT INTO game_bindings (local_game_id, hikarinagi_id, manual, matched_title, updated_at)
       VALUES (?, ?, ?, ?, ?)
       ON CONFLICT(local_game_id) DO UPDATE SET
         hikarinagi_id = excluded.hikarinagi_id,
         manual = excluded.manual,
         matched_title = excluded.matched_title,
         updated_at = excluded.updated_at`,
      [localGameId, hikarinagiId || '', manual ? 1 : 0, matchedTitle || '', now]
    );
  }

  async function deleteBinding(localGameId: string): Promise<void> {
    await ensureDB();
    await ctx.db.exec('DELETE FROM game_bindings WHERE local_game_id = ?', [localGameId]);
  }

  return {
    readCachedDetail,
    writeCachedDetail,
    readBinding,
    writeBinding,
    deleteBinding,
  };
}

export type CacheDB = ReturnType<typeof createCacheDB>;
