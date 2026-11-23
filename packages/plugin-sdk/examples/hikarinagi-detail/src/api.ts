import type { Context, Game } from 'lunabox';
import { pickBestSearchHit } from './format';
import type { HikarinagiGalgameDetail, HikarinagiSearchHit, PluginConfig } from './types';

const API_BASE = 'https://api.hikarinagi.org/v3';

interface ApiEnvelope<T> {
  success?: boolean;
  data?: T;
  message?: string;
  error?: { message?: string };
}

export function createHikarinagiApi(ctx: Context) {
  async function resolveToken(): Promise<string> {
    const cfg = (ctx.config.get<PluginConfig>() || {}) as PluginConfig;
    const custom = String(cfg.accessToken || '').trim();
    if (custom) return custom;
    const token = await ctx.metadata.getAuthToken('hikarinagi');
    return String(token || '').trim();
  }

  async function apiGet<T>(path: string): Promise<T> {
    const token = await resolveToken();
    if (!token) {
      throw new Error('未获取到 Hikarinagi 访问令牌，请先在设置中登录 Hikarinagi 或填写自定义 Token');
    }
    const resp = await fetch(`${API_BASE}${path}`, {
      method: 'GET',
      headers: {
        Authorization: `Bearer ${token}`,
        Accept: 'application/json',
      },
    });
    const text = await resp.text();
    let json: ApiEnvelope<T> | null = null;
    try {
      json = text ? (JSON.parse(text) as ApiEnvelope<T>) : null;
    } catch {
      json = null;
    }
    if (!resp.ok) {
      const msg = json?.message || json?.error?.message || `HTTP ${resp.status}`;
      throw new Error(`Hikarinagi API 请求失败：${msg}`);
    }
    if (!json || json.success === false) {
      const msg = json?.message || '接口返回异常';
      throw new Error(`Hikarinagi API 错误：${msg}`);
    }
    return json.data as T;
  }

  async function fetchGalgameDetail(hikarinagiId: string): Promise<HikarinagiGalgameDetail> {
    const data = await apiGet<HikarinagiGalgameDetail>(
      `/galgames/${encodeURIComponent(hikarinagiId)}`
    );
    if (!data || !data.id) {
      throw new Error(`未找到 Hikarinagi 条目 #${hikarinagiId}`);
    }
    return data;
  }

  async function searchGalgameId(
    game: Game
  ): Promise<{ hikarinagiId: string; matchedTitle: string } | null> {
    const queries: string[] = [];
    if (game?.name && String(game.name).trim()) {
      queries.push(String(game.name).trim());
    }
    if (Array.isArray(game?.aliases)) {
      for (const raw of game.aliases) {
        if (queries.length >= 3) break;
        const alias = String(raw || '').trim();
        if (alias && !queries.includes(alias)) {
          queries.push(alias);
        }
      }
    }
    for (const q of queries) {
      const data = await apiGet<{ items?: HikarinagiSearchHit[] }>(
        `/search?q=${encodeURIComponent(q)}&types=galgame&page=1&page_size=5`
      );
      const hit = pickBestSearchHit(data?.items, game);
      if (hit && Number(hit.id) > 0) {
        return {
          hikarinagiId: String(hit.id),
          matchedTitle: String(hit.title || ''),
        };
      }
    }
    return null;
  }

  return {
    fetchGalgameDetail,
    searchGalgameId,
  };
}

export type HikarinagiApi = ReturnType<typeof createHikarinagiApi>;
