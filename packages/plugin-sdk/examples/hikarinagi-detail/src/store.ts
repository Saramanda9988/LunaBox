import { useEffect, useState } from 'lunabox';
import type { Context, Game } from 'lunabox';
import type { HikarinagiApi } from './api';
import type { CacheDB } from './db';
import { normalizeHikarinagiId } from './format';
import type { DetailEntryState, PluginConfig } from './types';

const NEGATIVE_CACHE_MS = 6 * 60 * 60 * 1000;

function createEmptyEntry(): DetailEntryState {
  return {
    status: 'idle',
    hikarinagiId: '',
    manualBound: false,
    detail: null,
    fetchedAt: 0,
    fromCache: false,
    refreshing: false,
    error: '',
  };
}

export function createDetailStore(ctx: Context, db: CacheDB, api: HikarinagiApi) {
  const entries = new Map<string, DetailEntryState>();
  const inflight = new Map<string, Promise<void>>();
  const listeners = new Map<string, Set<() => void>>();
  const globalListeners = new Set<() => void>();
  let configVersion = 0;

  function getConfig(): PluginConfig {
    return (ctx.config.get<PluginConfig>() || {}) as PluginConfig;
  }

  function getCacheTtlMs(): number {
    const hours = Number(getConfig().cacheTtlHours);
    const clamped = Number.isFinite(hours) && hours >= 1 ? hours : 24;
    return clamped * 60 * 60 * 1000;
  }

  function getEntry(gameId: string): DetailEntryState {
    if (!gameId) return createEmptyEntry();
    return entries.get(gameId) || createEmptyEntry();
  }

  function setEntry(gameId: string, patch: Partial<DetailEntryState>): DetailEntryState {
    const next: DetailEntryState = { ...getEntry(gameId), ...patch };
    entries.set(gameId, next);
    const set = listeners.get(gameId);
    if (set) {
      set.forEach((fn) => {
        try {
          fn();
        } catch (e) {
          ctx.logger.error(e);
        }
      });
    }
    return next;
  }

  function subscribeGame(gameId: string, fn: () => void): () => void {
    let set = listeners.get(gameId);
    if (!set) {
      set = new Set();
      listeners.set(gameId, set);
    }
    set.add(fn);
    globalListeners.add(fn);
    return () => {
      set!.delete(fn);
      if (set!.size === 0) listeners.delete(gameId);
      globalListeners.delete(fn);
    };
  }

  ctx.config.onChange(() => {
    configVersion++;
    globalListeners.forEach((fn) => {
      try {
        fn();
      } catch (e) {
        ctx.logger.error(e);
      }
    });
  });

  async function resolveHikarinagiIdForGame(
    game: Game,
    forceSearch: boolean
  ): Promise<{ hikarinagiId: string; manualBound: boolean }> {
    const localId = String(game.id);
    const binding = await db.readBinding(localId);

    // 1. 手动绑定优先级最高
    if (binding && binding.manual && binding.hikarinagiId) {
      return { hikarinagiId: binding.hikarinagiId, manualBound: true };
    }

    // 2. 游戏自身元数据来源就是 Hikarinagi
    if (String(game.source_type || '').toLowerCase() === 'hikarinagi') {
      const directId = normalizeHikarinagiId(game.source_id);
      if (directId) {
        return { hikarinagiId: directId, manualBound: false };
      }
    }

    // 3. 已保存的自动匹配结果（含 6 小时负缓存）
    if (binding && !forceSearch) {
      if (binding.hikarinagiId) {
        return { hikarinagiId: binding.hikarinagiId, manualBound: false };
      }
      if (Date.now() - binding.updatedAt < NEGATIVE_CACHE_MS) {
        return { hikarinagiId: '', manualBound: false };
      }
    }

    // 4. 自动搜索匹配
    if (getConfig().autoSearch === false && !forceSearch) {
      return { hikarinagiId: '', manualBound: false };
    }

    const found = await api.searchGalgameId(game);
    if (found && found.hikarinagiId) {
      await db.writeBinding(localId, found.hikarinagiId, false, found.matchedTitle);
      return { hikarinagiId: found.hikarinagiId, manualBound: false };
    }

    await db.writeBinding(localId, '', false, '');
    return { hikarinagiId: '', manualBound: false };
  }

  function loadGame(
    game: Game | undefined,
    opts?: { force?: boolean; bindId?: string }
  ): Promise<void> {
    if (!game || !game.id) return Promise.resolve();
    const gameId = String(game.id);
    const force = Boolean(opts?.force);
    const explicitId = opts?.bindId ? normalizeHikarinagiId(opts.bindId) : '';

    if (!force && !explicitId && inflight.has(gameId)) {
      return inflight.get(gameId)!;
    }

    const current = getEntry(gameId);
    if (!force && !explicitId && current.status === 'ready' && current.detail) {
      const age = Date.now() - Number(current.fetchedAt || 0);
      if (age < getCacheTtlMs()) {
        return Promise.resolve();
      }
    }

    let task!: Promise<void>;
    task = (async () => {
      try {
        let hikarinagiId = explicitId;
        let manualBound = Boolean(explicitId);

        if (explicitId) {
          setEntry(gameId, {
            refreshing: true,
            error: '',
            status: current.detail ? current.status : 'loading',
          });
        } else {
          setEntry(gameId, {
            status: current.detail ? current.status : 'loading',
            refreshing: Boolean(current.detail),
            error: '',
          });
          const resolved = await resolveHikarinagiIdForGame(game, force);
          hikarinagiId = resolved.hikarinagiId;
          manualBound = resolved.manualBound;
        }

        if (!hikarinagiId) {
          setEntry(gameId, {
            status: 'unmatched',
            hikarinagiId: '',
            manualBound: false,
            detail: null,
            fetchedAt: 0,
            fromCache: false,
            refreshing: false,
            error: '',
          });
          return;
        }

        // 先读取 SQLite 缓存：有缓存先立即渲染，过期再静默回源
        const cached = await db.readCachedDetail(hikarinagiId);
        const ttlMs = getCacheTtlMs();
        const cacheFresh = Boolean(cached?.detail && Date.now() - cached.fetchedAt < ttlMs);

        if (cached?.detail) {
          setEntry(gameId, {
            status: 'ready',
            hikarinagiId,
            manualBound,
            detail: cached.detail,
            fetchedAt: cached.fetchedAt,
            fromCache: true,
            refreshing: force || !cacheFresh,
            error: '',
          });
          if (explicitId) {
            const title = cached.detail.trans_title || cached.detail.origin_title || '';
            await db.writeBinding(gameId, hikarinagiId, true, title);
          }
          if (!force && cacheFresh) {
            return;
          }
        }

        const fresh = await api.fetchGalgameDetail(hikarinagiId);
        const fetchedAt = await db.writeCachedDetail(hikarinagiId, fresh);
        if (explicitId) {
          const title = fresh.trans_title || fresh.origin_title || '';
          await db.writeBinding(gameId, hikarinagiId, true, title);
        }
        setEntry(gameId, {
          status: 'ready',
          hikarinagiId,
          manualBound,
          detail: fresh,
          fetchedAt,
          fromCache: false,
          refreshing: false,
          error: '',
        });
        if (force || explicitId) {
          ctx.ui.toast('已从 Hikarinagi 同步最新游戏详情', 'success');
        }
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        ctx.logger.warn('拉取 Hikarinagi 详情失败:', msg);
        const latest = getEntry(gameId);
        if (latest.detail) {
          setEntry(gameId, { refreshing: false, error: msg });
        } else {
          setEntry(gameId, { status: 'error', refreshing: false, error: msg });
        }
        if (force || explicitId) {
          ctx.ui.toast(msg, 'error');
        }
      } finally {
        if (inflight.get(gameId) === task) {
          inflight.delete(gameId);
        }
      }
    })();

    inflight.set(gameId, task);
    return task;
  }

  async function unbindGame(game: Game | undefined): Promise<void> {
    if (!game || !game.id) return;
    const gameId = String(game.id);
    await db.deleteBinding(gameId);
    entries.delete(gameId);
    ctx.ui.toast('已重置 Hikarinagi 关联，正在重新匹配…', 'info');
    await loadGame(game, { force: true });
  }

  function useGameDetail(game: Game | undefined): DetailEntryState {
    const gameId = game?.id ? String(game.id) : '';
    const [, setTick] = useState(0);

    useEffect(() => {
      if (!gameId) return undefined;
      const unsub = subscribeGame(gameId, () => setTick((n) => n + 1));
      loadGame(game, { force: false });
      return unsub;
    }, [gameId]);

    void configVersion;
    return getEntry(gameId);
  }

  return {
    getConfig,
    loadGame,
    unbindGame,
    useGameDetail,
  };
}

export type DetailStore = ReturnType<typeof createDetailStore>;
